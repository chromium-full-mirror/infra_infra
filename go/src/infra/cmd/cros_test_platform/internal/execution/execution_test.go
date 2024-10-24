// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package execution_test contains blackbox tests for the execution package.
//
// Tests are split across multiple files, grouping together logically related
// tests. `execution_test.go` contains unclassified tests and common helpers.
package execution_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/duration"
	"google.golang.org/grpc"

	buildapi "go.chromium.org/chromiumos/infra/proto/go/chromite/api"
	"go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps/execute"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/memlogger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/luciexe/exe"

	"infra/cmd/cros_test_platform/internal/execution"
	trservice "infra/cmd/cros_test_platform/internal/execution/testrunner/service"
	"infra/libs/skylab/request"
	ufsapi "infra/unifiedfleet/api/v1/rpc"
)

func TestLaunchAndWaitTest(t *testing.T) {
	ftt.Run("Given two enumerated test", t, func(t *ftt.Test) {
		t.Run("when running a skylab execution", func(t *ftt.Test) {
			trClient := &trservice.CallCountingClientWrapper{
				Client: trservice.StubClient{},
			}
			resps, err := runWithDefaults(
				context.Background(),
				trClient,
				[]*steps.EnumerationResponse_AutotestInvocation{
					clientTestInvocation("", ""),
					clientTestInvocation("", ""),
				},
			)
			assert.Loosely(t, err, should.BeNil)
			resp := extractSingleResponse(t, resps)

			t.Run("then results for all tests are reflected.", func(t *ftt.Test) {
				assert.Loosely(t, resp.TaskResults, should.HaveLength(2))
				for _, tr := range resp.TaskResults {
					assert.Loosely(t, tr.State.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_COMPLETED))
				}
			})
			t.Run("then the expected number of external test_runner calls are made.", func(t *ftt.Test) {
				assert.Loosely(t, trClient.CallCounts.LaunchTask, should.Equal(2))
				assert.Loosely(t, trClient.CallCounts.FetchResults, should.Equal(2))
			})
		})
	})
}

