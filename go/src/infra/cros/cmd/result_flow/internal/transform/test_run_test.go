// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package transform_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/analytics"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/result_flow"
	runner "go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/result_flow/internal/transform"
)

var (
	fakeTestRunnerBuildID int64 = 8875806983290035888
	fakeBuildbucketConfig       = &result_flow.BuildbucketConfig{
		Host:    "fakehost",
		Project: "fakeProject",
		Bucket:  "fakeBucket",
		Builder: "fakeBuilder",
	}
)

type in struct {
	label         string
	status        bbpb.Status
	prejobVerdict runner.Result_Prejob_Step_Verdict
	incomplete    bool
	botDimensions []string
	testCases     []*runner.Result_Autotest_TestCase
}

type out struct {
	label         string
	prejobVerdict string
	lifeCycle     string
	verdict       string
	model         string
	testCases     []*analytics.TestCaseResult
}

func TestRunnerOutputToTestRun(t *testing.T) {
	cases := []struct {
		description string
		in          *in
		out         *out
	}{
		{
			"Given an ongoing test runner build",
			&in{
				label:         "foo",
				status:        bbpb.Status_STARTED,
				prejobVerdict: runner.Result_Prejob_Step_VERDICT_UNDEFINED,
				incomplete:    false,
				botDimensions: []string{},
				testCases:     []*runner.Result_Autotest_TestCase{},
			},
			&out{
				label:         "foo",
				prejobVerdict: "",
				lifeCycle:     "RUNNING",
				verdict:       "",
				model:         "",
				testCases:     []*analytics.TestCaseResult{},
			},
		},
		{
			"Given a test runner build with failed prejob",
			&in{
				label:         "foo",
				status:        bbpb.Status_FAILURE,
				prejobVerdict: runner.Result_Prejob_Step_VERDICT_FAIL,
				incomplete:    false,
				botDimensions: []string{"label-model:fake-model"},
				testCases:     []*runner.Result_Autotest_TestCase{},
			},
			&out{
				label:         "foo",
				prejobVerdict: "FAILED",
				lifeCycle:     "COMPLETED",
				verdict:       "NO_VERDICT",
				model:         "fake-model",
				testCases:     []*analytics.TestCaseResult{},
			},
		},
		{
			"Given a completed test runner build",
			&in{
				label:         "hoo",
				status:        bbpb.Status_SUCCESS,
				prejobVerdict: runner.Result_Prejob_Step_VERDICT_PASS,
				incomplete:    false,
				botDimensions: []string{"label-model:fake-model", "label-foo:fake-foo"},
				testCases: []*runner.Result_Autotest_TestCase{
					genFakeAutotestTestCase("foo", runner.Result_Autotest_TestCase_VERDICT_PASS),
					genFakeAutotestTestCase("hoo", runner.Result_Autotest_TestCase_VERDICT_FAIL),
				},
			},
			&out{
				label:         "hoo",
				prejobVerdict: "PASSED",
				lifeCycle:     "COMPLETED",
				verdict:       "FAILED",
				model:         "fake-model",
				testCases: []*analytics.TestCaseResult{
					genFakeTestCaseResult("foo", "VERDICT_PASS"),
					genFakeTestCaseResult("hoo", "VERDICT_FAIL"),
				},
			},
		},
		{
			"Given an incomplete test runner build",
			&in{
				label:         "hoo",
				status:        bbpb.Status_SUCCESS,
				prejobVerdict: runner.Result_Prejob_Step_VERDICT_PASS,
				incomplete:    true,
				botDimensions: []string{"label-model:fake-model"},
				testCases: []*runner.Result_Autotest_TestCase{
					genFakeAutotestTestCase("foo", runner.Result_Autotest_TestCase_VERDICT_PASS),
				},
			},
			&out{
				label:         "hoo",
				prejobVerdict: "PASSED",
				lifeCycle:     "COMPLETED",
				verdict:       "FAILED",
				model:         "fake-model",
				testCases: []*analytics.TestCaseResult{
					genFakeTestCaseResult("foo", "VERDICT_PASS"),
				},
			},
		},
		{
			"Given an unrecognizable bot label",
			&in{
				label:         "hoo",
				status:        bbpb.Status_SUCCESS,
				prejobVerdict: runner.Result_Prejob_Step_VERDICT_PASS,
				incomplete:    false,
				botDimensions: []string{"fake-key:fake-value"},
				testCases: []*runner.Result_Autotest_TestCase{
					genFakeAutotestTestCase("foo", runner.Result_Autotest_TestCase_VERDICT_PASS),
				},
			},
			&out{
				label:         "hoo",
				prejobVerdict: "PASSED",
				lifeCycle:     "COMPLETED",
				verdict:       "PASSED",
				model:         "",
				testCases: []*analytics.TestCaseResult{
					genFakeTestCaseResult("foo", "VERDICT_PASS"),
				},
			},
		},
	}
	ctx := context.Background()
	for _, c := range cases {
		ftt.Run(c.description, t, func(t *ftt.Test) {
			t.Run("then test runner build is correctly converted to TestRun and TestCaseResult.", func(t *ftt.Test) {
				build, _ := transform.LoadTestRunnerBuild(
					ctx,
					"TestPlanRuns/fake-build-id/fake-test-run",
					genFakeTestRunnerBuild(c.in),
					fakeBuildbucketConfig,
				)
				got := build.ToTestRun(ctx)
				assert.Loosely(t, got, should.NotBeNil)
				checkTestRunEquality(t, got, genFakeTestRun(c.out))
				if l := len(c.out.testCases); l > 0 {
					cs := build.ToTestCaseResults()
					sort.Slice(cs, func(i, j int) bool { return cs[i].Uid < cs[j].Uid })
					for i := range l {
						checkTestCaseEquality(t, cs[i], c.out.testCases[i])
					}
				}
			})
		})
	}
}

