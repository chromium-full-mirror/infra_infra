// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/duration"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	configpb "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	apipb "go.chromium.org/chromiumos/config/go/test/api"
	artifactpb "go.chromium.org/chromiumos/config/go/test/artifact"
	labpb "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
)

// GetMockedTestResultProto returns a mock result proto
// that can be used for testing.
func GetMockedTestResultProto() *artifactpb.TestResult {
	testResult := &artifactpb.TestResult{
		Version: 1,
		TestInvocation: &artifactpb.TestInvocation{
			PrimaryExecutionInfo: &artifactpb.ExecutionInfo{
				BuildInfo: &artifactpb.BuildInfo{
					Name:            "hatch-cq/R106-15048.0.0",
					Milestone:       106,
					ChromeOsVersion: "15048.0.0",
					Source:          "hatch-cq",
					Board:           "hatch",
				},
				DutInfo: &artifactpb.DutInfo{
					Dut: &labpb.Dut{
						DutType: &labpb.Dut_Chromeos{
							Chromeos: &labpb.Dut_ChromeOS{
								Name: "chromeos15-row4-rack5-host1",
								DutModel: &labpb.DutModel{
									ModelName: "nipperkin",
								},
							},
						},
					},
				},
			},
		},
		TestRuns: []*artifactpb.TestRun{
			{
				TestCaseInfo: &artifactpb.TestCaseInfo{
					TestCaseMetadata: &apipb.TestCaseMetadata{
						TestCase: &apipb.TestCase{
							Id: &apipb.TestCase_Id{
								Value: "invocations/build-8803850119519478545/tests/rlz_CheckPing/results/567764de-00001",
							},
							Name: "rlz_CheckPing",
						},
					},
					TestCaseResult: &apipb.TestCaseResult{
						Verdict:   &apipb.TestCaseResult_Pass_{},
						StartTime: timestamppb.New(parseTime("2022-09-07T18:53:33.983328614Z")),
						Duration:  &duration.Duration{Seconds: 60},
					},
				},
				LogsInfo: []*configpb.StoragePath{
					{
						HostType: configpb.StoragePath_GS,
						Path:     "gs://chromeos-test-logs/test-runner/prod/2022-09-07/98098abe-da4f-4bfa-bef5-9cbc4936da03",
					},
				},
			},
			{
				TestCaseInfo: &artifactpb.TestCaseInfo{
					TestCaseMetadata: &apipb.TestCaseMetadata{
						TestCase: &apipb.TestCase{
							Id: &apipb.TestCase_Id{
								Value: "invocations/build-8803850119519478545/tests/power_Resume/results/567764de-00002",
							},
							Name: "power_Resume",
						},
					},
					TestCaseResult: &apipb.TestCaseResult{
						Verdict:   &apipb.TestCaseResult_Fail_{},
						Reason:    "Test failed",
						StartTime: timestamppb.New(parseTime("2022-09-07T18:53:34.983328614Z")),
						Duration:  &duration.Duration{Seconds: 120, Nanos: 100000000},
					},
				},
				LogsInfo: []*configpb.StoragePath{
					{
						HostType: configpb.StoragePath_GS,
						Path:     "gs://chromeos-test-logs/test-runner/prod/2022-09-07/98098abe-da4f-4bfa-bef5-9cbc4936da04",
					},
				},
			},
		},
	}

	return testResult
}

func parseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05.99Z", s)
	return t
}

// GetValueFromRequestKeyvals gets value from provided keyvals based on key.
func GetValueFromRequestKeyvals(ctx context.Context, cftReq *skylab_test_runner.CFTTestRequest, ctrrReq *api.CrosTestRunnerDynamicRequest, key string) string {
	if cftReq == nil && ctrrReq == nil {
		return ""
	}
	keyvals := map[string]string{}
	if ctrrReq != nil {
		keyvals = ctrrReq.GetParams().GetKeyvals()
	} else {
		keyvals = cftReq.GetAutotestKeyvals()
	}

	value, ok := keyvals[key]
	if !ok {
		return ""
	}

	return value
}

// GetTesthausURL gets testhaus log viewer url.
func GetTesthausURL(gcsURL string) string {
	return fmt.Sprintf("%s%s", TesthausURLPrefix, gcsURL[len("gs://"):])
}

// GetGcsURL gets gcs url where all the artifacts will be uploaded.
func GetGcsURL(gsRoot string) string {
	return fmt.Sprintf(
		"%s/%s/%s",
		gsRoot,
		time.Now().Format("2006-01-02"),
		uuid.New().String())
}

// GetGcsClickableLink constructs the gcs cliclable link from provided gs url.
func GetGcsClickableLink(gsURL string) string {
	if gsURL == "" {
		return ""
	}
	gsPrefix := "gs://"
	urlSuffix := gsURL
	if strings.HasPrefix(gsURL, gsPrefix) {
		urlSuffix = gsURL[len(gsPrefix):]
	}
	return fmt.Sprintf("%s%s", GcsURLPrefix, urlSuffix)
}

// IsAnyTestFailure returns if there is any failed tests in test results
func IsAnyTestFailure(testResults []*apipb.TestCaseResult) bool {
	for _, testResult := range testResults {
		switch testResult.Verdict.(type) {
		case *apipb.TestCaseResult_Fail_, *apipb.TestCaseResult_Abort_, *apipb.TestCaseResult_Crash_:
			return true
		default:
			continue
		}
	}

	return false
}

