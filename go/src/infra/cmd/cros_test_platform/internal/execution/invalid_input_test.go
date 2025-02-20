// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execution_test

// This file contains tests for behaviour when the user input is invalid in
// some way.

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/protobuf/ptypes/duration"

	buildapi "go.chromium.org/chromiumos/infra/proto/go/chromite/api"
	"go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	trservice "go.chromium.org/infra/cmd/cros_test_platform/internal/execution/testrunner/service"
)

func TestLaunchForNonExistentBot(t *testing.T) {
	ftt.Run("In an execution with one invocation but not bots", t, func(t *ftt.Test) {
		trc := &trservice.CallCountingClientWrapper{
			Client: trservice.NewBotsAwareFakeClient(),
		}
		resps, err := runWithParams(
			context.Background(),
			trc,
			&test_platform.Request_Params{
				FreeformAttributes: &test_platform.Request_Params_FreeformAttributes{
					SwarmingDimensions: []string{"freeform-key:freeform-value"},
				},
				// Irrelevant required fields follow.
				Scheduling: &test_platform.Request_Params_Scheduling{
					Pool: &test_platform.Request_Params_Scheduling_ManagedPool_{
						ManagedPool: test_platform.Request_Params_Scheduling_MANAGED_POOL_CQ,
					},
					Priority: 79,
				},
				Time: &test_platform.Request_Params_Time{
					MaximumDuration: &duration.Duration{Seconds: 60},
				},
			},
			[]*steps.EnumerationResponse_AutotestInvocation{
				clientTestInvocation("", ""),
			},
			"",
		)
		assert.Loosely(t, err, should.BeNil)

		resp := extractSingleResponse(t, resps)

		t.Run("then task result is complete with unspecified verdict.", func(t *ftt.Test) {
			assert.Loosely(t, resp.TaskResults, should.HaveLength(1))
			tr := resp.TaskResults[0]
			assert.Loosely(t, tr.State.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_REJECTED))
			assert.Loosely(t, tr.State.Verdict, should.Equal(test_platform.TaskState_VERDICT_UNSPECIFIED))
			assert.Loosely(t, tr.RejectedTaskDimensions, should.ContainKey("freeform-key"))
			assert.Loosely(t, tr.RejectedDimensions[1].Key, should.Equal("freeform-key"))
		})
		t.Run("and overall result is complete with failed verdict.", func(t *ftt.Test) {
			assert.Loosely(t, resp.State.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_COMPLETED))
			assert.Loosely(t, resp.State.Verdict, should.Equal(test_platform.TaskState_VERDICT_FAILED))
		})
		t.Run("and no skylab tasks are created.", func(t *ftt.Test) {
			assert.Loosely(t, trc.CallCounts.LaunchTask, should.BeZero)
			assert.Loosely(t, trc.CallCounts.FetchResults, should.BeZero)
		})
	})
}

func TestRequestShouldNotSetBothQSAccountAndPriority(t *testing.T) {
	ftt.Run("Given a client test with both quota account and priority set", t, func(t *ftt.Test) {
		params := basicParams()
		params.Scheduling = &test_platform.Request_Params_Scheduling{
			Pool: &test_platform.Request_Params_Scheduling_UnmanagedPool{
				UnmanagedPool: "foo-pool",
			},
			QsAccount: "foo-account",
			Priority:  50,
		}
		t.Run("The test should end up with a panic.", func(t *ftt.Test) {
			assert.Loosely(t,
				func() {
					runWithParams(
						context.Background(),
						trservice.StubClient{},
						params,
						[]*steps.EnumerationResponse_AutotestInvocation{
							serverTestInvocation("name1", ""),
						},
						"",
					)
				},
				should.Panic)
		})
	})
}

func TestIncompatibleDependencies(t *testing.T) {

	ftt.Run("In testing context", t, func(t *ftt.Test) {
		cases := []struct {
			Tag    string
			Params *test_platform.Request_Params
			Invs   []*steps.EnumerationResponse_AutotestInvocation
		}{
			{
				Tag: "incompatible build target between enumeration and request",
				Params: &test_platform.Request_Params{
					SoftwareAttributes: &test_platform.Request_Params_SoftwareAttributes{
						BuildTarget: &chromiumos.BuildTarget{Name: "requested"},
					},
					Time: &test_platform.Request_Params_Time{
						MaximumDuration: &duration.Duration{Seconds: 3600},
					},
				},
				Invs: []*steps.EnumerationResponse_AutotestInvocation{
					testInvocationWithDependency("some_test", "board:enumerated"),
				},
			},
			{
				Tag: "incompatible model between enumeration and request",
				Params: &test_platform.Request_Params{
					HardwareAttributes: &test_platform.Request_Params_HardwareAttributes{
						Model: "requested",
					},
					Time: &test_platform.Request_Params_Time{
						MaximumDuration: &duration.Duration{Seconds: 3600},
					},
				},
				Invs: []*steps.EnumerationResponse_AutotestInvocation{
					testInvocationWithDependency("some_test", "model:enumerated"),
				},
			},
		}

		for _, c := range cases {
			t.Run(fmt.Sprintf("with %s", c.Tag), func(t *ftt.Test) {
				trClient := &trservice.CallCountingClientWrapper{
					Client: trservice.StubClient{},
				}
				resps, err := runWithParams(
					context.Background(),
					trClient,
					c.Params,
					c.Invs,
					"",
				)
				assert.Loosely(t, err, should.BeNil)
				resp := extractSingleResponse(t, resps)

				t.Run("then task result is rejected with unspecified verdict.", func(t *ftt.Test) {
					assert.Loosely(t, resp.TaskResults, should.HaveLength(1))
					tr := resp.TaskResults[0]
					assert.Loosely(t, tr.State.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_REJECTED))
					assert.Loosely(t, tr.State.Verdict, should.Equal(test_platform.TaskState_VERDICT_UNSPECIFIED))

				})
				t.Run("and overall result is complete with failed verdict.", func(t *ftt.Test) {
					assert.Loosely(t, resp.State.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_COMPLETED))
					assert.Loosely(t, resp.State.Verdict, should.Equal(test_platform.TaskState_VERDICT_FAILED))
				})
				t.Run("and no skylab swarming tasks are created.", func(t *ftt.Test) {
					assert.Loosely(t, trClient.CallCounts.LaunchTask, should.BeZero)
					assert.Loosely(t, trClient.CallCounts.FetchResults, should.BeZero)
				})
			})
		}
	})
}

func testInvocationWithDependency(name string, deps ...string) *steps.EnumerationResponse_AutotestInvocation {
	inv := steps.EnumerationResponse_AutotestInvocation{
		Test: &buildapi.AutotestTest{
			Name:                 name,
			ExecutionEnvironment: buildapi.AutotestTest_EXECUTION_ENVIRONMENT_SERVER,
		},
	}
	for _, d := range deps {
		inv.Test.Dependencies = append(inv.Test.Dependencies, &buildapi.AutotestTaskDependency{Label: d})
	}
	return &inv
}