func genFakeTestRun(out *out) *analytics.TestRun {
	return &analytics.TestRun{
		BuildId:     fakeTestRunnerBuildID,
		DisplayName: fmt.Sprintf("fake-test-run-%s", out.label),
		ExecutionUrl: fmt.Sprintf(
			"https://ci.chromium.org/p/%s/builders/%s/%s/b%d",
			fakeBuildbucketConfig.Project,
			fakeBuildbucketConfig.Bucket,
			fakeBuildbucketConfig.Builder,
			fakeTestRunnerBuildID,
		),
		Model:      out.model,
		ParentUid:  "TestPlanRuns/fake-build-id/fake-test-run",
		FullLogUrl: "gs://fakeLogURL",
		Prejob: &analytics.TestRun_Prejob{
			Verdict: &analytics.Verdict{
				Value: out.prejobVerdict,
			},
		},
		CreateTime: fakeCreateTime,
		StartTime:  fakeStartTime,
		EndTime:    fakeEndTime,
		Status: &analytics.Status{
			Value: out.lifeCycle,
		},
		Verdict: &analytics.Verdict{
			Value: out.verdict,
		},
	}
}

func genFakeTestCaseResult(label, verdict string) *analytics.TestCaseResult {
	return &analytics.TestCaseResult{
		Uid:           fmt.Sprintf("testCaseResults/%d/%s/%s", fakeTestRunnerBuildID, label, "fake-test-case"),
		DisplayName:   fmt.Sprintf("%s/%s", label, "fake-test-case"),
		ParentBuildId: fakeTestRunnerBuildID,
		Verdict: &analytics.Verdict{
			Value: verdict,
		},
		HumanReadableSummary: fmt.Sprintf("fake summary for %s", label),
		CreateTime:           fakeEndTime,
	}
}

func genFakeTestRunnerBuild(in *in) *bbpb.Build {
	res := &bbpb.Build{
		Id:         fakeTestRunnerBuildID,
		CreateTime: fakeCreateTime,
		StartTime:  fakeStartTime,
		EndTime:    fakeEndTime,
		Status:     in.status,
		Input: testRunnerRequestsToInputField(
			genFakeTestRunnerRequest(in.label),
		),
	}
	if int(in.status)&int(bbpb.Status_ENDED_MASK) != 0 {
		res.Output = pbToOutputField(
			genFakeTestRunnerResult(in.prejobVerdict, in.incomplete, in.testCases...),
			"compressed_result",
		)
		res.Infra = genBuildInfra(in.botDimensions)
	}
	return res
}

