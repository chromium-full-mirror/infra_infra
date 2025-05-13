// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package driver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
)

var cmpOpts = cmpopts.IgnoreUnexported(
	api.TestCaseResult{},
	api.TestCase_Id{},
	api.TestHarness{},
	api.TestHarness_Mobly{},
	api.TestCaseResult_Pass{},
	api.TestCaseResult_Fail{},
	api.TestCaseResult_Crash{},
	api.TestCaseResult_Abort{},
	api.TestCaseResult_Skip{},
	api.TestCaseResult_NotRun{},
	api.TestCaseMetadata{},
	api.TestCase{},
	timestamppb.Timestamp{},
	durationpb.Duration{},
)
var defaultStartTime = time.Date(2023, 12, 12, 0, 0, 0, 0, time.UTC)
var defaultDuration = time.Second * 120
var defaultMoblyTestDataDir = "mobly_test_data"

func TestToSessionRequest(t *testing.T) {
	path, _ := os.Getwd()

	for _, input := range []string{
		"omnilab_session_details_accept.json",
		"omnilab_session_details_done.json",
		"omnilab_session_details_error.json",
		"omnilab_session_details_running.json",
	} {
		filePath := filepath.Join(path, defaultMoblyTestDataDir, input)
		dat, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("failed to read file %v: %v", filePath, err)
		}

		// Verify if json can be deserialized into structs
		jsonText := string(dat)
		response, err := toSessionResponse(jsonText)
		if err != nil {
			t.Errorf("failed when calling toSessionResponse on json (%v): %v", jsonText, err)
		}

		// Verify if the reconstructed json from structs is the same representation as the inputs.
		reconstructedJSONText, err := structToJSONStr(response)
		if err != nil {
			t.Errorf("Failed to structToJsonStr %v: %v", response, err)
		}

		diff, err := jsonDiff([]byte(jsonText), []byte(reconstructedJSONText))
		if err != nil {
			t.Errorf("Failed to jsonDiff: %v \n", err)
		}
		if diff != "" {
			t.Errorf("%s", diff)
		}
	}
}

func TestBuildTestCaseResult(t *testing.T) {

	type testInput struct {
		verdict string
		reason  string
	}

	for _, tc := range []struct {
		input    *testInput
		expected *api.TestCaseResult
	}{
		{
			input: &testInput{
				verdict: "PASS",
			},
			expected: &api.TestCaseResult{
				Verdict:     &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
				TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Mobly_{Mobly: &api.TestHarness_Mobly{}}},
				StartTime:   timestamppb.New(defaultStartTime),
				Duration:    &durationpb.Duration{Seconds: int64(defaultDuration.Seconds())},
			},
		},
		{
			input: &testInput{
				verdict: "FAIL",
				reason:  "Failure reason",
			},
			expected: &api.TestCaseResult{
				Verdict:     &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}},
				TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Mobly_{Mobly: &api.TestHarness_Mobly{}}},
				StartTime:   timestamppb.New(defaultStartTime),
				Duration:    &durationpb.Duration{Seconds: int64(defaultDuration.Seconds())},
				Reason:      "Failure reason",
			},
		},
	} {
		actual := buildTestCaseResult(nil,
			defaultStartTime,
			defaultDuration,
			tc.input.verdict,
			tc.input.reason)

		if diff := cmp.Diff(actual, tc.expected, cmpOpts); diff != "" {
			t.Errorf("%s", fmt.Sprintf("%v: results not the same: %v", tc.input.verdict, diff))
		}
	}
}