// Note: the purpose of this test is the test the behavior when a parsed
// autotest result is not available from a task, because the task didn't run
// far enough to output one.
//
// For detailed tests on the handling of autotest test results, see results_test.go.
func TestTaskStates(t *testing.T) {
	ftt.Run("Given a single test", t, func(t *ftt.Test) {
		cases := []struct {
			description   string
			lifeCycle     test_platform.TaskState_LifeCycle
			expectVerdict test_platform.TaskState_Verdict
		}{
			{
				description:   "that was never scheduled",
				lifeCycle:     test_platform.TaskState_LIFE_CYCLE_CANCELLED,
				expectVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
			{
				description:   "that was killed",
				lifeCycle:     test_platform.TaskState_LIFE_CYCLE_ABORTED,
				expectVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
			{
				description:   "that completed",
				lifeCycle:     test_platform.TaskState_LIFE_CYCLE_COMPLETED,
				expectVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
		}
		for _, c := range cases {
			t.Run(c.description, func(t *ftt.Test) {
				resps, err := runWithDefaults(
					context.Background(),
					trservice.NewStubClientWithCannedIncompleteTasks(c.lifeCycle),
					[]*steps.EnumerationResponse_AutotestInvocation{
						clientTestInvocation("", ""),
					},
				)
				assert.Loosely(t, err, should.BeNil)
				resp := extractSingleResponse(t, resps)

				t.Run("then the task state is correct.", func(t *ftt.Test) {
					assert.Loosely(t, resp.TaskResults, should.HaveLength(1))
					assert.Loosely(t, resp.TaskResults[0].State.LifeCycle, should.Equal(c.lifeCycle))
					assert.Loosely(t, resp.TaskResults[0].State.Verdict, should.Resemble(c.expectVerdict))
				})
			})
		}
	})
}

func TestLaunchTaskError(t *testing.T) {
	ftt.Run("Error in creating test_runner builds is surfaced correctly", t, func(t *ftt.Test) {
		_, err := runWithDefaults(
			context.Background(),
			errorProneLaunchTaskClient{},
			[]*steps.EnumerationResponse_AutotestInvocation{clientTestInvocation("", "")},
		)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, err.Error(), should.ContainSubstring("new task"))
		assert.Loosely(t, err.Error(), should.ContainSubstring("simulated error from fake client"))
	})
}

type errorProneLaunchTaskClient struct {
	trservice.StubClient
}

// LaunchTask implements Client interface.
func (c errorProneLaunchTaskClient) LaunchTask(ctx context.Context, args *request.Args) (trservice.TaskReference, error) {
	return "", errors.Reason("simulated error from fake client").Err()
}

func TestFetchResultsError(t *testing.T) {
	ftt.Run("Error in fetching test_runner results is surfaced correctly", t, func(t *ftt.Test) {
		_, err := runWithDefaults(
			context.Background(),
			errorProneFetchResultsClient{},
			[]*steps.EnumerationResponse_AutotestInvocation{clientTestInvocation("", "")},
		)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, err.Error(), should.ContainSubstring("tick for task"))
		assert.Loosely(t, err.Error(), should.ContainSubstring("simulated error from fake client"))
	})
}

type errorProneFetchResultsClient struct {
	trservice.StubClient
}

// FetchResults implements Client interface.
func (c errorProneFetchResultsClient) FetchResults(context.Context, trservice.TaskReference) (*trservice.FetchResultsResponse, error) {
	return nil, errors.Reason("simulated error from fake client").Err()
}

func TestTaskURL(t *testing.T) {
	ftt.Run("Given a single enumerated test running to completion, its task URL is propagated correctly.", t, func(t *ftt.Test) {
		resps, err := runWithDefaults(
			context.Background(),
			stubTestRunnerClientWithCannedURL{
				Client:    trservice.NewStubClientWithSuccessfulTasks(),
				CannedURL: "foo-url",
			},
			[]*steps.EnumerationResponse_AutotestInvocation{
				clientTestInvocation("", ""),
			},
		)
		assert.Loosely(t, err, should.BeNil)
		resp := extractSingleResponse(t, resps)
		assert.Loosely(t, resp.TaskResults, should.HaveLength(1))
		assert.Loosely(t, resp.TaskResults[0].TaskUrl, should.Equal("foo-url"))
	})
}

type fleetPolicyFailureClient struct {
	trservice.StubClient
}

// CheckFleetTestsPolicy implements Client interface.
func (c fleetPolicyFailureClient) CheckFleetTestsPolicy(ctx context.Context, req *ufsapi.CheckFleetTestsPolicyRequest, opt ...grpc.CallOption) (*ufsapi.CheckFleetTestsPolicyResponse, error) {
	return &ufsapi.CheckFleetTestsPolicyResponse{
		IsTestValid: false,
		TestStatus: &ufsapi.TestStatus{
			Code:    ufsapi.TestStatus_NOT_A_PUBLIC_BOARD,
			Message: "Not a Public Board",
		},
	}, nil
}

func TestCheckFleetPolicyError(t *testing.T) {
	ftt.Run("Error in Fleet Policy fails the run", t, func(t *ftt.Test) {
		responses, err := runWithDefaults(
			context.Background(),
			fleetPolicyFailureClient{},
			[]*steps.EnumerationResponse_AutotestInvocation{clientTestInvocation("", "")},
		)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, err.Error(), should.ContainSubstring(ufsapi.TestStatus_NOT_A_PUBLIC_BOARD.String()))
		assert.Loosely(t, responses, should.BeNil)
	})
}

type fleetPolicyValidClient struct {
	trservice.StubClient
}

// CheckFleetTestsPolicy implements Client interface.
func (c fleetPolicyValidClient) CheckFleetTestsPolicy(ctx context.Context, req *ufsapi.CheckFleetTestsPolicyRequest, opt ...grpc.CallOption) (*ufsapi.CheckFleetTestsPolicyResponse, error) {
	return &ufsapi.CheckFleetTestsPolicyResponse{
		IsTestValid: true,
		TestStatus: &ufsapi.TestStatus{
			Code: ufsapi.TestStatus_OK,
		},
	}, nil
}

func TestCheckFleetPolicyValid(t *testing.T) {
	ftt.Run("Valid Fleet Policy continues normal test run flow", t, func(t *ftt.Test) {
		responses, err := runWithDefaults(
			context.Background(),
			fleetPolicyValidClient{},
			[]*steps.EnumerationResponse_AutotestInvocation{clientTestInvocation("", "")},
		)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, responses, should.NotBeNil)
		assert.Loosely(t, len(responses), should.Equal(1))
	})
}

