// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/luciexe/build"
	resultdbpb "go.chromium.org/luci/resultdb/proto/v1"

	"go.chromium.org/infra/experimental/golangbuild/golangbuildpb"
)

// coordRunner ensures a prebuilt Go toolchain exists (launching a build to build one if
// necessary) and then launches test builds, potentially more than one (to shard test execution).
//
// This implements "coordinator mode" for golangbuild. It's called coordinator mode because it
// coordinates build and test from afar, and provides part of the functionality that the old Go
// CI system's coordinator provided.
type coordRunner struct {
	props *golangbuildpb.CoordinatorMode
}

// newCoordRunner creates a new CoordinatorMode runner.
func newCoordRunner(props *golangbuildpb.CoordinatorMode) *coordRunner {
	return &coordRunner{props: props}
}

// Run implements the runner interface for coordRunner.
func (r *coordRunner) Run(ctx context.Context, spec *buildSpec, opts runOptions) error {
	// If we only want to fetch, there's nothing else to do. "fetchOnly" is a bit strange
	// to interpret for the coordinator builder, but a reasonable interpretation (and in
	// the spirit of the option) we definitely shouldn't trigger downstream builds.
	if opts.fetchOnly() {
		return nil
	}
	// Ensure prebuilt Go exists.
	if err := ensurePrebuiltGoExists(ctx, spec, r.props.BuildBuilder); err != nil {
		return err
	}
	if isGoProject(spec.inputs.Project) {
		// Trigger downstream builders (subrepo builders) with the commit and/or Gerrit change we got.
		if builders := r.props.GetBuildersToTriggerAfterToolchainBuild(); len(builders) > 0 {
			if err := triggerDownstreamBuilds(ctx, spec, builders...); err != nil {
				return err
			}
		}
	}
	// Launch and wait on test shards.
	return runTestShards(ctx, spec, r.props.NumTestShards, r.props.TestBuilder)
}

// triggerDownstreamBuilds triggers a single build for each of a bunch of builders,
// and does not wait on them to complete.
func triggerDownstreamBuilds(ctx context.Context, spec *buildSpec, builders ...string) (err error) {
	step, ctx := build.StartStep(ctx, "trigger downstream builds")
	defer endInfraStep(step, &err) // Any failure in this function is an infrastructure failure.

	// Scribble down the builders we're triggering.
	buildersLog := step.Log("builders")
	if _, err := io.WriteString(buildersLog, strings.Join(builders, "\n")+"\n"); err != nil {
		return err
	}

	// Figure out the arguments to bb.
	bbArgs := []string{"add"}
	if spec.invokedSrc.commit != nil {
		bbArgs = append(bbArgs, "-commit", spec.invokedSrc.asURL())
		bbArgs = append(bbArgs, "-ref", spec.invokedSrc.commit.Ref)
	}
	if spec.invokedSrc.change != nil {
		bbArgs = append(bbArgs, "-cl", spec.invokedSrc.asURL())
	}
	bbArgs = append(bbArgs, builders...)

	// Note: The hide-in-gerrit tag should never be added to these builders, since this triggers
	// top-level builds.
	return cmdStepRun(ctx, "bb add", toolCmd(ctx, "bb", bbArgs...), true)
}

// ensurePrebuiltGoExists checks if a prebuilt Go exists for the invoked source, and if
// not, spawns a new build for the provided builder to generate that prebuilt Go.
func ensurePrebuiltGoExists(ctx context.Context, spec *buildSpec, builder string) (err error) {
	step, ctx := build.StartStep(ctx, "ensure prebuilt go exists")
	defer endStep(step, &err)

	// Check to see if we might have a prebuilt Go in CAS.
	digest, err := checkForPrebuiltGo(ctx, spec.goSrc, spec.inputs)
	if err != nil {
		return err
	}
	if digest != "" {
		// Try to fetch from CAS. Note this might fail if the digest is stale enough.
		//
		// TODO(mknyszek): Rather than download the toolchain, it would be nice to check
		// this more directly.
		ok, err := fetchGoFromCAS(ctx, digest, spec.goroot)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}

	// There was no prebuilt toolchain we could grab. Launch a build.
	//
	// N.B. We can theoretically just launch a build without checking, since the build
	// will already back out if it turns out there's a prebuilt Go already hanging around.
	// But it's worthwhile to check first because we don't have to wait to acquire the
	// resources for a build.
	build, err := triggerBuild(ctx, spec, noSharding, builder)
	if err != nil {
		return err
	}

	// Include the ResultDB invocations in ours. It will contain build results as test results.
	if err := includeResultDBInvocations(ctx, spec, build.GetInfra().GetResultdb().GetInvocation()); err != nil {
		return infraWrap(err)
	}

	// Wait on build to finish.
	return waitOnBuilds(ctx, spec, "wait for make.bash", build.Id)
}

