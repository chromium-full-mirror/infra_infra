// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fileset contains an abstraction for a set of files.
package fileset

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/errors"
)

// File is a file inside a file set.
type File struct {
	Path          string // file path using "/" separator
	Directory     bool   // true if this is a directory
	SymlinkTarget string // non-empty if this is a symlink

	Size       int64 // size of the file, only for regular files
	Writable   bool  // true if the file is writable, only for regular files
	Executable bool  // true if the file is executable, only for regular files

	Body func() (io.ReadCloser, error) // emits the body, only for regular files
}

// ReadAll reads the body of this file if this is a regular file.
func (f *File) ReadAll() ([]byte, error) {
	if f.Body == nil {
		return nil, errors.New("not a regular file")
	}
	rc, err := f.Body()
	if err != nil {
		return nil, err
	}
	blob, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	if err := rc.Close(); err != nil {
		return nil, err
	}
	return blob, nil
}

// normalize clears redundant fields and converts file paths to Unix style.
//
// Returns an error if the file entry is invalid.
func (f *File) normalize() error {
	f.Path = path.Clean(filepath.ToSlash(f.Path))
	if f.Path == "." || strings.HasPrefix(f.Path, "../") {
		return errors.Fmt("bad file path %q, not in the set", f.Path)
	}
	switch {
	case f.Directory:
		f.SymlinkTarget = ""
		f.Size = 0
		f.Writable = false
		f.Executable = false
		f.Body = nil
	case f.SymlinkTarget != "":
		f.SymlinkTarget = path.Clean(filepath.ToSlash(f.SymlinkTarget))
		targetAbs := path.Clean(path.Join(path.Dir(f.Path), f.SymlinkTarget))
		if targetAbs == "." || strings.HasPrefix(targetAbs, "../") {
			return errors.Fmt("bad symlink %q, its target %q is not in the set", f.Path, f.SymlinkTarget)
		}
		f.Size = 0
		f.Writable = false
		f.Executable = false
		f.Body = nil
	}
	return nil
}

// filePerm returns FileMode with file permissions.
func (f *File) filePerm() os.FileMode {
	var mode os.FileMode = 0444
	if f.Writable {
		mode |= 0200
	}
	if f.Executable {
		mode |= 0111
	}
	return mode
}

// Excluder takes an absolute path to a file on disk and returns true or false.
type Excluder func(absPath string, isDir bool) bool

// Set represents a set of regular files, directories and symlinks.
//
// Such set can be constructed from existing files on disk (perhaps scattered
// across many directories), and it then can be either materialized on disk
// in some root directory, or written into a tarball.
//
// A set can optionally have an overlay set. Files in the overlay set are always
// written to the output instead of files in the main set. This is useful for
// emitting "overrides" that work regardless in which order files are added
// to the main set.
type Set struct {
	files   map[string]File // unix-style path inside the set => File
	overlay *Set            // the overlay set, initialized lazily
}

// Overlay returns a set of files that "override" files in the main set when
// the set is materialized or enumerated.
//
// Note that overriding regular files with directories in the overlay set is
// not supported and will result in errors when trying to materialize such set.
func (s *Set) Overlay() *Set {
	if s.overlay == nil {
		s.overlay = &Set{}
	}
	return s.overlay
}

// Add adds a file or directory to the set, overriding an existing one, if any.
//
// Adds all intermediary directories, if necessary.
//
// Returns an error if the file path is invalid (e.g. starts with "../"").
func (s *Set) Add(f File) error {
	if err := f.normalize(); err != nil {
		return err
	}

	if s.files == nil {
		s.files = make(map[string]File, 1)
	}

	// Add intermediary directories. Bail if some of them are already added as
	// regular files.
	cur := ""
	for _, chr := range f.Path {
		if chr == '/' {
			switch existing, ok := s.files[cur]; {
			case !ok:
				s.files[cur] = File{Path: cur, Directory: true}
			case ok && !existing.Directory:
				return errors.Fmt("%q in file path %q is not a directory", cur, f.Path)
			}
		}
		cur += string(chr)
	}

	// Add the leaf file.
	s.files[f.Path] = f
	return nil
}

