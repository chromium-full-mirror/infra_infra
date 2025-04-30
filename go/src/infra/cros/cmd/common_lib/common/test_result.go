// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/duration"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	configpb "go.chromium.org/chromiumos/config/go"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	artifactpb "go.chromium.org/chromiumos/config/go/test/artifact"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
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
					Dut: &labapi.Dut{
						DutType: &labapi.Dut_Chromeos{
							Chromeos: &labapi.Dut_ChromeOS{
								Name: "chromeos15-row4-rack5-host1",
								DutModel: &labapi.DutModel{
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
					TestCaseMetadata: &testapi.TestCaseMetadata{
						TestCase: &testapi.TestCase{
							Id: &testapi.TestCase_Id{
								Value: "invocations/build-8803850119519478545/tests/rlz_CheckPing/results/567764de-00001",
							},
							Name: "rlz_CheckPing",
						},
					},
					TestCaseResult: &testapi.TestCaseResult{
						Verdict:   &testapi.TestCaseResult_Pass_{},
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
					TestCaseMetadata: &testapi.TestCaseMetadata{
						TestCase: &testapi.TestCase{
							Id: &testapi.TestCase_Id{
								Value: "invocations/build-8803850119519478545/tests/power_Resume/results/567764de-00002",
							},
							Name: "power_Resume",
						},
					},
					TestCaseResult: &testapi.TestCaseResult{
						Verdict:   &testapi.TestCaseResult_Fail_{},
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
func GetValueFromRequestKeyvals(ctx context.Context, cftReq *skylab_test_runner.CFTTestRequest, ctrrReq *testapi.CrosTestRunnerDynamicRequest, key string) string {
	if cftReq == nil && ctrrReq == nil {
		return ""
	}
	var keyvals map[string]string
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

// GetTesthausURL gets testhaus log viewer url based on the invocation name.
// If invocation name is empty, it constructs the URL based on gcs URL instead.
func GetTesthausURL(invocationName string, gcsURL string) string {
	if invocationName != "" {
		return fmt.Sprintf("%s%s", TesthausURLPrefix, invocationName)
	}

	gcsPrefix := "gs://"
	postfix := ""
	if strings.HasPrefix(gcsURL, gcsPrefix) {
		postfix = gcsURL[len(gcsPrefix):]
	}
	return fmt.Sprintf("%s%s", TesthausURLPrefix, postfix)
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
func IsAnyTestFailure(testResults []*testapi.TestCaseResult) bool {
	for _, testResult := range testResults {
		switch testResult.Verdict.(type) {
		case *testapi.TestCaseResult_Fail_, *testapi.TestCaseResult_Abort_, *testapi.TestCaseResult_Crash_:
			return true
		case *testapi.TestCaseResult_NotRun_:
			// Normally NotRun's are not considered as test failures.
			// But when containing the reason of "exit status 1", it can be
			// safely inferred that the testing environment had some form of
			// error overall.
			if strings.Contains(testResult.GetReason(), "exit status 1") {
				return true
			}
			continue
		default:
			continue
		}
	}

	return false
}

func GetTaskStateVerdict(trResult *skylab_test_runner.Result) test_platform.TaskState_Verdict {
	if trResult == nil {
		return test_platform.TaskState_VERDICT_UNSPECIFIED
	}

	switch trResult.Harness.(type) {
	case *skylab_test_runner.Result_AutotestResult:
		return getAutotestResultTaskStateVerdict(trResult)
	case *skylab_test_runner.Result_AndroidGenericResult:
		return getAndroidGenericResultTaskStateVerdict(trResult)
	default:
	}

	return test_platform.TaskState_VERDICT_UNSPECIFIED
}

func getAndroidGenericResultTaskStateVerdict(trResult *skylab_test_runner.Result) test_platform.TaskState_Verdict {
	androidGenericResult := trResult.GetAndroidGenericResult()
	if androidGenericResult == nil {
		return test_platform.TaskState_VERDICT_UNSPECIFIED
	}

	// By default (if no test cases ran), then there is no verdict.
	verdict := test_platform.TaskState_VERDICT_NO_VERDICT
	for _, testCase := range androidGenericResult.GetGivenTestCases() {
		for _, childTestCase := range testCase.GetChildTestCases() {
			outVerdict, returnVerdict := handleVerdict(childTestCase.GetVerdict())
			if returnVerdict != test_platform.TaskState_VERDICT_UNSPECIFIED {
				return returnVerdict
			}
			if outVerdict != test_platform.TaskState_VERDICT_UNSPECIFIED {
				verdict = outVerdict
			}
		}
	}
	return verdict
}

func getAutotestResultTaskStateVerdict(trResult *skylab_test_runner.Result) test_platform.TaskState_Verdict {
	autoTestResult := trResult.GetAutotestResult()
	if autoTestResult == nil {
		return test_platform.TaskState_VERDICT_UNSPECIFIED
	}
	if autoTestResult.Incomplete {
		return test_platform.TaskState_VERDICT_FAILED
	}

	// By default (if no test cases ran), then there is no verdict.
	verdict := test_platform.TaskState_VERDICT_NO_VERDICT
	for _, c := range autoTestResult.GetTestCases() {
		outVerdict, returnVerdict := handleVerdict(c.GetVerdict())
		if returnVerdict != test_platform.TaskState_VERDICT_UNSPECIFIED {
			return returnVerdict
		}
		if outVerdict != test_platform.TaskState_VERDICT_UNSPECIFIED {
			verdict = outVerdict
		}
	}
	return verdict
}

func handleVerdict(inVerdict skylab_test_runner.Result_Autotest_TestCase_Verdict) (outVerdict, returnVerdict test_platform.TaskState_Verdict) {
	switch inVerdict {
	case skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL:
		// Any case failing means the flat verdict is a failure.
		returnVerdict = test_platform.TaskState_VERDICT_FAILED
	case skylab_test_runner.Result_Autotest_TestCase_VERDICT_ERROR:
		// Any case failing means the flat verdict is a failure.
		returnVerdict = test_platform.TaskState_VERDICT_FAILED
	case skylab_test_runner.Result_Autotest_TestCase_VERDICT_ABORT:
		// Any case failing means the flat verdict is a failure.
		returnVerdict = test_platform.TaskState_VERDICT_FAILED
	case skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS:
		// Otherwise, at least 1 passing verdict means a pass.
		outVerdict = test_platform.TaskState_VERDICT_PASSED
	default: // VERDICT_UNDEFINED and VERDICT_NO_VERDICT
		// Treat as no-op and do not affect flat verdict.
	}

	return
}

var liftTestCaseRunnerVerdict = map[skylab_test_runner.Result_Autotest_TestCase_Verdict]test_platform.TaskState_Verdict{
	skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS:       test_platform.TaskState_VERDICT_PASSED,
	skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL:       test_platform.TaskState_VERDICT_FAILED,
	skylab_test_runner.Result_Autotest_TestCase_VERDICT_ERROR:      test_platform.TaskState_VERDICT_FAILED,
	skylab_test_runner.Result_Autotest_TestCase_VERDICT_ABORT:      test_platform.TaskState_VERDICT_FAILED,
	skylab_test_runner.Result_Autotest_TestCase_VERDICT_NO_VERDICT: test_platform.TaskState_VERDICT_NO_VERDICT,
}

func TestCasesToTestCaseResult(trResult *skylab_test_runner.Result) []*steps.ExecuteResponse_TaskResult_TestCaseResult {
	switch trResult.Harness.(type) {
	case *skylab_test_runner.Result_AutotestResult:
		return autotestTestCasesToTestCaseResult(trResult)
	case *skylab_test_runner.Result_AndroidGenericResult:
		return androidGenericTestCasesToTestCaseResult(trResult)
	default:
	}
	return nil
}

func androidGenericTestCasesToTestCaseResult(trResult *skylab_test_runner.Result) []*steps.ExecuteResponse_TaskResult_TestCaseResult {
	androidGeneric := trResult.GetAndroidGenericResult()
	if androidGeneric == nil || len(androidGeneric.GetGivenTestCases()) == 0 {
		// Prefer a nil over an empty slice since it's the proto default.
		return nil
	}
	ret := []*steps.ExecuteResponse_TaskResult_TestCaseResult{}
	for _, testCase := range androidGeneric.GetGivenTestCases() {
		for _, childTestCase := range testCase.GetChildTestCases() {
			ret = append(ret, &steps.ExecuteResponse_TaskResult_TestCaseResult{
				Name:                 testCase.GetParentTest() + "." + childTestCase.GetName(),
				Verdict:              liftTestCaseRunnerVerdict[childTestCase.GetVerdict()],
				HumanReadableSummary: childTestCase.GetHumanReadableSummary(),
			})
		}
	}
	return ret
}

func autotestTestCasesToTestCaseResult(trResult *skylab_test_runner.Result) []*steps.ExecuteResponse_TaskResult_TestCaseResult {
	autotestResult := trResult.GetAutotestResult()
	if autotestResult == nil || len(autotestResult.GetTestCases()) == 0 {
		// Prefer a nil over an empty slice since it's the proto default.
		return nil
	}
	tcs := autotestResult.GetTestCases()
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

// GetProductName gets the product/device name in the following format:
// <board>.<model>-<board_variant> expected by the XTS result pipeline.
// The board and model info is obtained from dutModel falling back to labels in
// botDims, and the board variant is extracted from the build.
// If any of the properties is missing, it will be left out of the product.
// TODO: b/379711782 - Verify and update this logic for AL
func GetProductName(dutModel *labapi.DutModel, botDims []*buildbucketpb.StringPair, build string) string {
	// Get primary board, model from DUT model falling back to bot dimensions
	board := dutModel.GetBuildTarget()
	model := dutModel.GetModelName()

	if board == "" || model == "" {
		for _, dim := range botDims {
			if board == "" && dim.GetKey() == "label-board" {
				board = dim.GetValue()
			}

			if model == "" && dim.GetKey() == "label-model" {
				model = dim.GetValue()
			}
		}
	}

	// If board name is empty, skip finding the board variant
	if board == "" {
		return model
	}

	// Extract the variant from the build
	// e.g. "-arc-t-release" from "brya-arc-t-release/R114-15437.0.0"
	variant := ""
	variantRegexp := regexp.MustCompile(fmt.Sprintf(`^%s(.*)\/.*`, board))
	matches := variantRegexp.FindStringSubmatch(build)
	if len(matches) > 1 {
		// remove the "-release" suffix if present in the variant
		variant = strings.TrimSuffix(matches[1], "-release")
	}

	if model == "" {
		return fmt.Sprintf("%s%s", board, variant)
	}

	return fmt.Sprintf("%s.%s%s", board, model, variant)
}

// SanitizeGCSPrefix sanitizes the GCS path prefix to ensure it does not contain
// trailing "/".
func SanitizeGCSPrefix(prefix string) string {
	return strings.TrimSuffix(prefix, "/")
}
