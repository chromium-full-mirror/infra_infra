// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Displays steps from the compilator to the chromium orchestrator

package main

import (
	"compress/zlib"
	"context"
	"flag"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	buildbucket_pb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/buildbucket/protoutil"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/luciexe/exe"

	"go.chromium.org/infra/chromium/compilator_watcher/internal/bb"
)

func main() {
	exe.Run(luciEXEMain, exe.WithZlibCompression(zlib.BestCompression))
}

// The exe.MainFn entry point for this luciexe binary.
func luciEXEMain(ctx context.Context, input *buildbucket_pb.Build, userArgs []string, send exe.BuildSender) (err error) {
	ctx = logging.SetLevel(ctx, logging.Info)

	defer func() {
		// processErr updates the returned err and input's SummaryMarkdown
		err = processErr(ctx, err, input, send)
		send()
	}()

	input.Status = buildbucket_pb.Status_STARTED
	input.StartTime = timestamppb.New(clock.Now(ctx))
	send()
	parsedArgs, err := parseArgs(userArgs)
	if err != nil {
		return err
	}

	compBuild, foundEndTag, err := copySteps(ctx, input, parsedArgs, send)
	if err != nil {
		return err
	}

	err = copyOutputProperties(ctx, input, compBuild, parsedArgs, send)
	if err != nil {
		return err
	}

	input.Output.GitilesCommit = compBuild.GetOutput().GetGitilesCommit()

	// If it successfully copied steps up until the end tagged step,
	// just return a SUCCESS status. When there is no endStepTag, the
	// steps are copied until the build is actually over, so then in that
	// case we'll also copy over the build status and summary.
	if parsedArgs.endStepTag != "" && foundEndTag {
		input.Status = buildbucket_pb.Status_SUCCESS
	} else {
		input.Status = compBuild.GetStatus()
		input.SummaryMarkdown = compBuild.GetSummaryMarkdown()
	}
	input.EndTime = timestamppb.New(clock.Now(ctx))
	send()
	return nil
}

type cmdArgs struct {
	compilatorID                   int64
	startStepTag                   string
	endStepTag                     string
	compPollingTimeoutSec          time.Duration
	compPollingIntervalSec         time.Duration
	maxConsecutiveGetBuildTimeouts int64
}

func parseArgs(args []string) (cmdArgs, error) {
	fs := flag.NewFlagSet("f1", flag.ContinueOnError)

	compBuildId := fs.String("compilator-id", "", "Buildbucket ID of triggered compilator")
	startStepTag := fs.String("start-step-tag", "", "Tag of the first step that should be copied over. All subsequent steps will be copied too. If empty, then it will default to the first step in the build.")
	endStepTag := fs.String("end-step-tag", "", "Tag of the last step that should be copied over. If empty, steps will be copied until the build ends.")
	compPollingTimeoutSec := fs.Int64(
		"compilator-polling-timeout-sec",
		7200,
		"Max number of seconds to poll compilator")

	compPollingIntervalSec := fs.Int64(
		"compilator-polling-interval-sec",
		10,
		"Number of seconds to wait between compilator polls")

	maxGetBuildTimeouts := fs.Int64(
		"max-consecutive-get-build-timeouts",
		3,
		"The maximum amount of consecutive timeouts allowed when running GetBuild for the compilator build")

	if err := fs.Parse(args); err != nil {
		return cmdArgs{}, err
	}

	errs := errors.NewMultiError()
	if *compBuildId == "" {
		errs = append(errs, errors.Reason("compilator-id is required").Err())
	}
	if errs.First() != nil {
		return cmdArgs{}, errs
	}

	convertedCompBuildID, err := strconv.ParseInt(*compBuildId, 10, 64)
	if err != nil {
		return cmdArgs{}, err
	}

	return cmdArgs{
		compilatorID:                   convertedCompBuildID,
		startStepTag:                   *startStepTag,
		endStepTag:                     *endStepTag,
		compPollingTimeoutSec:          time.Duration(*compPollingTimeoutSec) * time.Second,
		compPollingIntervalSec:         time.Duration(*compPollingIntervalSec) * time.Second,
		maxConsecutiveGetBuildTimeouts: *maxGetBuildTimeouts,
	}, nil
}

func containsTagKey(tagKey string, tags []*buildbucket_pb.StringPair) bool {
	for _, tagPair := range tags {
		if tagPair.GetKey() == tagKey {
			return true
		}
	}
	return false
}

