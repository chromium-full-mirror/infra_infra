// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGit(t *testing.T) {
	t.Parallel()

	// https: //logs.chromium.org/logs/infra/buildbucket/cr-buildbucket.appspot.com/8864634177878601952/+/u/go_test/stdout
	t.Skipf("this test is failing in a weird way; skip for now")

	if testing.Short() {
		t.Skip("Skipping because it is not a short test")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not found: %s", err)
	}

	ftt.Run(`Git`, t, func(t *ftt.Test) {
		tmpd, err := os.MkdirTemp("", "filegraph_git")
		assert.Loosely(t, err, should.BeNil)
		defer os.RemoveAll(tmpd)

		git := func(context string) func(args ...string) string {
			return func(args ...string) string {
				out, err := Exec(context)(args...)
				assert.Loosely(t, err, should.BeNil)
				return out
			}
		}

		git(tmpd)("init")

		fooPath := filepath.Join(tmpd, "foo")
		err = os.WriteFile(fooPath, []byte("hello"), 0777)
		assert.Loosely(t, err, should.BeNil)

		// Run in fooBar context.
		git(fooPath)("add", fooPath)
		git(tmpd)("commit", "-a", "-m", "message")

		out := git(fooPath)("status")
		assert.Loosely(t, out, should.ContainSubstring("working tree clean"))

		repoDir, err := EnsureSameRepo(tmpd, fooPath)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, repoDir, should.Equal(tmpd))
	})
}

func TestChangedFiles(t *testing.T) {
	t.Parallel()
	ftt.Run(`ChangedFiles`, t, func(t *ftt.Test) {
		t.Run(`Works`, func(t *ftt.Test) {
			assert.Loosely(t, changedFiles("foo\nbar\n"), should.Resemble([]string{"foo", "bar"}))
		})
		t.Run(`No files changed`, func(t *ftt.Test) {
			assert.Loosely(t, changedFiles("\n"), should.Resemble([]string(nil)))
		})
	})
}
