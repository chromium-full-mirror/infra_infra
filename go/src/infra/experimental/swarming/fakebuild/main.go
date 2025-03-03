// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Binary fakebuild is a luciexe binary that pretends to do some work.
//
// To be used for Swarming and Buildbucket load testing.
package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/buildbucket"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/buildbucket/protoutil"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/data/strpair"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	"go.chromium.org/luci/lucictx"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/experimental/swarming/fakebuild/fakebuildpb"
)

var inputProps = build.RegisterInputProperty[*fakebuildpb.Inputs]("")

func main() {
	build.Main(func(ctx context.Context, args []string, st *build.State) error {
		inputs := inputProps.GetInput(ctx)

		for i := range int(inputs.Steps) {
			sleepStep(ctx, inputs, i)
		}

		httpClient, err := auth.NewAuthenticator(ctx, auth.SilentLogin, chromeinfra.DefaultAuthOptions()).Client()
		switch {
		case err == auth.ErrLoginRequired:
			return errors.New("Login required: run `bb auth-login`")
		case err != nil:
			return err
		}
		bbClient := bbpb.NewBuildsPRPCClient(&prpc.Client{
			C:       httpClient,
			Host:    "cr-buildbucket-dev.appspot.com",
			Options: prpc.DefaultOptions(),
		})

		buildIds, err := scheduleChildBuilds(ctx, bbClient, inputs)
		if err != nil {
			return err
		}
		if len(buildIds) > 0 {
			if err = waitChildBuilds(ctx, bbClient, buildIds, inputs); err != nil {
				return err
			}
		}
		return searchBuilds(ctx, bbClient, inputs)
	})
}

func randomSecs(min, max int64) int64 {
	var secs int64
	if dt := max - min; dt > 0 {
		secs = min + rand.Int63n(dt)
	} else {
		secs = min
	}
	return secs
}

func sleepStep(ctx context.Context, inputs *fakebuildpb.Inputs, idx int) {
	secs := randomSecs(inputs.SleepMinSec, inputs.SleepMaxSec)

	step, ctx := build.StartStep(ctx, fmt.Sprintf("Step %d: sleep %d", idx+1, secs))
	defer func() { step.End(nil) }()

	clock.Sleep(ctx, time.Duration(secs)*time.Second)
}

func gerritChange(change, patchset int64) *bbpb.GerritChange {
	return &bbpb.GerritChange{
		Host:     "chromium-review.googlesource.com",
		Project:  "chromium/src",
		Change:   change,
		Patchset: patchset,
	}
}

func generateGerritChangesAndTags(req *bbpb.ScheduleBuildRequest) {
	change := rand.Int63n(5000000)

	var changes []*bbpb.GerritChange
	tags := strpair.Map{}
	for i := 1; i <= 4; i++ {
		changes = append(changes, gerritChange(change, int64(i)))
		tags.Add("buildset", fmt.Sprintf("patch/gerrit/chromium-review.googlesource.com/%d/%d", change, i))
	}
	req.GerritChanges = changes
	req.Tags = protoutil.StringPairs(tags)
}

func generateScheduleRequest(builder *bbpb.BuilderID, waitForChildren bool, batchSize int) *bbpb.BatchRequest {
	req := &bbpb.BatchRequest{
		Requests: []*bbpb.BatchRequest_Request{},
	}
	for range batchSize {
		subReq := &bbpb.ScheduleBuildRequest{
			Builder: builder,
		}
		if waitForChildren {
			subReq.CanOutliveParent = bbpb.Trinary_NO
		}
		generateGerritChangesAndTags(subReq)
		req.Requests = append(req.Requests, &bbpb.BatchRequest_Request{
			Request: &bbpb.BatchRequest_Request_ScheduleBuild{
				ScheduleBuild: subReq,
			},
		})
	}
	return req
}

func scheduleOneBatch(ctx context.Context, bbClient bbpb.BuildsClient, idx, batchSize int, cbs *fakebuildpb.ChildBuilds) ([]int64, error) {
	step, ctx := build.StartStep(ctx, fmt.Sprintf("schedule children (%d)", idx))
	defer func() { step.End(nil) }()

	buildIDs := make([]int64, 0, batchSize)
	req := generateScheduleRequest(cbs.Builder, cbs.WaitForChildren, batchSize)
	res, err := bbClient.Batch(ctx, req)
	if err != nil {
		return nil, errors.Annotate(err, "batch %d", idx).Err()
	}

	summary := make([]string, 0, batchSize)
	for _, r := range res.GetResponses() {
		b := r.GetScheduleBuild()
		if b == nil {
			continue
		}
		buildIDs = append(buildIDs, b.Id)
		summary = append(summary, fmt.Sprintf("* [%d](https://luci-milo-dev.appspot.com/b/%d)", b.Id, b.Id))
	}
	step.SetSummaryMarkdown(strings.Join(summary, "\n"))

	log := step.Log("response")
	marsh := jsonpb.Marshaler{Indent: "  "}
	if err = marsh.Marshal(log, res); err != nil {
		return nil, errors.Annotate(err, "failed to marshal proto").Err()
	}

	secs := randomSecs(cbs.SleepMinSec, cbs.SleepMaxSec)
	clock.Sleep(ctx, time.Duration(secs)*time.Second)
	return buildIDs, nil
}

