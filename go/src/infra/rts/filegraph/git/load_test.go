// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/memlogger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/convey"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGraphCache(t *testing.T) {
	t.Parallel()

	ftt.Run(`GraphCache`, t, func(t *ftt.Test) {
		ctx := context.Background()
		ctx = memlogger.Use(ctx)

		t.Run(`empty file is cache-miss`, func(t *ftt.Test) {
			tmpd, err := os.MkdirTemp("", "filegraph_git")
			assert.Loosely(t, err, should.BeNil)
			defer os.RemoveAll(tmpd)

			var cache graphCache
			cache.File, err = os.Create(filepath.Join(tmpd, "empty"))
			assert.Loosely(t, err, should.BeNil)
			defer cache.Close()

			_, err = cache.tryReading(ctx)
			assert.Loosely(t, err, should.BeNil)

			log := logging.Get(ctx).(*memlogger.MemLogger)
			assert.Loosely(t, log, convey.Adapt(memlogger.ShouldHaveLog)(logging.Info, "populating cache"))
		})
	})
}
