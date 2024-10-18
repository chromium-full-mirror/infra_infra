// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestHelperFunctions(t *testing.T) {
	ftt.Run("possibleGitattributesPaths lists all relevant paths", t, func(t *ftt.Test) {
		assert.Loosely(t,
			possibleGitattributesPaths([]string{"one/two/foo.c"}),
			should.Match(
				[]string{
					".gitattributes",
					"one/.gitattributes",
					"one/two/.gitattributes",
				}))
	})

	ftt.Run("possibleGitattributesPaths works for multiple paths", t, func(t *ftt.Test) {
		assert.Loosely(t,
			possibleGitattributesPaths([]string{
				"one/bar.c",
				"one/two/foo.c",
				"one/two/foo.h",
				"one/other/x.txt",
			}),
			should.Match(
				[]string{
					".gitattributes",
					"one/.gitattributes",
					"one/other/.gitattributes",
					"one/two/.gitattributes",
				}))
	})

	ftt.Run("ancestorDirectories gives the union of all ancestor dir paths", t, func(t *ftt.Test) {
		assert.Loosely(t, ancestorDirectories([]string{"a/b/c/foo.proto"}),
			should.Match(
				stringset.NewFromSlice("", "a", "a/b", "a/b/c")))
		assert.Loosely(t, ancestorDirectories([]string{"a/b/c/foo.proto", "x/y/foo.c"}),
			should.Match(
				stringset.NewFromSlice("", "a", "a/b", "a/b/c", "x", "x/y")))
	})

	ftt.Run("splitNull splits null-separated and terminated strings", t, func(t *ftt.Test) {
		assert.Loosely(t, splitNull("f 1\x00"), should.Match([]string{"f 1"}))
		assert.Loosely(t, splitNull("f 1\x00f 2\x00"), should.Match([]string{"f 1", "f 2"}))
	})
}
