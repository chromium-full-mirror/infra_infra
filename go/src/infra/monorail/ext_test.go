// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package monorail

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestExt(t *testing.T) {
	t.Parallel()

	ftt.Run("FindCC", t, func(t *ftt.Test) {
		issue := &Issue{
			Cc: []*AtomPerson{{Name: "a"}, {Name: "b"}},
		}
		b := issue.FindCC("b")
		assert.Loosely(t, b, should.NotBeNil)
		assert.Loosely(t, b.Name, should.Equal("b"))

		c := issue.FindCC("c")
		assert.Loosely(t, c, should.BeNil)
	})
}
