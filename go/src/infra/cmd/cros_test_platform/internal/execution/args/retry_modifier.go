// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package args contains the logic for assembling all data required for
// creating an individual task request.
package args

import (
	"context"

	"google.golang.org/protobuf/proto"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"

	"go.chromium.org/infra/libs/skylab/request"
)

// RetryModifier defines the inputs for modifying a retry attempt's arguments.
type RetryModifier struct {
	// RetryNumber represents how many retries this build has performed.
	RetryNumber int32
	// PrevTestCaseResults represents the test case results from the build that is being retried.
	PrevTestCaseResults map[string]test_platform.TaskState_Verdict
}

// ModifyArgs injects the Retry modifications into the arguments
func (rm *RetryModifier) ModifyArgs(ctx context.Context, args request.Args) error {
	if args.TestRunnerRequest != nil && !proto.Equal(args.TestRunnerRequest, &skylab_test_runner.Request{}) {
		args.TestRunnerRequest.RetryNumber = rm.RetryNumber
		return nil
	} else if !args.CFTIsEnabled || args.CFTTestRunnerRequest == nil || proto.Equal(args.CFTTestRunnerRequest, &skylab_test_runner.CFTTestRequest{}) {
		return nil
	}
	args.CFTTestRunnerRequest.RetryNumber = rm.RetryNumber

	testSuites := []*testapi.TestSuite{}
	for _, testSuite := range args.CFTTestRunnerRequest.TestSuites {
		testCases := testSuite.GetTestCaseIds().GetTestCaseIds()
		if len(testCases) > 1 {
			testSuite.GetTestCaseIds().TestCaseIds = rm.filterTestsWithoutVerdictPassed(testCases)
		}
		testSuites = append(testSuites, testSuite)
	}
	args.CFTTestRunnerRequest.TestSuites = testSuites

	return nil
}

// filterTestsWithoutVerdictPassed removes any test case that exists inside the previous test case
// results and contains a verdict of passed.
func (rm *RetryModifier) filterTestsWithoutVerdictPassed(testCases []*testapi.TestCase_Id) []*testapi.TestCase_Id {
	testCaseIds := []*testapi.TestCase_Id{}
	for _, testCase := range testCases {
		if verdict, exists := rm.PrevTestCaseResults[testCase.Value]; exists && verdict == test_platform.TaskState_VERDICT_PASSED {
			continue
		}
		testCaseIds = append(testCaseIds, testCase)
	}
	return testCaseIds
}
