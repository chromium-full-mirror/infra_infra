// Copyright 2024 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestCommonDirFromFiles(t *testing.T) {
	t.Parallel()

	ftt.Run(`Finds common directory when it exists`, t, func(t *ftt.Test) {
		commonDir := filepath.Join("test_data", "cros_test_result", "artifacts")
		wantCommonDir := commonDir + string(filepath.Separator)
		filepaths := []string{
			filepath.Join(commonDir, "test_artifact_1.txt"),
			filepath.Join(commonDir, "test_artifact_2.txt"),
		}

		gotCommonDir := commonDirFromFiles(filepaths)
		assert.Loosely(t, gotCommonDir, should.Equal(wantCommonDir))
	})

	ftt.Run(`Returns empty string when no common directory exists`, t, func(t *ftt.Test) {
		filepaths := []string{
			filepath.Join("dir1", "test_artifact_1.txt"),
			filepath.Join("dir2", "test_artifact_2.txt"),
		}

		gotCommonDir := commonDirFromFiles(filepaths)
		assert.Loosely(t, gotCommonDir, should.BeEmpty)
	})
}
