// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execution_test

// This file contains tests to cover the update of the Build proto during an
// execution.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/luciexe/exe"

	"go.chromium.org/infra/cmd/cros_test_platform/internal/execution"
	"go.chromium.org/infra/cmd/cros_test_platform/internal/execution/testrunner"
	trservice "go.chromium.org/infra/cmd/cros_test_platform/internal/execution/testrunner/service"
)

func TestFinalBuildForSingleInvocation(t *testing.T) {
	ftt.Run("For a run with one request with one invocation", t, func(t *ftt.Test) {
		ba := newBuildAccumulator()
		_, err := runWithBuildAccumulator(
			context.Background(),
			stubTestRunnerClientWithCannedURL{
				Client:    trservice.NewStubClientWithSuccessfulTasks(),
				CannedURL: exampleTestRunnerURL,
			},
			ba,
			&steps.ExecuteRequests{
				TaggedRequests: map[string]*steps.ExecuteRequest{
					"request-with-single-invocation": {
						RequestParams: basicParams(),
						Enumeration: &steps.EnumerationResponse{
							AutotestInvocations: []*steps.EnumerationResponse_AutotestInvocation{
								clientTestInvocation("first-invocation", ""),
							},
						},
					},
				},
			},
		)
		assert.Loosely(t, err, should.BeNil)

		b := ba.GetLatestBuild()
		assert.Loosely(t, b, should.NotBeNil)
		assert.Loosely(t, b.GetSteps(), should.HaveLength(2))

		rs := stepForRequest(b, "request-with-single-invocation")
		assert.Loosely(t, rs, should.NotBeNil)

		is := stepForInvocation(b, "first-invocation")
		assert.Loosely(t, is, should.NotBeNil)
		assert.Loosely(t, is.Name, should.ContainSubstring("request-with-single-invocation"))
		markdownContainsURL(t, is.GetSummaryMarkdown(), "latest attempt", exampleTestRunnerURL)
	})
}

func TestFinalBuildForTwoInvocations(t *testing.T) {
	ftt.Run("For a run with one request with two invocations", t, func(t *ftt.Test) {
		ba := newBuildAccumulator()
		_, err := runWithBuildAccumulator(
			context.Background(),
			stubTestRunnerClientWithCannedURL{
				Client:    trservice.NewStubClientWithSuccessfulTasks(),
				CannedURL: exampleTestRunnerURL,
			},
			ba,
			&steps.ExecuteRequests{
				TaggedRequests: map[string]*steps.ExecuteRequest{
					"request-with-two-invocations": {
						RequestParams: basicParams(),
						Enumeration: &steps.EnumerationResponse{
							AutotestInvocations: []*steps.EnumerationResponse_AutotestInvocation{
								clientTestInvocation("first-invocation", ""),
								clientTestInvocation("second-invocation", ""),
							},
						},
					},
				},
			},
		)
		assert.Loosely(t, err, should.BeNil)

		b := ba.GetLatestBuild()
		assert.Loosely(t, b, should.NotBeNil)
		assert.Loosely(t, b.GetSteps(), should.HaveLength(3))

		rs := stepForRequest(b, "request-with-two-invocations")
		assert.Loosely(t, rs, should.NotBeNil)

		is := stepForInvocation(b, "first-invocation")
		assert.Loosely(t, is, should.NotBeNil)
		assert.Loosely(t, is.Name, should.ContainSubstring("request-with-two-invocations"))
		markdownContainsURL(t, is.GetSummaryMarkdown(), "latest attempt", exampleTestRunnerURL)

		is = stepForInvocation(b, "second-invocation")
		assert.Loosely(t, is, should.NotBeNil)
		assert.Loosely(t, is.Name, should.ContainSubstring("request-with-two-invocations"))
		markdownContainsURL(t, is.GetSummaryMarkdown(), "latest attempt", exampleTestRunnerURL)
	})
}

