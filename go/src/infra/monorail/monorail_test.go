// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package monorail

import (
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"testing"
)

func TestMonorail(t *testing.T) {
	t.Parallel()

	ftt.Run("Monorail", t, func(t *ftt.Test) {
		t.Run("IssueURL", func(t *ftt.Test) {
			expected := "https://bugs.chromium.org/p/chromium/issues/detail?id=123"
			actual := IssueURL("bugs.chromium.org", "chromium", 123)
			assert.Loosely(t, actual, should.Equal(expected))
		})
	})
}
