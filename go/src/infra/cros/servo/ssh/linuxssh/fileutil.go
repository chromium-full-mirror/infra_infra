// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

// Package linuxssh provides Linux specific operations conducted via SSH
package linuxssh

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"go.chromium.org/infra/cros/servo/errors"
	"go.chromium.org/infra/cros/servo/ssh"
)

// SymlinkPolicy describes how symbolic links should be handled by PutFiles.
type SymlinkPolicy int

const (
	// PreserveSymlinks indicates that symlinks should be preserved during the copy.
	PreserveSymlinks SymlinkPolicy = iota
	// DereferenceSymlinks indicates that symlinks should be dereferenced and turned into normal files.
	DereferenceSymlinks
)

// WordCountInfo describes the result from the unix command wc.
type WordCountInfo struct {
	Lines int64
	Words int64
	Bytes int64
}

// GetFile copies a file or directory from the host to the local machine.
// dst is the full destination name for the file or directory being copied, not
// a destination directory into which it will be copied. dst will be replaced
// if it already exists.
func GetFile(ctx context.Context, s *ssh.Conn, src, dst string, symlinkPolicy SymlinkPolicy) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if err := os.RemoveAll(dst); err != nil {
		return err
	}

	path, close, err := getFile(ctx, s, src, dst, symlinkPolicy)
	if err != nil {
		return err
	}
	defer close()

	if err := os.Rename(path, dst); err != nil {
		return fmt.Errorf("moving local file failed: %w", err)
	}
	return nil
}

// getFile copies a file or directory from the host to the local machine.
// It creates a temporary directory under the directory of dst, and copies
// src to it. It returns the filepath where src has been copied to.
// Caller must call close to remove the temporary directory.
func getFile(ctx context.Context, s *ssh.Conn, src, dst string, symlinkPolicy SymlinkPolicy) (path string, close func() error, retErr error) {
	// Create a temporary directory alongside the destination path.
	td, err := os.MkdirTemp(filepath.Dir(dst), filepath.Base(dst)+".")
	if err != nil {
		return "", nil, fmt.Errorf("creating local temp dir failed: %w", err)
	}
	defer func() {
		if retErr != nil {
			os.RemoveAll(td)
		}
	}()
	close = func() error {
		return os.RemoveAll(td)
	}

	sb := filepath.Base(src)
	taropts := []string{"-c", "--gzip", "-C", filepath.Dir(src)}
	if symlinkPolicy == DereferenceSymlinks {
		taropts = append(taropts, "--dereference")
	}
	taropts = append(taropts, sb)
	rcmd := s.CommandContext(ctx, "tar", taropts...)
	p, err := rcmd.StdoutPipe()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	if err := rcmd.Start(); err != nil {
		return "", nil, fmt.Errorf("running remote tar failed: %w", err)
	}
	defer rcmd.Wait()
	defer rcmd.Abort()

	cmd := exec.CommandContext(ctx, "/bin/tar", "-x", "--gzip", "--no-same-owner", "-p", "-C", td)
	cmd.Stdin = p
	if err := cmd.Run(); err != nil {
		return "", nil, fmt.Errorf("running local tar failed: %w", err)
	}
	return filepath.Join(td, sb), close, nil
}

// RemoteFileDelta describes the result from the function NewRemoteFileDelta.
type RemoteFileDelta struct {
	src       string
	dst       string
	maxsize   int64
	startline int64
}

// NewRemoteFileDelta gets the starting line of from DUT and then save the
// source file, destintion file and the starting line to RemoteFileDelta which
// will be returned to the caller.
func NewRemoteFileDelta(ctx context.Context, conn *ssh.Conn, src, dst string, maxSize int64) (*RemoteFileDelta, error) {
	wordCountInfo, err := WordCount(ctx, conn, src)
	if err != nil {
		return nil, fmt.Errorf("failed the get line count: %w", err)
	}
	rtd := RemoteFileDelta{
		src:       src,
		dst:       dst,
		maxsize:   maxSize,
		startline: wordCountInfo.Lines + 1,
	}

	return &rtd, nil
}

