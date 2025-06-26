// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package common is the common package.
package common

import (
	"testing"

	"gotest.tools/assert"

	"go.chromium.org/chromiumos/config/go/test/api"
)

func TestConverter(t *testing.T) {
	testCases := []struct {
		name         string
		inputDeps    []*api.TestCase_Dependency
		expectedDeps []*api.TestCase_Dependency
	}{
		{
			name:         "Empty input",
			inputDeps:    []*api.TestCase_Dependency{},
			expectedDeps: []*api.TestCase_Dependency{},
		},
		{
			name: "No conversion needed",
			inputDeps: []*api.TestCase_Dependency{
				{Value: "servo_deployed:true"},
				{Value: "dep2:value"}, // Already in the correct format
			},
			expectedDeps: []*api.TestCase_Dependency{
				{Value: "servo_deployed:true"},
				{Value: "dep2:value"},
			},
		},
		{
			name: "Legacy style conversion",
			inputDeps: []*api.TestCase_Dependency{
				{Value: "suite:foo_test"},
				{Value: "stable-version:12345.0.0"},
			},
			expectedDeps: []*api.TestCase_Dependency{
				{Value: "suite:foo_test"},
				{Value: "stable-version:12345.0.0"}, // The mock skylab functions in the current tests mean this does not get converted.
			},
		},
		{
			name: "Mixed dependencies",
			inputDeps: []*api.TestCase_Dependency{
				{Value: "suite:foo_test"},
				{Value: "some-other-dep"},
				{Value: "cros-version:12345.0.0"},
			},
			expectedDeps: []*api.TestCase_Dependency{
				{Value: "suite:foo_test"},
				{Value: "some-other-dep"},
				{Value: "cros-version:12345.0.0"}, // same again as above.
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualDeps := Converter(tc.inputDeps)

			if len(tc.expectedDeps) != len(actualDeps) {
				// Check if slices have the same length
				t.Fatalf("%s: Expecting: %v, \n\tGot: %v, size does not match!", tc.name, tc.expectedDeps, actualDeps)
			}
			// Compare elements item by item
			for i := range tc.expectedDeps {
				assert.Equal(t, tc.expectedDeps[i].GetValue(), actualDeps[i].GetValue())
			}
		})
	}
}

func TestTfToCTPTestCase(t *testing.T) {
	testData := GenerateTestMetadata()[0]
	converted := tfToCTPTestCase(testData)
	if converted.GetName() != testData.GetTestCase().Id.GetValue() {
		t.Fatalf("misconversion of name. Got %s, expected %s", converted.GetName(), testData.GetTestCase().GetName())
	}
	if converted.Metadata != testData {
		t.Fatalf("Metadata does not match 1 to 1")
	}
}
