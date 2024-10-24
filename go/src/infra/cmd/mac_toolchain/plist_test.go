// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"testing"
)

func TestPlistReading(t *testing.T) {
	t.Parallel()

	ftt.Run("getXcodeVersion works", t, func(t *ftt.Test) {
		t.Run("for valid plist", func(t *ftt.Test) {
			cfbv, xv, bv, err := getXcodeVersion("testdata/version.plist")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, cfbv, should.Equal("12345"))
			assert.Loosely(t, xv, should.Equal("TESTXCODEVERSION"))
			assert.Loosely(t, bv, should.Equal("TESTBUILDVERSION"))
		})
		t.Run("when version is missing", func(t *ftt.Test) {
			_, _, _, err := getXcodeVersion("testdata/badKeys.plist")
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("when version file is broken", func(t *ftt.Test) {
			_, _, _, err := getXcodeVersion("testdata/broken.plist")
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("when version file is missing", func(t *ftt.Test) {
			_, _, _, err := getXcodeVersion("testdata/nonexistent")
			assert.Loosely(t, err, should.NotBeNil)
		})
	})

	ftt.Run("getXcodeLicenseInfo works", t, func(t *ftt.Test) {
		t.Run("for valid plist", func(t *ftt.Test) {
			lid, lt, err := getXcodeLicenseInfo("testdata/licenseInfoGood.plist")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lid, should.Equal("TESTID"))
			assert.Loosely(t, lt, should.Equal("Beta"))
		})
		t.Run("when license keys are missing", func(t *ftt.Test) {
			_, _, err := getXcodeLicenseInfo("testdata/badKeys.plist")
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("when license file is broken", func(t *ftt.Test) {
			_, _, err := getXcodeLicenseInfo("testdata/broken.plist")
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("when license file is missing", func(t *ftt.Test) {
			_, _, err := getXcodeLicenseInfo("testdata/nonexistent")
			assert.Loosely(t, err, should.NotBeNil)
		})
	})

	ftt.Run("getSimulatorVersionInfo works", t, func(t *ftt.Test) {
		t.Run("for valid plist", func(t *ftt.Test) {
			name, id, err := getSimulatorVersion("testdata/simulatorInfo.plist")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, name, should.Equal("iOS 14.4"))
			assert.Loosely(t, id, should.Equal("ios-14-4"))
		})
		t.Run("when simulator version keys are missing", func(t *ftt.Test) {
			_, _, err := getSimulatorVersion("testdata/badKeys.plist")
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("when simulator version file is broken", func(t *ftt.Test) {
			_, _, err := getSimulatorVersion("testdata/broken.plist")
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("when simulator version file is missing", func(t *ftt.Test) {
			_, _, err := getSimulatorVersion("testdata/nonexistent")
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}