// runTestShards spawns `shards` builds from the provided `builder` and waits on them to complete.
//
// It passes the current build's source information to the child builds and includes the child builds'
// ResultDB invocations in the current invocation.
func runTestShards(ctx context.Context, spec *buildSpec, shards uint32, builder string) (err error) {
	step, ctx := build.StartStep(ctx, "run tests")
	defer endStep(step, &err)

	// Trigger test shards.
	buildIDs, err := triggerTestShards(ctx, spec, shards, builder)
	if err != nil {
		return err
	}

	// Wait on test shards to finish.
	return waitOnBuilds(ctx, spec, "wait for test shards", buildIDs...)
}

// triggerTestShards spawns `shards` builds from the provided `builder`.
//
// It passes the current build's source information to the child builds and includes the child builds'
// ResultDB invocations in the current invocation.
func triggerTestShards(ctx context.Context, spec *buildSpec, shards uint32, builder string) (ids []int64, err error) {
	step, ctx := build.StartStep(ctx, "trigger test shards")
	defer endStep(step, &err)

	// Start N shards and collect their build IDs and invocation IDs.
	buildIDs := make([]int64, 0, shards)
	invocationIDs := make([]string, 0, shards)
	for i := range uint32(shards) {
		shardBuild, err := triggerBuild(ctx, spec, testShard{shardID: i, nShards: shards}, builder)
		if err != nil {
			return nil, err
		}
		buildIDs = append(buildIDs, shardBuild.Id)
		invocationIDs = append(invocationIDs, shardBuild.GetInfra().GetResultdb().GetInvocation())
	}
	// Include the ResultDB invocations in ours.
	if err := includeResultDBInvocations(ctx, spec, invocationIDs...); err != nil {
		return nil, infraWrap(err)
	}
	return buildIDs, nil
}

