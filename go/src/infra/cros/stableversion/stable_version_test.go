// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	"go.chromium.org/chromiumos/infra/proto/go/device"
	sv "go.chromium.org/chromiumos/infra/proto/go/lab_platform"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

// TODO(gregorynisbet): replace with table-driven test
func TestCompareCrOSVersions(t *testing.T) {
	ftt.Run("Test v1 > v2", t, func(t *ftt.Test) {
		v1 := "R2-2.3.4"
		v2 := "R1-2.3.4"
		cv, err := CompareCrOSVersions(v1, v2)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cv, should.Equal(1))

		v1 = "R1-2.5.4"
		v2 = "R1-2.3.4"
		cv, err = CompareCrOSVersions(v1, v2)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cv, should.Equal(1))
	})
	ftt.Run("Test v1 < v2", t, func(t *ftt.Test) {
		v1 := "R2-1.3.4"
		v2 := "R2-2.3.4"
		cv, err := CompareCrOSVersions(v1, v2)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cv, should.Equal(-1))

		v1 = "R1-2.3.4"
		v2 = "R1-2.3.5"
		cv, err = CompareCrOSVersions(v1, v2)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cv, should.Equal(-1))
	})
	ftt.Run("Test v1 == v2", t, func(t *ftt.Test) {
		v1 := "R1-2.3.4"
		v2 := "R1-2.3.4"
		cv, err := CompareCrOSVersions(v1, v2)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cv, should.BeZero)
	})
}

// TODO(gregorynisbet): replace with table-driven test
func TestValidateCrOSVersion(t *testing.T) {
	good := func(s string) {
		if err := ValidateCrOSVersion(s); err != nil {
			t.Errorf("expected `%s' to be good (%s)", s, err)
		}
	}
	bad := func(s string) {
		if ValidateCrOSVersion(s) == nil {
			t.Errorf("expected `%s' to be bad", s)
		}
	}
	bad("")
	good("R1-2.3.4")
	bad("a-firmware/R1-2.3.4")
	bad("octopus-firmware/R72-11297.75.0")
	bad("Google_Rammus.11275.41.0")
}

// TODO(gregorynisbet): replace with table-driven test
func TestSerializeCrOSVersion(t *testing.T) {
	out := SerializeCrOSVersion(1, 2, 3, 4)
	if out != "R1-2.3.4" {
		t.Errorf("expected: R1-2.3.4 got:%s", out)
	}
}

// TODO(gregorynisbet): replace with table-driven test
func TestParseCrOSVersion(t *testing.T) {
	ftt.Run("Test parsing CrOS Version", t, func(t *ftt.Test) {
		release, tip, branch, branchBranch, err := ParseCrOSVersion("R1-2.3.4")
		if err != nil {
			t.Errorf("expected R1-2.3.4 to parse: %s", err)
		} else {
			assert.Loosely(t, release, should.Equal(1))
			assert.Loosely(t, tip, should.Equal(2))
			assert.Loosely(t, branch, should.Equal(3))
			assert.Loosely(t, branchBranch, should.Equal(4))
		}
	})
}

// TestParseFaftVersion tests parsing specific FAFT versions.
func TestValidateFaftVersion(t *testing.T) {
	good := func(s string) {
		if err := ValidateFaftVersion(s); err != nil {
			t.Errorf("expected `%s' to be good (%s)", s, err)
		}
	}
	bad := func(s string) {
		if ValidateFaftVersion(s) == nil {
			t.Errorf("expected `%s' to be bad", s)
		}
	}
	bad("")
	bad("R1-2.3.4")
	good("a-firmware/R1-2.3.4")
	good("octopus-firmware/R72-11297.75.0")
	good("octopus-release/R72-11297.75.0")
	bad("octopus-something/R72-11297.75.0")
	bad("Google_Rammus.11275.41.0")
}

