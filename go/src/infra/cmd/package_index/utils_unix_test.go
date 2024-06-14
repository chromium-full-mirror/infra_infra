// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package main

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestConvertPathSlashesUnix(t *testing.T) {
	t.Parallel()
	Convey("Convert path with forward slashes", t, func() {
		p := "\\test\\path\\"

		Convey("On Unix", func() {
			Convey("The path should be the same", func() {
				So(convertPathToForwardSlashes(p), ShouldEqual, p)
			})
		})
	})
}