func TestFinalBuildForTwoRequests(t *testing.T) {
	ftt.Run("For a run with two requests with one invocation each", t, func(t *ftt.Test) {
		ba := newBuildAccumulator()
		_, err := runWithBuildAccumulator(
			context.Background(),
			stubTestRunnerClientWithCannedURL{
				Client:    trservice.NewStubClientWithSuccessfulTasks(),
				CannedURL: exampleTestRunnerURL,
			},
			ba,
			&steps.ExecuteRequests{
				TaggedRequests: map[string]*steps.ExecuteRequest{
					"first-request": {
						RequestParams: basicParams(),
						Enumeration: &steps.EnumerationResponse{
							AutotestInvocations: []*steps.EnumerationResponse_AutotestInvocation{
								clientTestInvocation("first-request-invocation", ""),
							},
						},
					},
					"second-request": {
						RequestParams: basicParams(),
						Enumeration: &steps.EnumerationResponse{
							AutotestInvocations: []*steps.EnumerationResponse_AutotestInvocation{
								clientTestInvocation("second-request-invocation", ""),
							},
						},
					},
				},
			},
		)
		assert.Loosely(t, err, should.BeNil)

		b := ba.GetLatestBuild()
		assert.Loosely(t, b, should.NotBeNil)
		assert.Loosely(t, b.GetSteps(), should.HaveLength(4))

		rs := stepForRequest(b, "first-request")
		assert.Loosely(t, rs, should.NotBeNil)
		is := stepForInvocation(b, "first-request-invocation")
		assert.Loosely(t, is, should.NotBeNil)
		assert.Loosely(t, is.Name, should.ContainSubstring("first-request"))
		markdownContainsURL(t, is.GetSummaryMarkdown(), "latest attempt", exampleTestRunnerURL)

		rs = stepForRequest(b, "second-request")
		assert.Loosely(t, rs, should.NotBeNil)
		is = stepForInvocation(b, "second-request-invocation")
		assert.Loosely(t, is, should.NotBeNil)
		assert.Loosely(t, is.Name, should.ContainSubstring("second-request"))
		markdownContainsURL(t, is.GetSummaryMarkdown(), "latest attempt", exampleTestRunnerURL)
	})
}

func TestFinalBuildForSingleInvocationWithRetries(t *testing.T) {
	ftt.Run("For a run with one request with one invocation that needs 1 retry", t, func(t *ftt.Test) {
		params := basicParams()
		params.Retry = &test_platform.Request_Params_Retry{
			Allow: true,
			Max:   1,
		}
		inv := clientTestInvocation("failing-invocation", "")
		inv.Test.AllowRetries = true
		req := &steps.ExecuteRequests{
			TaggedRequests: map[string]*steps.ExecuteRequest{
				"request-with-one-retry-allowed": {
					RequestParams: params,
					Enumeration: &steps.EnumerationResponse{
						AutotestInvocations: []*steps.EnumerationResponse_AutotestInvocation{inv},
					},
				},
			},
		}

		ba := newBuildAccumulator()
		_, err := runWithBuildAccumulator(
			setFakeTimeWithImmediateTimeout(context.Background()),
			stubTestRunnerClientWithCannedURL{
				Client:    trservice.NewStubClientWithFailedTasks(),
				CannedURL: exampleTestRunnerURL,
			},
			ba,
			req,
		)
		assert.Loosely(t, err, should.BeNil)

		b := ba.GetLatestBuild()
		assert.Loosely(t, b, should.NotBeNil)
		assert.Loosely(t, b.GetSteps(), should.HaveLength(2))

		is := stepForInvocation(b, "failing-invocation")
		assert.Loosely(t, is, should.NotBeNil)

		markdownContainsURL(t, is.GetSummaryMarkdown(), "latest attempt", exampleTestRunnerURL)
		markdownContainsURL(t, is.GetSummaryMarkdown(), "1", exampleTestRunnerURL)
	})
}

func TestBuildUpdatesWithRetries(t *testing.T) {
	ftt.Run("Compared to a run without retries", t, func(t *ftt.Test) {
		inv := clientTestInvocation("failing-invocation", "")
		inv.Test.AllowRetries = true
		e := &steps.EnumerationResponse{
			AutotestInvocations: []*steps.EnumerationResponse_AutotestInvocation{inv},
		}
		req := &steps.ExecuteRequests{
			TaggedRequests: map[string]*steps.ExecuteRequest{
				"no-retry": {
					RequestParams: basicParams(),
					Enumeration:   e,
				},
			},
		}

		ba := newBuildAccumulator()
		_, err := runWithBuildAccumulator(
			setFakeTimeWithImmediateTimeout(context.Background()),
			stubTestRunnerClientWithCannedURL{
				Client:    trservice.NewStubClientWithFailedTasks(),
				CannedURL: exampleTestRunnerURL,
			},
			ba,
			req,
		)
		assert.Loosely(t, err, should.BeNil)
		noRetryUpdateCount := len(ba.Sent)
		_ = noRetryUpdateCount

		t.Run("a run with a retry should send more updates", func(t *ftt.Test) {
			params := basicParams()
			params.Retry = &test_platform.Request_Params_Retry{
				Allow: true,
				Max:   1,
			}
			req := &steps.ExecuteRequests{
				TaggedRequests: map[string]*steps.ExecuteRequest{
					"one-retry": {
						RequestParams: params,
						Enumeration:   e,
					},
				},
			}

			ba := newBuildAccumulator()
			_, err := runWithBuildAccumulator(
				setFakeTimeWithImmediateTimeout(context.Background()),
				stubTestRunnerClientWithCannedURL{
					Client:    trservice.NewStubClientWithFailedTasks(),
					CannedURL: exampleTestRunnerURL,
				},
				ba,
				req,
			)
			assert.Loosely(t, err, should.BeNil)
			oneRetryUpdateCount := len(ba.Sent)

			assert.Loosely(t, oneRetryUpdateCount, should.BeGreaterThan(noRetryUpdateCount))
		})
	})
}

