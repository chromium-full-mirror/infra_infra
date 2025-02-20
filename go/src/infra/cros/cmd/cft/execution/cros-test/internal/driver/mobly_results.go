// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// Unmarshal Json respresentation of SessionDetailResponse to golang struct
func toSessionResponse(jsonText string) (*sessionDetailResponse, error) {
	var response *sessionDetailResponse
	err := json.Unmarshal([]byte(jsonText), &response)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize (%v): %w", jsonText, err)
	}
	return response, nil
}

// Helper function for creating a CFT TestCaseResult skeleton
func buildCommonTestCaseResult(test *api.TestCaseMetadata,
	startTime time.Time,
	duration time.Duration) *api.TestCaseResult {
	tcResult := &api.TestCaseResult{}
	tcResult.TestHarness = &api.TestHarness{TestHarnessType: &api.TestHarness_Mobly_{Mobly: &api.TestHarness_Mobly{}}}
	if test != nil && test.TestCase != nil && test.TestCase.Id != nil {
		tcResult.TestCaseId = test.TestCase.Id
	}
	tcResult.TestCaseMetadata = test
	tcResult.StartTime = timestamppb.New(startTime)
	tcResult.Duration = &durationpb.Duration{Seconds: int64(duration.Seconds())}
	return tcResult
}

// Create CFT TestCaseResult based on verdict and reason.
func buildTestCaseResult(test *api.TestCaseMetadata,
	startTime time.Time,
	duration time.Duration,
	verdict string,
	reason string) *api.TestCaseResult {
	tcResult := buildCommonTestCaseResult(test, startTime, duration)

	switch verdict {
	case "PASS":
		tcResult.Verdict = &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}}
	case "FAIL":
		tcResult.Verdict = &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}}
	case "CRASH":
		tcResult.Verdict = &api.TestCaseResult_Crash_{Crash: &api.TestCaseResult_Crash{}}
	case "ABORT":
		tcResult.Verdict = &api.TestCaseResult_Abort_{Abort: &api.TestCaseResult_Abort{}}
	case "SKIP":
		tcResult.Verdict = &api.TestCaseResult_Skip_{Skip: &api.TestCaseResult_Skip{}}
	case "NOT_RUN":
		tcResult.Verdict = &api.TestCaseResult_NotRun_{NotRun: &api.TestCaseResult_NotRun{}}
	default:
		tcResult.Verdict = &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}}
	}

	if reason != "" {
		tcResult.Reason = reason
	}

	return tcResult
}

// Convert SessionDetailResponse in CFT TestCaseResult
func buildTestCaseResultFromSessionResponse(test *api.TestCaseMetadata,
	startTime time.Time,
	duration time.Duration,
	response *sessionDetailResponse) *api.TestCaseResult {
	tcResult := buildCommonTestCaseResult(test, startTime, duration)

	if response != nil && response.SessionDetail != nil && response.SessionDetail.SessionSummary != nil {
		switch result := response.SessionDetail.SessionSummary.Result; result {
		case "PASS":
			tcResult.Verdict = &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}}
		case "ABORT":
			tcResult.Verdict = &api.TestCaseResult_Abort_{Abort: &api.TestCaseResult_Abort{}}
		case "SKIP":
			tcResult.Verdict = &api.TestCaseResult_Skip_{Skip: &api.TestCaseResult_Skip{}}
		case "FAIL", "UNKNOWN", "INFRA_ERROR", "ALLOC_FAIL", "ALLOC_ERROR":
			// we lump all these cases into "FAIL". There could be fine granularity control
			// on fetching the errors from sessionDetailResponse as we learn more about those scenarios.
			tcResult.Verdict = &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}}

			reasons := []string{}
			reasons = append(reasons, fmt.Sprintf("Omnilab result: %v", result))
			if response.SessionDetail != nil {
				for _, jd := range response.SessionDetail.JobDetail {
					if jd.JobSummary != nil {
						reasons = append(reasons, jd.JobSummary.Error...)
					}
				}
			}
			tcResult.Reason = strings.Join(reasons, "\n")

		case "TIMEOUT":
			tcResult.Verdict = &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}}
			tcResult.Reason = "timeout during omnilab execution"
		}
	} else {
		tcResult.Verdict = &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}}
		tcResult.Reason = "Omnilab result not available"
	}
	return tcResult
}
