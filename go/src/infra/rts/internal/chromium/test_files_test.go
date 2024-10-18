// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chromium

import (
	"bytes"
	"context"
	"testing"

	"github.com/golang/protobuf/proto"
	"google.golang.org/api/iterator"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestTestFileSet(t *testing.T) {
	t.Parallel()

	ftt.Run("TestFileSet", t, func(t *ftt.Test) {
		ctx := context.Background()

		buf := bytes.NewBuffer(nil)

		expected := []*TestFile{
			{Path: "a.cc"},
			{Path: "b.cc"},
			{Path: "c.cc"},
		}

		// Write expected protos.
		remaining := expected
		err := writeTestFilesFrom(ctx, buf, func(dest interface{}) error {
			if len(remaining) == 0 {
				return iterator.Done
			}
			proto.Merge(dest.(proto.Message), remaining[0])
			remaining = remaining[1:]
			return nil
		})
		assert.Loosely(t, err, should.BeNil)

		// Read protos.
		var actual []*TestFile
		err = ReadTestFiles(buf, func(f *TestFile) error {
			actual = append(actual, f)
			return nil
		})

		assert.Loosely(t, actual, should.Match(expected))
	})
}
