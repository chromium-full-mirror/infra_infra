// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mh contains functions to work with mobileharness.
package mh

import (
	"os"
	"strings"
)

var (
	// Tells if that is MH or not.
	IsMH = false
	// Path to ADB in MH container.
	adbPath = ""
)

func init() {
	IsMH = strings.TrimSpace(os.Getenv("PARIS_ADB_PATH")) != ""
}
