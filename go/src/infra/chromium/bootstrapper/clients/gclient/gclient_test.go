// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gclient

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	. "go.chromium.org/luci/common/testing/assertions"
)

func TestGetDep(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	client, _ := NewClientForTesting()

	Convey("getDep", t, func() {

		Convey("returns the revision for the specified path", func() {
			depsContents := `deps = {
				'foo': 'https://chromium.googlesource.com/foo.git@foo-revision',
			}`

			revision, err := client.GetDep(ctx, depsContents, "foo")

			So(err, ShouldBeNil)
			So(revision, ShouldEqual, "foo-revision")
		})

		Convey("fails for unknown path", func() {
			depsContents := `deps = {
				'foo': 'https://chromium.googlesource.com/foo.git@foo-revision',
			}`

			revision, err := client.GetDep(ctx, depsContents, "bar")

			So(err, ShouldErrLike, "Could not find any dependency called bar")
			So(revision, ShouldBeEmpty)
		})

	})
}
