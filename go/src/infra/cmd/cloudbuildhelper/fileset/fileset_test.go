// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fileset

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestSet(t *testing.T) {
	t.Parallel()

	ftt.Run("Regular files", t, func(c *ftt.Test) {
		dir1 := newTempDir(c)
		dir1.touch("f1")
		dir1.mkdir("dir")
		dir1.touch("dir/a")
		dir1.mkdir("dir/empty")
		dir1.mkdir("dir/nested")
		dir1.touch("dir/nested/f")

		dir2 := newTempDir(c)
		dir2.touch("f2")
		dir2.mkdir("dir")
		dir2.touch("dir/b")

		dir3 := newTempDir(c)
		dir3.touch("f")

		s := &Set{}
		assert.Loosely(c, s.AddFromDisk(dir1.join(""), "", nil), should.BeNil)
		assert.Loosely(c, s.AddFromDisk(dir2.join(""), "", nil), should.BeNil)
		assert.Loosely(c, s.AddFromDisk(dir3.join(""), "dir/deep/", nil), should.BeNil)
		assert.Loosely(c, s.AddFromMemory("mem", nil, nil), should.BeNil)
		assert.Loosely(c, s.Len(), should.Equal(11))
		assert.Loosely(c, collect(s), should.Resemble([]string{
			"D dir",
			"F dir/a",
			"F dir/b",
			"D dir/deep",
			"F dir/deep/f",
			"D dir/empty",
			"D dir/nested",
			"F dir/nested/f",
			"F f1",
			"F f2",
			"F mem",
		}))
	})

	ftt.Run("Symlinks", t, func(c *ftt.Test) {
		addOne := func(path, target string) (*File, error) {
			s := Set{}
			if err := s.AddSymlink(path, target); err != nil {
				return nil, err
			}
			for _, f := range s.Files() {
				if !f.Directory {
					return &f, nil
				}
			}
			panic("impossible")
		}

		f, err := addOne("path", "target")
		assert.Loosely(c, err, should.BeNil)
		assert.Loosely(c, f, should.Resemble(&File{
			Path:          "path",
			SymlinkTarget: "target",
		}))

		f, err = addOne("a/b/c/path", "target")
		assert.Loosely(c, err, should.BeNil)
		assert.Loosely(c, f, should.Resemble(&File{
			Path:          "a/b/c/path",
			SymlinkTarget: "target",
		}))

		f, err = addOne("a/b/c/path", ".././.")
		assert.Loosely(c, err, should.BeNil)
		assert.Loosely(c, f, should.Resemble(&File{
			Path:          "a/b/c/path",
			SymlinkTarget: "..",
		}))

		f, err = addOne("a/b/c/path", "../..")
		assert.Loosely(c, err, should.BeNil)
		assert.Loosely(c, f, should.Resemble(&File{
			Path:          "a/b/c/path",
			SymlinkTarget: "../..",
		}))

		_, err = addOne("a/b/c/path", "../../..")
		assert.Loosely(c, err, should.ErrLike("is not in the set"))
	})

	ftt.Run("Reading body", t, func(c *ftt.Test) {
		s := &Set{}

		dir1 := newTempDir(c)
		dir1.put("f", "1", 0666)
		assert.Loosely(c, s.AddFromDisk(dir1.join(""), "", nil), should.BeNil)
		assert.Loosely(c, s.Files(), should.HaveLength(1))

		f, ok := s.File("f")
		assert.Loosely(c, ok, should.BeTrue)
		assert.Loosely(c, read(t, f), should.Equal("1"))

		dir2 := newTempDir(c)
		dir2.put("f", "2", 0666)
		assert.Loosely(c, s.AddFromDisk(dir2.join(""), "", nil), should.BeNil)
		assert.Loosely(c, s.Files(), should.HaveLength(1)) // overwritten

		// Overwritten.
		f, ok = s.File("f")
		assert.Loosely(c, ok, should.BeTrue)
		assert.Loosely(c, read(t, f), should.Equal("2"))
	})

	ftt.Run("Reading memfile", t, func(c *ftt.Test) {
		s := &Set{}
		assert.Loosely(c, s.AddFromMemory("mem", []byte("123456"), &File{
			Writable:   true,
			Executable: true,
		}), should.BeNil)
		files := s.Files()
		assert.Loosely(c, files, should.HaveLength(1))
		assert.Loosely(c, files[0].Writable, should.BeTrue)
		assert.Loosely(c, files[0].Executable, should.BeTrue)
		assert.Loosely(c, read(t, files[0]), should.Equal("123456"))
	})

	if runtime.GOOS != "windows" {
		ftt.Run("Recognizes read-only", t, func(c *ftt.Test) {
			s := &Set{}

			dir := newTempDir(c)
			dir.put("ro", "", 0444)
			dir.put("rw", "", 0666)
			assert.Loosely(c, s.AddFromDisk(dir.join(""), "", nil), should.BeNil)

			files := s.Files()
			assert.Loosely(c, files, should.HaveLength(2))
			assert.Loosely(c, files[0].Writable, should.BeFalse)
			assert.Loosely(c, files[1].Writable, should.BeTrue)
		})

		ftt.Run("Recognizes executable", t, func(c *ftt.Test) {
			s := &Set{}

			dir := newTempDir(c)
			dir.put("n", "", 0666)
			dir.put("y", "", 0777)
			assert.Loosely(c, s.AddFromDisk(dir.join(""), "", nil), should.BeNil)

			files := s.Files()
			assert.Loosely(c, files, should.HaveLength(2))
			assert.Loosely(c, files[0].Executable, should.BeFalse)
			assert.Loosely(c, files[1].Executable, should.BeTrue)
		})

		ftt.Run("Follows symlinks", t, func(c *ftt.Test) {
			dir := newTempDir(c)
			dir.touch("file")
			dir.mkdir("dir")
			dir.touch("dir/a")
			dir.mkdir("stage")
			dir.symlink("stage/filelink", "file")
			dir.symlink("stage/dirlink", "dir")
			dir.symlink("stage/broken", "broken") // skipped

			s := &Set{}
			assert.Loosely(c, s.AddFromDisk(dir.join("stage"), "", nil), should.BeNil)
			assert.Loosely(c, collect(s), should.Resemble([]string{
				"D dirlink",
				"F dirlink/a",
				"F filelink",
			}))
		})
	}

	ftt.Run("Materialize works", t, func(c *ftt.Test) {
		set := &Set{}
		set.Add(memFile("f", "hello"))
		set.Add(File{Path: "dir", Directory: true})
		set.Add(memFile("dir/f", "another"))

		rw := memFile("rw", "read-write")
		rw.Writable = true
		set.Add(rw)

		exe := memFile("exe", "executable")
		exe.Executable = runtime.GOOS != "windows"
		set.Add(exe)

		d := newTempDir(c)
		assert.Loosely(c, set.Materialize(d.join("")), should.BeNil)

		scanned := &Set{}
		assert.Loosely(c, scanned.AddFromDisk(d.join(""), "", nil), should.BeNil)
		assertEqualSets(t, scanned, set)
	})

	ftt.Run("ToTar works", t, func(c *ftt.Test) {
		s := prepSet()

		buf := bytes.Buffer{}
		tb := tar.NewWriter(&buf)
		assert.Loosely(c, s.ToTar(tb), should.BeNil)
		assert.Loosely(c, tb.Close(), should.BeNil)

		scan := &Set{}
		tr := tar.NewReader(&buf)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			assert.Loosely(c, err, should.BeNil)

			if hdr.Typeflag == tar.TypeDir {
				scan.Add(File{
					Path:      hdr.Name,
					Directory: true,
				})
				continue
			}

			if hdr.Typeflag == tar.TypeSymlink {
				scan.AddSymlink(hdr.Name, hdr.Linkname)
				continue
			}

			body := bytes.Buffer{}
			_, err = io.Copy(&body, tr)
			assert.Loosely(c, err, should.BeNil)

			f := memFile(hdr.Name, body.String())
			if runtime.GOOS != "windows" {
				f.Writable = (hdr.Mode & 0222) != 0
				f.Executable = (hdr.Mode & 0111) != 0
			}
			scan.Add(f)
		}

		assertEqualSets(t, s, scan)
	})

	ftt.Run("ToTarGz works", t, func(c *ftt.Test) {
		buf := bytes.Buffer{}
		assert.Loosely(c, prepSet().ToTarGz(&buf), should.BeNil)
		assert.Loosely(c, buf.Len(), should.NotEqual(0)) // writes something...
	})

	ftt.Run("ToTarGzFile works", t, func(c *ftt.Test) {
		hash, err := prepSet().ToTarGzFile(newTempDir(c).join("tmp"))
		assert.Loosely(c, err, should.BeNil)
		assert.Loosely(c, hash, should.HaveLength(64))
	})

	ftt.Run("Overlay set works", t, func(c *ftt.Test) {
		set := &Set{}
		set.Add(memFile("f1", "main"))
		set.Add(memFile("f2", "main"))
		set.Add(memFile("f3", "main"))

		set.Overlay().Add(memFile("f1", "overlay"))
		set.Overlay().Add(memFile("f4", "overlay"))

		assert.Loosely(c, set.Len(), should.Equal(4))
		assert.Loosely(c, collect(set), should.Resemble([]string{
			"F f1",
			"F f2",
			"F f3",
			"F f4",
		}))

		f1, _ := set.File("f1")
		assert.Loosely(c, read(t, f1), should.Equal("overlay"))
		f2, _ := set.File("f2")
		assert.Loosely(c, read(t, f2), should.Equal("main"))
		f4, _ := set.File("f4")
		assert.Loosely(c, read(t, f4), should.Equal("overlay"))
	})
}

