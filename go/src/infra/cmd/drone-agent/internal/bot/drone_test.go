// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bot

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestAbbreviateWord tests abbreviating a dash-delimited segment of a hostname.
func TestAbbreviateWord(t *testing.T) {
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
			name: "happy path",
			in:   "chromeos256",
			out:  "c256",
		},
		{
			name: "almost number",
			in:   "14a",
			out:  "14a",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.out
			actual := abbreviateWord(tt.in)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("unexpected diff in subtest %q: %s", tt.name, diff)
			}
		})
	}
}

// TestAbbreviate tests we abbreviate hostnames containing dashes in the expected way and truncate strings.
func TestAbbreviate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		n    int
		out  string
	}{
		{
			name: "empty string",
			in:   "",
			n:    37,
			out:  "",
		},
		{
			name: "happy path with 2 segments",
			in:   "chromeos256-column8888",
			n:    37,
			out:  "c256-c8888",
		},
		{
			name: "happy path with no dashes",
			in:   "eeeeeeee",
			n:    37,
			out:  "eeeeeeee",
		},
		{
			name: "happy path with truncation",
			in:   "aa4-bb3-cc2-dd1",
			n:    5,
			out:  "a4-b3",
		},
		{
			name: "truncate",
			in:   "abcdef",
			n:    2,
			out:  "ab",
		},
		{
			name: "edge case: dash seperated numbers",
			in:   "1-22-3-4",
			n:    200,
			out:  "1-22-3-4",
		},
		{
			name: "edge case: pseudonumber and non-pseudonumber",
			in:   "e1-a14-b18a-c17zz",
			n:    200,
			out:  "e1-a14-b18a-c",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.out
			actual := abbreviate(tt.in, tt.n)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("unexpected diff in subtest %q: %s", tt.name, diff)
			}
		})
	}
}

// TestTruncate tests that the helper function getSuffix returns a suffix of length at most n
// when n is positive.
func TestTruncate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		n    int
		out  string
	}{
		{
			name: "empty string",
			in:   "",
			n:    4,
			out:  "",
		},
		{
			name: "non-positive n",
			in:   "aaa",
			n:    0,
			out:  "aaa",
		},
		{
			name: "strictly negative n",
			in:   "aaa",
			n:    -1,
			out:  "aaa",
		},
		{
			name: "n longer than string",
			in:   "aaa",
			n:    7000,
			out:  "aaa",
		},
		{
			name: "happy path",
			in:   "abcdef",
			n:    2,
			out:  "ab",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.out
			actual := truncate(tt.in, tt.n)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("unexpected diff in subtest %q: %s", tt.name, diff)
			}
		})
	}
}
