// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestSet(t *testing.T) {
	t.Parallel()
	ftt.Run("Concurrent set", t, func(t *ftt.Test) {
		cs := NewConcurrentSet(0)

		t.Run("Adding to set", func(t *ftt.Test) {
			t.Run("Adding a new value should return true", func(t *ftt.Test) {
				assert.Loosely(t, cs.Add("hello"), should.Equal(true))
				assert.Loosely(t, cs.Add("hello"), should.Equal(false))
			})
		})
	})
}

func TestMap(t *testing.T) {
	t.Parallel()
	ftt.Run("FileHashMap", t, func(t *ftt.Test) {
		m := NewFileHashMap()

		t.Run("Getting nonexistent values", func(t *ftt.Test) {
			hash, ok := m.Filehash("nonexistent")
			t.Run("Filehash should not be fetched", func(t *ftt.Test) {
				assert.Loosely(t, hash, should.BeEmpty)
				assert.Loosely(t, ok, should.Equal(false))
			})

			fname, ok := m.Filename("nonexistent")
			t.Run("Filename should not be fetched", func(t *ftt.Test) {
				assert.Loosely(t, fname, should.BeEmpty)
				assert.Loosely(t, ok, should.Equal(false))
			})
		})

		t.Run("Adding and retrieving data", func(t *ftt.Test) {
			t.Run("Adding should return true if new and false if not", func(t *ftt.Test) {
				assert.Loosely(t, m.Add("hello", "hash"), should.Equal(true))
				assert.Loosely(t, m.Add("bye", "hash"), should.Equal(false))

				fname, ok := m.Filename("hash")
				assert.Loosely(t, fname, should.Equal("hello"))
				assert.Loosely(t, ok, should.Equal(true))

				hash, ok := m.Filehash("hello")
				assert.Loosely(t, hash, should.Equal("hash"))
				assert.Loosely(t, ok, should.Equal(true))
			})
		})
	})
}