func TestBuildTestCaseResultFromSessionResponse(t *testing.T) {
	type testInput struct {
		test     *api.TestCaseMetadata
		response *sessionDetailResponse
	}

	for _, tc := range []struct {
		input            *testInput
		defaultStartTime *time.Time
		defaultDuration  *time.Duration
		expected         *api.TestCaseResult
	}{
		{
			input: &testInput{
				test: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Id:   &api.TestCase_Id{Value: "TestcaseId 1"},
					Name: "Testcase Pass",
				}},
				response: &sessionDetailResponse{
					SessionDetail: &sessionDetail{
						SessionSummary: &sessionSummary{
							Status: "DONE",
							Result: "PASS",
						},
					},
				},
			},
			expected: &api.TestCaseResult{
				TestCaseId:  &api.TestCase_Id{Value: "TestcaseId 1"},
				Verdict:     &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
				TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Mobly_{Mobly: &api.TestHarness_Mobly{}}},
				StartTime:   timestamppb.New(defaultStartTime),
				Duration:    &durationpb.Duration{Seconds: int64(defaultDuration.Seconds())},

				TestCaseMetadata: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Id:   &api.TestCase_Id{Value: "TestcaseId 1"},
					Name: "Testcase Pass",
				}},
			},
		},
		{
			input: &testInput{
				test: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Name: "Testcase Fail with error messages",
				}},
				response: &sessionDetailResponse{
					SessionDetail: &sessionDetail{
						SessionSummary: &sessionSummary{
							Status: "DONE",
							Result: "FAIL",
						},
						JobDetail: []*jobDetail{
							{
								JobSummary: &jobSummary{
									Error: []string{
										"Job1_ErrorMsg1",
										"Job1_ErrorMsg2",
									},
								},
							},
							{
								JobSummary: &jobSummary{
									Error: []string{
										"Job2_ErrorMsg1",
										"Job2_ErrorMsg2",
									},
								},
							},
						},
					},
				},
			},
			expected: &api.TestCaseResult{
				Verdict:     &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}},
				Reason:      "Omnilab result: FAIL\nJob1_ErrorMsg1\nJob1_ErrorMsg2\nJob2_ErrorMsg1\nJob2_ErrorMsg2",
				TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Mobly_{Mobly: &api.TestHarness_Mobly{}}},
				StartTime:   timestamppb.New(defaultStartTime),
				Duration:    &durationpb.Duration{Seconds: int64(defaultDuration.Seconds())},

				TestCaseMetadata: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Name: "Testcase Fail with error messages",
				}},
			},
		},
		{
			input: &testInput{
				test: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Name: "Testcase Fail without error messages",
				}},
				response: &sessionDetailResponse{
					SessionDetail: &sessionDetail{
						SessionSummary: &sessionSummary{
							Status: "DONE",
							Result: "FAIL",
						},
					},
				},
			},
			expected: &api.TestCaseResult{
				Verdict:     &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}},
				Reason:      "Omnilab result: FAIL",
				TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Mobly_{Mobly: &api.TestHarness_Mobly{}}},
				StartTime:   timestamppb.New(defaultStartTime),
				Duration:    &durationpb.Duration{Seconds: int64(defaultDuration.Seconds())},

				TestCaseMetadata: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Name: "Testcase Fail without error messages",
				}},
			},
		},
		{
			input: &testInput{
				test: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Name: "Testcase Abort",
				}},
				response: &sessionDetailResponse{
					SessionDetail: &sessionDetail{
						SessionSummary: &sessionSummary{
							Status: "DONE",
							Result: "ABORT",
						},
					},
				},
			},
			expected: &api.TestCaseResult{
				Verdict:     &api.TestCaseResult_Abort_{Abort: &api.TestCaseResult_Abort{}},
				TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Mobly_{Mobly: &api.TestHarness_Mobly{}}},
				StartTime:   timestamppb.New(defaultStartTime),
				Duration:    &durationpb.Duration{Seconds: int64(defaultDuration.Seconds())},
				TestCaseMetadata: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Name: "Testcase Abort",
				}},
			},
		},
		{
			input: &testInput{
				test: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Name: "Testcase Timeout",
				}},
				response: &sessionDetailResponse{
					SessionDetail: &sessionDetail{
						SessionSummary: &sessionSummary{
							Status: "DONE",
							Result: "TIMEOUT",
						},
					},
				},
			},
			expected: &api.TestCaseResult{
				Verdict:     &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}},
				Reason:      "timeout during omnilab execution",
				TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Mobly_{Mobly: &api.TestHarness_Mobly{}}},
				StartTime:   timestamppb.New(defaultStartTime),
				Duration:    &durationpb.Duration{Seconds: int64(defaultDuration.Seconds())},
				TestCaseMetadata: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Name: "Testcase Timeout",
				}},
			},
		},
		{
			input: &testInput{
				test: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Name: "Testcase Omnilab no results",
				}},
				response: &sessionDetailResponse{},
			},
			expected: &api.TestCaseResult{
				Verdict:     &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}},
				Reason:      "Omnilab result not available",
				TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Mobly_{Mobly: &api.TestHarness_Mobly{}}},
				StartTime:   timestamppb.New(defaultStartTime),
				Duration:    &durationpb.Duration{Seconds: int64(defaultDuration.Seconds())},
				TestCaseMetadata: &api.TestCaseMetadata{TestCase: &api.TestCase{
					Name: "Testcase Omnilab no results",
				}},
			},
		},
	} {
		actual := buildTestCaseResultFromSessionResponse(tc.input.test,
			defaultStartTime,
			defaultDuration,
			tc.input.response)

		if diff := cmp.Diff(actual, tc.expected, cmpOpts); diff != "" {
			t.Errorf("%s", fmt.Sprintf("%v: results not the same: %v", tc.input.test.TestCase.Name, diff))
		}
	}

}

// jsonDiff compares the JSON in two byte slices.
func jsonDiff(jsonBytes1, jsonBytes2 []byte) (string, error) {
	var json1, json2 any
	if err := json.Unmarshal(jsonBytes1, &json1); err != nil {
		return "", err
	}
	if err := json.Unmarshal(jsonBytes2, &json2); err != nil {
		return "", err
	}
	diff := cmp.Diff(json2, json1)
	return diff, nil
}

// convert a struct to a json string
func structToJSONStr(data any) (string, error) {
	val, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}
