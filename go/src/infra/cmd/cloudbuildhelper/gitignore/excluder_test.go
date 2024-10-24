// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gitignore

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"infra/cmd/cloudbuildhelper/fileset"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestExcluder(t *testing.T) {
	t.Parallel()

	ftt.Run("findRepoRoot works", t, func(c *ftt.Test) {
		tmp := newTempDir(c)

		c.Run("No .git at all", func(c *ftt.Test) {
			root, err := findRepoRoot(tmp.join("."))
			assert.Loosely(c, err, should.BeNil)
			assert.Loosely(c, root, should.Equal(tmp.join(".")))
		})

		c.Run("Already given the root", func(c *ftt.Test) {
			tmp.mkdir("a/.git")

			root, err := findRepoRoot(tmp.join("a"))
			assert.Loosely(c, err, should.BeNil)
			assert.Loosely(c, root, should.Equal(tmp.join("a")))
		})

		c.Run("Discovers it few layers up", func(c *ftt.Test) {
			tmp.mkdir("a/.git")
			tmp.mkdir("a/b/c")

			root, err := findRepoRoot(tmp.join("a/b/c"))
			assert.Loosely(c, err, should.BeNil)
			assert.Loosely(c, root, should.Equal(tmp.join("a")))
		})

		c.Run("Skips files", func(c *ftt.Test) {
			tmp.mkdir("a/.git")
			tmp.touch("a/b/.git")
			tmp.mkdir("a/b/c")

			root, err := findRepoRoot(tmp.join("a/b/c"))
			assert.Loosely(c, err, should.BeNil)
			assert.Loosely(c, root, should.Equal(tmp.join("a")))
		})
	})

	ftt.Run("scanUp works", t, func(c *ftt.Test) {
		tmp := newTempDir(c)
		assert.Loosely(c, scanUp(tmp.join("a/b/c"), tmp.join("."), ".gitignore"), should.Resemble([]string{
			tmp.join(".gitignore"),
			tmp.join("a/.gitignore"),
			tmp.join("a/b/.gitignore"),
		}))
	})

	ftt.Run("scanDown works", t, func(c *ftt.Test) {
		tmp := newTempDir(c)
		tmp.touch(".gitignore")
		tmp.touch("stuff/stuff")
		tmp.touch("a/b/c/.gitignore")
		tmp.touch("a/d/.gitignore")
		tmp.touch("a/d/stuff")

		paths, err := scanDown(nil, tmp.join("."), ".gitignore")
		assert.Loosely(c, err, should.BeNil)
		assert.Loosely(c, paths, should.Resemble([]string{
			tmp.join(".gitignore"),
			tmp.join("a/b/c/.gitignore"),
			tmp.join("a/d/.gitignore"),
		}))
	})

	ftt.Run("With temp dir", t, func(c *ftt.Test) {
		tmp := newTempDir(c)
		tmp.mkdir(".git") // pretend to be the repo root

		excluder := func(p string) fileset.Excluder {
			cb, err := NewExcluder(tmp.join(p), ".gitignore")
			assert.Loosely(c, err, should.BeNil)
			return func(rel string, isDir bool) bool {
				return cb(tmp.join(rel), isDir)
			}
		}

		c.Run("Noop excluder", func(c *ftt.Test) {
			cb := excluder(".")

			assert.Loosely(c, cb(".", true), should.BeFalse)
			assert.Loosely(c, cb("a/b/c", false), should.BeFalse)
		})

		c.Run("Simple excluder", func(c *ftt.Test) {
			tmp.put(".gitignore", "*.out")
			cb := excluder(".")

			assert.Loosely(c, cb("abc.go", false), should.BeFalse)
			assert.Loosely(c, cb("abc.out", false), should.BeTrue)
			assert.Loosely(c, cb("abc.out", true), should.BeTrue)
			assert.Loosely(c, cb("1/2/3/abc.go", false), should.BeFalse)
			assert.Loosely(c, cb("1/2/3/abc.out", false), should.BeTrue)
			assert.Loosely(c, cb("abc.out/1/2/3", false), should.BeTrue)
		})

		c.Run("Complex excluder", func(c *ftt.Test) {
			tmp.put(".gitignore", "/dir/*\n!/dir/?z")
			cb := excluder(".")

			assert.Loosely(c, cb("dir", true), should.BeFalse)
			assert.Loosely(c, cb("dir/az", false), should.BeFalse)
			assert.Loosely(c, cb("dir/bz", false), should.BeFalse)
			assert.Loosely(c, cb("dir/ay", false), should.BeTrue)
			assert.Loosely(c, cb("dir/abc", false), should.BeTrue)
			assert.Loosely(c, cb("another/dir/abc", false), should.BeFalse)
		})

		c.Run("Inherited .gitignore", func(c *ftt.Test) {
			tmp.put(".gitignore", "*.pyc")
			tmp.put("a/.gitignore", "*.a\n/hidden")
			tmp.put("a/z/z/b/.gitignore", "*.b")

			// No matter where we start, all .gitignore files are respected.
			for _, start := range []string{".", "a", "a/z/z", "a/z/z/b"} {
				cb := excluder(start)

				assert.Loosely(c, cb("a/z/z/b/1.pyc", false), should.BeTrue)
				assert.Loosely(c, cb("a/z/z/b/1.a", false), should.BeTrue)
				assert.Loosely(c, cb("a/z/z/b/1.b", false), should.BeTrue)
				assert.Loosely(c, cb("a/z/z/b/1.good", false), should.BeFalse)
			}

			// Entries relative to .gitignore location are respected.
			cb := excluder(".")
			assert.Loosely(c, cb("hidden", true), should.BeFalse)
			assert.Loosely(c, cb("a/hidden", true), should.BeTrue)
			assert.Loosely(c, cb("a/z/hidden", true), should.BeFalse)
		})

		c.Run("#include support", func(c *ftt.Test) {
			tmp.put(".gitignore", "root-hidden\n#comment\n#!include:included")
			tmp.put("included", "include-hidden")

			cb := excluder(".")
			assert.Loosely(c, cb("visible", false), should.BeFalse)
			assert.Loosely(c, cb("root-hidden", false), should.BeTrue)
			assert.Loosely(c, cb("include-hidden", false), should.BeTrue)
		})
	})
}