// triggerBuild spawns a single build from the provided `builder`.
//
// If shard is not noSharding, then this function will pass the test shard identity
// as a set of properties to the build.
//
// This function is intended to be used for "worker" builds and adds some specific
// details to the builds with that assumption. When using this function for other
// purposes, make sure to take that into consideration.
func triggerBuild(ctx context.Context, spec *buildSpec, shard testShard, builder string) (b *bbpb.Build, err error) {
	step, ctx := build.StartStep(ctx, fmt.Sprintf("trigger %s (%d of %d)", builder, shard.shardID+1, shard.nShards))
	defer endStep(step, &err)

	props := &golangbuildpb.Inputs{
		VersionFile: spec.inputs.VersionFile,
		TestShard: &golangbuildpb.TestShard{
			ShardId:   shard.shardID,
			NumShards: shard.nShards,
		},
	}
	if !isGoProject(spec.invokedSrc.project) && spec.goSrc.commit != nil {
		props.GoCommit = spec.goSrc.commit.Id
	}

	builderParts := strings.Split(builder, "/")
	buildReq := &bbpb.ScheduleBuildRequest{
		Builder: &bbpb.BuilderID{
			Project: builderParts[0],
			Bucket:  builderParts[1],
			Builder: builderParts[2],
		},
		GitilesCommit: spec.invokedSrc.commit,
		Properties:    &structpb.Struct{},
		Experiments:   map[string]bool{},
		Priority:      spec.priority,
		Exe: &bbpb.Executable{
			CipdVersion: spec.golangbuildVersion,
		},
		Mask: &bbpb.BuildMask{
			AllFields: true, // Notably, we need the ResultDB invocation ID.
		},
		Tags: []*bbpb.StringPair{
			// Always hide "worker" builds that run tests or build Go.
			// See https://chromium.googlesource.com/infra/gerrit-plugins/buildbucket/+/refs/heads/main/README.md.
			{Key: "hide-in-gerrit", Value: "redundant"},
		},
	}
	if spec.invokedSrc.change != nil {
		buildReq.GerritChanges = []*bbpb.GerritChange{spec.invokedSrc.change}
	}
	for ex := range spec.experiments {
		switch ex {
		case "golang.force_test_outside_repository":
			buildReq.Experiments[ex] = true
		}
	}

	// This dance is apparently the canonical way to convert a Message to a Struct.
	// https://github.com/golang/protobuf/issues/1259#issuecomment-750453617
	propsBytes, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(props)
	if err != nil {
		return nil, infraWrap(err)
	}
	if err := protojson.Unmarshal(propsBytes, buildReq.Properties); err != nil {
		return nil, infraWrap(err)
	}

	reqBytes, err := protojson.Marshal(&bbpb.BatchRequest{
		Requests: []*bbpb.BatchRequest_Request{{
			Request: &bbpb.BatchRequest_Request_ScheduleBuild{ScheduleBuild: buildReq},
		}},
	})
	if err != nil {
		return nil, infraWrap(err)
	}

	// Execute `bb batch` for this shard and collect the output.
	stepName := fmt.Sprintf("bb batch (%d of %d)", shard.shardID+1, shard.nShards)
	bbBatch := toolCmd(ctx, "bb", "batch")
	bbBatch.Stdin = bytes.NewReader(reqBytes)
	out, err := cmdStepOutput(ctx, stepName, bbBatch, true)
	if err != nil {
		return nil, err
	}

	// Handle the response. bb batch always succeeds and returns a batch response.
	batchResp := &bbpb.BatchResponse{}
	if err := protojson.Unmarshal(out, batchResp); err != nil {
		return nil, infraWrap(err)
	}
	if len(batchResp.Responses) != 1 {
		return nil, infraErrorf("unexpected response count %v", len(batchResp.Responses))
	}
	switch resp := batchResp.Responses[0].Response.(type) {
	case *bbpb.BatchResponse_Response_Error:
		return nil, infraWrap(status.ErrorProto(resp.Error))
	case *bbpb.BatchResponse_Response_ScheduleBuild:
		build := resp.ScheduleBuild
		step.SetSummaryMarkdown(fmt.Sprintf(`[build link](https://ci.chromium.org/b/%d)`, build.Id))
		return build, nil
	default:
		return nil, infraErrorf("unexpected batch request result type %T", resp)
	}
}

// includeResultDBInvocations includes the provided ResultDB invocation IDs in the
// current invocation, as found in the buildSpec.
func includeResultDBInvocations(ctx context.Context, spec *buildSpec, ids ...string) (err error) {
	step, ctx := build.StartStep(ctx, "include ResultDB invocations")
	defer endStep(step, &err)

	// Set up the request and marshal it as JSON.
	updateReq := resultdbpb.UpdateIncludedInvocationsRequest{
		IncludingInvocation: spec.invocation,
		AddInvocations:      ids,
	}
	out, err := protojson.Marshal(&updateReq)
	if err != nil {
		return infraWrap(err)
	}

	// Write out the JSON as a log for debugging.
	reqLog := step.Log("request json")
	reqLog.Write(out)

	// Set up the `rdb rpc` command and pass the request through stdin.
	//
	// TODO(mknyszek): It's a bit silly to shell out for something that is
	// overtly just making an RPC call. However, there's currently no API
	// for pulling some of the ResultDB information out of LUCI_CONTEXT,
	// so we'd have to hard-code that and copy it here, or send a patch
	// to luci-go. The latter is preferable and should be considered as
	// part of a more general unit testing story for golangbuild.
	// For now, just shell out.
	cmd := toolCmd(ctx, "rdb", "rpc", "-include-update-token", "luci.resultdb.v1.Recorder", "UpdateIncludedInvocations")
	cmd.Stdin = bytes.NewReader(out)
	return cmdStepRun(ctx, "rdb rpc", cmd, true)
}