// Save calls the GetFileTail with struct RemoteFileDelta. The range between
// beginning of the file and rtd.startline will be truncated.
func (rtd *RemoteFileDelta) Save(ctx context.Context, conn *ssh.Conn) error {
	return GetFileTail(ctx, conn, rtd.src, rtd.dst, rtd.startline, rtd.maxsize)
}

// PutFiles copies files on the local machine to the host. files describes
// a mapping from a local file path to a remote file path. For example, the call:
//
//	PutFiles(ctx, conn, map[string]string{"/src/from": "/dst/to"})
//
// will copy the local file or directory /src/from to /dst/to on the remote host.
// Local file paths can be absolute or relative. Remote file paths must be absolute.
// SHA1 hashes of remote files are checked in advance to send updated files only.
// bytes is the amount of data sent over the wire (possibly after compression).
func PutFiles(ctx context.Context, s *ssh.Conn, files map[string]string,
	symlinkPolicy SymlinkPolicy) (bytes int64, err error) {
	af := make(map[string]string)
	for src, dst := range files {
		if !filepath.IsAbs(src) {
			p, err := filepath.Abs(src)
			if err != nil {
				return 0, fmt.Errorf("source path %q could not be resolved", src)
			}
			src = p
		}
		if !filepath.IsAbs(dst) {
			return 0, fmt.Errorf("destination path %q should be absolute", dst)
		}
		af[src] = dst
	}

	// TODO(derat): When copying a small amount of data, it may be faster to avoid the extra
	// comparison round trip(s) and instead just copy unconditionally.
	cf, err := findChangedFiles(ctx, s, af)
	if err != nil {
		return 0, err
	}
	if len(cf) == 0 {
		return 0, nil
	}

	args := []string{"-c", "--gzip", "-C", "/"}
	if symlinkPolicy == DereferenceSymlinks {
		args = append(args, "--dereference")
	}
	for l, r := range cf {
		args = append(args, tarTransformFlag(strings.TrimPrefix(l, "/"), strings.TrimPrefix(r, "/")))
	}
	for l := range cf {
		args = append(args, strings.TrimPrefix(l, "/"))
	}
	cmd := exec.CommandContext(ctx, "/bin/tar", args...)
	p, err := cmd.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("failed to open stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("running local tar failed: %w", err)
	}
	defer cmd.Wait()
	defer unix.Kill(cmd.Process.Pid, unix.SIGKILL)

	var rcmd *ssh.Cmd
	if s.Type() == ssh.ADB {
		rcmd = s.CommandContext(ctx, "tar", "-f", "-", "-x", "--gzip", "--no-same-owner", "-p", "-C", "/")
	} else {
		rcmd = s.CommandContext(ctx, "tar", "-x", "--gzip", "--no-same-owner", "--recursive-unlink", "-p", "-C", "/")
	}

	cr := &countingReader{r: p}
	rcmd.Stdin = cr
	if err := rcmd.Run(ssh.DumpLogOnError); err != nil {
		return 0, fmt.Errorf("remote tar failed: %w", err)
	}
	return cr.bytes, nil
}

// ReadFile reads the file on the path and returns the contents.
func ReadFile(ctx context.Context, conn *ssh.Conn, path string) ([]byte, error) {
	return conn.CommandContext(ctx, "cat", path).Output(ssh.DumpLogOnError)
}

// GetFileTail reads the file on the path and returns the file truncate by command tail
// with the Max System Message Log Size and destination.
func GetFileTail(ctx context.Context, s *ssh.Conn, src, dst string, startLine, maxSize int64) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if err := os.RemoveAll(dst); err != nil {
		return err
	}

	path, close, err := getFileTail(ctx, s, src, dst, startLine, maxSize)
	if err != nil {
		return err
	}
	defer close()

	if err := os.Rename(path, dst); err != nil {
		return fmt.Errorf("moving local file failed: %w", err)
	}
	return nil
}

