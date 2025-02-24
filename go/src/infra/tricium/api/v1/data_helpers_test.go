// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tricium

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGetPathForDataType(t *testing.T) {
	ftt.Run("Known data type has path", t, func(t *ftt.Test) {
		d := &Data_GitFileDetails{}
		_, err := GetPathForDataType(d)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("Unknown data type returns an error", t, func(t *ftt.Test) {
		_, err := GetPathForDataType("jkgdsjf")
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestFilterFiles(t *testing.T) {

	ftt.Run("Filter with an empty list of patterns", t, func(t *ftt.Test) {
		// The result is the union of all files that match any of the
		// patterns, so if no patterns are given, then the result is
		// empty.
		files := []*Data_File{{Path: "x/y/z.py"}, {Path: "x/y/z.txt"}}
		filtered, err := FilterFiles(files)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filtered, should.BeEmpty)
	})

	ftt.Run("Filter with one pattern", t, func(t *ftt.Test) {
		// Note that the pattern only has to match the basename.
		files := []*Data_File{{Path: "x/y/z.py"}, {Path: "x/y/z.txt"}}
		filtered, err := FilterFiles(files, "*.py")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filtered, should.Match([]*Data_File{{Path: "x/y/z.py"}}))
	})

	ftt.Run("Filter with one invalid pattern", t, func(t *ftt.Test) {
		files := []*Data_File{{Path: "x/y/z.py"}, {Path: "x/y/z.txt"}}
		_, err := FilterFiles(files, "[-]")
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Filter with two patterns", t, func(t *ftt.Test) {
		files := []*Data_File{{Path: "x/y/z.py"}, {Path: "x/y/z.txt"}}
		filtered, err := FilterFiles(files, "*.py", "*.txt")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filtered, should.Match(files))
	})

	ftt.Run("Filter with two patterns that overlap", t, func(t *ftt.Test) {
		files := []*Data_File{{Path: "x/y/z.py"}, {Path: "x/y/z.txt"}}
		filtered, err := FilterFiles(files, "*", "*.txt")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filtered, should.Match(files))
	})
}