// waitOnBuilds polls until the provided builds (by int64 ID) complete and
// reports a step that represents the result of those builds. Returns an error if
// any of the builds fail or if for some reason it fails to wait on the builds. The error
// returned by this function reflects the "worst" state of all builds. More specifically,
// an infra failure takes precedence over a regular test failure among the builds.
func waitOnBuilds(ctx context.Context, spec *buildSpec, stepName string, buildIDs ...int64) (err error) {
	step, ctx := build.StartStep(ctx, stepName)
	defer endStep(step, &err)

	// Run `bb collect`.
	collectArgs := []string{
		"collect",
		"-A", // Get all Build fields.
		"-json",
		"-interval", "20s",
	}
	for _, id := range buildIDs {
		collectArgs = append(collectArgs, strconv.FormatInt(id, 10))
	}
	collectCmd := toolCmd(ctx, "bb", collectArgs...)
	out, err := cmdStepOutput(ctx, "bb collect", collectCmd, true)
	if err != nil {
		return err
	}

	// Presentation state.
	var summary strings.Builder
	writeSummaryLine := func(shardID int, buildID int64, result string) {
		summary.WriteString(fmt.Sprintf("* [shard %d %s](%s)\n", shardID, result, buildURL(buildID)))
	}

	// Parse the protojson output: one per line.
	//
	// Trim trailing newline, it'll mess with the proto parser.
	buildsBytes := bytes.Split(bytes.TrimSuffix(out, []byte{'\n'}), []byte{'\n'})
	var foundFailure, foundInfraFailure bool
	var failures []error
	for i, buildBytes := range buildsBytes {
		build := new(bbpb.Build)
		if err := protojson.Unmarshal(buildBytes, build); err != nil {
			return infraWrap(err)
		}
		failed := false
		switch build.Status {
		case bbpb.Status_SUCCESS:
			// Tests passed. Nothing to do.
		case bbpb.Status_FAILURE:
			// Something was wrong with the change being tested.
			writeSummaryLine(i+1, build.Id, "failed")
			failed = true
			foundFailure = true
		case bbpb.Status_INFRA_FAILURE:
			// Something was wrong with the infrastructure.
			writeSummaryLine(i+1, build.Id, "infra-failed")
			failed = true
			foundInfraFailure = true
		case bbpb.Status_CANCELED:
			// Build got canceled, which is very unexpected. Call it out.
			writeSummaryLine(i+1, build.Id, "canceled")
			failed = true
			foundInfraFailure = true
		default:
			return infraErrorf("unexpected build status from `bb collect` for build %d: %s", build.Id, build.Status)
		}
		if failed {
			// Get output properties and derive an error from them.
			props, err := parseOutputProperties(build)
			if err != nil {
				return infraWrap(err)
			}
			e := errorFromOutputProperties(props, fmt.Sprintf("shard %d", i+1))
			if e != nil {
				if !errorTestsFailed(e) {
					// Attach build page link for shard when it's not just a test failure. Test failures
					// should already all be aggregated in the coordinator build page.
					e = attachLinks(e, fmt.Sprintf("Shard %d encountered a non-build/non-test failure, click here for the shard's build page", i+1), buildURL(build.Id))
				}
				failures = append(failures, e)
			}
		}
	}
	step.SetSummaryMarkdown(summary.String())

	// Report an error for regular test failure or infra failure.
	if len(failures) == 0 {
		if foundInfraFailure {
			return infraErrorf("one or more test shards experienced an unknown infra failure")
		} else if foundFailure {
			return fmt.Errorf("one or more test shards experienced an unknown failure")
		}
	} else {
		err := errors.Join(failures...)
		if foundInfraFailure {
			err = infraWrap(err)
		}
		return err
	}
	return nil
}

// parseOutputProperties parses the output properties of a build into golangbuildpb.Outputs.
func parseOutputProperties(build *bbpb.Build) (*golangbuildpb.Outputs, error) {
	props := build.GetOutput().GetProperties()
	if props == nil {
		return nil, nil
	}
	json, err := protojson.Marshal(props)
	if err != nil {
		return nil, infraErrorf("failed to marshal output properties: %w", err)
	}
	dst := new(golangbuildpb.Outputs)
	return dst, protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(json, dst)
}

// errorFromOutputProperties synthesizes any failures described by the output properties
// into an error.
func errorFromOutputProperties(props *golangbuildpb.Outputs, detail string) error {
	if props == nil || props.GetFailure() == nil {
		return nil
	}
	err := fmt.Errorf("%s: %s", detail, props.GetFailure().GetDescription())
	for _, link := range props.GetFailure().GetLinks() {
		err = attachLinks(err, fmt.Sprintf("%s [%s]", link.Name, detail), link.Url)
	}
	if props.GetFailure().GetTestsFailed() {
		err = attachTestsFailed(err)
	}
	return err
}

// buildURL is a helper that produces a build page URL from a buildbucket build ID.
func buildURL(buildID int64) string {
	return fmt.Sprintf("https://ci.chromium.org/b/%d", buildID)
}
