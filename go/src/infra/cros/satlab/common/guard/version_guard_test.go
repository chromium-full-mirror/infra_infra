// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package guard

import (
	"testing"
)

func TestShouldVerifyVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Contains beta-channel",
			input:    "beta-channel",
			expected: true,
		},
		{
			name:     "Contains stable-channel",
			input:    "stable-channel",
			expected: true,
		},
		{
			name:     "Contains beta-channel with other text",
			input:    "some-prefix-beta-channel-some-suffix",
			expected: true,
		},
		{
			name:     "Contains stable-channel with other text",
			input:    "some-prefix-stable-channel-some-suffix",
			expected: true,
		},
		{
			name:     "Does not contain beta or stable - dev channel",
			input:    "dev-channel",
			expected: false,
		},
		{
			name:     "Does not contain beta or stable - canary channel",
			input:    "canary-channel",
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "Partial match - beta",
			input:    "beta",
			expected: false,
		},
		{
			name:     "Case sensitive - Beta-channel",
			input:    "Beta-channel",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual := shouldVerifyVersion(tc.input)
			if actual != tc.expected {
				t.Errorf("shouldVerifyVersion(%q) = %t; want %t", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		input         string
		expected      SatlabVersion
		expectError   bool
		expectedError string
	}{
		{
			name:        "Valid version",
			input:       "R-1.2.3",
			expected:    SatlabVersion{Major: 1, Minor: 2, Patch: 3},
			expectError: false,
		},
		{
			name:          "Missing R- prefix",
			input:         "1.2.3",
			expectError:   true,
			expectedError: "invalid version format: missing 'R-' prefix in 1.2.3",
		},
		{
			name:          "Incorrect prefix",
			input:         "S-1.2.3",
			expectError:   true,
			expectedError: "invalid version format: missing 'R-' prefix in S-1.2.3",
		},
		{
			name:          "Too few parts",
			input:         "R-1.2",
			expectError:   true,
			expectedError: "invalid version format: expected X.Y.Z, got 1.2",
		},
		{
			name:          "Too many parts",
			input:         "R-1.2.3.4",
			expectError:   true,
			expectedError: "invalid version format: expected X.Y.Z, got 1.2.3.4",
		},
		{
			name:          "Non-numeric major",
			input:         "R-a.2.3",
			expectError:   true,
			expectedError: "invalid major version: a",
		},
		{
			name:          "Non-numeric minor",
			input:         "R-1.b.3",
			expectError:   true,
			expectedError: "invalid minor version: b",
		},
		{
			name:          "Non-numeric patch",
			input:         "R-1.2.c",
			expectError:   true,
			expectedError: "invalid patch version: c",
		},
		{
			name:          "Empty string",
			input:         "",
			expectError:   true,
			expectedError: "invalid version format: missing 'R-' prefix in ",
		},
		{
			name:          "Only R- prefix",
			input:         "R-",
			expectError:   true,
			expectedError: "invalid version format: expected X.Y.Z, got ",
		},
		{
			name:        "Large version numbers",
			input:       "R-100.200.300",
			expected:    SatlabVersion{Major: 100, Minor: 200, Patch: 300},
			expectError: false,
		},
		{
			name:        "Leading zeros in version numbers",
			input:       "R-01.02.03",
			expected:    SatlabVersion{Major: 1, Minor: 2, Patch: 3},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual, err := parseVersion(tc.input)

			if tc.expectError {
				if err == nil {
					t.Errorf("parseVersion(%q) expected error, but got nil", tc.input)
				} else if err.Error() != tc.expectedError {
					t.Errorf("parseVersion(%q) expected error msg %q, but got %q", tc.input, tc.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("parseVersion(%q) unexpected error: %v", tc.input, err)
				}
				if actual != tc.expected {
					t.Errorf("parseVersion(%q) = %v, want %v", tc.input, actual, tc.expected)
				}
			}
		})
	}
}

func TestIsVersionGreaterOrEqual(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                  string
		requiredSatlabVersion SatlabVersion
		actualSatlabVersion   SatlabVersion
		expected              bool
	}{
		{
			name:                  "Actual Major Greater",
			requiredSatlabVersion: SatlabVersion{Major: 1, Minor: 0, Patch: 0},
			actualSatlabVersion:   SatlabVersion{Major: 2, Minor: 0, Patch: 0},
			expected:              true,
		},
		{
			name:                  "Actual Minor Greater",
			requiredSatlabVersion: SatlabVersion{Major: 1, Minor: 0, Patch: 0},
			actualSatlabVersion:   SatlabVersion{Major: 1, Minor: 1, Patch: 0},
			expected:              true,
		},
		{
			name:                  "Actual Patch Greater",
			requiredSatlabVersion: SatlabVersion{Major: 1, Minor: 0, Patch: 0},
			actualSatlabVersion:   SatlabVersion{Major: 1, Minor: 0, Patch: 1},
			expected:              true,
		},
		{
			name:                  "Versions Equal",
			requiredSatlabVersion: SatlabVersion{Major: 1, Minor: 1, Patch: 1},
			actualSatlabVersion:   SatlabVersion{Major: 1, Minor: 1, Patch: 1},
			expected:              true,
		},
		{
			name:                  "Actual Major Smaller",
			requiredSatlabVersion: SatlabVersion{Major: 2, Minor: 0, Patch: 0},
			actualSatlabVersion:   SatlabVersion{Major: 1, Minor: 0, Patch: 0},
			expected:              false,
		},
		{
			name:                  "Actual Minor Smaller",
			requiredSatlabVersion: SatlabVersion{Major: 1, Minor: 1, Patch: 0},
			actualSatlabVersion:   SatlabVersion{Major: 1, Minor: 0, Patch: 0},
			expected:              false,
		},
		{
			name:                  "Actual Patch Smaller",
			requiredSatlabVersion: SatlabVersion{Major: 1, Minor: 0, Patch: 1},
			actualSatlabVersion:   SatlabVersion{Major: 1, Minor: 0, Patch: 0},
			expected:              false,
		},
		{
			name:                  "Complex: Actual 2.0.0 vs Required 1.5.5",
			requiredSatlabVersion: SatlabVersion{Major: 1, Minor: 5, Patch: 5},
			actualSatlabVersion:   SatlabVersion{Major: 2, Minor: 0, Patch: 0},
			expected:              true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual := IsVersionGreaterOrEqual(tc.requiredSatlabVersion, tc.actualSatlabVersion)
			if actual != tc.expected {
				t.Errorf("IsVersionGreaterOrEqual(%v, %v) = %t; want %t", tc.requiredSatlabVersion, tc.actualSatlabVersion, actual, tc.expected)
			}
		})
	}
}

