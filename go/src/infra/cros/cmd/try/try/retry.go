// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package try

import (
	"context"
	"encoding/json"
	gerr "errors"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/types/known/structpb"

	pb "go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/internal/cmd"
	bb "go.chromium.org/infra/cros/lib/buildbucket"
)

var (
	recipeSteps = map[string][]pb.RetryStep{
		"orchestrator": {
			pb.RetryStep_CREATE_BUILDSPEC,
			pb.RetryStep_RUN_CHILDREN,
			pb.RetryStep_LAUNCH_TESTS,
		},
		"build_release": {
			pb.RetryStep_STAGE_ARTIFACTS,
			pb.RetryStep_PUSH_IMAGES,
			pb.RetryStep_DEBUG_SYMBOLS,
			pb.RetryStep_COLLECT_SIGNING,
			pb.RetryStep_PAYGEN,
		},
	}
)

func GetCmdRetry() *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "retry [flags]",
		ShortDesc: "(Experimental) Rerun the specified (release) build.",
		CommandRun: func() subcommands.CommandRun {
			c := &retryRun{}
			c.tryRunBase.cmdRunner = cmd.RealCommandRunner{}
			c.addDryrunFlag()
			c.Flags.BoolVar(&c.paygenRetry, "paygen", false, "If set, only retries paygen.")
			c.Flags.StringVar(&c.originalBBID, "bbid", "", "Buildbucket ID of the builder to retry.")
			if flag.NArg() > 1 && flag.Args()[1] == "help" {
				fmt.Printf("Run `cros try help` or `cros try help ${subcomand}` for help.")
				os.Exit(0)
			}
			return c
		},
	}
}

// retryRun tracks relevant info for a given `try retry` run.
type retryRun struct {
	tryRunBase
	originalBBID string
	paygenRetry  bool
	// Used for testing purposes. If set, props will be written to this file
	// rather than a temporary one.
	propsFile *os.File
}

// RunRetryOpts contains options for the RetryClient.
type RetryRunOpts struct {
	StdoutLog *log.Logger
	StderrLog *log.Logger
	CmdRunner cmd.CommandRunner
	// Used for testing purposes. If set, props will be written to this file
	// rather than a temporary one.
	PropsFile *os.File

	BBID   string
	Dryrun bool
}

// RetryClient allows users to call `cros try retry` through code instead of the
// CLI.
type RetryClient interface {
	DoRetry(opts *RetryRunOpts) (string, error)
}

// Client is an actual implementation of RetryClient.
type Client struct{}

// DoRetry offers an entry point to `cros try retry`. Returns the BBID of the
// new build.
func (c *Client) DoRetry(opts *RetryRunOpts) (string, error) {
	r := &retryRun{
		tryRunBase: tryRunBase{
			stdoutLog: opts.StdoutLog,
			stderrLog: opts.StderrLog,
			cmdRunner: opts.CmdRunner,
			dryrun:    opts.Dryrun,
		},
		originalBBID: opts.BBID,
		propsFile:    opts.PropsFile,
	}
	bbid, ret := r.innerRun()
	if ret != 0 {
		return "", fmt.Errorf("`cros try retry` had non-zero return code (%d)", ret)
	}
	return bbid, nil
}

// validate validates retry-specific args for the command.
func (r *retryRun) validate() error {
	if r.originalBBID == "" {
		return gerr.New("--bbid is required")
	}
	return nil
}

// getRetrySummary gets the retry_summary from the specified build.
func (r *retryRun) getRetrySummary(ctx context.Context, bbid string, outputProps *structpb.Struct, allowEmpty bool) (map[pb.RetryStep]string, error) {
	v, ok := outputProps.AsMap()["retry_summary"]
	if !ok {
		if allowEmpty {
			return map[pb.RetryStep]string{}, nil
		}
		return nil, fmt.Errorf("Could not get `retry_summary` property from %s", bbid)
	}
	retrySummary := reflect.ValueOf(v)
	summary := map[pb.RetryStep]string{}
	for _, k := range retrySummary.MapKeys() {
		step := pb.RetryStep(pb.RetryStep_value[k.Interface().(string)])
		summary[step] = retrySummary.MapIndex(k).Interface().(string)
	}
	return summary, nil
}

