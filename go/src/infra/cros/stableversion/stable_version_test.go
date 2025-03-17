// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"testing"

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

// TestValidateFwPathVersion tests parsing specific fw path.
func TestValidateFwPathVersion(t *testing.T) {
	cases := map[string]bool{
		"":                                 false,
		"R1-2.3.4":                         false,
		"a-firmware/R1-2.3.4":              true,
		"octopus-firmware/R72-11297.75.0":  true,
		"octopus-release/R72-11297.75.0":   true,
		"octopus-something/R72-11297.75.0": true,
		"Google_Rammus.11275.41.0":         false,
	}
	t.Parallel()
	for in, good := range cases {
		t.Run(in, func(t *testing.T) {
			err := ValidateFirmwarePath(in)
			if good && err != nil {
				t.Errorf("TestValidateFwPathVersion %s: fail: %s", in, err)
			}
			if !good && err == nil {
				t.Errorf("TestValidateFwPathVersion %s: expected to fail but succeeded", in)
			}
		})
	}
}

func TestParseFirmwareVersion(t *testing.T) {
	cases := map[string]string{
		"Google_Rammus.11275.41.0":                 "11275.41.0",
		"Google_Something.19999.0.2018_01_06_3333": "19999.0.2018_01_06_3333",
		"Google.11297.75.0":                        "11297.75.0",
	}
	t.Parallel()
	for in, out := range cases {
		t.Run(in, func(t *testing.T) {
			version, err := ParseFirmwareVersion(in)
			if err != nil {
				t.Errorf("TestParseFirmwareVersion %s: fail: %s", in, err)
			} else {
				assert.Loosely(t, version, should.Equal(out))
			}
		})
	}
}

func TestWriteSVToString(t *testing.T) {
	cases := []struct {
		name string
		in   []versions
		out  string
	}{
		{
			"single",
			[]versions{{"b1", "m1", "R1-1.1.1", "R1-1.1.1", "a-firmware/R1-1.1.1"}},
			`{
	"versions": [
		{
			"target": {
				"deviceType": "cros",
				"board": "b1",
				"model": "m1"
			},
			"osVersion": "R1-1.1.1",
			"firmwareRoVersion": "R1-1.1.1",
			"firmwareRoImagePath": "a-firmware/R1-1.1.1"
		}
	]
}`,
		},
		{
			"double",
			[]versions{
				{"b1", "m1", "R1-1.1.1", "R1-1.1.1", "a-firmware/R1-1.1.1"},
				{"b2", "m2", "R2-2.2.2", "R2-2.2.2", "a-firmware/R2-2.2.2"},
			},
			`{
	"versions": [
		{
			"target": {
				"deviceType": "cros",
				"board": "b1",
				"model": "m1"
			},
			"osVersion": "R1-1.1.1",
			"firmwareRoVersion": "R1-1.1.1",
			"firmwareRoImagePath": "a-firmware/R1-1.1.1"
		},
		{
			"target": {
				"deviceType": "cros",
				"board": "b2",
				"model": "m2"
			},
			"osVersion": "R2-2.2.2",
			"firmwareRoVersion": "R2-2.2.2",
			"firmwareRoImagePath": "a-firmware/R2-2.2.2"
		}
	]
}`,
		},
	}
	t.Parallel()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := WriteSVToString(makeBaseStableVersions(c.in))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s, should.Equal(c.out))
		})
	}
}

type versions struct {
	bt             string
	m              string
	os, fw, fwPath string
}

func makeBaseStableVersions(vs []versions) *sv.StableVersions {
	r := &sv.StableVersions{}
	for _, c := range vs {
		r.Versions = append(r.Versions, &sv.StableVersion{
			Target: &sv.StableVersionTarget{
				DeviceType: "cros",
				Board:      c.bt,
				Model:      c.m,
				Pool:       "",
			},
			OsVersion:           c.os,
			OsImagePath:         "",
			FirmwareRoVersion:   c.fw,
			FirmwareRoImagePath: c.fwPath,
		})
	}
	return r
}