func collect(s *Set) []string {
	out := []string{}
	s.Enumerate(func(f File) error {
		t := "F"
		if f.Directory {
			t = "D"
		}
		out = append(out, fmt.Sprintf("%s %s", t, f.Path))
		return nil
	})
	return out
}

func read(t testing.TB, f File) string {
	t.Helper()

	if f.Directory || f.SymlinkTarget != "" {
		return ""
	}
	blob, err := f.ReadAll()
	assert.Loosely(t, err, should.BeNil, truth.LineContext())
	return string(blob)
}

func prepSet() *Set {
	s := &Set{}
	s.Add(memFile("f", "hello"))
	s.Add(File{Path: "dir", Directory: true})
	s.Add(memFile("dir/f", "another"))
	s.AddSymlink("dir/link", "f")

	rw := memFile("rw", "read-write")
	rw.Writable = true
	s.Add(rw)

	exe := memFile("exe", "executable")
	exe.Executable = runtime.GOOS != "windows"
	s.Add(exe)

	return s
}

func memFile(path, body string) File {
	return File{
		Path:     path,
		Size:     int64(len(body)),
		Writable: runtime.GOOS == "windows", // FileMode perms don't work on windows
		Body: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(body)), nil
		},
	}
}

func assertEqualSets(t testing.TB, a, b *Set) {
	t.Helper()
	aMeta, aBodies := splitBodies(t, a.Files())
	bMeta, bBodies := splitBodies(t, b.Files())
	assert.Loosely(t, aMeta, should.Resemble(bMeta), truth.LineContext())
	assert.Loosely(t, aBodies, should.Resemble(bBodies), truth.LineContext())
}