// TestParseNewFaftPrefix tests parsing a new FAFT prefix.
func TestParseNewFaftPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input  string
		output map[string]string
	}{
		{
			input:  "",
			output: nil,
		},
		{
			input: "firmware-a-99.B-branch-firmware",
			output: map[string]string{
				"builder":   "a",
				"tip":       "99",
				"tipSuffix": "B",
			},
		},
		{
			input: "firmware-a-99-branch-firmware",
			output: map[string]string{
				"builder":   "a",
				"tip":       "99",
				"tipSuffix": "",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			out, _ := parseNewFaftPrefix(tt.input)
			if diff := cmp.Diff(tt.output, out); diff != "" {
				t.Errorf("-want +got: %s", diff)
			}
		})
	}
}

// TestParseNewFaftSuffix tests parsing a new FAFT suffix.
func TestParseNewFaftSuffix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input  string
		output map[string]string
	}{
		{
			input: "R99-44.33.22",
			output: map[string]string{
				"release":      "R99",
				"tip":          "44",
				"branch":       "33",
				"branchbranch": "22",
				"board":        "",
			},
		},
		{
			input: "R99-44.33.22/octopus",
			output: map[string]string{
				"release":      "R99",
				"tip":          "44",
				"branch":       "33",
				"branchbranch": "22",
				"board":        "octopus",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			out, _ := parseNewFaftSuffix(tt.input)
			if diff := cmp.Diff(tt.output, out); diff != "" {
				t.Errorf("-want +got: %s", diff)
			}
		})
	}
}

// TestParseFaftVersion tests parsing a faft version.
func TestParseFaftVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		out   *FaftVersionResult
		ok    bool
	}{
		{
			input: "a-firmware/R1-2.3.4",
			out: &FaftVersionResult{
				Platform:     "a",
				Kind:         "firmware",
				Release:      1,
				Tip:          2,
				Branch:       3,
				BranchBranch: 4,
			},
			ok: true,
		},
		{
			input: "firmware-a-42.B-branch-firmware/R99-42.43.44",
			out: &FaftVersionResult{
				Platform:     "a",
				Kind:         "firmware",
				Release:      99,
				Tip:          42,
				Branch:       43,
				BranchBranch: 44,
			},
			ok: true,
		},
		{
			input: "firmware-a-42.B-branch-firmware/R99-42.43.44/a",
			out: &FaftVersionResult{
				Platform:     "a",
				Kind:         "firmware",
				Release:      99,
				Tip:          42,
				Branch:       43,
				BranchBranch: 44,
			},
			ok: true,
		},
		{
			input: "firmware-something-42.B-branch-firmware/R99-42.43.44/a",
			out: &FaftVersionResult{
				Platform:     "a",
				Kind:         "firmware",
				Release:      99,
				Tip:          42,
				Branch:       43,
				BranchBranch: 44,
			},
			ok: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			out, err := ParseFaftVersion(tt.input)
			if diff := cmp.Diff(tt.out, out); diff != "" {
				t.Errorf("-want +got: %s", diff)
			}
			if diff := cmp.Diff(tt.ok, (err == nil)); diff != "" {
				t.Errorf("-want +got: %s %s", diff, err)
			}
		})
	}
}

// TODO(gregorynisbet): replace with table-driven test
func TestValidateFirmwareVersion(t *testing.T) {
	good := func(s string) {
		if err := ValidateFirmwareVersion(s); err != nil {
			t.Errorf("expected `%s' to be good (%s)", s, err)
		}
	}
	bad := func(s string) {
		if ValidateFirmwareVersion(s) == nil {
			t.Errorf("expected `%s' to be bad", s)
		}
	}
	bad("")
	bad("R1-2.3.4")
	bad("a-firmware/R1-2.3.4")
	bad("octopus-firmware/R72-11297.75.0")
	good("Google_Rammus.11275.41.0")
	good("Google_Something.19999.0.2018_01_06_3333")
}

