// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build windows
// +build windows

package main

import "log"

func createFifoForSource(dir string, s *sourceFile) (string, error) {
	log.Fatal("windows not supported")
	return "", nil
}
