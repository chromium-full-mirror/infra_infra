// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package statemachine contains the individual states representing the kernel
// provision state machine.
package statemachine

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMakeExecutable(t *testing.T) {
	// The x bit is not used in Windows, so test cases would be doomed to fail.
	if runtime.GOOS == "windows" {
		return
	}

	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		// Create a temporary file
		tmpFile, err := os.CreateTemp(t.TempDir(), "testfile-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		filePath := tmpFile.Name()
		tmpFile.Close() // Close the file before changing permissions.

		// Initial mode: 0764 = rwxrw-r--
		initialMode := os.FileMode(0764)
		if err := os.Chmod(filePath, initialMode); err != nil {
			t.Fatalf("Failed to set initial permissions: %v", err)
		}

		if err = makeExecutable(filePath); err != nil {
			t.Fatalf("makeExecutable failed: %v", err)
		}

		fileInfo, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("Failed to stat file after makeExecutable: %v", err)
		}

		// Expected mode: 0775 = rwxrwxr-x
		expectedMode := os.FileMode(0775)
		if fileInfo.Mode() != expectedMode {
			t.Errorf("Expected file mode %v, got %v", expectedMode, fileInfo.Mode())
		}
	})

	t.Run("file_not_exist", func(t *testing.T) {
		t.Parallel()
		nonExistentPath := filepath.Join(t.TempDir(), "nonexistentfile.sh")
		err := makeExecutable(nonExistentPath)
		if err == nil {
			t.Fatalf("makeExecutable should have failed for a non-existent file, but it didn't")
		}
	})
}
