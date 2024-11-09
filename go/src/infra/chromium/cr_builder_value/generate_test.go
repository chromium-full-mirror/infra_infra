// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	ftt.Run("Check isChromeOrChromiumProject", t, func(t *ftt.Test) {
		assert.Loosely(t, isChromeOrChromiumProject("chrome"), should.Equal(true))
		assert.Loosely(t, isChromeOrChromiumProject("chromium"), should.Equal(true))
		assert.Loosely(t, isChromeOrChromiumProject("chromeos"), should.Equal(false))
		assert.Loosely(t, isChromeOrChromiumProject("chrome-100"), should.Equal(true))
		assert.Loosely(t, isChromeOrChromiumProject("chromium-100"), should.Equal(true))
		assert.Loosely(t, isChromeOrChromiumProject("turquoise"), should.Equal(false))
	})
}