// If startStepTag and endStepTag are both empty strings, then all steps
// will be copied
func copyStepsBetweenStartAndEndStepTags(
	compBuild *buildbucket_pb.Build, startStepTag string, endStepTag string) ([]*buildbucket_pb.Step, bool) {

	compBuildSteps := compBuild.GetSteps()
	start := 0
	if startStepTag != "" {
		for i, compBuildStep := range compBuildSteps {
			if containsTagKey(startStepTag, compBuildStep.GetTags()) {
				start = i
				break
			}
		}
	}

	var displayedSteps []*buildbucket_pb.Step
	for _, compBuildStep := range compBuildSteps[start:] {
		displayedSteps = append(displayedSteps, compBuildStep)
		if endStepTag != "" && containsTagKey(endStepTag, compBuildStep.GetTags()) {
			return displayedSteps, true
		}
	}

	return displayedSteps, false
}

func processErr(ctx context.Context, err error, luciBuild *buildbucket_pb.Build, send exe.BuildSender) error {
	if err == nil {
		return nil
	}
	// We want the status to show CANCELED instead of INFRA_FAILURE so
	// the orchestrator can handle a CANCELED status differently.
	if errors.Unwrap(err) == context.Canceled {
		luciBuild.SummaryMarkdown = "compilator_watcher context was canceled. Probably due to the parent orchestrator build being canceled."
		luciBuild.Status = buildbucket_pb.Status_CANCELED
		// Returning an err would automatically set the build status to FAILURE
		// See runUserCode() in https://source.chromium.org/chromium/infra/infra/+/main:go/src/go.chromium.org/luci/luciexe/exe/exe.go
		return nil
	}
	// This enforces the triggered sub_build to have an INFRA_FAILURE
	// status
	err = exe.InfraErrorTag.Apply(err)

	summaryMarkdownFmt := "Error while running compilator_watcher: %s"
	if errors.Unwrap(err) == context.DeadlineExceeded {
		luciBuild.SummaryMarkdown = fmt.Sprintf(
			summaryMarkdownFmt, "Timeout waiting for compilator build")
	} else {
		luciBuild.SummaryMarkdown = fmt.Sprintf(
			summaryMarkdownFmt, err)
	}
	return err
}

func copySteps(ctx context.Context, luciBuild *buildbucket_pb.Build, parsedArgs cmdArgs, send exe.BuildSender) (*buildbucket_pb.Build, bool, error) {
	// Poll the compilator build until it's complete or the swarming props
	// are found, while copying over filtered steps depending on the given
	// phase.
	// Return the compilator build from the most recent GetBuild call

	bClient, err := bb.NewClient(ctx)
	if err != nil {
		return nil, false, err
	}

	cctx, cancel := clock.WithTimeout(ctx, parsedArgs.compPollingTimeoutSec)
	defer cancel()

	var timeoutCounts int64 = 0
	for {
		// Check for context err like a timeout or cancelation before
		// continuing with the for loop
		if cctx.Err() != nil {
			return nil, false, cctx.Err()
		}

		compBuild, err := bClient.GetBuild(cctx, parsedArgs.compilatorID)

		// Check that the err is from the GetBuild call, not the
		// timeout set for polling
		if err != nil {
			if grpcutil.Code(err) == codes.DeadlineExceeded {
				if timeoutCounts < parsedArgs.maxConsecutiveGetBuildTimeouts {
					timeoutCounts += 1
					continue
				}
			}
			return nil, false, err
		}
		// Reset counter
		timeoutCounts = 0

		foundEndTag := false
		luciBuild.Steps, foundEndTag = copyStepsBetweenStartAndEndStepTags(
			compBuild,
			parsedArgs.startStepTag,
			parsedArgs.endStepTag,
		)
		send()

		if protoutil.IsEnded(compBuild.GetStatus()) || (parsedArgs.endStepTag != "" && foundEndTag) {
			return compBuild, foundEndTag, nil
		}

		if tr := clock.Sleep(cctx, parsedArgs.compPollingIntervalSec); tr.Err != nil {
			return compBuild, foundEndTag, tr.Err
		}
	}
}

func copyOutputProperties(ctx context.Context, luciBuild *buildbucket_pb.Build, compBuild *buildbucket_pb.Build, parsedArgs cmdArgs, send exe.BuildSender) error {
	err := exe.WriteProperties(
		luciBuild.Output.Properties,
		compBuild.GetOutput().GetProperties().AsMap())
	if err != nil {
		return err
	}

	send()
	return nil
}
