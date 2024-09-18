// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"testing"
)

func TestUpdateTestCases_extractBuildNumberFromResponse(t *testing.T) {
	jsonResponse := `{
		"nextPageToken": "MCwxNzI2MTI1MzQzMTI3LCJicnlhLXRydW5rX3N0YWdpbmctdXNlcmRlYnVnIiwxLCIwMDAwMDAwMDAwMDAxMjM1ODIyNSIsImJyeWEtdHJ1bmtfc3RhZ2luZy11c2VyZGVidWci",
		"builds": [
			{
				"buildId": "12358225",
				"target": {}
			}
		]
	}`
	buildNumber, err := extractBuildNumberFromResponse(jsonResponse)
	if err != nil {
		t.Errorf("extractBuildNumberFromResponse() error = %v", err)
	}
	if buildNumber != 12358225 {
		t.Errorf("extractBuildNumberFromResponse() = %v, want %v", buildNumber, "12358225")
	}
}