func scheduleChildBuilds(ctx context.Context, bbClient bbpb.BuildsClient, inputs *fakebuildpb.Inputs) ([]int64, error) {
	cbs := inputs.GetChildBuilds()
	if cbs == nil {
		return nil, nil
	}

	numBatch := 1
	if cbs.BatchSize > 0 && cbs.BatchSize < cbs.Children {
		numBatch = int(cbs.Children / cbs.BatchSize)
		if cbs.Children%cbs.BatchSize > 0 {
			numBatch += 1
		}
	}

	bbCtx := lucictx.GetBuildbucket(ctx)
	if bbCtx != nil {
		if bbCtx.GetScheduleBuildToken() != "" {
			scheduleBuildToken := bbCtx.ScheduleBuildToken
			if scheduleBuildToken != "" && scheduleBuildToken != buildbucket.DummyBuildbucketToken {
				ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(buildbucket.BuildbucketTokenHeader, scheduleBuildToken))
			}
		}
	}

	step, ctx := build.StartStep(ctx, "schedule children")
	defer func() { step.End(nil) }()

	buildIDs := make([]int64, 0, cbs.Children)
	for i := range numBatch {
		var batchSize int64
		switch {
		case cbs.BatchSize == 0:
			batchSize = cbs.Children
		case i >= 1 && i == numBatch-1:
			// last one of multiple batches.
			batchSize = cbs.Children - cbs.BatchSize*int64(i)
		default:
			batchSize = cbs.BatchSize
		}
		if batchBuildIds, err := scheduleOneBatch(ctx, bbClient, i, int(batchSize), cbs); err != nil {
			return nil, err
		} else {
			buildIDs = append(buildIDs, batchBuildIds...)
		}
	}
	return buildIDs, nil
}

// getBuild gets a build and returns if the build is ended.
func getBuild(ctx context.Context, bbClient bbpb.BuildsClient, bID int64) (bool, error) {
	req := &bbpb.GetBuildRequest{
		Id: bID,
		Mask: &bbpb.BuildMask{
			Fields: &fieldmaskpb.FieldMask{
				Paths: []string{
					"status",
				},
			},
		},
	}
	bld, err := bbClient.GetBuild(ctx, req)
	if err != nil {
		return false, errors.Annotate(err, "get build %d", bID).Err()
	}
	if bld != nil && protoutil.IsEnded(bld.Status) {
		return true, nil
	}
	return false, nil
}

func waitOnce(ctx context.Context, bbClient bbpb.BuildsClient, buildIds []int64, cbs *fakebuildpb.ChildBuilds, idx int) ([]int64, error) {
	step, ctx := build.StartStep(ctx, fmt.Sprintf("wait build (%d)", idx))
	defer func() { step.End(nil) }()
	finishedBuilds := make([]int64, 0, len(buildIds))
	for _, bID := range buildIds {
		// Send GetBuild requests instead of Batch to better compare the performance
		// with prod.
		ended, err := getBuild(ctx, bbClient, bID)
		if err != nil {
			return nil, err
		}
		if ended {
			finishedBuilds = append(finishedBuilds, bID)
		}
	}

	secs := cbs.SleepMaxSec
	if idx < 20 {
		secs = randomSecs(cbs.SleepMinSec, cbs.SleepMaxSec)
	}
	clock.Sleep(ctx, time.Duration(secs)*time.Second)
	return finishedBuilds, nil
}

func waitChildBuilds(ctx context.Context, bbClient bbpb.BuildsClient, buildIds []int64, inputs *fakebuildpb.Inputs) error {
	cbs := inputs.GetChildBuilds()
	if cbs == nil || !cbs.WaitForChildren || len(buildIds) == 0 {
		return nil
	}

	step, ctx := build.StartStep(ctx, "wait children")
	defer func() { step.End(nil) }()

	for idx := range 200 {
		endedBuilds, err := waitOnce(ctx, bbClient, buildIds, cbs, idx)
		if err != nil {
			return err
		}
		if len(endedBuilds) == len(buildIds) {
			return nil
		}
	}
	return nil
}

func searchBuildStep(ctx context.Context, stepName string, bbClient bbpb.BuildsClient, req *bbpb.SearchBuildsRequest, sbs *fakebuildpb.SearchBuilds) error {
	step, ctx := build.StartStep(ctx, stepName)
	defer func() { step.End(nil) }()

	res, err := bbClient.SearchBuilds(ctx, req)
	if err != nil {
		return errors.Annotate(err, stepName).Err()
	}
	log := step.Log("response")
	marsh := jsonpb.Marshaler{Indent: "  "}
	if err = marsh.Marshal(log, res); err != nil {
		return errors.Annotate(err, "failed to marshal proto").Err()
	}
	secs := randomSecs(sbs.SleepMinSec, sbs.SleepMaxSec)
	clock.Sleep(ctx, time.Duration(secs)*time.Second)
	return nil
}