// AddFromDisk adds a given file or directory to the set.
//
// A file or directory located at 'fsPath' on disk will become 'setPath' in
// the set. Directories are added recursively. Symlinks are always expanded into
// whatever they point to. Broken symlinks are silently skipped. To add a
// symlink explicitly use AddSymlink.
func (s *Set) AddFromDisk(fsPath, setPath string, exclude Excluder) error {
	fsPath, err := filepath.Abs(fsPath)
	if err != nil {
		return err
	}
	setPath = path.Clean(filepath.ToSlash(setPath))
	return s.addImpl(fsPath, setPath, exclude)
}

// AddFromMemory adds the given blob to the set as a file.
//
// 'blob' is retained as a pointer, the memory is not copied.
//
// 'f', if not nil, is used to populate the file metadata. If nil, the blob is
// added as a non-executable read-only file.
func (s *Set) AddFromMemory(setPath string, blob []byte, f *File) error {
	nf := File{}
	if f != nil {
		nf = *f
	}
	nf.Path = setPath
	nf.Directory = false
	nf.Size = int64(len(blob))
	nf.Body = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(blob)), nil
	}
	return s.Add(nf)
}

// AddSymlink adds a relative symlink to the set.
//
// Doesn't verify that the target exists in the set.
func (s *Set) AddSymlink(setPath, target string) error {
	if target == "" {
		return errors.New("symlink target can't be empty")
	}
	return s.Add(File{
		Path:          setPath,
		SymlinkTarget: target,
	})
}

// Len returns number of files in the set (including the overlay set).
func (s *Set) Len() int {
	if s.overlay == nil {
		return len(s.files)
	}
	paths := stringset.New(len(s.files))
	s.enumeratePaths(paths)
	return paths.Len()
}

// Enumerate calls the callback for each file in the set, in alphabetical order.
//
// Returns whatever error the callback returns.
func (s *Set) Enumerate(cb func(f File) error) error {
	paths := stringset.New(len(s.files))
	s.enumeratePaths(paths)
	for _, n := range paths.ToSortedSlice() {
		file, ok := s.get(n)
		if !ok {
			panic(fmt.Sprintf("path %q is in enumeratePaths set, but get(...) can't get it", n))
		}
		if err := cb(file); err != nil {
			return err
		}
	}
	return nil
}

// Files returns all files in the set, in alphabetical order.
func (s *Set) Files() []File {
	out := make([]File, 0, len(s.files))
	_ = s.Enumerate(func(f File) error {
		out = append(out, f)
		return nil
	})
	return out
}

// File returns an existing file in the set, if any.
//
// If there's no such file returns `File{}, false`.
func (s *Set) File(setPath string) (File, bool) {
	return s.get(path.Clean(filepath.ToSlash(setPath)))
}

// Materialize dumps all files in this set into the given directory.
//
// The directory will be created if it doesn't exist. If it exists, the contents
// of 's' will be written on top of whatever is in the directory already.
//
// Doesn't cleanup on errors.
func (s *Set) Materialize(root string) error {
	if err := os.MkdirAll(root, 0777); err != nil {
		return errors.Fmt("failed to create the output directory: %w", err)
	}
	buf := make([]byte, 64*1024)
	return s.Enumerate(func(f File) error {
		p := filepath.Join(root, filepath.FromSlash(f.Path))
		switch {
		case f.Directory:
			return os.Mkdir(p, 0700)
		case f.SymlinkTarget != "":
			return os.Symlink(f.SymlinkTarget, p)
		}

		r, err := f.Body()
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()

		w, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.filePerm())
		if err != nil {
			return err
		}
		defer func() { _ = w.Close() }() // this is for early exits, we'll also explicitly close later

		copied, err := io.CopyBuffer(w, r, buf)
		if err != nil {
			return err
		}
		if copied != f.Size {
			return errors.Fmt("file %q has unexpected size (expecting %d, got %d)", f.Path, f.Size, copied)
		}
		return w.Close()
	})
}

// ToTar dumps all files in this set into a tar.Writer.
func (s *Set) ToTar(w *tar.Writer) error {
	buf := make([]byte, 64*1024)
	return s.Enumerate(func(f File) (err error) {
		defer func() {
			if err != nil {
				err = errors.Fmt("when tarring %q: %w", f.Path, err)
			}
		}()

		switch {
		case f.Directory:
			return w.WriteHeader(&tar.Header{
				Typeflag: tar.TypeDir,
				Name:     f.Path + "/",
				Mode:     0755,
			})
		case f.SymlinkTarget != "":
			return w.WriteHeader(&tar.Header{
				Typeflag: tar.TypeSymlink,
				Name:     f.Path,
				Linkname: f.SymlinkTarget,
				Mode:     0444,
			})
		}

		err = w.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     f.Path,
			Size:     f.Size,
			Mode:     int64(f.filePerm()),
		})
		if err != nil {
			return err
		}

		r, err := f.Body()
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()

		_, err = io.CopyBuffer(w, r, buf)
		return err
	})
}

