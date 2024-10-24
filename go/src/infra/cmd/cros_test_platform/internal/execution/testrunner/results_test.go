// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testrunner

import (
	"context"
	"sort"
	"testing"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/common"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	trservice "infra/cmd/cros_test_platform/internal/execution/testrunner/service"
	"infra/libs/skylab/request"
	"infra/libs/skylab/worker"
)

type fakeArgsGenerator struct {
	cannedArgs request.Args
}

func (g *fakeArgsGenerator) GenerateArgs(ctx context.Context) (request.Args, error) {
	return g.cannedArgs, nil
}

func (g *fakeArgsGenerator) CheckConsistency() error {
	return nil
}

func TestResultBeforeRefresh(t *testing.T) {
	ftt.Run("Give a single task that has not be Refresh()ed", t, func(t *ftt.Test) {
		tsk, err := NewBuild(
			context.Background(),
			trservice.StubClient{},
			&fakeArgsGenerator{
				cannedArgs: request.Args{
					Cmd: worker.Command{
						TaskName: "foo-task",
					},
				},
			},
			nil,
		)
		assert.Loosely(t, err, should.BeNil)
		t.Run("Result() returns known values and reasonable defaults", func(t *ftt.Test) {
			r := tsk.Result()
			assert.Loosely(t, r.Name, should.Equal("foo-task"))
			assert.Loosely(t, r.State, should.NotBeNil)
			assert.Loosely(t, r.State.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_PENDING))
			assert.Loosely(t, r.State.Verdict, should.Equal(test_platform.TaskState_VERDICT_UNSPECIFIED))
			assert.Loosely(t, r.LogUrl, should.BeEmpty)
			assert.Loosely(t, r.LogData, should.BeNil)
			assert.Loosely(t, r.TestCases, should.HaveLength(0))
			assert.Loosely(t, r.PrejobSteps, should.HaveLength(0))
		})
	})
}

// Test that autotest results for a single completed task map correctly.
func TestSingleAutotestTaskResults(t *testing.T) {
	ftt.Run("Given a single task's autotest results", t, func(t *ftt.Test) {
		cases := []struct {
			description   string
			result        *skylab_test_runner.Result_Autotest
			expectVerdict test_platform.TaskState_Verdict
		}{
			// 0 autotest test cases.
			{
				description:   "with no test cases",
				result:        &skylab_test_runner.Result_Autotest{},
				expectVerdict: test_platform.TaskState_VERDICT_NO_VERDICT,
			},

			// 1 autotest test case.
			{
				description: "with 1 passing test case",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_PASSED,
			},
			{
				description: "with 1 failing test case",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
			{
				description: "with 1 error test case",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_ERROR},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
			{
				description: "with 1 abort test case",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_ABORT},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
			{
				description: "with 1 undefined-verdict test case",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_UNDEFINED},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_NO_VERDICT,
			},
			{
				description: "with 1 not-available-verdict test case",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_NO_VERDICT},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_NO_VERDICT,
			},

			// multiple autotest test cases.
			{
				description: "with 2 passing test cases",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS},
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_PASSED,
			},
			{
				description: "with 1 passing and 1 not-applicable test case",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS},
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_NO_VERDICT},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_PASSED,
			},
			{
				description: "with 1 passing and 1 undefined-verdict test case",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS},
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_UNDEFINED},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_PASSED,
			},
			{
				description: "with 1 passing and 1 failing test case",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS},
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
			{
				description: "with 1 passing and 1 error test case",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS},
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_ERROR},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
			{
				description: "with 1 passing and 1 abort test case",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS},
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_ABORT},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_FAILED,
			},

			// task with incomplete test cases
			{
				description: "with 1 passing test case, but incomplete results",
				result: &skylab_test_runner.Result_Autotest{
					Incomplete: true,
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS},
					},
				},
				expectVerdict: test_platform.TaskState_VERDICT_FAILED,
			},

			// task with no results
			{
				description:   "with no autotest results",
				expectVerdict: test_platform.TaskState_VERDICT_UNSPECIFIED,
			},
		}
		for _, c := range cases {
			t.Run(c.description, func(t *ftt.Test) {
				t.Run("then task results are correctly converted to verdict.", func(t *ftt.Test) {
					result := callTaskResult(c.result, nil)
					assert.Loosely(t, result, should.NotBeNil)
					assert.Loosely(t, result.State.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_COMPLETED))
					assert.Loosely(t, result.State.Verdict, should.Equal(c.expectVerdict))
					assert.Loosely(t, result.LogData, should.NotBeNil)
					if result.LogData != nil {
						assert.Loosely(t, result.LogData.GsUrl, should.Equal("gs://some-url"))
					}
				})
			})
		}
	})
}

