// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package main

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestConvertPathSlashesUnix(t *testing.T) {
	t.Parallel()
	ftt.Run("Convert path with forward slashes", t, func(t *ftt.Test) {
		p := "\\test\\path\\"

		t.Run("On Unix", func(t *ftt.Test) {
			t.Run("The path should be the same", func(t *ftt.Test) {
				assert.Loosely(t, convertPathToForwardSlashes(p), should.Equal(p))
			})
		})
	})
}
