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
func TestConfigScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("Happy case", t, func(t *ftt.Test) {
		ctx := WithConfigScope(ctx)
		PutConfigParam(ctx, "key1", "hello")
		v, ok := ReadConfigParam(ctx, "key1")
		assert.Loosely(t, ok, should.BeTrue)
		assert.Loosely(t, v, should.Equal("hello"))
	})
	ftt.Run("Read non existent key", t, func(t *ftt.Test) {
		ctx := WithConfigScope(ctx)
		_, ok := ReadConfigParam(ctx, "key1")
		assert.Loosely(t, ok, should.BeFalse)
	})
	ftt.Run("Read all keys", t, func(t *ftt.Test) {
		ctx := WithConfigScope(ctx)
		PutConfigParam(ctx, "key1", "hello1")
		PutConfigParam(ctx, "key2", "hello2")
		PutConfigParam(ctx, "key3", "hello3")
		v, ok := ReadConfigParam(ctx, "key1")
		assert.Loosely(t, ok, should.BeTrue)
		assert.Loosely(t, v, should.Equal("hello1"))
		v, ok = ReadConfigParam(ctx, "key2")
		assert.Loosely(t, ok, should.BeTrue)
		assert.Loosely(t, v, should.Equal("hello2"))
		v, ok = ReadConfigParam(ctx, "key3")
		assert.Loosely(t, ok, should.BeTrue)
		assert.Loosely(t, v, should.Equal("hello3"))
	})
	ftt.Run("Try to put before initilize", t, func(t *ftt.Test) {
		PutConfigParam(ctx, "key1", "hello")
		_, ok := ReadConfigParam(ctx, "key1")
		assert.Loosely(t, ok, should.BeFalse)
	})
}