func TestPatternExcluder(t *testing.T) {
	t.Parallel()

	ftt.Run("Works", t, func(t *ftt.Test) {
		exc := NewPatternExcluder([]string{"*.bad", "*.worse", "hidden"})

		assert.Loosely(t, exc("something.good", false), should.BeFalse)
		assert.Loosely(t, exc("something.good", true), should.BeFalse)
		assert.Loosely(t, exc("a/b/c/something.good", false), should.BeFalse)
		assert.Loosely(t, exc("something.good/a/b/c", false), should.BeFalse)

		assert.Loosely(t, exc("something.bad", false), should.BeTrue)
		assert.Loosely(t, exc("something.bad", true), should.BeTrue)
		assert.Loosely(t, exc("a/b/c/something.bad", false), should.BeTrue)
		assert.Loosely(t, exc("something.bad/a/b/c", false), should.BeTrue)

		assert.Loosely(t, exc("something.worse", false), should.BeTrue)
		assert.Loosely(t, exc("something.worse", true), should.BeTrue)
		assert.Loosely(t, exc("a/b/c/something.worse", false), should.BeTrue)
		assert.Loosely(t, exc("something.worse/a/b/c", false), should.BeTrue)

		assert.Loosely(t, exc("something/hidden", true), should.BeTrue)
		assert.Loosely(t, exc("hidden", true), should.BeTrue)
		assert.Loosely(t, exc("hidden/something", false), should.BeTrue)
	})

	ftt.Run("Empty", t, func(t *ftt.Test) {
		exc := NewPatternExcluder(nil)
		assert.Loosely(t, exc("stuff", false), should.BeFalse)
	})
}

type tmpDir struct {
	p string
	t testing.TB
}

func newTempDir(t testing.TB) tmpDir {
	t.Helper()

	tmp, err := ioutil.TempDir("", "gitignore_test")
	assert.Loosely(t, err, should.BeNil, truth.LineContext())
	t.Cleanup(func() { os.RemoveAll(tmp) })
	return tmpDir{tmp, t}
}

func (t tmpDir) join(p string) string {
	t.t.Helper()
	return filepath.Join(t.p, filepath.FromSlash(p))
}

func (t tmpDir) mkdir(p string) {
	t.t.Helper()
	assert.Loosely(t.t, os.MkdirAll(t.join(p), 0777), should.ErrLike(nil), truth.LineContext())
}

func (t tmpDir) put(p, data string) {
	t.t.Helper()
	t.mkdir(path.Dir(p))
	f, err := os.Create(t.join(p))
	assert.Loosely(t.t, err, should.ErrLike(nil), truth.LineContext())
	_, err = f.Write([]byte(data))
	assert.Loosely(t.t, err, should.ErrLike(nil), truth.LineContext())
	assert.Loosely(t.t, f.Close(), should.ErrLike(nil), truth.LineContext())
}

func (t tmpDir) touch(p string) {
	t.t.Helper()
	t.put(p, "")
}