func TestParseMilestoneNumber(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		input         string
		expected      int
		expectError   bool
		expectedError string
	}{
		{
			name:        "Valid milestone",
			input:       "R123-456.789.012",
			expected:    123,
			expectError: false,
		},
		{
			name:          "Invalid format - missing R prefix",
			input:         "123-456.789.012",
			expectError:   true,
			expectedError: "string does not match expected format: 123-456.789.012",
		},
		{
			name:          "Invalid format - missing hyphen",
			input:         "R123456.789.012",
			expectError:   true,
			expectedError: "string does not match expected format: R123456.789.012",
		},
		{
			name:          "Invalid format - too few parts after hyphen",
			input:         "R123-456.789",
			expectError:   true,
			expectedError: "string does not match expected format: R123-456.789",
		},
		{
			name:          "Invalid format - too many parts after hyphen",
			input:         "R123-456.789.012.345",
			expectError:   true,
			expectedError: "string does not match expected format: R123-456.789.012.345",
		},
		{
			name:          "Non-numeric milestone",
			input:         "RABC-123.456.789",
			expectError:   true,
			expectedError: "string does not match expected format: RABC-123.456.789",
		},
		{
			name:          "Empty string",
			input:         "",
			expectError:   true,
			expectedError: "string does not match expected format: ",
		},
		{
			name:          "Only R",
			input:         "R",
			expectError:   true,
			expectedError: "string does not match expected format: R",
		},
		{
			name:          "R and milestone but incomplete",
			input:         "R123-",
			expectError:   true,
			expectedError: "string does not match expected format: R123-",
		},
		{
			name:          "Milestone is not an integer after R",
			input:         "RXYZ-123.456.789",
			expectError:   true,
			expectedError: "string does not match expected format: RXYZ-123.456.789",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual, err := parseMilestoneNumber(tc.input)

			if tc.expectError {
				if err == nil {
					t.Errorf("parseMilestoneNumber(%q) expected error, but got nil", tc.input)
				} else if err.Error() != tc.expectedError {
					t.Errorf("parseMilestoneNumber(%q) expected error msg %q, but got %q", tc.input, tc.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("parseMilestoneNumber(%q) unexpected error: %v", tc.input, err)
				}
				if actual != tc.expected {
					t.Errorf("parseMilestoneNumber(%q) = %d, want %d", tc.input, actual, tc.expected)
				}
			}
		})
	}
}
