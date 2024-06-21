// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFindDirWithPrefix(t *testing.T) {
	t.Parallel()
	dirPath := "test_data"

	Convey("success with valid dir", t, func() {
		wantDir := filepath.Join(dirPath, "artifacts")
		gotPath, err := FindDirWithPrefix(dirPath, "art")
		So(err, ShouldBeNil)
		So(gotPath, ShouldEqual, wantDir)
	})

	Convey("failure with invalid dir", t, func() {
		gotPath, err := FindDirWithPrefix(dirPath, "invalid-dir")
		So(err, ShouldBeError)
		So(gotPath, ShouldBeEmpty)
	})

	Convey("failure with only files", t, func() {
		dir := filepath.Join(dirPath, "artifacts")
		gotPath, err := FindDirWithPrefix(dir, "sample_artifact")
		So(err, ShouldBeError)
		So(gotPath, ShouldBeEmpty)
	})
}
