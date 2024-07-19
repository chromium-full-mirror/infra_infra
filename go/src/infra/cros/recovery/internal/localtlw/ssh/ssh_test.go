// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ssh

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Test cases for TestDUTPlans
var trancateStringCases = []struct {
	name   string
	in     string
	length int
	out    string
}{
	{
		"empty",
		"",
		10,
		"",
	},
	{
		"empty2",
		" ",
		0,
		"...",
	},
	{
		"normal",
		"1234567890",
		4,
		"1234",
	},
	{
		"same length",
		"1234567890",
		10,
		"1234567890",
	},
	{
		"shorter than needed",
		"123456",
		10,
		"123456",
	},
	{
		"with space",
		"12 34567890",
		4,
		"12...",
	},
	{
		"with space",
		`12
34567890`,
		4,
		"12...",
	},
	{
		"with space",
		`12
345
67890`,
		7,
		`12
345...`,
	},
}

// TestTrancateString tests trancateString function.
func TestTrancateString(t *testing.T) {
	t.Parallel()
	for _, c := range trancateStringCases {
		cs := c
		t.Run(cs.name, func(t *testing.T) {
			got := trancateString(c.in, c.length)
			if diff := cmp.Diff(c.out, got); diff != "" {
				t.Errorf("%s:mismatch (-want +got):\n%s", c.name, diff)
			}
		})
	}
}