type transientBBErrorClient struct {
	trservice.StubClient
}

type nonTransientBBErrorClient struct {
	trservice.StubClient
}

// FetchResults implements Client interface.
func (c transientBBErrorClient) FetchResults(context.Context, trservice.TaskReference) (*trservice.FetchResultsResponse, error) {
	return &trservice.FetchResultsResponse{BuildBucketTransientFailure: true}, errors.New("simulated error from fake client")
}

// FetchResults implements Client interface.
func (c nonTransientBBErrorClient) FetchResults(context.Context, trservice.TaskReference) (*trservice.FetchResultsResponse, error) {
	return &trservice.FetchResultsResponse{BuildBucketTransientFailure: false}, errors.New("simulated error from fake client")
}

func TestRefreshWithTransientBuildBuckerError(t *testing.T) {
	ftt.Run("For a refresh call with build bucket errors", t, func(t *ftt.Test) {
		err := testrunner.NewBuildForTesting("task1", "url1").Refresh(
			context.Background(),
			transientBBErrorClient{},
		)
		// Test that the error is swallowed because BuildBucketTransientFailure is
		// true.
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestRefreshWithNonTransientBuildBuckerError(t *testing.T) {
	ftt.Run("For a refresh call without build bucket errors", t, func(t *ftt.Test) {
		err := testrunner.NewBuildForTesting("task1", "url1").Refresh(
			context.Background(),
			nonTransientBBErrorClient{},
		)
		// Test that the error is swallowed because BuildBucketTransientFailure is
		// true.
		assert.Loosely(t, err, should.NotBeNil)
	})
}

const exampleTestRunnerURL = "https://ci.chromium.org/p/chromeos/builders/test_runner/test_runner/b8872341436802087200"

// buildAccumulator supports a Send method to accumulate the bbpb.Build sent
// to the host application.
//
// Typical usage:
//
//	ba := newBuildAccumulator()
//	err := runWithBuildAccumulator(..., ba, ...)
//	So(err, ShouldBeNil)
//	So(ba.GetLatestBuild(), ShouldNotBeNil)
//	...
type buildAccumulator struct {
	Input *bbpb.Build
	Sent  []*bbpb.Build
}

func newBuildAccumulator() *buildAccumulator {
	return &buildAccumulator{
		Input: &bbpb.Build{},
		Sent:  []*bbpb.Build{},
	}
}

func (s *buildAccumulator) Send() {
	s.Sent = append(s.Sent, proto.Clone(s.Input).(*bbpb.Build))
}

func (s *buildAccumulator) GetLatestBuild() *bbpb.Build {
	if len(s.Sent) == 0 {
		return nil
	}
	return s.Sent[len(s.Sent)-1]
}

func runWithBuildAccumulator(ctx context.Context, skylab trservice.Client, ba *buildAccumulator, request *steps.ExecuteRequests) (map[string]*steps.ExecuteResponse, error) {
	args := execution.Args{
		Build:   ba.Input,
		Send:    exe.BuildSender(ba.Send),
		Request: request,
		WorkerConfig: &config.Config_SkylabWorker{
			LuciProject: "foo-luci-project",
			LogDogHost:  "foo-logdog-host",
		},
		ParentTaskID: "foo-parent-task-id",
		Deadline:     time.Now().Add(time.Hour),
	}

	inputJson, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return execution.Run(ctx, skylab, args, filepath.Dir(inputJson))
}

type stubTestRunnerClientWithCannedURL struct {
	trservice.Client
	CannedURL string
}

// URL implements Client interface.
func (c stubTestRunnerClientWithCannedURL) URL(trservice.TaskReference) string {
	return c.CannedURL
}

// stepForRequest returns the first step for a request with the given name.
//
// Returns nil if no such step is found.
func stepForRequest(build *bbpb.Build, name string) *bbpb.Step {
	for _, s := range build.Steps {
		if isRequestStep(s.GetName()) && strings.Contains(s.GetName(), name) {
			return s
		}
	}
	return nil
}

// stepForInvocation returns the first step for an invocation with the given
// name.
//
// Returns nil if no such step is found.
func stepForInvocation(build *bbpb.Build, name string) *bbpb.Step {
	for _, s := range build.Steps {
		if isInvocationStep(s.GetName()) && strings.Contains(s.GetName(), name) {
			return s
		}
	}
	return nil
}

var (
	requestStepRe    = regexp.MustCompile(`\s*request.*\|\s*invocation.*`)
	invocationStepRe = regexp.MustCompile(`.*|\s*invocation.*`)
)

func isRequestStep(name string) bool {
	return requestStepRe.Match([]byte(name))
}

func isInvocationStep(name string) bool {
	return invocationStepRe.Match([]byte(name))
}

func markdownContainsURL(t testing.TB, md string, target string, url string) {
	t.Helper()
	assert.Loosely(t, md, should.ContainSubstring(fmt.Sprintf("[%s](%s)", target, url)), truth.LineContext())
}