func TestPrejobSteps(t *testing.T) {
	ftt.Run("Given a single task's prejob steps", t, func(t *ftt.Test) {
		cases := []struct {
			description     string
			result          *skylab_test_runner.Result_Prejob
			expectTestCases []*steps.ExecuteResponse_TaskResult_TestCaseResult
		}{
			{
				description: "with no result",
			},
			{
				description: "with no prejob step",
				result:      &skylab_test_runner.Result_Prejob{},
			},
			{
				description: "with passing prejob step",
				result: &skylab_test_runner.Result_Prejob{
					Step: []*skylab_test_runner.Result_Prejob_Step{
						{
							Name:                 "foo-pass",
							Verdict:              skylab_test_runner.Result_Prejob_Step_VERDICT_PASS,
							HumanReadableSummary: "foo-pass",
						},
					},
				},
				expectTestCases: []*steps.ExecuteResponse_TaskResult_TestCaseResult{
					{
						Name:                 "foo-pass",
						Verdict:              test_platform.TaskState_VERDICT_PASSED,
						HumanReadableSummary: "foo-pass",
					},
				},
			},
			{
				description: "with failing prejob step",
				result: &skylab_test_runner.Result_Prejob{
					Step: []*skylab_test_runner.Result_Prejob_Step{
						{
							Name:                 "foo-fail",
							Verdict:              skylab_test_runner.Result_Prejob_Step_VERDICT_FAIL,
							HumanReadableSummary: "foo-fail",
						},
					},
				},
				expectTestCases: []*steps.ExecuteResponse_TaskResult_TestCaseResult{
					{
						Name:                 "foo-fail",
						Verdict:              test_platform.TaskState_VERDICT_FAILED,
						HumanReadableSummary: "foo-fail",
					},
				},
			},
			{
				description: "with undefined prejob step",
				result: &skylab_test_runner.Result_Prejob{
					Step: []*skylab_test_runner.Result_Prejob_Step{
						{
							Name:                 "foo-undefined",
							Verdict:              skylab_test_runner.Result_Prejob_Step_VERDICT_UNDEFINED,
							HumanReadableSummary: "foo-undefined",
						},
					},
				},
				expectTestCases: []*steps.ExecuteResponse_TaskResult_TestCaseResult{
					{
						Name:                 "foo-undefined",
						Verdict:              test_platform.TaskState_VERDICT_FAILED,
						HumanReadableSummary: "foo-undefined",
					},
				},
			},
		}
		for _, c := range cases {
			t.Run(c.description, func(t *ftt.Test) {
				t.Run("then prejob steps are reported correctly.", func(t *ftt.Test) {
					result := callTaskResult(nil, c.result)
					sort.SliceStable(result.PrejobSteps, func(i, j int) bool {
						return result.PrejobSteps[i].Name < result.PrejobSteps[j].Name
					})
					sort.SliceStable(c.expectTestCases, func(i, j int) bool {
						return c.expectTestCases[i].Name < c.expectTestCases[j].Name
					})
					assert.Loosely(t, result.PrejobSteps, should.Resemble(c.expectTestCases))
				})
			})
		}
	})
}

func TestAutotestTestCases(t *testing.T) {
	ftt.Run("Given a single task's autotest results", t, func(t *ftt.Test) {
		cases := []struct {
			description     string
			result          *skylab_test_runner.Result_Autotest
			expectTestCases []*steps.ExecuteResponse_TaskResult_TestCaseResult
		}{
			{
				description: "with no autotest results",
			},
			{
				description: "with no test cases",
				result:      &skylab_test_runner.Result_Autotest{},
			},
			{
				description: "with multiple test cases",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{
							Name:    "foo-pass",
							Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS,
						},
						{
							Name:    "foo-fail",
							Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL,
						},
						{
							Name:    "foo-error",
							Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_ERROR,
						},
						{
							Name:    "foo-abort",
							Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_ABORT,
						},
						{
							Name: "foo-undefined",
						},
					},
				},
				expectTestCases: []*steps.ExecuteResponse_TaskResult_TestCaseResult{
					{
						Name:    "foo-pass",
						Verdict: test_platform.TaskState_VERDICT_PASSED,
					},
					{
						Name:    "foo-fail",
						Verdict: test_platform.TaskState_VERDICT_FAILED,
					},
					{
						Name:    "foo-error",
						Verdict: test_platform.TaskState_VERDICT_FAILED,
					},
					{
						Name:    "foo-abort",
						Verdict: test_platform.TaskState_VERDICT_FAILED,
					},
					{
						Name: "foo-undefined",
					},
				},
			},
			{
				description: "with a test case that has an informational string",
				result: &skylab_test_runner.Result_Autotest{
					TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
						{
							Name:                 "foo-fail",
							Verdict:              skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL,
							HumanReadableSummary: "Something horrible happened.",
						},
					},
				},
				expectTestCases: []*steps.ExecuteResponse_TaskResult_TestCaseResult{
					{
						Name:                 "foo-fail",
						Verdict:              test_platform.TaskState_VERDICT_FAILED,
						HumanReadableSummary: "Something horrible happened.",
					},
				},
			},
		}
		for _, c := range cases {
			t.Run(c.description, func(t *ftt.Test) {
				t.Run("then test cases are reported correctly.", func(t *ftt.Test) {
					result := callTaskResult(c.result, nil)
					sort.SliceStable(result.TestCases, func(i, j int) bool {
						return result.TestCases[i].Name < result.TestCases[j].Name
					})
					sort.SliceStable(c.expectTestCases, func(i, j int) bool {
						return c.expectTestCases[i].Name < c.expectTestCases[j].Name
					})
					assert.Loosely(t, result.TestCases, should.Resemble(c.expectTestCases))
				})
			})
		}
	})
}

func callTaskResult(autotestResult *skylab_test_runner.Result_Autotest, prejob *skylab_test_runner.Result_Prejob) *steps.ExecuteResponse_TaskResult {
	t := &Build{
		result: &skylab_test_runner.Result{
			Harness: &skylab_test_runner.Result_AutotestResult{
				AutotestResult: autotestResult,
			},
			LogData: &common.TaskLogData{
				GsUrl: "gs://some-url",
			},
			Prejob: prejob,
		},
		lifeCycle:      test_platform.TaskState_LIFE_CYCLE_COMPLETED,
		swarmingTaskID: "foo-task-ID",
	}
	return t.Result()
}
