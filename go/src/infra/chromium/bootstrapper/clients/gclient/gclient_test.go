// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gclient

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGetDep(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	client, _ := NewClientForTesting()

	ftt.Run("getDep", t, func(t *ftt.Test) {

		t.Run("returns the revision for the specified path", func(t *ftt.Test) {
			depsContents := `deps = {
				'foo': 'https://chromium.googlesource.com/foo.git@foo-revision',
			}`

			revision, err := client.GetDep(ctx, depsContents, "foo")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, revision, should.Equal("foo-revision"))
		})

		t.Run("fails for unknown path", func(t *ftt.Test) {
			depsContents := `deps = {
				'foo': 'https://chromium.googlesource.com/foo.git@foo-revision',
			}`

			revision, err := client.GetDep(ctx, depsContents, "bar")

			assert.Loosely(t, err, should.ErrLike("Could not find any dependency called bar"))
			assert.Loosely(t, revision, should.BeEmpty)
		})

	})
}
