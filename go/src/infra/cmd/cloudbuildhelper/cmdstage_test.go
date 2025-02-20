// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cmd/cloudbuildhelper/fileset"
	"go.chromium.org/infra/cmd/cloudbuildhelper/manifest"
)

const testPinsYAML = `
pins:
- image: example.com/some/image
  tag: some-tag
  digest: sha256:431d3cca5e6a1f043b48ec556a9c60876e7ae1db18e20d0b860cb9c77e4c0b1b
`

func TestStage(t *testing.T) {
	t.Parallel()

	ftt.Run("With temp", t, func(t *ftt.Test) {
		ctx := context.Background()

		write := func(root, path, body string) string {
			p := filepath.Join(root, path)
			assert.Loosely(t, os.WriteFile(p, []byte(body), 0600), should.BeNil)
			return p
		}
		read := func(root, path string) string {
			blob, err := os.ReadFile(filepath.Join(root, path))
			assert.Loosely(t, err, should.BeNil)
			return string(blob)
		}

		t.Run("Bunch of files", func(t *ftt.Test) {
			ctxDir := t.TempDir()
			outDir := t.TempDir()

			write(ctxDir, "a", "file a")
			write(ctxDir, "b", "file b")

			err := stage(ctx,
				&manifest.Manifest{
					ContextDir: ctxDir,
				},
				func(fs *fileset.Set) error {
					return fs.Materialize(outDir)
				},
			)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, read(outDir, "a"), should.Equal("file a"))
			assert.Loosely(t, read(outDir, "b"), should.Equal("file b"))
		})

		t.Run("Resolving explicitly set Dockerfile", func(t *ftt.Test) {
			ctxDir := t.TempDir()
			outDir := t.TempDir()

			dockerfile := write(ctxDir, "Dockerfile", "FROM example.com/some/image:some-tag")

			err := stage(ctx,
				&manifest.Manifest{
					ContextDir: ctxDir,
					Dockerfile: dockerfile,
					ImagePins:  write(t.TempDir(), "pins.yaml", testPinsYAML),
				},
				func(fs *fileset.Set) error {
					return fs.Materialize(outDir)
				},
			)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, read(outDir, "Dockerfile"), should.Equal(
				"FROM example.com/some/image@sha256:431d3cca5e6a1f043b48ec556a9c60876e7ae1db18e20d0b860cb9c77e4c0b1b"))
		})

		t.Run("Resolving ${contextdir}/Dockerfile", func(t *ftt.Test) {
			ctxDir := t.TempDir()
			outDir := t.TempDir()

			write(ctxDir, "Dockerfile", "FROM example.com/some/image:some-tag")

			err := stage(ctx,
				&manifest.Manifest{
					ContextDir: ctxDir,
					ImagePins:  write(t.TempDir(), "pins.yaml", testPinsYAML),
				},
				func(fs *fileset.Set) error {
					return fs.Materialize(outDir)
				},
			)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, read(outDir, "Dockerfile"), should.Equal(
				"FROM example.com/some/image@sha256:431d3cca5e6a1f043b48ec556a9c60876e7ae1db18e20d0b860cb9c77e4c0b1b"))
		})

		t.Run("Explicitly set Dockerfile is missing", func(t *ftt.Test) {
			ctxDir := t.TempDir()
			outDir := t.TempDir()

			err := stage(ctx,
				&manifest.Manifest{
					ContextDir: ctxDir,
					Dockerfile: filepath.Join(ctxDir, "Dockerfile"),
				},
				func(fs *fileset.Set) error {
					return fs.Materialize(outDir)
				},
			)

			assert.Loosely(t, errors.Is(err, fs.ErrNotExist), should.BeTrue)
		})

		t.Run("Using build steps", func(t *ftt.Test) {
			ctxDir := t.TempDir()
			inpDir := t.TempDir()
			outDir := t.TempDir()

			write(ctxDir, "a", "file a")
			write(inpDir, "b", "file b")

			m, err := manifest.Load(write(inpDir, "manifest.yaml", `{
				"name": "ignored",
				"contextdir": "to-be-set-later",
				"inputsdir": "to-be-set-later",
				"build": [
					{
						"copy": "${inputsdir}/b",
						"dest": "${contextdir}/b"
					}
				]
			}`))
			assert.Loosely(t, err, should.BeNil)
			m.ContextDir = ctxDir
			m.InputsDir = inpDir
			assert.Loosely(t, m.Finalize(), should.BeNil)

			err = stage(ctx, m, func(fs *fileset.Set) error {
				return fs.Materialize(outDir)
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, read(outDir, "a"), should.Equal("file a"))
			assert.Loosely(t, read(outDir, "b"), should.Equal("file b"))
		})
	})
}