// getFileTail copies a file starting from startLine in src from the host to the local machine.
// It creates a temporary directory under the directory of dst, and copies
// src to it. It returns the filepath where src has been copied to.
// Caller must call close to remove the temporary directory.
func getFileTail(ctx context.Context, conn *ssh.Conn, src, dst string, startLine, maxSize int64) (path string, close func() error, retErr error) {
	// Create a temporary directory alongside the destination path.
	td, err := os.MkdirTemp(filepath.Dir(dst), filepath.Base(dst)+".")
	if err != nil {
		return "", nil, fmt.Errorf("creating local temp dir failed: %w", err)
	}
	defer func() {
		if retErr != nil {
			os.RemoveAll(td)
		}
	}()
	close = func() error {
		return os.RemoveAll(td)
	}

	// The first tail, "tail -n +%d %q", will print file starting from startLine to stdout.
	// The second tail, "tail -c +d", will truncate the beginning of stdin to fit maxSize bytes.
	// The gzip line will compress the data from stdin.
	tailCmd := fmt.Sprintf("tail -n +%d %q | tail -c %d | gzip -c", startLine, src, maxSize)
	rcmd := conn.CommandContext(ctx, "sh", "-c", tailCmd)

	p, err := rcmd.StdoutPipe()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	if err := rcmd.Start(); err != nil {
		return "", nil, fmt.Errorf("running remote gzip failed: %w", err)
	}
	defer rcmd.Wait()
	defer rcmd.Abort()

	sb := filepath.Base(src)
	outPath := filepath.Join(td, sb)
	outfile, err := os.Create(outPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary output file %v: %w", outPath, err)
	}
	defer outfile.Close()

	cmd := exec.CommandContext(ctx, "gzip", "-d")
	cmd.Stdin = p
	cmd.Stdout = outfile
	if err := cmd.Run(); err != nil {
		return "", nil, errors.Wrapf(err, "failed to unzip Data")
	}

	return outPath, close, nil
}

// findChangedFiles returns a subset of files that differ between the local machine
// and the remote machine. This function is intended for use when pushing files to s;
// an error is returned if one or more files are missing locally, but not if they're
// only missing remotely. Local directories are always listed as having been changed.
func findChangedFiles(ctx context.Context, s *ssh.Conn, files map[string]string) (map[string]string, error) {
	if len(files) == 0 {
		return nil, nil
	}

	// Sort local names.
	lp := make([]string, 0, len(files))
	for l := range files {
		lp = append(lp, l)
	}
	sort.Strings(lp)

	// TODO(derat): For large binary files, it may be faster to do an extra round trip first
	// to get file sizes. If they're different, there's no need to spend the time and
	// CPU to run sha1sum.
	rp := make([]string, len(lp))
	for i, l := range lp {
		rp[i] = files[l]
	}

	var lh, rh map[string]string
	ch := make(chan error, 2)
	go func() {
		var err error
		lh, err = getLocalSHA1s(lp)
		ch <- err
	}()
	go func() {
		var err error
		rh, err = getRemoteSHA1s(ctx, s, rp)
		ch <- err
	}()
	for i := 0; i < 2; i++ {
		if err := <-ch; err != nil {
			return nil, fmt.Errorf("failed to get SHA1(s): %w", err)
		}
	}

	cf := make(map[string]string)
	for i, l := range lp {
		r := rp[i]
		// TODO(derat): Also check modes, maybe.
		if lh[l] != rh[r] {
			cf[l] = r
		}
	}
	return cf, nil
}

type exitStatusError interface {
	ExitStatus() int
}

// getRemoteSHA1s returns SHA1s for the files paths on s.
// Missing files are excluded from the returned map.
func getRemoteSHA1s(ctx context.Context, s *ssh.Conn, paths []string) (map[string]string, error) {
	var out []byte
	// Getting shalsum for 1000 files at a time to avoid argument list too long with ssh.
	// b/270380606
	const numFilesToRead = 1000
	for i := 0; i < len(paths); i = i + numFilesToRead {
		endIndex := i + numFilesToRead
		if endIndex > len(paths) {
			endIndex = len(paths)
		}
		currentOut, err := s.CommandContext(ctx, "sha1sum", paths[i:endIndex]...).Output()
		if err != nil {
			// TODO(derat): Find a classier way to ignore missing files.
			if _, ok := err.(exitStatusError); ok {
				continue
			}
			return nil, fmt.Errorf("failed to hash files: %w", err)
		}
		out = append(out, currentOut...)
	}

	sums := make(map[string]string, len(paths))
	for _, l := range strings.Split(string(out), "\n") {
		if l == "" {
			continue
		}
		f := strings.SplitN(l, " ", 2)
		if len(f) != 2 {
			return nil, fmt.Errorf("unexpected line %q from sha1sum", l)
		}
		if len(f[0]) != 40 {
			return nil, fmt.Errorf("invalid sha1 in line %q from sha1sum", l)
		}
		sums[strings.TrimLeft(f[1], " ")] = f[0]
	}
	return sums, nil
}

