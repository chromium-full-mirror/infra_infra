// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package source

import (
	"bytes"
	"context"
	"crypto/sha256"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"io"
	"os"
	"strings"
	"testing"
)

func TestSource(t *testing.T) {
	t.Parallel()

	ftt.Run("gs://...", t, func(t *ftt.Test) {
		t.Run("Works", func(t *ftt.Test) {
			src, err := New("gs://stuff", strings.Repeat("a", 64))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, src, should.Resemble(&gsSource{path: "gs://stuff", sha256: bytes.Repeat([]byte{170}, 32)}))
			assert.Loosely(t, src.SHA256(), should.Resemble(bytes.Repeat([]byte{170}, 32)))
		})

		t.Run("Wants hash", func(t *ftt.Test) {
			_, err := New("gs://stuff", "")
			assert.Loosely(t, err, should.ErrLike("-tarball-sha256 is required"))
		})

		t.Run("Bad digest format", func(t *ftt.Test) {
			_, err := New("gs://stuff", "ZZZ")
			assert.Loosely(t, err, should.ErrLike("not hex"))

			_, err = New("gs://stuff", "aaaa")
			assert.Loosely(t, err, should.ErrLike("wrong length"))
		})
	})

	ftt.Run("Local file", t, func(t *ftt.Test) {
		t.Run("Missing", func(t *ftt.Test) {
			_, err := New("missing_file", "")
			assert.Loosely(t, err, should.ErrLike("can't open the file"))
		})

		t.Run("Present", func(t *ftt.Test) {
			f, err := os.CreateTemp("", "gaedeploy_test")
			assert.Loosely(t, err, should.BeNil)
			defer os.Remove(f.Name())

			_, err = f.Write([]byte("boo"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, f.Close(), should.BeNil)

			expected := sha256.New()
			expected.Write([]byte("boo"))

			fs, err := New(f.Name(), "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, fs, should.Resemble(&fileSource{
				path:   f.Name(),
				sha256: expected.Sum(nil),
			}))
			assert.Loosely(t, fs.SHA256(), should.Resemble(expected.Sum(nil)))

			rc, err := fs.Open(context.Background(), "unused")
			assert.Loosely(t, err, should.BeNil)
			blob, err := io.ReadAll(rc)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, blob, should.Resemble([]byte("boo")))
			assert.Loosely(t, rc.Close(), should.BeNil)
		})
	})
}
