// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package validateconfig

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/chromiumos/infra/proto/go/lab_platform"
)

var testInspectBufferData = []struct {
	name string
	in   string
	out  string
}{
	{
		"len zero string",
		"",
		"file unexpectedly has length zero",
	},
	{
		"not UTF-8",
		"\xee\xee\xee\xff",
		"file is not valid UTF-8",
	},
	{
		"invalid JSON",
		"aaaa",
		"file is not valid JSON",
	},
	{
		"doesn't fit schema",
		"[2, 3, 4]",
		"JSON does not conform to schema",
	},
	{
		"well-formed but empty",
		"{}",
		"file has no 'versions' entries",
	},
	{
		"not lowed name",
		`{
			"versions": [{
				"target": {
					"deviceType": "cros",
					"board": "naMi",
					"model": "nonexistent-model"
				},
				"osVersion": "R81-12835.0.0",
				"osImagePath": "nami-release/R81-12835.0.0"
			}]
		}`,
		`validate target: board "naMi" is not lowercase`,
	},
}

func TestInspectBuffer(t *testing.T) {
	t.Parallel()
	for _, tt := range testInspectBufferData {
		t.Run(tt.name, func(t *testing.T) {
			_, e := InspectBuffer([]byte(tt.in))
			if tt.out == "" && e != nil {
				t.Errorf("TestInspectBuffer (%s): unexpected error: %s", tt.name, e)
			} else if tt.out == "" && e == nil {
				// Evetything good.
			} else if tt.out != "" && e != nil && !strings.Contains(e.Error(), tt.out) {
				t.Errorf("TestInspectBuffer (%s): got: (%q), want: (%q)", tt.name, e.Error(), tt.out)
			}
		})
	}
}

var testIsValidJSONData = []struct {
	name string
	in   string
	out  bool
}{
	{
		"empty",
		"{}",
		true,
	},
	{
		"not full",
		"{",
		false,
	},
}

func TestIsValidJSON(t *testing.T) {
	t.Parallel()
	for _, tt := range testIsValidJSONData {
		t.Run(tt.name, func(t *testing.T) {
			if res := isValidJSON([]byte(tt.in)); res != tt.out {
				t.Errorf("TestIsValidJSON (%s): got: (%v), want: (%v)", tt.name, res, tt.out)
			}
		})
	}
}

var testShallowValidateVersionsData = []struct {
	name   string
	in     []*lab_platform.StableVersion
	errMsg string
}{
	{
		"happy path",
		[]*lab_platform.StableVersion{
			{
				Target: &lab_platform.StableVersionTarget{
					DeviceType: "cros",
					Board:      "board1",
					Model:      "model1"},
				OsVersion:   "R81-12835.0.0",
				OsImagePath: "model1-release/R81-12835.0.0",
			},
		},
		"",
	},
	{
		"duplicate entry",
		[]*lab_platform.StableVersion{
			{
				Target: &lab_platform.StableVersionTarget{
					DeviceType: "cros",
					Board:      "board1",
					Model:      "model1"},
				OsVersion:   "R81-12835.0.1",
				OsImagePath: "model1-release/R81-12835.0.1",
			},
			{
				Target: &lab_platform.StableVersionTarget{
					DeviceType: "cros",
					Board:      "board1",
					Model:      "model1"},
				OsVersion:   "R81-12835.0.4",
				OsImagePath: "model1-release/R81-12835.0.4",
			},
		},
		fmt.Sprintf(fileShallowlyDuplicateEntry, 1, "devicetype=cros;board=board1;model=model1"),
	},
}

func TestShallowValidateVersions(t *testing.T) {
	for _, tt := range testShallowValidateVersionsData {
		t.Run(tt.name, func(t *testing.T) {
			err := shallowValidateVersions(tt.in)
			if tt.errMsg == "" && err != nil {
				t.Errorf("TestShallowValidateVersions %s: unexpected error: %s", tt.name, err)
			} else if tt.errMsg != "" && err != nil {
				if diff := cmp.Diff(tt.errMsg, err.Error()); diff != "" {
					t.Errorf("TestShallowValidateVersions %s: diff: %s", tt.name, diff)
				}
			} else if tt.errMsg != "" && err == nil {
				t.Errorf("TestShallowValidateVersions %s: error expected but got none", tt.name)
			}
		})
	}
}

func TestIsLowercase(t *testing.T) {
	cases := []struct {
		in  string
		out bool
	}{
		{
			"",
			true,
		},
		{
			"a",
			true,
		},
		{
			"A",
			false,
		},
		{
			"aA",
			false,
		},
	}

	t.Parallel()

	for _, tt := range cases {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if isLowercase(tt.in) != tt.out {
				t.Errorf("isLowercase(%s) is unexpectedly %v", tt.in, tt.out)
			}
		})
	}
}