func genBuildInfra(botDimensions []string) *bbpb.BuildInfra {
	dimensions := make([]*bbpb.StringPair, len(botDimensions))
	for i, d := range botDimensions {
		substrings := strings.Split(d, ":")
		dimensions[i] = &bbpb.StringPair{
			Key:   substrings[0],
			Value: substrings[1],
		}
	}
	return &bbpb.BuildInfra{
		Swarming: &bbpb.BuildInfra_Swarming{
			BotDimensions: dimensions,
		},
	}
}

func testRunnerRequestsToInputField(request *runner.Request) *bbpb.Build_Input {
	m, _ := marshalPB(request)
	return &bbpb.Build_Input{
		Properties: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"request": m,
			},
		},
	}
}

func genFakeTestRunnerRequest(label string) *runner.Request {
	return &runner.Request{
		Test: &runner.Request_Test{
			Harness: &runner.Request_Test_Autotest_{
				Autotest: &runner.Request_Test_Autotest{
					DisplayName: fmt.Sprintf("fake-test-run-%s", label),
				},
			},
		},
	}
}

func genFakeTestRunnerResult(prejobVerdict runner.Result_Prejob_Step_Verdict, incomplete bool, cs ...*runner.Result_Autotest_TestCase) *runner.Result {
	return &runner.Result{
		Prejob: &runner.Result_Prejob{
			Step: []*runner.Result_Prejob_Step{
				{
					Verdict: prejobVerdict,
				},
			},
		},
		Harness: &runner.Result_AutotestResult{
			AutotestResult: &runner.Result_Autotest{
				TestCases:  cs,
				Incomplete: incomplete,
			},
		},
	}
}

func genFakeAutotestTestCase(label string, verdict runner.Result_Autotest_TestCase_Verdict) *runner.Result_Autotest_TestCase {
	return &runner.Result_Autotest_TestCase{
		Name:                 fmt.Sprintf("%s/%s", label, "fake-test-case"),
		HumanReadableSummary: fmt.Sprintf("fake summary for %s", label),
		Verdict:              verdict,
	}
}

func checkTestRunEquality(t testing.TB, want, got *analytics.TestRun) {
	assert.Loosely(t, got.BuildId, should.Equal(want.BuildId))
	assert.Loosely(t, got.DisplayName, should.Equal(want.DisplayName))
	assert.Loosely(t, got.ExecutionUrl, should.Equal(want.ExecutionUrl))
	assert.Loosely(t, got.ParentUid, should.Equal(want.ParentUid))
	assert.Loosely(t, got.Model, should.Equal(want.Model))
	assert.Loosely(t, got.CreateTime, should.Equal(want.CreateTime))
	assert.Loosely(t, got.StartTime, should.Equal(want.StartTime))
	assert.Loosely(t, got.EndTime, should.Equal(want.EndTime))
	assert.Loosely(t, got.GetVerdict().GetValue(), should.Equal(want.GetVerdict().GetValue()))
	assert.Loosely(t, got.GetStatus().GetValue(), should.Equal(want.GetStatus().GetValue()))
	assert.Loosely(t, got.GetPrejob().GetVerdict().GetValue(), should.Equal(want.GetPrejob().GetVerdict().GetValue()))
}

func checkTestCaseEquality(t testing.TB, want, got *analytics.TestCaseResult) {
	assert.Loosely(t, got.Uid, should.Equal(want.Uid))
	assert.Loosely(t, got.DisplayName, should.Equal(want.DisplayName))
	assert.Loosely(t, got.ParentBuildId, should.Equal(want.ParentBuildId))
	assert.Loosely(t, got.GetVerdict().GetValue(), should.Equal(want.GetVerdict().GetValue()))
	assert.Loosely(t, got.HumanReadableSummary, should.Equal(want.HumanReadableSummary))
	assert.Loosely(t, got.CreateTime, should.Equal(want.CreateTime))
}