// getSigningSummary gets the signing_summary from the specified build.
func (r *retryRun) getSigningSummary(ctx context.Context, bbid string, outputProps *structpb.Struct, allowEmpty bool) (map[string]string, error) {
	v, ok := outputProps.AsMap()["signing_summary"]
	if !ok {
		if allowEmpty {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("Could not get `signing_summary` property from %s", bbid)
	}
	signingSummary := reflect.ValueOf(v)
	summary := map[string]string{}
	for _, k := range signingSummary.MapKeys() {
		summary[k.Interface().(string)] = signingSummary.MapIndex(k).Interface().(string)
	}
	return summary, nil
}

type buildInfo struct {
	bbid           string
	status         bbpb.Status
	retrySummary   map[pb.RetryStep]string
	signingSummary map[string]string
}

// getRetryBBID will return the BBID of the retry build for the given BBID or
// just the BBID if no retry exists.
func (r *retryRun) getRetryBBID(ctx context.Context, bbid string) (string, error) {
	buildData, err := r.bbClient.GetBuild(ctx, bbid)
	if err != nil {
		return "", err
	}

	builder := buildData.GetBuilder()
	builderName := fmt.Sprintf("%s/%s/%s", builder.GetProject(), builder.GetBucket(), builder.GetBuilder())

	// Limit search space to builds that were created after the original build.
	createTime := time.Unix(buildData.GetCreateTime().Seconds, int64(buildData.GetCreateTime().Nanos)).UTC()
	createTimestamp := createTime.Format(time.RFC3339)
	builderPredicateJSON, err := json.Marshal(builder)
	if err != nil {
		return "", err
	}
	// Can't use BuildPredicate proto because it uses the wrong timestamp type.
	predicate := fmt.Sprintf(`{"builder": %s, "create_time": {"start_time": "%s"}}`,
		builderPredicateJSON, createTimestamp)

	r.LogOut("Checking for previous retries...\n")
	builds, err := r.bbClient.ListBuildsWithPredicate(ctx, predicate)
	if err != nil {
		return "", err
	}
	for {
		var retryBuild string

		r.LogOut("Checking for previous retries for %s (%s)...\n", bbid, builderName)
		// Scan through all builds with the given name (there's no way to query by
		// input property). Builds are returned ordered by time so we should be good.
		for _, build := range builds {
			inputProps := build.GetInput().GetProperties().AsMap()
			if retryBBID, ok := bb.GetProp(inputProps, "$chromeos/checkpoint.original_build_bbid"); ok {
				buildBBID := fmt.Sprintf("%v", build.GetId())
				if retryBBID.(string) == bbid {
					if retryBuild != "" {
						return "", fmt.Errorf("Found multiple retries (%s, %s) for build %s. "+
							"This should never happen, the build is likely corrupted. Please file a go/cros-rbs-bug.",
							retryBuild, buildBBID, bbid)
					}
					retryBuild = buildBBID
				}
			}
		}

		if retryBuild == "" {
			return bbid, nil
		}
		r.LogOut("Found retry %s.\n", retryBuild)
		bbid = retryBuild
	}
}

// getChildBuildInfo gets information about child builders. The map keys are the
// builder name.
func (r *retryRun) getChildBuildInfo(ctx context.Context, parentBuildOutputProps *structpb.Struct) (map[string]buildInfo, error) {
	childBuildBBIDs, ok := parentBuildOutputProps.AsMap()["child_builds"]
	if !ok {
		return nil, fmt.Errorf("Could not get `child_builds` property from %s", r.originalBBID)
	}

	childBuildInfo := map[string]buildInfo{}
	for _, v := range childBuildBBIDs.([]any) {
		bbid := v.(string)

		buildData, err := r.bbClient.GetBuild(ctx, bbid)
		if err != nil {
			return nil, errors.Annotate(err, "Could not get output props for %s", bbid).Err()
		}
		originalBuildInputProps := buildData.GetInput().GetProperties()
		if originalBuildInputProps.AsMap()["recipe"] != "build_release" {
			continue
		}

		// If the build wasn't successful, get the last build in retry chain.
		if buildData.GetStatus() != bbpb.Status_SUCCESS {
			bbid, err = r.getRetryBBID(ctx, v.(string))
			if err != nil {
				return nil, err
			}
			buildData, err = r.bbClient.GetBuild(ctx, bbid)
			if err != nil {
				return nil, errors.Annotate(err, "Could not get output props for %s", bbid).Err()
			}
		}
		originalBuildOutputProps := buildData.GetOutput().GetProperties()

		retrySummary, err := r.getRetrySummary(ctx, bbid, originalBuildOutputProps, true)
		if err != nil {
			return nil, err
		}
		signingSummary, err := r.getSigningSummary(ctx, bbid, originalBuildOutputProps, true)
		if err != nil {
			return nil, err
		}
		childBuildInfo[buildData.GetBuilder().GetBuilder()] = buildInfo{
			bbid:           bbid,
			status:         buildData.GetStatus(),
			retrySummary:   retrySummary,
			signingSummary: signingSummary,
		}

	}
	return childBuildInfo, nil
}

func hasFailedChild(childData map[string]buildInfo) bool {
	for _, data := range childData {
		if data.status != bbpb.Status_SUCCESS {
			return true
		}
	}
	return false
}

// getExecStep looks at the retry summary and decides what step we need to pick
// up at during the retry run.
func getExecStep(recipe string, buildData buildInfo) (pb.RetryStep, error) {
	steps, ok := recipeSteps[recipe]
	if !ok {
		return pb.RetryStep_UNDEFINED, fmt.Errorf("unsupported recipe \"%s\"", recipe)
	}

	// Make sure that the build we're trying to retry doesn't violate the suffix
	// constraint.
	missingStep := false
	foundFailedStep := pb.RetryStep_UNDEFINED
	for _, step := range steps {
		status, stepRan := buildData.retrySummary[step]
		if !stepRan {
		} else if missingStep {
			return pb.RetryStep_UNDEFINED, fmt.Errorf("retry summary is missing step %v but has later ones. Can't retry.", step)
		}
		if status != "SUCCESS" && status != "SKIPPED" {
			foundFailedStep = step
		} else if foundFailedStep != pb.RetryStep_UNDEFINED {
			return pb.RetryStep_UNDEFINED, fmt.Errorf("step %v failed but a subsequent step (%v) succeeded. Can't retry.", foundFailedStep, step)
		}
	}

	// Refuse to retry builds with failed EBUILD_TESTS.
	if ebuildTestRetrySummary, ok := buildData.retrySummary[pb.RetryStep_EBUILD_TESTS]; ok {
		if recipe == "build_release" && ebuildTestRetrySummary == "FAILED" {
			return pb.RetryStep_UNDEFINED, fmt.Errorf("ebuild tests failed. Can't retry.")
		}
	}

	// If there are signing failures, start at PUSH_IMAGES to rekick signing.
	// The suffix constraint steps above will prevent us from skipping earlier
	// steps that didn't pass in the previous build, and we know that if
	// signing_summary is set then we at least got to COLLECT_SIGNING in the
	// previous build. Everything between PUSH_IMAGES and COLLECT_SIGNING
	// (currently just DEBUG_SYMBOLS) can be rerun without consequence /
	// clobbering (if this changes we'll need to tweak this approach).
	if recipe == "build_release" && len(buildData.signingSummary) > 0 {
		for _, status := range buildData.signingSummary {
			// TODO(b/262388770): Do we want to retry failures, or just timeouts?
			if status == "FAILED" || status == "TIMED_OUT" {
				return pb.RetryStep_PUSH_IMAGES, nil
			}
		}
	}

	// Return the earliest failed step, or the first one that didn't run.
	for _, step := range steps {
		if status, stepRan := buildData.retrySummary[step]; !stepRan || (status != "SUCCESS" && status != "SKIPPED") {
			return step, nil
		}
	}
	// If all the steps succeeded, there's nothing to retry.
	// If the build is successful, this isn't a problem (this function shouldn't
	// really be getting called anyways). If the build failed, we should fail
	// too.
	if buildData.status != bbpb.Status_SUCCESS {
		return pb.RetryStep_UNDEFINED, fmt.Errorf("build %v was not successful but all retry steps passed, not sure what to retry", buildData.bbid)
	}
	return pb.RetryStep_UNDEFINED, nil
}

// Process a standard retry.
func (r *retryRun) processRetry(ctx context.Context, buildData *bbpb.Build, propsStruct *structpb.Struct) int {
	originalBuildProps := buildData.GetOutput().GetProperties()

	// Empty retry summaries are OK, they indicate that the build failed before
	// it could begin the first step (and thus are almost certainly OK to
	// retry).
	retrySummary, err := r.getRetrySummary(ctx, r.originalBBID, originalBuildProps, true)
	if err != nil {
		r.LogErr(err.Error())
		return CmdError
	}
	signingSummary, err := r.getSigningSummary(ctx, r.originalBBID, originalBuildProps, true)
	if err != nil {
		r.LogErr(err.Error())
		return CmdError
	}

	recipe := propsStruct.AsMap()["recipe"].(string)
	if recipe != "orchestrator" && recipe != "build_release" {
		r.LogErr(fmt.Errorf("unsupported recipe `%s`", recipe).Error())
		return CmdError
	}

	checkpointProps := map[string]any{
		"retry":               true,
		"original_build_bbid": r.originalBBID,
	}

	// Set exec_steps.
	execStep, err := getExecStep(recipe, buildInfo{
		bbid:           r.originalBBID,
		status:         buildData.GetStatus(),
		retrySummary:   retrySummary,
		signingSummary: signingSummary,
	})
	if err != nil {
		r.LogErr(err.Error())
		return CmdError
	}

	var childInfo map[string]buildInfo
	if recipe == "orchestrator" {
		childInfo, err = r.getChildBuildInfo(ctx, originalBuildProps)
		if err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
	}

	if recipe == "orchestrator" && hasFailedChild(childInfo) {
		execStep = pb.RetryStep_RUN_FAILED_CHILDREN
	}
	checkpointProps["exec_steps"] = map[string]any{
		"steps": []any{int32(execStep.Number())},
	}
	if err := bb.SetProperty(propsStruct, "$chromeos/checkpoint", checkpointProps); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}
	if err := bb.SetProperty(propsStruct, "$chromeos/signing.ignore_already_exists_errors", true); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}
	// If we're retrying an orchestrator, try and set builder_exec_steps.
	if recipe == "orchestrator" && execStep == pb.RetryStep_RUN_FAILED_CHILDREN {
		for builder, info := range childInfo {
			if info.status == bbpb.Status_SUCCESS {
				continue
			}
			execStep, err := getExecStep("build_release", info)
			if err != nil {
				r.LogErr(err.Error())
				return CmdError
			}
			steps := map[string]any{
				"steps": []any{int32(execStep.Number())},
			}
			subproperty := fmt.Sprintf("$chromeos/checkpoint.builder_exec_steps.%s", builder)
			if err := bb.SetProperty(propsStruct, subproperty, steps); err != nil {
				r.LogErr(err.Error())
				return CmdError
			}
		}
	}
	return Success
}

