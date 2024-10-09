// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service provides the API handlers for ants publish.
package service

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
)

func TestAntsStatus(t *testing.T) {
	testCases := []struct {
		name   string
		result *api.TestCaseResult
		want   string
	}{
		{
			name:   "pass",
			result: &api.TestCaseResult{Verdict: &api.TestCaseResult_Pass_{}},
			want:   "pass",
		},
		{
			name:   "fail",
			result: &api.TestCaseResult{Verdict: &api.TestCaseResult_Fail_{}},
			want:   "fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := antsTestStatus(tc.result)
			t.Log(tc.result)
			t.Log(tc.result.GetVerdict())
			if got != tc.want {
				t.Errorf("TestAntsStatus: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestTradefedNames(t *testing.T) {
	testCases := []struct {
		name       string
		testCaseID string
		want       []string
	}{
		{
			name:       "success",
			testCaseID: "tradefed.cts.Module1#Testcase1#Name1",
			want:       []string{"Module1", "Testcase1", "Name1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotModule, gotCase, gotName, err := tradefedNames(tc.testCaseID)
			if err != nil {
				t.Error(err)
			}

			if tc.want[0] != gotModule {
				t.Errorf("Module name different. got: %s want: %s", gotModule, tc.want[0])
			}

			if tc.want[1] != gotCase {
				t.Errorf("Test case name different. got: %s want: %s", gotCase, tc.want[1])
			}

			if tc.want[2] != gotName {
				t.Errorf("Test name different. got: %s want: %s", gotName, tc.want[2])
			}

		})
	}
}
