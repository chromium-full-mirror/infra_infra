// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testexec runs tests.
package testexec

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
)

func TestPrepareTestResponse_NilTestCaseResults(t *testing.T) {
	resultRootDir := "/path/to/results"

	testCaseResult := &api.TestCaseResult{
		TestCaseId:    &api.TestCase_Id{Value: "foo"},
		ResultDirPath: nil,
		Verdict:       &api.TestCaseResult_NotRun_{},
	}
	testCaseResults := []*api.TestCaseResult{testCaseResult}

	res, err := prepareTestResponse(resultRootDir, testCaseResults)

	// Verify that the returned response is not nil.
	if res == nil {
		t.Fatalf("Expected non-nil response, got nil")
	}

	// Verify that the TestCaseResults has the testCase.
	if len(res.TestCaseResults) != 1 {
		t.Errorf("Expected empty TestCaseResults, got %d elements", len(res.TestCaseResults))
	}
	if res.TestCaseResults[0].TestCaseId.Value != "foo" {
		t.Errorf("Expected TestCase 'foo' got: %s", res.TestCaseResults[0].TestCaseId.Value)
	}

	// Verify that the returned error is nil.
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}