// TODO (b/247796561): re-enable the test
// As retries are introduced for newBuild,
// the retry function is never executed for canceled context.
// Due to this, the test broke and will have to be re-written.
// func TestIncompleteWait(t *testing.T) {
// 	Convey("Given a run that is cancelled while running, response reflects cancellation.", t, func() {
// 		ctx, cancel := context.WithCancel(context.Background())
// 		var gresps map[string]*steps.ExecuteResponse
// 		var gerr error
// 		wg := sync.WaitGroup{}
// 		wg.Add(1)
// 		go func() {
// 			gresps, gerr = runWithDefaults(
// 				ctx,
// 				trservice.NewStubClientWithCannedIncompleteTasks(test_platform.TaskState_LIFE_CYCLE_RUNNING),
// 				[]*steps.EnumerationResponse_AutotestInvocation{
// 					clientTestInvocation("", ""),
// 				},
// 			)
// 			wg.Done()
// 		}()

// 		cancel()
// 		wg.Wait()
// 		So(gerr, ShouldBeNil)

// 		resp := extractSingleResponse(t, gresps)
// 		So(resp, ShouldNotBeNil)
// 		So(resp.TaskResults, ShouldHaveLength, 1)
// 		So(resp.TaskResults[0].State.LifeCycle, ShouldEqual, test_platform.TaskState_LIFE_CYCLE_RUNNING)
// 	})
// }

func TestEnumerationResponseWithRetries(t *testing.T) {
	ftt.Run("Given a request with retry enabled", t, func(t *ftt.Test) {
		ctx := setFakeTimeWithImmediateTimeout(context.Background())
		params := basicParams()
		params.Retry = &test_platform.Request_Params_Retry{
			Allow: true,
		}
		t.Run("and two tests that always fail and retry limit", func(t *ftt.Test) {
			invs := invocationsWithServerTests("name1", "name2")
			for _, inv := range invs {
				inv.Test.AllowRetries = true
				inv.Test.MaxRetries = 2
			}
			t.Run("for skylab execution", func(t *ftt.Test) {
				resps, err := runWithParams(
					ctx,
					trservice.NewStubClientWithFailedTasks(),
					params,
					invs,
					"",
				)
				assert.Loosely(t, err, should.BeNil)
				resp := extractSingleResponse(t, resps)
				t.Run("response should contain two enumerated results", func(t *ftt.Test) {
					assert.Loosely(t, resp.ConsolidatedResults, should.HaveLength(2))
				})

				for i, er := range resp.ConsolidatedResults {
					t.Run(fmt.Sprintf("%dst enumerated result should contain 3 attempts of a single test", i), func(t *ftt.Test) {
						as := er.GetAttempts()
						n := as[0].Name
						for _, a := range as {
							assert.Loosely(t, a.Name, should.Equal(n))
						}
					})
				}
				t.Run("both tests' results should be enumerated", func(t *ftt.Test) {
					names := make([]string, 2)
					for i := range resp.ConsolidatedResults {
						names[i] = resp.ConsolidatedResults[i].Attempts[0].Name
					}
					sort.Strings(names)
					assert.Loosely(t, names, should.Resemble([]string{"name1", "name2"}))
				})
			})
		})
	})
}

