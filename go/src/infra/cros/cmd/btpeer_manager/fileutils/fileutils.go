// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fileutils defines generic file utilities.
package fileutils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// CopyFile copies the file or directory at srcFilePath to dstPath. If dstPath
// is a directory, the file is placed within it.
func CopyFile(ctx context.Context, srcFilePath string, dstPath string) error {
	cmd := exec.CommandContext(ctx, "cp", "-R", srcFilePath, dstPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file %q to dir %q: %w", srcFilePath, dstPath, err)
	}
	return nil
}

// ReadFile reads a file's entire contents.
func ReadFile(ctx context.Context, filePath string) ([]byte, error) {
	inFile, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", filePath, err)
	}
	defer func() {
		_ = inFile.Close()
	}()
	inFileReader := NewContextualReaderWrapper(ctx, inFile)
	return io.ReadAll(inFileReader)
}

// MarshalPrettyJSON marshals the proto message into pretty, human-readable JSON.
func MarshalPrettyJSON(m proto.Message) ([]byte, error) {
	marshaller := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: true,
	}
	return marshaller.Marshal(m)
}

// WriteBytesToFile writes input bytes to file.
// Directories in path are created if they do not exist.
// Existing file contents are overwritten.
func WriteBytesToFile(ctx context.Context, input []byte, outFilePath string) error {
	outFileDir := path.Dir(outFilePath)
	if outFileDir != "" {
		if err := os.MkdirAll(outFileDir, os.FileMode(0777)); err != nil {
			return fmt.Errorf("failed to make dirs for file %q: %w", outFilePath, err)
		}
	}
	outFile, err := os.Create(outFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", outFilePath, err)
	}
	defer func() {
		_ = outFile.Close()
	}()
	outFileWriter := NewContextualWriterWrapper(ctx, outFile)
	if _, err := outFileWriter.Write(input); err != nil {
		return fmt.Errorf("failed to write to file %q: %w", outFilePath, err)
	}
	return nil
}

// PathExists returns whether the path exists. Returns a non-nil error if
// unable to determine if the path exists.
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check if path %q exists: %w", path, err)
}
