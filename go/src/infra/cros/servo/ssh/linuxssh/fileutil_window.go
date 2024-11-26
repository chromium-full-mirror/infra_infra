//Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build windows
// +build windows

// Package linuxssh provides Linux specific operations conducted via SSH
package linuxssh

import (
	"time"

	"context"
	"os"

	"infra/cros/servo/errors"
	"infra/cros/servo/ssh"
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
	return errors.New("the function GetFiles is not supported in window")
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
	return nil, errors.New("the function RemoteFileDelta, is not supported in window")
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
	return 0, errors.New("the function PutFiles is not supported in window")
}

// ReadFile reads the file on the path and returns the contents.
func ReadFile(ctx context.Context, conn *ssh.Conn, path string) ([]byte, error) {
	return nil, errors.New("the function ReadFile is not supported in window")
}

// GetFileTail reads the file on the path and returns the file truncate by command tail
// with the Max System Message Log Size and destination.
func GetFileTail(ctx context.Context, s *ssh.Conn, src, dst string, startLine, maxSize int64) error {
	return errors.New("the function GetFileTail is not supported in window")
}

// WriteFile writes data to the file on the path. If the file does not exist,
// WriteFile creates it with permissions perm; otherwise WriteFile truncates it
// before writing, without changing permissions.
// Unlike ioutil.WriteFile, it doesn't apply umask on perm.
func WriteFile(ctx context.Context, conn *ssh.Conn, path string, data []byte, perm os.FileMode) error {
	return errors.New("the function WriteFile is not supported in window")
}

// WordCount get the line, word and byte counts of a remote text file.
func WordCount(ctx context.Context, conn *ssh.Conn, path string) (*WordCountInfo, error) {
	return nil, errors.New("the function WordCount is not supported in window")
}

// WaitUntilFileExists checks if a file exists on an interval until it either
// exists or the timeout is reached.
func WaitUntilFileExists(ctx context.Context, conn *ssh.Conn, path string, timeout, interval time.Duration) error {
	return errors.New("the function WaitUntilFileExists is not supported in window")
}