func TestRetries(t *testing.T) {
	ftt.Run("Given a test with", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctx, ts := testclock.UseTime(ctx, time.Now())
		// Setup testclock to immediately advance whenever timer is set; this
		// avoids slowdown due to timer inside of LaunchAndWait.
		ts.SetTimerCallback(func(d time.Duration, t clock.Timer) {
			ts.Add(2 * d)
		})
		params := basicParams()

		cases := []struct {
			name           string
			invocations    []*steps.EnumerationResponse_AutotestInvocation
			trClient       trservice.Client
			retryParams    *test_platform.Request_Params_Retry
			testAllowRetry bool
			testMaxRetry   int32

			// Total number of expected tasks is this +1
			expectedRetryCount          int
			expectedSummaryVerdict      test_platform.TaskState_Verdict
			expectedLogShouldContain    string
			expectedLogShouldNotContain string
		}{
			{
				name:        "1 test; no retry configuration in test or request params",
				invocations: invocationsWithServerTests("name1"),
				trClient:    trservice.NewStubClientWithFailedTasks(),

				expectedRetryCount:       0,
				expectedSummaryVerdict:   test_platform.TaskState_VERDICT_FAILED,
				expectedLogShouldContain: "Hit the test retry limit",
			},
			{
				name:        "1 passing test; retries allowed",
				invocations: invocationsWithServerTests("name1"),
				retryParams: &test_platform.Request_Params_Retry{
					Allow: true,
				},
				testAllowRetry: true,
				testMaxRetry:   1,
				trClient:       trservice.NewStubClientWithSuccessfulTasks(),

				expectedRetryCount:          0,
				expectedSummaryVerdict:      test_platform.TaskState_VERDICT_PASSED,
				expectedLogShouldNotContain: "retry",
			},
			{
				name:        "1 failing test; retries disabled globally",
				invocations: invocationsWithServerTests("name1"),
				retryParams: &test_platform.Request_Params_Retry{
					Allow: false,
				},
				testAllowRetry: true,
				testMaxRetry:   1,
				trClient:       trservice.NewStubClientWithFailedTasks(),

				expectedRetryCount:          0,
				expectedSummaryVerdict:      test_platform.TaskState_VERDICT_FAILED,
				expectedLogShouldContain:    "Hit the task set retry limit",
				expectedLogShouldNotContain: "Hit the test retry limit",
			},
			{
				name:        "1 failing test; retries allowed globally and for test",
				invocations: invocationsWithServerTests("name1"),
				retryParams: &test_platform.Request_Params_Retry{
					Allow: true,
				},
				testAllowRetry: true,
				testMaxRetry:   1,
				trClient:       trservice.NewStubClientWithFailedTasks(),

				expectedRetryCount:     1,
				expectedSummaryVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
			{
				name:        "1 failing test; retries allowed globally, disabled for test",
				invocations: invocationsWithServerTests("name1"),
				retryParams: &test_platform.Request_Params_Retry{
					Allow: true,
				},
				testAllowRetry: false,
				trClient:       trservice.NewStubClientWithFailedTasks(),

				expectedRetryCount:     0,
				expectedSummaryVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
			{
				name:        "1 failing test; retries allowed globally with test maximum",
				invocations: invocationsWithServerTests("name1"),
				retryParams: &test_platform.Request_Params_Retry{
					Allow: true,
				},
				testAllowRetry: true,
				testMaxRetry:   10,
				trClient:       trservice.NewStubClientWithFailedTasks(),

				expectedRetryCount:     10,
				expectedSummaryVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
			{
				name:        "1 failing test; retries allowed globally with global maximum",
				invocations: invocationsWithServerTests("name1"),
				retryParams: &test_platform.Request_Params_Retry{
					Allow: true,
					Max:   5,
				},
				testAllowRetry: true,
				trClient:       trservice.NewStubClientWithFailedTasks(),

				expectedRetryCount:     5,
				expectedSummaryVerdict: test_platform.TaskState_VERDICT_FAILED,
			},
			{
				name:        "1 failing test; retries allowed globally with global maximum smaller than test maxium",
				invocations: invocationsWithServerTests("name1"),
				retryParams: &test_platform.Request_Params_Retry{
					Allow: true,
					Max:   5,
				},
				testAllowRetry: true,
				testMaxRetry:   7,
				trClient:       trservice.NewStubClientWithFailedTasks(),

				expectedRetryCount:          5,
				expectedSummaryVerdict:      test_platform.TaskState_VERDICT_FAILED,
				expectedLogShouldContain:    "Hit the task set retry limit",
				expectedLogShouldNotContain: "Hit the test retry limit",
			},
			{
				name:        "1 failing test; retries allowed globally with test maximum smaller than global maximum",
				invocations: invocationsWithServerTests("name1"),
				retryParams: &test_platform.Request_Params_Retry{
					Allow: true,
					Max:   7,
				},
				testAllowRetry: true,
				testMaxRetry:   5,
				trClient:       trservice.NewStubClientWithFailedTasks(),

				expectedRetryCount:          5,
				expectedSummaryVerdict:      test_platform.TaskState_VERDICT_FAILED,
				expectedLogShouldContain:    "Hit the test retry limit",
				expectedLogShouldNotContain: "Hit the task set retry limit",
			},
			{
				name:        "2 failing tests; retries allowed globally with global maximum",
				invocations: invocationsWithServerTests("name1", "name2"),
				retryParams: &test_platform.Request_Params_Retry{
					Allow: true,
					Max:   5,
				},
				testAllowRetry: true,
				trClient:       trservice.NewStubClientWithFailedTasks(),

				expectedRetryCount:     5,
				expectedSummaryVerdict: test_platform.TaskState_VERDICT_FAILED,
			},

			{
				name:        "1 test that fails then passes; retries allowed",
				invocations: invocationsWithServerTests("name1"),
				retryParams: &test_platform.Request_Params_Retry{
					Allow: true,
				},
				testAllowRetry: true,
				trClient: &trservice.StubClientWithCannedResults{
					CannedResponses: []trservice.FetchResultsResponse{
						{
							LifeCycle: test_platform.TaskState_LIFE_CYCLE_COMPLETED,
							Result: &skylab_test_runner.Result{
								Harness: &skylab_test_runner.Result_AutotestResult{
									AutotestResult: &skylab_test_runner.Result_Autotest{
										Incomplete: false,
										TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
											{
												Name:    "foo",
												Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL,
											},
										},
									},
								},
							},
						},
						{
							LifeCycle: test_platform.TaskState_LIFE_CYCLE_COMPLETED,
							Result: &skylab_test_runner.Result{
								Harness: &skylab_test_runner.Result_AutotestResult{
									AutotestResult: &skylab_test_runner.Result_Autotest{
										Incomplete: false,
										TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
											{
												Name:    "foo",
												Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS,
											},
										},
									},
								},
							},
						},
					},
				},

				expectedRetryCount: 1,
				// TODO(crbug.com/1005609) Indicate in *some way* that a test
				// passed only on retry.
				expectedSummaryVerdict:      test_platform.TaskState_VERDICT_PASSED,
				expectedLogShouldContain:    "Retrying name1",
				expectedLogShouldNotContain: "retry limit",
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *ftt.Test) {
				params.Retry = c.retryParams
				for _, inv := range c.invocations {
					inv.Test.AllowRetries = c.testAllowRetry
					inv.Test.MaxRetries = c.testMaxRetry
				}
				var ml memlogger.MemLogger
				ctx = logging.SetFactory(ctx, func(context.Context) logging.Logger { return &ml })
				trClient := &trservice.CallCountingClientWrapper{
					Client: c.trClient,
				}
				resps, err := runWithParams(ctx, trClient, params, c.invocations, "")
				assert.Loosely(t, err, should.BeNil)
				resp := extractSingleResponse(t, resps)

				t.Run("then the launched task count should be correct.", func(t *ftt.Test) {
					// Each test is tried at least once.
					attemptCount := len(c.invocations) + c.expectedRetryCount
					assert.Loosely(t, resp.TaskResults, should.HaveLength(attemptCount))
				})
				t.Run("then task (name, attempt) should be unique.", func(t *ftt.Test) {
					s := make(stringset.Set)
					for _, res := range resp.TaskResults {
						s.Add(fmt.Sprintf("%s__%d", res.Name, res.Attempt))
					}
					assert.Loosely(t, s, should.HaveLength(len(resp.TaskResults)))
				})

				t.Run("then the build verdict should be correct.", func(t *ftt.Test) {
					assert.Loosely(t, resp.State.Verdict, should.Equal(c.expectedSummaryVerdict))
				})
				t.Run("then the log output should match the retry.", func(t *ftt.Test) {
					if len(c.expectedLogShouldContain) > 0 {
						assert.Loosely(t, loggerInfo(ml), should.ContainSubstring(c.expectedLogShouldContain))
					}
					if len(c.expectedLogShouldNotContain) > 0 {
						assert.Loosely(t, loggerInfo(ml), should.NotContainSubstring(c.expectedLogShouldNotContain))
					}
				})
			})
		}
	})
}

