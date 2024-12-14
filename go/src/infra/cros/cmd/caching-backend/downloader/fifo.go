// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func createFifoForSource(dir string, s *sourceFile) (string, error) {
	fifoPath := filepath.Join(dir, strings.Replace(s.name, "/", "_", -1)+"-fifo")
	err := unix.Mkfifo(fifoPath, 0666)
	if err != nil {
		return "", fmt.Errorf("create fifo for %q: %w", s.name, err)
	}
	return fifoPath, nil
}