// searchBuildsByBuildsetTag simulates milo to search related builds by buildset tag.
func searchBuildsByBuildsetTag(ctx context.Context, bbClient bbpb.BuildsClient, sbs *fakebuildpb.SearchBuilds) error {
	change := rand.Int63n(5000000)
	patchset := rand.Int63n(20)
	req := &bbpb.SearchBuildsRequest{
		Predicate: &bbpb.BuildPredicate{
			Tags:                []*bbpb.StringPair{{Key: "buildset", Value: fmt.Sprintf("patch/gerrit/chromium-review.googlesource.com/%d/%d", change, patchset)}},
			IncludeExperimental: true,
		},
	}
	return searchBuildStep(ctx, fmt.Sprintf("search related builds for CL %d/%d", change, patchset), bbClient, req, sbs)
}

// SearchBuildsByGerritChange simulates CV to search builds by gerrit change.
func SearchBuildsByGerritChange(ctx context.Context, bbClient bbpb.BuildsClient, sbs *fakebuildpb.SearchBuilds, idx int) error {
	change := rand.Int63n(5000000)
	patchset := rand.Int63n(20)

	req := &bbpb.SearchBuildsRequest{
		Predicate: &bbpb.BuildPredicate{
			GerritChanges:       []*bbpb.GerritChange{gerritChange(change, patchset)},
			IncludeExperimental: true,
		},
		Mask: &bbpb.BuildMask{
			Fields: &fieldmaskpb.FieldMask{
				Paths: []string{
					"builder",
					"create_time",
					"id",
					"output.properties",
					"status",
					"status_details",
					"summary_markdown",
					"update_time",
					"input.gerrit_changes",
					"infra.buildbucket.requested_properties",
				},
			},
		},
	}
	return searchBuildStep(ctx, fmt.Sprintf("search builds (%d)", idx), bbClient, req, sbs)
}

func searchBuildsByBuilder(ctx context.Context, bbClient bbpb.BuildsClient, sbs *fakebuildpb.SearchBuilds) error {
	req := &bbpb.SearchBuildsRequest{
		Predicate: &bbpb.BuildPredicate{
			Builder: &bbpb.BuilderID{
				Project: "infra",
				Bucket:  "loadtest",
				Builder: "fake-1m-no-bn",
			},
			Status:              bbpb.Status_ENDED_MASK,
			IncludeExperimental: true,
		},
		PageSize: 200,
	}
	return searchBuildStep(ctx, "search builds by builder", bbClient, req, sbs)
}

func getMostRecentRootBuild(ctx context.Context, bbClient bbpb.BuildsClient) (int64, error) {
	req := &bbpb.SearchBuildsRequest{
		Predicate: &bbpb.BuildPredicate{
			Builder: &bbpb.BuilderID{
				Project: "infra",
				Bucket:  "loadtest",
				Builder: "fake-tree-0-no-bn",
			},
			Status: bbpb.Status_ENDED_MASK,
		},
		PageSize: 1,
		Mask: &bbpb.BuildMask{
			Fields: &fieldmaskpb.FieldMask{
				Paths: []string{
					"id",
				},
			},
		},
	}
	res, err := bbClient.SearchBuilds(ctx, req)
	if err != nil {
		return 0, err
	}
	if len(res.GetBuilds()) == 0 {
		return 0, errors.Reason("got empty search build response").Err()
	}
	return res.Builds[0].Id, nil
}

func searchBuildsByAncestor(ctx context.Context, bbClient bbpb.BuildsClient, sbs *fakebuildpb.SearchBuilds) error {
	ancestorID, err := getMostRecentRootBuild(ctx, bbClient)
	if err != nil {
		return err
	}
	req := &bbpb.SearchBuildsRequest{
		Predicate: &bbpb.BuildPredicate{
			Builder: &bbpb.BuilderID{
				Project: "infra",
				Bucket:  "loadtest",
			},
			DescendantOf: ancestorID,
		},
	}
	return searchBuildStep(ctx, fmt.Sprintf("search builds by ancestor %d", ancestorID), bbClient, req, sbs)
}

func searchBuilds(ctx context.Context, bbClient bbpb.BuildsClient, inputs *fakebuildpb.Inputs) error {
	sbs := inputs.GetSearchBuilds()
	if sbs == nil {
		return nil
	}

	if err := searchBuildsByBuildsetTag(ctx, bbClient, sbs); err != nil {
		return err
	}
	if err := searchBuildsByBuilder(ctx, bbClient, sbs); err != nil {
		return err
	}

	if err := searchBuildsByAncestor(ctx, bbClient, sbs); err != nil {
		return err
	}

	steps := int(sbs.Steps)
	if steps > 3 {
		steps = steps - 3
	}

	for i := range steps {
		if err := SearchBuildsByGerritChange(ctx, bbClient, sbs, i); err != nil {
			return errors.Annotate(err, "search build %d", i).Err()
		}
	}
	return nil
}