func splitBodies(t testing.TB, fs []File) (files []File, bodies map[string]string) {
	t.Helper()

	files = make([]File, len(fs))
	bodies = make(map[string]string, len(fs))
	for i, f := range fs {
		bodies[f.Path] = read(t, f)
		f.Body = nil
		files[i] = f
	}
	return
}

type tmpDir struct {
	p string
	t testing.TB
}

func newTempDir(t testing.TB) tmpDir {
	t.Helper()
	tmp, err := os.MkdirTemp("", "fileset_test")
	assert.Loosely(t, err, should.BeNil, truth.LineContext())
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	return tmpDir{tmp, t}
}

func (t tmpDir) join(p string) string {
	return filepath.Join(t.p, filepath.FromSlash(p))
}

func (t tmpDir) mkdir(p string) {
	t.t.Helper()
	assert.Loosely(t.t, os.MkdirAll(t.join(p), 0777), should.ErrLike(nil), truth.LineContext())
}

func (t tmpDir) put(p, data string, mode os.FileMode) {
	t.t.Helper()
	f, err := os.OpenFile(t.join(p), os.O_CREATE|os.O_WRONLY, mode)
	assert.Loosely(t.t, err, should.ErrLike(nil), truth.LineContext())
	_, err = f.Write([]byte(data))
	assert.Loosely(t.t, err, should.ErrLike(nil), truth.LineContext())
	assert.Loosely(t.t, f.Close(), should.ErrLike(nil), truth.LineContext())
}

func (t tmpDir) touch(p string) {
	t.t.Helper()

	t.put(p, "", 0666)
}

func (t tmpDir) symlink(name, target string) {
	t.t.Helper()

	assert.Loosely(t.t, os.Symlink(t.join(target), t.join(name)), should.BeNil, truth.LineContext())
}
