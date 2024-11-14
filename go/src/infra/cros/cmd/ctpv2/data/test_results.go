// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/tools/suitelimits"
)

type TestResults struct {
	Suite         string
	ShardIndex    int
	Key           string // [board-model-variant]-shard-%d
	TopLevelError error
	Results       *skylab_test_runner.Result
	Attempt       int // 0 means no retry
	BuildUrl      string
	BuildID       int64
	RequestKey    string // this is used to link back the results to original request
	Name          string
	TestCases     []*api.TestCase_Id

	// For ATP reporting
	CreationTimestamp time.Time
	StartTimestamp    time.Time
	EndTimestamp      time.Time
}

func (t *TestResults) GetProvisionErrIfAny() error {
	if t.Results.GetPrejob() != nil && len(t.Results.GetPrejob().GetStep()) > 0 {
		for _, step := range t.Results.GetPrejob().GetStep() {
			if step.GetName() == "provision" && step.GetVerdict() != skylab_test_runner.Result_Prejob_Step_VERDICT_PASS {
				failureMsg := "provision failed"
				if step.GetHumanReadableSummary() != "" {
					failureMsg = fmt.Sprintf("%s: %s", failureMsg, step.GetHumanReadableSummary())
				}
				return fmt.Errorf(failureMsg)
			}
		}
	}

	return nil
}

func (t *TestResults) GetFailureErr() error {
	if t.TopLevelError != nil {
		return t.TopLevelError
	}

	// Propagate provision failure if any
	if err := t.GetProvisionErrIfAny(); err != nil {
		return err
	}

	if t.Results.GetAutotestResults() == nil && t.Results.GetAndroidGenericResult() == nil {
		return fmt.Errorf("no test result found")
	}

	// Handle autotest results
	if t.Results.GetAutotestResults() != nil {

		testResults, ok := t.Results.GetAutotestResults()["original_test"]
		if !ok {
			// the test results from trv2 should be here, if not,
			// something else failed before test execution. so fail.
			return fmt.Errorf("no test result found")
		}

		// Incomplete == err.
		if testResults.GetIncomplete() {
			return fmt.Errorf("test(s) incomplete")
		}

		for _, testCase := range testResults.GetTestCases() {
			if testCase.GetVerdict() != skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS {
				return fmt.Errorf("test(s) failed")
			}
		}

		return nil
	}

	// Handle android generic results
	if t.Results.GetAndroidGenericResult() != nil {
		for _, givenTestCase := range t.Results.GetAndroidGenericResult().GetGivenTestCases() {
			if givenTestCase.GetIncomplete() {
				return fmt.Errorf("test(s) incomplete")
			}

			for _, testCase := range givenTestCase.GetChildTestCases() {
				if testCase.GetVerdict() != skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS {
					return fmt.Errorf("test(s) failed")
				}
			}
		}
	}
	return nil
}

func (t *TestResults) GetTestCounts() (int, int, int) {
	if t.TopLevelError != nil {
		return 0, 0, 0
	}

	totalTestCount := 0
	totalFailedTestCount := 0
	totalFailedTestRunCount := 0
	testCasesFoundInResutls := 0
	testCasesNames := common.GetFlattenedTestCases(t.TestCases)

	// Handle android generic results
	genericResults := t.Results.GetAndroidGenericResult().GetGivenTestCases()
	if len(genericResults) > 0 {
		for _, givenTestCase := range genericResults {
			if slices.Contains(testCasesNames, givenTestCase.ParentTest) {
				testCasesFoundInResutls++
			}
			for _, testCase := range givenTestCase.GetChildTestCases() {
				totalTestCount++
				if testCase.GetVerdict() != skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS {
					totalFailedTestCount++
				}
			}
		}

		totalFailedTestRunCount = len(testCasesNames) - testCasesFoundInResutls
		// totalFailedTestRunCount cannot be negative
		if totalFailedTestRunCount < 0 {
			totalFailedTestRunCount = 0
		}

		return totalTestCount, totalFailedTestCount, totalFailedTestRunCount
	}

	// Handle legacy autotest format
	testResults, ok := t.Results.GetAutotestResults()["original_test"]
	if !ok {
		// the test results from trv2 should be here, if not,
		// something else failed before test execution. so fail.
		return 0, 0, 0
	}

	for _, testCase := range testResults.GetTestCases() {
		totalTestCount++
		if testCase.GetVerdict() != skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS {
			totalFailedTestCount++
		}
	}

	return totalTestCount, totalFailedTestCount, totalFailedTestRunCount
}

type ByAttempt []*TestResults

func (a ByAttempt) Len() int           { return len(a) }
func (a ByAttempt) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByAttempt) Less(i, j int) bool { return a[i].Attempt < a[j].Attempt }

type BotParamsRejectedError struct {
	Key          string
	RejectedDims []string
}

func (e *BotParamsRejectedError) Error() string {
	return fmt.Sprintf("rejected params: %s", strings.Join(e.RejectedDims, ", "))
}

type EnumerationError struct {
	SuiteName string
}

func (e *EnumerationError) Error() string {
	return fmt.Sprintf("no test found for suite '%s'", e.SuiteName)
}

type SuiteLimitsError struct {
	SuiteName     string
	RequestName   string
	TotalDUTHours time.Duration
}

func (e *SuiteLimitsError) Error() string {
	return fmt.Sprintf("SUITE LIMITS: request %s for suite %s ran for %d DUT seconds, %d maximum allowed", e.RequestName, e.SuiteName, int(e.TotalDUTHours.Seconds()), suitelimits.DutHourMaximumSeconds)
}
