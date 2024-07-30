package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/time/rate"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/timestamppb"

	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/logdog/client/butlerlib/streamclient"
	"go.chromium.org/luci/logdog/common/types"
	"go.chromium.org/luci/luciexe/build"
)

// Prepare OptSend func to append each new build proto update from LuciExe to a provided array.
func prepOptsForLuciexeEnv(ctx context.Context, timeline *[]*bbpb.Build) (initial *bbpb.Build, opts []build.StartOption) {
	// ensure that send NEVER blocks while testing
	mainSendRate := rate.Inf

	opts = append(opts,
		build.OptSend(mainSendRate, func(vers int64, build *bbpb.Build) {
			*timeline = append(*timeline, build)
		}),
	)
	return
}

func expectedFinalBuildPb(status bbpb.Status, fakeTime *timestamppb.Timestamp) (expected *bbpb.Build) {
	return &bbpb.Build{
		StartTime: fakeTime,
		EndTime:   fakeTime,
		Status:    status,
		Input:     &bbpb.Build_Input{},
		Output: &bbpb.Build_Output{
			Status: status,
		},
		Steps: []*bbpb.Step{
			{
				Name:      "dockerbuild",
				StartTime: fakeTime,
				EndTime:   fakeTime,
				Status:    status,
				Logs: []*bbpb.Log{
					{
						Name: "stdout",
						Url:  "step/0/log/1",
					},
					{
						Name: "stderr",
						Url:  "step/0/log/2",
					},
				},
			},
		},
	}
}

// Constructs a string with each build, step, and log in the timeline.
func printBuildTimeline(buildTimeline []*bbpb.Build, scFake streamclient.Fake) string {
	var sb strings.Builder

	for b, build := range buildTimeline {
		fmt.Fprintf(&sb, "%v. Build with proto \n %v\n", b, prototext.Format(build))

		for s, step := range build.Steps {
			fmt.Fprintf(
				&sb,
				"%v.%v Step \"%v\" with status \"%v\"\n",
				b, s, step.Name, step.Status,
			)

			for l, log := range step.Logs {
				viewUrl := types.StreamName(fmt.Sprintf("%v/%v", "fakeNS", log.Url))
				logOutput := scFake.Data()[viewUrl].GetStreamData()
				fmt.Fprintf(
					&sb,
					"%v.%v.%v Log \"%v\" at URL \"%v\": %v\n",
					b, s, l, log.Name, log.Url, logOutput,
				)
			}
		}
	}

	return sb.String()
}

type testParams struct {
	desc       string
	args       []string
	expectedPb *bbpb.Build
	executor   func(*exec.Cmd) error
}

func TestBuildWheelsLuciExe(t *testing.T) {
	t.Parallel()

	ftt.Run(`Build wheels`, t, func(t *ftt.Test) {
		scFake, lc := streamclient.NewUnregisteredFake("fakeNS")
		ctx, _ := testclock.UseTime(context.Background(), testclock.TestRecentTimeUTC)
		nowpb := timestamppb.New(testclock.TestRecentTimeUTC)

		var bbpbUpdates []*bbpb.Build
		initial, opts := prepOptsForLuciexeEnv(ctx, &bbpbUpdates)
		opts = append(opts, build.OptLogsink(lc))

		state, ictx, err := build.Start(ctx, initial, opts...)

		testSets := []*testParams{
			{
				"simple success state",
				[]string{"--help"},
				expectedFinalBuildPb(bbpb.Status_SUCCESS, nowpb),
				CreateDryRunExecutor(true),
			},
			{
				"simple failure state",
				[]string{"--non-existent-flag"},
				expectedFinalBuildPb(bbpb.Status_FAILURE, nowpb),
				CreateDryRunExecutor(false),
			},
		}

		for _, param := range testSets {
			t.Run(param.desc, func(t *ftt.Test) {
				err = RunDockerBuild(ictx, param.args, state, param.executor)
				state.End(err)

				assert.That(t, bbpbUpdates[len(bbpbUpdates)-1], should.Resemble(param.expectedPb))

				// Workaround to print error on test failure.
				// Convey.SoMsg() seemed to only print the assertion diff.
				if t.Failed() {
					fmt.Printf(
						"Test: %v\nCommand: %v\n Output:\n%v\n",
						param.desc, param.args, printBuildTimeline(bbpbUpdates, scFake),
					)
				}

			})
		}
	})
}
