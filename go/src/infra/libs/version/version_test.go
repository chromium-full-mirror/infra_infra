// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package version

import (
	"testing"
	"testing/quick"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGEQ(t *testing.T) {
	t.Parallel()
	ftt.Run("Test Greater Than or Equal to", t, func(t *ftt.Test) {
		t.Run("A: 1.2.3.4 B: 1.1.2.3.5-rc-4", func(t *ftt.Test) {
			assert.Loosely(t, GEQ("1.2.3.4", "1.1.2.3.5-rc-4"), should.BeTrue)
		})
		t.Run("A: 1.1.2.3.5-rc-4 B: 1.2.3.4", func(t *ftt.Test) {
			assert.Loosely(t, GEQ("1.1.2.3.5-rc-4", "1.2.3.4"), should.BeFalse)
		})
		t.Run("A: 1.1-debug.1.1 B: 1.1.1.0", func(t *ftt.Test) {
			assert.Loosely(t, GEQ("1.1-debug.1.1", "1.1.1.0"), should.BeTrue)
		})
		t.Run("A: 1.1.1.0 B: 1.1-debug.1.1", func(t *ftt.Test) {
			assert.Loosely(t, GEQ("1.1.1.0", "1.1-debug.1.1"), should.BeFalse)
		})
		t.Run("A: 10.12.33.1 B: 10.12.33.1-rc4", func(t *ftt.Test) {
			assert.Loosely(t, GEQ("10.12.33.1", "10.12.33.1-rc4"), should.BeFalse)
		})
		t.Run("A: 10.12.33.1-rc4 B: 10.12.33.1", func(t *ftt.Test) {
			assert.Loosely(t, GEQ("10.12.33.1-rc4", "10.12.33.1"), should.BeTrue)
		})
		t.Run("A:  B: 10.12.33.1", func(t *ftt.Test) {
			assert.Loosely(t, GEQ("", "10.12.33.1"), should.BeFalse)
		})
		t.Run("A: 10.12.33.1 B: ", func(t *ftt.Test) {
			assert.Loosely(t, GEQ("10.12.33.1", ""), should.BeTrue)
		})
		t.Run("A: Batman B: 10.12.33.1", func(t *ftt.Test) {
			assert.Loosely(t, GEQ("Batman", "10.12.33.1"), should.BeFalse)
		})
		t.Run("A: 10.12.33.1 B: Superman", func(t *ftt.Test) {
			assert.Loosely(t, GEQ("10.12.33.1", "Superman"), should.BeTrue)
		})
		t.Run("Transitive property test. If A >= B and B >= C then A>=C", func(t *ftt.Test) {
			MaxCount := 100000 // 100k tests each time ??
			transitiveTest := func(a, b, c string) bool {
				if GEQ(a, b) && GEQ(b, c) {
					return GEQ(a, c)
				}
				// Ignore the cases where we can't check
				return true
			}
			// Run a test to figure out if it fails,
			err := quick.Check(transitiveTest, &quick.Config{MaxCount: MaxCount})
			assert.Loosely(t, err, should.BeNil)
		})
	})
}
