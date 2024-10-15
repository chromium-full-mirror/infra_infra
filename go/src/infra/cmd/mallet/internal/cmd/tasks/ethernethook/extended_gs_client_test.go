// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ethernethook

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestCountSections(t *testing.T) {
	t.Parallel()
	e := extendedGSClient{}
	ftt.Run("test count sections", t, func(t *ftt.Test) {
		assert.Loosely(t, e.CountSections(""), should.BeZero)
		assert.Loosely(t, e.CountSections("gs://"), should.BeZero)
		assert.Loosely(t, e.CountSections("gs://a/b/c"), should.Equal(3))
		assert.Loosely(t, e.CountSections("gs://a/b/c/"), should.Equal(3))
	})
}