// getLocalSHA1s returns SHA1s for files in paths.
// An error is returned if any files are missing.
func getLocalSHA1s(paths []string) (map[string]string, error) {
	sums := make(map[string]string, len(paths))

	for _, p := range paths {
		if fi, err := os.Stat(p); err != nil {
			return nil, err
		} else if fi.IsDir() {
			// Use a bogus hash for directories to ensure they're copied.
			sums[p] = "dir-hash"
			continue
		}

		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		h := sha1.New()
		if _, err := io.Copy(h, f); err != nil {
			return nil, err
		}
		sums[p] = hex.EncodeToString(h.Sum(nil))
	}

	return sums, nil
}

// tarTransformFlag returns a GNU tar --transform flag for renaming path s to d when
// creating an archive.
func tarTransformFlag(s, d string) string {
	esc := func(s string, bad []string) string {
		for _, b := range bad {
			s = strings.Replace(s, b, "\\"+b, -1)
		}
		return s
	}
	// Transform foo -> bar but not foobar -> barbar. Therefore match foo$ or foo/
	return fmt.Sprintf(`--transform=s,^%s\($\|/\),%s,`,
		esc(regexp.QuoteMeta(s), []string{","}),
		esc(d, []string{"\\", ",", "&"}))
}

// countingReader is an io.Reader wrapper that counts the transferred bytes.
type countingReader struct {
	r     io.Reader
	bytes int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	c, err := r.r.Read(p)
	r.bytes += int64(c)
	return c, err
}

// WriteFile writes data to the file on the path. If the file does not exist,
// WriteFile creates it with permissions perm; otherwise WriteFile truncates it
// before writing, without changing permissions.
// Unlike ioutil.WriteFile, it doesn't apply umask on perm.
func WriteFile(ctx context.Context, conn *ssh.Conn, path string, data []byte, perm os.FileMode) error {
	cmd := conn.CommandContext(ctx, "sh", "-c", `test -e "$0"; r=$?; cat > "$0"; if [ $r = 1 ]; then chmod "$1" "$0"; fi`, path, fmt.Sprintf("%o", perm&os.ModePerm))
	cmd.Stdin = bytes.NewBuffer(data)
	return cmd.Run(ssh.DumpLogOnError)
}

// WordCount get the line, word and byte counts of a remote text file.
func WordCount(ctx context.Context, conn *ssh.Conn, path string) (*WordCountInfo, error) {
	cmd := conn.CommandContext(ctx, "wc", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to call wc for %s", path)
	}
	// Output example: "   68201  1105834 14679551 /var/log/messages".
	strList := strings.Split(string(output), " ")
	var strs []string
	for _, s := range strList {
		if s != "" {
			strs = append(strs, s)
		}
	}
	if len(strs) < 3 {
		return nil, errors.Errorf("wc information is not available for %s", path)
	}
	lc, err := strconv.ParseInt(strs[0], 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse line count from string %s", string(output))
	}
	wc, err := strconv.ParseInt(strs[1], 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse word count from string %s", string(output))
	}
	bc, err := strconv.ParseInt(strs[2], 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse bytes count from string %s", string(output))
	}
	return &WordCountInfo{Lines: lc, Words: wc, Bytes: bc}, nil
}

// WaitUntilFileExists checks if a file exists on an interval until it either
// exists or the timeout is reached.
func WaitUntilFileExists(ctx context.Context, conn *ssh.Conn, path string, timeout, interval time.Duration) error {
	cmd := conn.CommandContext(ctx, "timeout", timeout.String(), "sh", "-c", `while [ ! -e "$0" ]; do sleep $1; done`, path, interval.String())
	return cmd.Run()
}
