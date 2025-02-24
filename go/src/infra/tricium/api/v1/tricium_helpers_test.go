// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tricium

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestIsDone(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {

		t.Run("Done means done", func(t *ftt.Test) {
			done := IsDone(State_SUCCESS)
			assert.Loosely(t, done, should.BeTrue)
			done = IsDone(State_FAILURE)
			assert.Loosely(t, done, should.BeTrue)
		})

		t.Run("Pending or running is not done", func(t *ftt.Test) {
			done := IsDone(State_PENDING)
			assert.Loosely(t, done, should.BeFalse)
			done = IsDone(State_RUNNING)
			assert.Loosely(t, done, should.BeFalse)
		})
	})
}