// Process a paygen retry.
func (r *retryRun) processPaygenRetry(ctx context.Context, buildData *bbpb.Build, propsStruct *structpb.Struct) int {
	recipe := propsStruct.AsMap()["recipe"].(string)
	if recipe != "build_release" {
		r.LogErr("A --paygen retry can only be launched from a child builder " +
			"(e.g. eve-release-main), please use the BBID for that builder.")
		return CmdError
	}

	retrySummary, err := r.getRetrySummary(ctx, r.originalBBID, buildData.GetOutput().GetProperties(), true)
	if err != nil {
		r.LogErr(err.Error())
		return CmdError
	}

	buildStatus := buildData.GetStatus()
	if len(retrySummary) == 0 {
		// If we don't have a retry summary, we can only retry a successful build.
		if buildStatus != bbpb.Status_SUCCESS {
			r.LogErr("no `retry_summary` and build was unsuccessful, can't retry paygen")
			return CmdError
		}
	} else {
		// If we do have a retry summary, everything before PAYGEN must be successful.
		for _, step := range recipeSteps["build_release"] {
			if step == pb.RetryStep_PAYGEN {
				break
			}
			status, stepRan := retrySummary[step]
			if !stepRan {
				r.LogErr("build did not run step %v, can't retry paygen", step)
				return CmdError
			} else if status != "SUCCESS" {
				r.LogErr("step %v failed, can't retry paygen", step)
				return CmdError
			}
		}
	}

	checkpointProps := map[string]any{
		"retry":               true,
		"original_build_bbid": r.originalBBID,
		"exec_steps": map[string]any{
			"steps": []any{int32(pb.RetryStep_PAYGEN.Number())},
		},
	}
	if err := bb.SetProperty(propsStruct, "$chromeos/checkpoint", checkpointProps); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}
	return Success
}

