// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"context"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"os"
	"testing"
	"time"
)

func TestMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ts := time.Now().Round(time.Millisecond) // convert to wall clock

	ftt.Run("modifyMetadata Works", t, func(t *ftt.Test) {
		tmp, err := os.MkdirTemp("", "gaedeploy_test")
		assert.Loosely(t, err, should.BeNil)
		defer os.RemoveAll(tmp)

		// Creates new file.
		err = modifyMetadata(ctx, tmp, func(m *cacheMetadata) {
			assert.Loosely(t, m.Created.IsZero(), should.BeTrue)
			assert.Loosely(t, m.Touched.IsZero(), should.BeTrue)
			m.Created = ts
			m.Touched = ts
		})
		assert.Loosely(t, err, should.BeNil)

		// Reads the existing file, writes back modifications.
		err = modifyMetadata(ctx, tmp, func(m *cacheMetadata) {
			assert.Loosely(t, m.Created.Equal(ts), should.BeTrue)
			assert.Loosely(t, m.Touched.Equal(ts), should.BeTrue)
			m.Touched = ts.Add(10 * time.Second)
		})
		assert.Loosely(t, err, should.BeNil)

		// Verify it is updated.
		m, err := readMetadata(ctx, tmp)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m.Created.Equal(ts), should.BeTrue)
		assert.Loosely(t, m.Touched.Equal(ts.Add(10*time.Second)), should.BeTrue)
	})
}