func GetTaskStateVerdict(trResult *skylab_test_runner.Result) test_platform.TaskState_Verdict {
	if trResult == nil || trResult.GetAutotestResult() == nil {
		return test_platform.TaskState_VERDICT_UNSPECIFIED
	}
	autoTestResult := trResult.GetAutotestResult()
	if autoTestResult.Incomplete {
		return test_platform.TaskState_VERDICT_FAILED
	}

	// By default (if no test cases ran), then there is no verdict.
	verdict := test_platform.TaskState_VERDICT_NO_VERDICT
	for _, c := range autoTestResult.GetTestCases() {
		switch c.Verdict {
		case skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL:
			// Any case failing means the flat verdict is a failure.
			return test_platform.TaskState_VERDICT_FAILED
		case skylab_test_runner.Result_Autotest_TestCase_VERDICT_ERROR:
			// Any case failing means the flat verdict is a failure.
			return test_platform.TaskState_VERDICT_FAILED
		case skylab_test_runner.Result_Autotest_TestCase_VERDICT_ABORT:
			// Any case failing means the flat verdict is a failure.
			return test_platform.TaskState_VERDICT_FAILED
		case skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS:
			// Otherwise, at least 1 passing verdict means a pass.
			verdict = test_platform.TaskState_VERDICT_PASSED
		default: // VERDICT_UNDEFINED and VERDICT_NO_VERDICT
			// Treat as no-op and do not affect flat verdict.
		}
	}
	return verdict
}

var liftTestCaseRunnerVerdict = map[skylab_test_runner.Result_Autotest_TestCase_Verdict]test_platform.TaskState_Verdict{
	skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS:       test_platform.TaskState_VERDICT_PASSED,
	skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL:       test_platform.TaskState_VERDICT_FAILED,
	skylab_test_runner.Result_Autotest_TestCase_VERDICT_ERROR:      test_platform.TaskState_VERDICT_FAILED,
	skylab_test_runner.Result_Autotest_TestCase_VERDICT_ABORT:      test_platform.TaskState_VERDICT_FAILED,
	skylab_test_runner.Result_Autotest_TestCase_VERDICT_NO_VERDICT: test_platform.TaskState_VERDICT_NO_VERDICT,
}

func TestCasesToTestCaseResult(tcs []*skylab_test_runner.Result_Autotest_TestCase) []*steps.ExecuteResponse_TaskResult_TestCaseResult {
	if len(tcs) == 0 {
		// Prefer a nil over an empty slice since it's the proto default.
		return nil
	}
	ret := make([]*steps.ExecuteResponse_TaskResult_TestCaseResult, len(tcs))
	for i, tc := range tcs {
		ret[i] = &steps.ExecuteResponse_TaskResult_TestCaseResult{
			Name:                 tc.GetName(),
			Verdict:              liftTestCaseRunnerVerdict[tc.Verdict],
			HumanReadableSummary: tc.GetHumanReadableSummary(),
		}
	}
	return ret
}

func GetTautoTestCaseNameOrDefault(tcs []*skylab_test_runner.Result_Autotest_TestCase, defaultName string) string {
	for _, tc := range tcs {
		if strings.HasPrefix(tc.GetName(), "tauto.tast") {
			return tc.GetName()
		}
	}
	return defaultName
}

var liftPreJobVerdict = map[skylab_test_runner.Result_Prejob_Step_Verdict]test_platform.TaskState_Verdict{
	skylab_test_runner.Result_Prejob_Step_VERDICT_PASS:      test_platform.TaskState_VERDICT_PASSED,
	skylab_test_runner.Result_Prejob_Step_VERDICT_FAIL:      test_platform.TaskState_VERDICT_FAILED,
	skylab_test_runner.Result_Prejob_Step_VERDICT_UNDEFINED: test_platform.TaskState_VERDICT_FAILED,
}

func PrejobStepsToTestCaseResult(pjs []*skylab_test_runner.Result_Prejob_Step) []*steps.ExecuteResponse_TaskResult_TestCaseResult {
	if len(pjs) == 0 {
		// Prefer a nil over an empty slice since it's the proto default.
		return nil
	}
	ret := make([]*steps.ExecuteResponse_TaskResult_TestCaseResult, len(pjs))
	for i, pj := range pjs {
		ret[i] = &steps.ExecuteResponse_TaskResult_TestCaseResult{
			Name:                 pj.GetName(),
			Verdict:              liftPreJobVerdict[pj.Verdict],
			HumanReadableSummary: pj.GetHumanReadableSummary(),
		}
	}
	return ret
}

func GetDims(dims []string) (map[string]string, []*steps.ExecuteResponse_TaskResult_RejectedTaskDimension) {
	r1 := map[string]string{}
	r2 := []*steps.ExecuteResponse_TaskResult_RejectedTaskDimension{}
	for _, dim := range dims {
		dimsList := strings.Split(dim, ":")
		if len(dimsList) != 2 {
			// should never happen
			return nil, nil
		}
		r1[dimsList[0]] = dimsList[1]
		r2 = append(r2, &steps.ExecuteResponse_TaskResult_RejectedTaskDimension{Key: dimsList[0], Value: dimsList[1]})
	}
	return r1, r2
}