// TODO(gregorynisbet): replace with table-driven test
func TestSerializeFirmwareVersion(t *testing.T) {
	out := SerializeFirmwareVersion("Google", "Rammus", 11275, 41, 0)
	if out != "Google_Rammus.11275.41.0" {
		t.Errorf("expected: R1-2.3.4 got:%s", out)
	}
}

// TODO(gregorynisbet): replace with table-driven test
func TestParseFirmwareVersion(t *testing.T) {
	ftt.Run("Test Parsing RW Faft Version", t, func(t *ftt.Test) {
		company, platform, tip, branch, branchBranch, err := ParseFirmwareVersion("Google_Rammus.11275.41.0")
		if err != nil {
			t.Errorf("expected Google_Rammus.11275.41.0 to parse: %s", err)
		} else {
			assert.Loosely(t, company, should.Equal("Google"))
			assert.Loosely(t, platform, should.Equal("Rammus"))
			assert.Loosely(t, tip, should.Equal(11275))
			assert.Loosely(t, branch, should.Equal(41))
			assert.Loosely(t, branchBranch, should.Equal("0"))
		}
	})
}

// TODO(gregorynisbet): replace with table-driven test
func TestAddUpdatedCros(t *testing.T) {
	old := makeBaseStableVersions(
		[]versions{
			{"b1", "m1", "R1-1.1.1"},
			{"b2", "m2", "R2-2.2.2"},
		},
		nil,
		nil,
	)
	updated := makeBaseStableVersions(
		[]versions{
			{"b1", "m1", "R1-1.1.1111"},
			{"b3", "m3", "R3-3.3.3"},
		},
		nil,
		nil,
	)
	res := AddUpdatedCros(old.Cros, updated.Cros)
	m := make(map[string]string, len(res))
	for _, r := range res {
		m[crosSVKey(r)] = r.GetVersion()
	}

	ftt.Run("Test add", t, func(t *ftt.Test) {
		assert.Loosely(t, m["b3"], should.Equal("R3-3.3.3"))
	})

	ftt.Run("Test update", t, func(t *ftt.Test) {
		assert.Loosely(t, m["b1"], should.Equal("R1-1.1.1111"))
	})

	ftt.Run("Test reserve", t, func(t *ftt.Test) {
		assert.Loosely(t, m["b2"], should.Equal("R2-2.2.2"))
	})
}

// TODO(gregorynisbet): replace with table-driven test
func TestAddUpdatedFirmware(t *testing.T) {
	old := makeBaseStableVersions(
		nil,
		nil,
		[]versions{
			{"b1", "m1", "a-firmware/R1-1.1.1"},
			{"b2", "m2", "a-firmware/R2-2.2.2"},
		},
	)
	updated := makeBaseStableVersions(
		nil,
		nil,
		[]versions{
			{"b1", "m1", "a-firmware/R1-1.1.1111"},
			{"b3", "m3", "a-firmware/R3-3.3.3"},
		},
	)
	res := AddUpdatedFirmware(old.Firmware, updated.Firmware)
	m := make(map[string]string, len(res))
	for _, r := range res {
		m[firmwareSVKey(r)] = r.GetVersion()
	}

	ftt.Run("Test add", t, func(t *ftt.Test) {
		assert.Loosely(t, m["b3:m3"], should.Equal("a-firmware/R3-3.3.3"))
	})

	ftt.Run("Test update", t, func(t *ftt.Test) {
		assert.Loosely(t, m["b1:m1"], should.Equal("a-firmware/R1-1.1.1111"))
	})

	ftt.Run("Test reserve", t, func(t *ftt.Test) {
		assert.Loosely(t, m["b2:m2"], should.Equal("a-firmware/R2-2.2.2"))
	})
}