// ToTarGz writes a *.tar.gz with files in the set to an io.Writer.
//
// Uses default compression level.
func (s *Set) ToTarGz(w io.Writer) error {
	gz := gzip.NewWriter(w)
	tb := tar.NewWriter(gz)
	if err := s.ToTar(tb); err != nil {
		_ = tb.Close()
		_ = gz.Close()
		return err
	}
	if err := tb.Close(); err != nil {
		_ = gz.Close()
		return err
	}
	if err := gz.Close(); gz != nil {
		return err
	}
	return nil
}

// ToTarGzFile writes a *.tar.gz with files in the set to a file on disk.
//
// Calculates its SHA256 on the fly and returns the digest as a hex string.
func (s *Set) ToTarGzFile(path string) (sha256hex string, err error) {
	out, err := os.Create(path)
	if err != nil {
		return "", errors.Fmt("failed to open for writing %s: %w", path, err)
	}
	defer func() { _ = out.Close() }() // for early exits
	h := sha256.New()
	if err := s.ToTarGz(io.MultiWriter(out, h)); err != nil {
		return "", errors.Fmt("failed to write to %s: %w", path, err)
	}
	if err := out.Close(); err != nil {
		return "", errors.Fmt("failed to flush %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

////////////////////////////////////////////////////////////////////////////////

// get returns a file given its normalized set path.
//
// Knows about the overlay set.
func (s *Set) get(setPath string) (File, bool) {
	if s.overlay != nil {
		if file, ok := s.overlay.get(setPath); ok {
			return file, true
		}
	}
	file, ok := s.files[setPath]
	return file, ok
}

// enumeratePaths adds all paths in the set into `out`.
//
// Knows about the overlay set.
func (s *Set) enumeratePaths(out stringset.Set) {
	if s.overlay != nil {
		s.overlay.enumeratePaths(out)
	}
	for p := range s.files {
		out.Add(p)
	}
}

// addImpl implements AddFromDisk.
func (s *Set) addImpl(fsPath, setPath string, exclude Excluder) error {
	switch stat, err := os.Stat(fsPath); {
	case os.IsNotExist(err):
		if _, lerr := os.Lstat(fsPath); lerr == nil {
			return nil // fsPath is a broken symlink, skip it
		}
		return err
	case err != nil:
		return err
	case stat.Mode().IsRegular():
		if exclude != nil && exclude(fsPath, false) {
			return nil
		}
		return s.addReg(fsPath, setPath, stat)
	case stat.Mode().IsDir():
		if exclude != nil && exclude(fsPath, true) {
			return nil
		}
		return s.addDir(fsPath, setPath, exclude)
	default:
		return errors.Fmt("file %q has unsupported type, its mode is %s", fsPath, stat.Mode())
	}
}

// addReg adds a regular file to the set.
func (s *Set) addReg(fsPath, setPath string, fi os.FileInfo) error {
	return s.Add(File{
		Path:       setPath,
		Size:       fi.Size(),
		Writable:   (fi.Mode() & 0222) != 0,
		Executable: (fi.Mode() & 0111) != 0,
		Body:       func() (io.ReadCloser, error) { return os.Open(fsPath) },
	})
}

// addDir recursively adds a directory to the set.
func (s *Set) addDir(fsPath, setPath string, exclude Excluder) error {
	// Don't add the set root itself, it is always implied. Allowing it explicitly
	// causes complication related to dealing with ".".
	if setPath != "." {
		if err := s.Add(File{Path: setPath, Directory: true}); err != nil {
			return err
		}
	}

	f, err := os.Open(fsPath)
	if err != nil {
		return err
	}
	files, err := f.Readdirnames(-1)
	if err != nil {
		return err
	}
	_ = f.Close()

	for _, f := range files {
		if err := s.addImpl(filepath.Join(fsPath, f), path.Join(setPath, f), exclude); err != nil {
			return err
		}
	}

	return nil
}