// Run provides the logic for a `try retry` command run.
func (r *retryRun) Run(_ subcommands.Application, _ []string, _ subcommands.Env) int {
	_, ret := r.innerRun()
	return ret
}

func (r *retryRun) innerRun() (string, int) {
	if r.stdoutLog == nil {
		r.stdoutLog = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	}
	if r.stderrLog == nil {
		r.stderrLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
	}

	if err := r.validate(); err != nil {
		r.LogErr(err.Error())
		return "", CmdError
	}
	// Allow the "b" suffix on bbids.
	r.originalBBID = strings.TrimPrefix(r.originalBBID, "b")

	ctx := context.Background()
	if ret, err := r.run(ctx); err != nil {
		r.LogErr(err.Error())
		return "", ret
	}

	buildData, err := r.bbClient.GetBuild(ctx, r.originalBBID)
	if err != nil {
		r.LogErr(err.Error())
		return "", CmdError
	}

	recipe := buildData.GetInput().GetProperties().AsMap()["recipe"].(string)

	// Find the end of the retry chain.
	// Doesn't apply to paygen / paygen_orchestrator as those recipes don't
	// support partial retries.
	if recipe != "paygen" && recipe != "paygen_orchestrator" {
		retryBBID, err := r.getRetryBBID(ctx, r.originalBBID)
		if err != nil {
			r.LogErr(err.Error())
			return "", CmdError
		}
		if retryBBID != r.originalBBID {
			r.LogOut("Found retry build %s for build %s, retrying that instead.", retryBBID, r.originalBBID)
		}
		r.originalBBID = retryBBID
	}

	// BBID may have changed, get new build data.
	buildData, err = r.bbClient.GetBuild(ctx, r.originalBBID)
	if err != nil {
		r.LogErr(err.Error())
		return "", CmdError
	}
	propsStruct := buildData.GetInput().GetProperties()

	// TODO(b/266850767): Remove in 2024.
	// crrev.com/c/4205799 updated `cros try` to track a CIPD ref instead of a
	// speific CIPD version, allowing us to push updates to users. We want to
	// invalidate try builds that (roughly) predated this change.
	// This can be removed after it has baked for a sufficiently long period of
	// time (several quarters).
	if err := bb.SetProperty(propsStruct, "$chromeos/cros_try.supported_build", true); err != nil {
		r.LogErr(err.Error())
		return "", CmdError
	}

	if recipe == "paygen_orchestrator" || recipe == "paygen" {
		r.LogOut("Warning: paygen-orchestrator/paygen builds do not communicate directly with GoldenEye. " +
			"This build will not be ingested unless a child builder reports its results -- this " +
			"will happen if this build was relaunched with the conductor, but not if it was " +
			"retried manually.")
		// No partial retries for paygen-orchestrator but clobbering is allowed
		// so just fire off a new one.
	} else if r.paygenRetry {
		ret := r.processPaygenRetry(ctx, buildData, propsStruct)
		if ret != Success {
			return "", ret
		}
	} else {
		if recipe == "build_release" && buildData.GetStatus() == bbpb.Status_SUCCESS {
			r.LogOut("Build was succesful, nothing to retry.")
			return "", Success
		}

		ret := r.processRetry(ctx, buildData, propsStruct)
		if ret != Success {
			return "", ret
		}
	}

	// Write props to file and launch builder.
	var propsFile *os.File
	if r.propsFile != nil {
		propsFile = r.propsFile
	} else {
		propsFile, err = os.CreateTemp("", "input_props")
		if err != nil {
			r.LogErr(err.Error())
			return "", CmdError
		}
	}
	if err := bb.WriteStructToFile(propsStruct, propsFile); err != nil {
		r.LogErr(errors.Annotate(err, "writing input properties to tempfile").Err().Error())
		return "", UnspecifiedError
	}
	if commit := buildData.GetInput().GetGitilesCommit(); commit != nil {
		r.bbAddArgs = append(r.bbAddArgs,
			"-commit", fmt.Sprintf("https://%s/%s/+/%s", commit.GetHost(), commit.GetProject(), commit.GetId()),
			"-ref", commit.GetRef())
	}
	if r.propsFile == nil {
		defer os.Remove(propsFile.Name())
	}
	r.bbAddArgs = append(r.bbAddArgs, "-p", fmt.Sprintf("@%s", propsFile.Name()))

	builder := buildData.GetBuilder()
	builderName := fmt.Sprintf("%s/%s/%s", builder.GetProject(), builder.GetBucket(), builder.GetBuilder())
	if bbid, err := r.bbClient.BBAdd(ctx, r.dryrun, append([]string{builderName}, r.bbAddArgs...)...); err != nil {
		r.LogErr(err.Error())
		return "", CmdError
	} else {
		return bbid, Success
	}
}
