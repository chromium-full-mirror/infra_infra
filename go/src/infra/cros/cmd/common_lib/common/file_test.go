// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestFindDirWithPrefix(t *testing.T) {
	t.Parallel()
	dirPath := "test_data"

	ftt.Run("success with valid dir", t, func(t *ftt.Test) {
		wantDir := filepath.Join(dirPath, "artifacts")
		gotPath, err := FindDirWithPrefix(dirPath, "art")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, gotPath, should.Equal(wantDir))
	})

	ftt.Run("failure with invalid dir", t, func(t *ftt.Test) {
		gotPath, err := FindDirWithPrefix(dirPath, "invalid-dir")
		assert.Loosely(t, err, should.ErrLike("no directory with prefix"))
		assert.Loosely(t, gotPath, should.BeEmpty)
	})

	ftt.Run("failure with only files", t, func(t *ftt.Test) {
		dir := filepath.Join(dirPath, "artifacts")
		gotPath, err := FindDirWithPrefix(dir, "sample_artifact")
		assert.Loosely(t, err, should.ErrLike("no directory with prefix"))
		assert.Loosely(t, gotPath, should.BeEmpty)
	})
}
