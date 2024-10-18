// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scopes

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

// Testing param methods.
func TestParams(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("Simple read", t, func(t *ftt.Test) {
		m := map[string]any{
			"k1": "v1",
			"k2": "v2",
			"k3": "v3",
		}
		ctx = WithParams(ctx, m)
		copy := GetParamCopy(ctx)
		assert.Loosely(t, m, should.Match(copy))
	})
	ftt.Run("Read an existent key", t, func(t *ftt.Test) {
		m := map[string]any{
			"k4": "v4",
		}
		ctx = WithParams(ctx, m)
		v, ok := GetParam(ctx, "k4")
		assert.Loosely(t, ok, should.Equal(true))
		assert.Loosely(t, v, should.Equal("v4"))
	})
	ftt.Run("Read nonexistent key", t, func(t *ftt.Test) {
		m := map[string]any{
			"k5": "v5",
		}
		ctx = WithParams(ctx, m)
		_, ok := GetParam(ctx, "k6")
		assert.Loosely(t, ok, should.Equal(false))

	})
	ftt.Run("Read nonexistent key (2)", t, func(t *ftt.Test) {
		_, ok := GetParam(ctx, "k6")
		assert.Loosely(t, ok, should.Equal(false))
	})
}
