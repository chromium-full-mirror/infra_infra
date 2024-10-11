// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"fmt"
	"strings"
	"time"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"

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

	// For ATP reporting
	CreationTimestamp time.Time
	StartTimestamp    time.Time
	EndTimestamp      time.Time
}

func (t *TestResults) GetFailureErr() error {
	if t.TopLevelError != nil {
		return t.TopLevelError
	}

	if t.Results.GetAutotestResults() == nil && t.Results.GetAndroidGenericResult() == nil {
		return fmt.Errorf("no test result found")
	}

	if t.Results.GetAutotestResults() != nil {
		testResults, ok := t.Results.GetAutotestResults()["original_test"]
		if !ok {
			// the test results from trv2 should be here, if not,
			// something else failed before test execution. so fail.
			return fmt.Errorf("no test result found")
		}

		for _, testCase := range testResults.GetTestCases() {
			if testCase.GetVerdict() != skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS {
				return fmt.Errorf("test(s) failure")
			}
		}

		return nil
	}

	if t.Results.GetAndroidGenericResult() != nil {
		for _, givenTestCase := range t.Results.GetAndroidGenericResult().GetGivenTestCases() {
			for _, testCase := range givenTestCase.GetChildTestCases() {
				if testCase.GetVerdict() != skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS {
					return fmt.Errorf("test(s) failure")
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

	testResults, ok := t.Results.GetAutotestResults()["original_test"]
	if !ok {
		// the test results from trv2 should be here, if not,
		// something else failed before test execution. so fail.
		return 0, 0, 0
	}

	totalTestCount := 0
	totalFailedTestCount := 0
	totalFailedTestRunCount := 0 // TODO: add run count when available; currently returning 0

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