func TestResponseVerdict(t *testing.T) {
	ftt.Run("Given a client test", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Setup testclock to immediately advance whenever timer is set; this
		// avoids slowdown due to timer inside of LaunchAndWait.
		ctx, ts := testclock.UseTime(ctx, time.Now())
		ts.SetTimerCallback(func(d time.Duration, t clock.Timer) {
			ts.Add(2 * d)
		})

		t.Run("when the test passed, response verdict is correct.", func(t *ftt.Test) {
			resps, err := runWithDefaults(
				ctx,
				trservice.NewStubClientWithSuccessfulTasks(),
				[]*steps.EnumerationResponse_AutotestInvocation{
					serverTestInvocation("name1", ""),
				},
			)
			assert.Loosely(t, err, should.BeNil)
			resp := extractSingleResponse(t, resps)
			assert.Loosely(t, resp.State.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_COMPLETED))
			assert.Loosely(t, resp.State.Verdict, should.Equal(test_platform.TaskState_VERDICT_PASSED))
		})

		t.Run("when the test failed, response verdict is correct.", func(t *ftt.Test) {
			resps, err := runWithDefaults(
				ctx,
				trservice.NewStubClientWithFailedTasks(),
				[]*steps.EnumerationResponse_AutotestInvocation{
					serverTestInvocation("name1", ""),
				},
			)
			assert.Loosely(t, err, should.BeNil)
			resp := extractSingleResponse(t, resps)
			assert.Loosely(t, resp.State.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_COMPLETED))
			assert.Loosely(t, resp.State.Verdict, should.Equal(test_platform.TaskState_VERDICT_FAILED))
		})

		t.Run("when execution is aborted (e.g., timeout), response verdict is correct.", func(t *ftt.Test) {
			t.Skip("crbug.com/187777891")
			wg := sync.WaitGroup{}
			wg.Add(1)
			var resps map[string]*steps.ExecuteResponse
			var err error
			go func() {
				resps, err = runWithDefaults(
					ctx,
					trservice.NewStubClientWithCannedIncompleteTasks(test_platform.TaskState_LIFE_CYCLE_RUNNING),
					[]*steps.EnumerationResponse_AutotestInvocation{
						serverTestInvocation("name1", ""),
					},
				)
				wg.Done()
			}()

			cancel()
			wg.Wait()
			assert.Loosely(t, err, should.BeNil)
			resp := extractSingleResponse(t, resps)
			assert.Loosely(t, resp.State.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_ABORTED))
			assert.Loosely(t, resp.State.Verdict, should.Equal(test_platform.TaskState_VERDICT_FAILED))
		})
	})
}

