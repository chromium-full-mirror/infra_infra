// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dockerfile

import (
	"os"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestLoadAndResolve(t *testing.T) {
	t.Parallel()

	ftt.Run("With temp dir", t, func(t *ftt.Test) {
		tmpDir, err := os.MkdirTemp("", "builder_test")
		assert.Loosely(t, err, should.BeNil)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })

		put := func(path, body string) string {
			fp := filepath.Join(tmpDir, filepath.FromSlash(path))
			assert.Loosely(t, os.WriteFile(fp, []byte(body), 0666), should.BeNil)
			return fp
		}

		t.Run("Works", func(t *ftt.Test) {
			body, err := LoadAndResolve(
				put("Dockerfile", `
FROM ubuntu AS builder # blah
FROM scratch
FROM ubuntu:xenial
`),
				put("pins.yaml", `pins:
- image: ubuntu
  tag: latest
  digest: sha256:123
- image: ubuntu
  tag: xenial
  digest: sha256:456
`),
			)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, string(body), should.Equal(`
FROM ubuntu@sha256:123 AS builder # blah
FROM scratch
FROM ubuntu@sha256:456
`))
		})

		t.Run("No pins, but using digests already", func(t *ftt.Test) {
			body, err := LoadAndResolve(
				put("Dockerfile", `
FROM ubuntu@sha256:123 AS builder # blah
FROM scratch
FROM ubuntu@sha256:456
`),
				"",
			)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, string(body), should.Equal(`
FROM ubuntu@sha256:123 AS builder # blah
FROM scratch
FROM ubuntu@sha256:456
`))
		})

		t.Run("No pins and using a tag", func(t *ftt.Test) {
			_, err := LoadAndResolve(
				put("Dockerfile", `FROM ubuntu`),
				"",
			)
			assert.Loosely(t, err, should.ErrLike(`line 1: resolving "ubuntu:latest": not using pins YAML, the Dockerfile must use @<digest> refs`))
		})

		t.Run("Unknown tag", func(t *ftt.Test) {
			_, err := LoadAndResolve(
				put("Dockerfile", `FROM ubuntu`),
				put("pins.yaml", `{"pins": [{"image": "zzz", "digest": "zzz"}]}`),
			)
			assert.Loosely(t, err, should.ErrLike(`line 1: resolving "ubuntu:latest": no such pinned <image>:<tag> combination in pins YAML`))
		})
	})
}
