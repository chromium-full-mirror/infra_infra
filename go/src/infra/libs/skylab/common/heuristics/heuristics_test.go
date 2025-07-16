// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package heuristics

import (
	"fmt"
	"strings"
	"testing"
	"testing/quick"

	"github.com/google/go-cmp/cmp"
)

// TestLooksLikeSatlab tests the looks-like-satlab heuristic.
func TestLooksLikeSatlab(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		out  bool
	}{
		{
			name: "empty string",
			in:   "",
			out:  false,
		},
		{
			name: "satlab device",
			in:   "satlab-0XXXXXXXXX-host1",
			out:  true,
		},
		{
			name: "satlab infix is not valid",
			in:   "some-prefix-satlab-0XXXXXXXXX-host1",
			out:  false,
		},
		{
			name: "crossk prefix should be ignored",
			in:   "crossk-satlab-0XXXXXXXXX-host1",
			out:  true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.out
			actual := LooksLikeSatlabDevice(tt.in)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

// TestLooksLikeValidPool tests whether strings are correctly identified as being valid pools.
func TestLooksLikeValidPool(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		out  bool
	}{
		{
			name: "empty string",
			in:   "",
			out:  false,
		},
		{
			name: "has [",
			in:   "a[",
			out:  false,
		},
		{
			name: "valid identifier",
			in:   "valid_identifier4",
			out:  true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.out
			actual := LooksLikeValidPool(tt.in)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

// TestNormalizeTextualData tests valid and invalid complete IDs.
func TestNormalizeTextualData(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "empty string",
			in:   "",
			out:  "",
		},
		{
			name: "whitespace",
			in:   " ",
			out:  "",
		},
		{
			name: "mixed case data",
			in:   "Aa",
			out:  "aa",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.out
			actual := NormalizeTextualData(tt.in)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

// TestLooksLikeFieldMask checks whether strings look like field masks or not.
func TestLooksLikeFieldMask(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		out  bool
	}{
		{
			name: "empty",
			in:   "",
			out:  false,
		},
		{
			name: "a",
			in:   "a",
			out:  true,
		},
		{
			name: "A",
			in:   "A",
			out:  false,
		},
		{
			name: "number",
			in:   "3",
			out:  false,
		},
		{
			name: "attherate",
			in:   "invalid_field_mask@",
			out:  false,
		},
		{
			name: "a6",
			in:   "a6",
			out:  true,
		},
		{
			name: "a6E8",
			in:   "a6E8",
			out:  true,
		},
		{
			name: "a6E8_aaaa",
			in:   "a6E8_aaaa",
			out:  true,
		},
		{
			name: "a6E8.aaaa",
			in:   "a6E8.aaaa",
			out:  true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.out
			actual := LooksLikeFieldMask(tt.in)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

var testNormalizeBotNameToDeviceNameData = []struct {
	startingHostname, wantCorrectedHostname string
}{
	{
		"crossk-foo-hostname.cros",
		"foo-hostname",
	},
	{
		"crossk-bar-hostname",
		"bar-hostname",
	},
	{
		"cros-chromeos1-bar-hostname",
		"chromeos1-bar-hostname",
	},
	{
		"baz-hostname.cros",
		"baz-hostname",
	},
	{
		"lol-hostname",
		"lol-hostname",
	},
}

func TestNormalizeBotNameToDeviceName(t *testing.T) {
	t.Parallel()
	for _, tt := range testNormalizeBotNameToDeviceNameData {
		t.Run(fmt.Sprintf("(%s)", tt.startingHostname), func(t *testing.T) {
			t.Parallel()
			gotCorrectedHostname := NormalizeBotNameToDeviceName(tt.startingHostname)
			if tt.wantCorrectedHostname != gotCorrectedHostname {
				t.Errorf("unexpected error: wanted '%s', got '%s'", tt.wantCorrectedHostname, gotCorrectedHostname)
			}
		})
	}
}

func TestRuncateErrorStringSmokeTest(t *testing.T) {
	t.Parallel()

	const truncatedString = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA...AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	cases := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "empty string",
			input:  "",
			output: "",
		},
		{
			name:   "singleton string",
			input:  "A",
			output: "A",
		},
		{
			name:   "singleton string",
			input:  strings.Repeat("A", 1400),
			output: truncatedString,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.output
			actual := TruncateErrorString(tt.input)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

// TestTruncateErrorStringAlwaysShort tests that the string in question always has a length shorter than 1400 characters.
func TestTruncateErrorStringAlwaysShort(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"A",
		strings.Repeat("A", 100),
		strings.Repeat("A", 1000),
		strings.Repeat("A", 10000),
	}

	for _, tt := range cases {
		t.Run(fmt.Sprintf("prefix of length %d", len(tt)), func(t *testing.T) {
			t.Parallel()

			hasRightLength := func(msg string) bool {
				return len(TruncateErrorString(msg)) < 1400
			}

			if err := quick.Check(hasRightLength, nil); err != nil {
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}

// TestTruncateErrorStringIsIdempotent tests that TruncateErrorString is idempotent.
// This high-level property checks that the logic for shortening the string is correct and checks that utf-8 sanitization logic is also correct.
func TestTruncateErrorStringIsIdempotent(t *testing.T) {
	t.Parallel()

	once := func(msg []byte) string {
		return TruncateErrorString(string(msg))
	}

	twice := func(msg []byte) string {
		return TruncateErrorString(TruncateErrorString(string(msg)))
	}

	if err := quick.CheckEqual(once, twice, nil); err != nil {
		t.Errorf("unexpected error: %s", err)
	}

}

// TestNormalizeServoNameToDeviceName tests normalizing a servo name
// to a device name.
func TestNormalizeServoNameToDeviceName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "empty string",
			input:  "",
			output: "",
		},
		{
			name:   "basic string",
			input:  "some-long-dut-name-servo",
			output: "some-long-dut-name",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.output
			actual := NormalizeServoNameToDeviceName(tt.input)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}
