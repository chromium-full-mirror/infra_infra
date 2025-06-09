// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package examples

import "go.chromium.org/chromiumos/config/go/test/api"

var testCasesToRemove = map[string]any{
	"example_test_case_1": nil,
	"example_test_case_2": nil,
}

func RemoveTestCases(req *api.InternalTestplan) {
	remainingTestCases := []*api.CTPTestCase{}
	for _, testCase := range req.GetTestCases() {
		if _, remove := testCasesToRemove[testCase.GetName()]; !remove {
			remainingTestCases = append(remainingTestCases, testCase)
		}
	}
	req.TestCases = remainingTestCases
}
