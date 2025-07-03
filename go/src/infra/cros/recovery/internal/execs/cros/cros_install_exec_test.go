// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"testing"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestExtractBranch(t *testing.T) {
	t.Run("Basic example", func(t *testing.T) {
		branch, err := extractBranch("android-build/build_explorer/artifacts_list/P70315493/betty-trunk_staging-userdebug/11811312-ota-11811312.zip")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, branch, should.Equal("trunk_staging-userdebug"))
	})

	t.Run("Empty string", func(t *testing.T) {
		_, err := extractBranch("")
		assert.Loosely(t, err, should.NotBeNil)
	})

	t.Run("Too few parts", func(t *testing.T) {
		_, err := extractBranch("android-build/build_explorer/artifacts_list/P70315493")
		assert.Loosely(t, err, should.NotBeNil)
	})

	t.Run("Missing branch in subpart", func(t *testing.T) {
		_, err := extractBranch("android-build/build_explorer/artifacts_list/P70315493/betty/11811312-ota-11811312.zip")
		assert.Loosely(t, err, should.NotBeNil)
	})

	t.Run("No hyphen", func(t *testing.T) {
		_, err := extractBranch("android-build/build_explorer/artifacts_list/P70315493/bettytrunkstaginguserdebug/11811312-ota-11811312.zip")
		assert.Loosely(t, err, should.NotBeNil)
	})

	t.Run("Another valid example", func(t *testing.T) {
		branch, err := extractBranch("android-build/build_explorer/artifacts_list/P70315493/board-main-userdebug/11811312-ota-11811312.zip")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, branch, should.Equal("main-userdebug"))
	})
}
