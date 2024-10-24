// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package eval

import (
	"math"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestScoreString(t *testing.T) {
	t.Parallel()

	ftt.Run(`ScoreString`, t, func(t *ftt.Test) {
		t.Run("NaN", func(t *ftt.Test) {
			assert.Loosely(t, scoreString(float32(math.NaN())), should.Equal("?"))
		})
		t.Run("0%", func(t *ftt.Test) {
			assert.Loosely(t, scoreString(0), should.Equal("0.00%"))
		})
		t.Run("0.0001%", func(t *ftt.Test) {
			assert.Loosely(t, scoreString(0.000001), should.Equal("<0.01%"))
		})
		t.Run("50%", func(t *ftt.Test) {
			assert.Loosely(t, scoreString(0.5), should.Equal("50.00%"))
		})
		t.Run("99.999%", func(t *ftt.Test) {
			assert.Loosely(t, scoreString(0.99999), should.Equal(">99.99%"))
		})
		t.Run("100%", func(t *ftt.Test) {
			assert.Loosely(t, scoreString(1), should.Equal("100.00%"))
		})
	})
}