// TODO(gregorynisbet): replace with table-driven test
func TestWriteSVToString(t *testing.T) {
	ftt.Run("Test order of stable versions after writing to strings", t, func(t *ftt.Test) {
		all := makeBaseStableVersions(
			[]versions{
				{"b1", "m1", "R1-1.1.1"},
				{"b2", "m2", "R2-2.2.2"},
			},
			[]versions{
				{"b1", "m1", "a-firmware/R1-1.1.1"},
				{"b1", "m2", "a-firmware/R2-2.2.2"},
			},
			[]versions{
				{"b1", "m2", "b-firmware/R1-1.1.1"},
				{"a1", "m1", "a-firmware/R1-1.1.1"},
			},
		)
		source :=
			`{
	"cros": [
		{
			"key": {
				"modelId": {
					"value": "m1"
				},
				"buildTarget": {
					"name": "b1"
				}
			},
			"version": "R1-1.1.1"
		},
		{
			"key": {
				"modelId": {
					"value": "m2"
				},
				"buildTarget": {
					"name": "b2"
				}
			},
			"version": "R2-2.2.2"
		}
	],
	"faft": [
		{
			"key": {
				"modelId": {
					"value": "m1"
				},
				"buildTarget": {
					"name": "b1"
				}
			},
			"version": "a-firmware/R1-1.1.1"
		},
		{
			"key": {
				"modelId": {
					"value": "m2"
				},
				"buildTarget": {
					"name": "b1"
				}
			},
			"version": "a-firmware/R2-2.2.2"
		}
	],
	"firmware": [
		{
			"key": {
				"modelId": {
					"value": "m1"
				},
				"buildTarget": {
					"name": "a1"
				}
			},
			"version": "a-firmware/R1-1.1.1"
		},
		{
			"key": {
				"modelId": {
					"value": "m2"
				},
				"buildTarget": {
					"name": "b1"
				}
			},
			"version": "b-firmware/R1-1.1.1"
		}
	]
}`
		s, err := WriteSVToString(all)
		diff := cmp.Diff(s, source)
		fmt.Printf("17f08250-8616-4063-b748-8a161b5c7489 (%s)\n", diff)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, s, should.Equal(source))
	})
}

type versions struct {
	bt string
	m  string
	v  string
}

func makeBaseStableVersions(cros, faft, firmware []versions) *sv.StableVersions {
	var cs []*sv.StableCrosVersion
	for _, c := range cros {
		cs = append(cs, &sv.StableCrosVersion{
			Key:     makeStableVersionKey(c.bt, c.m),
			Version: c.v,
		})
	}
	var fis []*sv.StableFirmwareVersion
	for _, c := range firmware {
		fis = append(fis, &sv.StableFirmwareVersion{
			Key:     makeStableVersionKey(c.bt, c.m),
			Version: c.v,
		})
	}
	var fas []*sv.StableFaftVersion
	for _, c := range faft {
		fas = append(fas, &sv.StableFaftVersion{
			Key:     makeStableVersionKey(c.bt, c.m),
			Version: c.v,
		})
	}
	return &sv.StableVersions{
		Cros:     cs,
		Firmware: fis,
		Faft:     fas,
	}
}

func makeStableVersionKey(buildTarget, model string) *sv.StableVersionKey {
	return &sv.StableVersionKey{
		ModelId: &device.ModelId{
			Value: model,
		},
		BuildTarget: &chromiumos.BuildTarget{
			Name: buildTarget,
		},
	}
}

func TestJoinBuildTargetModel(t *testing.T) {
	ftt.Run("test joining buildTarget and model", t, func(t *ftt.Test) {
		t.Run("non-empty strings good", func(t *ftt.Test) {
			s, err := JoinBuildTargetModel("a", "m")
			assert.Loosely(t, s, should.Equal("a;m"))
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("non-empty strings with upper case good", func(t *ftt.Test) {
			s, err := JoinBuildTargetModel("Aaa", "Mmm")
			assert.Loosely(t, s, should.Equal("aaa;mmm"))
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("empty string bad", func(t *ftt.Test) {
			s, err := JoinBuildTargetModel("", "m")
			assert.Loosely(t, s, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}
