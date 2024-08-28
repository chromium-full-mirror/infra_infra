// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tricium

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGetPlatforms(t *testing.T) {

	ftt.Run("PlatformBitPosToMask", t, func(t *ftt.Test) {
		assert.Loosely(t, PlatformBitPosToMask(0), should.BeZero)
		assert.Loosely(t, PlatformBitPosToMask(1), should.Equal(1))
		assert.Loosely(t, PlatformBitPosToMask(2), should.Equal(2))
		assert.Loosely(t, PlatformBitPosToMask(3), should.Equal(4))
		assert.Loosely(t, PlatformBitPosToMask(4), should.Equal(8))
		assert.Loosely(t, PlatformBitPosToMask(5), should.Equal(16))
	})

	ftt.Run("Platform: ANY", t, func(t *ftt.Test) {
		values, err := GetPlatforms(PlatformBitPosToMask(Platform_ANY))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, values, should.Resemble([]Platform_Name{Platform_ANY}))
	})

	ftt.Run("Platform: UBUNTU", t, func(t *ftt.Test) {
		values, err := GetPlatforms(PlatformBitPosToMask(Platform_UBUNTU))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, values, should.Resemble([]Platform_Name{Platform_UBUNTU}))
	})

	ftt.Run("Platform: ANDROID|OSX|WINDOWS", t, func(t *ftt.Test) {
		values, err := GetPlatforms(
			PlatformBitPosToMask(Platform_ANDROID) +
				PlatformBitPosToMask(Platform_OSX) +
				PlatformBitPosToMask(Platform_WINDOWS))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, values, should.Resemble([]Platform_Name{
			Platform_ANDROID,
			Platform_OSX,
			Platform_WINDOWS,
		}))
	})

	ftt.Run("Platform: Invalid", t, func(t *ftt.Test) {
		// Position 60 is unused.
		values, err := GetPlatforms(PlatformBitPosToMask(60))
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, values, should.BeNil)
	})
}