func runWithDefaults(ctx context.Context, skylab trservice.Client, invs []*steps.EnumerationResponse_AutotestInvocation) (map[string]*steps.ExecuteResponse, error) {
	return runWithParams(ctx, skylab, basicParams(), invs, "")
}

func runWithParams(ctx context.Context, skylab trservice.Client, params *test_platform.Request_Params, invs []*steps.EnumerationResponse_AutotestInvocation, pool string) (map[string]*steps.ExecuteResponse, error) {
	args := execution.Args{
		Build: &bbpb.Build{},
		Send:  exe.BuildSender(func() {}),
		Request: &steps.ExecuteRequests{
			TaggedRequests: map[string]*steps.ExecuteRequest{
				"12345678/foo": {
					RequestParams: params,
					Enumeration: &steps.EnumerationResponse{
						AutotestInvocations: invs,
					},
				},
			},
			Build: &execute.Build{
				Id: 42,
			},
		},
		WorkerConfig: &config.Config_SkylabWorker{
			LuciProject: "foo-luci-project",
			LogDogHost:  "foo-logdog-host",
		},
		ParentTaskID: "foo-parent-task-id",
		Deadline:     time.Now().Add(time.Hour),
		SwarmingPool: pool,
	}

	inputJson, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return execution.Run(ctx, skylab, args, filepath.Dir(inputJson))
}

