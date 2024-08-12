// Copyright 2024 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCommonDirFromFiles(t *testing.T) {
	t.Parallel()

	Convey(`Finds common directory when it exists`, t, func() {
		commonDir := filepath.Join("test_data", "cros_test_result", "artifacts")
		wantCommonDir := commonDir + string(filepath.Separator)
		filepaths := []string{
			filepath.Join(commonDir, "test_artifact_1.txt"),
			filepath.Join(commonDir, "test_artifact_2.txt"),
		}

		gotCommonDir := commonDirFromFiles(filepaths)
		So(gotCommonDir, ShouldEqual, wantCommonDir)
	})

	Convey(`Returns empty string when no common directory exists`, t, func() {
		filepaths := []string{
			filepath.Join("dir1", "test_artifact_1.txt"),
			filepath.Join("dir2", "test_artifact_2.txt"),
		}

		gotCommonDir := commonDirFromFiles(filepaths)
		So(gotCommonDir, ShouldBeEmpty)
	})
}