func basicParams() *test_platform.Request_Params {
	return &test_platform.Request_Params{
		SoftwareAttributes: &test_platform.Request_Params_SoftwareAttributes{
			BuildTarget: &chromiumos.BuildTarget{Name: "foo-board"},
		},
		HardwareAttributes: &test_platform.Request_Params_HardwareAttributes{
			Model: "foo-model",
		},
		FreeformAttributes: &test_platform.Request_Params_FreeformAttributes{
			SwarmingDimensions: []string{"freeform-key:freeform-value"},
		},
		SoftwareDependencies: []*test_platform.Request_Params_SoftwareDependency{
			{
				Dep: &test_platform.Request_Params_SoftwareDependency_ChromeosBuild{ChromeosBuild: "foo-build"},
			},
			{
				Dep: &test_platform.Request_Params_SoftwareDependency_RoFirmwareBuild{RoFirmwareBuild: "foo-ro-firmware"},
			},
			{
				Dep: &test_platform.Request_Params_SoftwareDependency_RwFirmwareBuild{RwFirmwareBuild: "foo-rw-firmware"},
			},
		},
		Scheduling: &test_platform.Request_Params_Scheduling{
			Pool: &test_platform.Request_Params_Scheduling_ManagedPool_{
				ManagedPool: test_platform.Request_Params_Scheduling_MANAGED_POOL_CQ,
			},
			Priority: 79,
		},
		Time: &test_platform.Request_Params_Time{
			MaximumDuration: &duration.Duration{Seconds: 60},
		},
		Decorations: &test_platform.Request_Params_Decorations{
			AutotestKeyvals: map[string]string{"k1": "v1"},
			Tags:            []string{"foo-tag1", "foo-tag2"},
		},
	}
}

func invocationsWithServerTests(names ...string) []*steps.EnumerationResponse_AutotestInvocation {
	ret := make([]*steps.EnumerationResponse_AutotestInvocation, len(names))
	for i, n := range names {
		ret[i] = serverTestInvocation(n, "")
	}
	return ret
}

func serverTestInvocation(name string, args string) *steps.EnumerationResponse_AutotestInvocation {
	return &steps.EnumerationResponse_AutotestInvocation{
		Test: &buildapi.AutotestTest{
			Name:                 name,
			ExecutionEnvironment: buildapi.AutotestTest_EXECUTION_ENVIRONMENT_SERVER,
		},
		TestArgs: args,
	}
}

func clientTestInvocation(name string, args string) *steps.EnumerationResponse_AutotestInvocation {
	return &steps.EnumerationResponse_AutotestInvocation{
		Test: &buildapi.AutotestTest{
			Name:                 name,
			ExecutionEnvironment: buildapi.AutotestTest_EXECUTION_ENVIRONMENT_CLIENT,
		},
		TestArgs: args,
	}
}

func setFakeTimeWithImmediateTimeout(ctx context.Context) context.Context {
	ctx, ts := testclock.UseTime(ctx, time.Now())
	// Setup testclock to immediately advance whenever timer is set; this
	// avoids slowdown due to timer inside of LaunchAndWait.
	ts.SetTimerCallback(func(d time.Duration, t clock.Timer) {
		ts.Add(2 * d)
	})
	return ctx
}

func extractSingleResponse(t testing.TB, resps map[string]*steps.ExecuteResponse) *steps.ExecuteResponse {
	t.Helper()
	assert.Loosely(t, resps, should.HaveLength(1), truth.LineContext())
	for _, resp := range resps {
		assert.Loosely(t, resp, should.NotBeNil, truth.LineContext())
		return resp
	}
	panic("unreachable")
}

func loggerInfo(ml memlogger.MemLogger) string {
	out := ""
	for _, m := range ml.Messages() {
		if m.Level == logging.Info {
			out = out + m.Msg
		}
	}
	return out
}

// TestSwarmingPool tests whether swarming pool is correctly passed to the
// ValidateArgs, LaunchTask functions
func TestSwarmingPool(t *testing.T) {
	ftt.Run("Given test", t, func(t *ftt.Test) {
		t.Run("when running a skylab execution", func(t *ftt.Test) {
			trClient := &trservice.ArgsCollectingClientWrapper{
				Client: trservice.StubClient{},
			}
			resps, err := runWithParams(
				context.Background(),
				trClient, basicParams(),
				[]*steps.EnumerationResponse_AutotestInvocation{
					clientTestInvocation("", ""),
				},
				"OtherPool",
			)

			assert.Loosely(t, err, should.BeNil)
			extractSingleResponse(t, resps)
			assert.Loosely(t, len(trClient.Calls.ValidateArgs), should.Equal(1))
			assert.Loosely(t, trClient.Calls.ValidateArgs[0].Args.SwarmingPool, should.Equal("OtherPool"))
			// we dont actually (explicitly) use this dimension when scheduling
			// tests; pool automatically added based on the bb builder
			assert.Loosely(t, len(trClient.Calls.LaunchTask), should.Equal(1))
		})
	})
}
