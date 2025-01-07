// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mh contains functions to work with mobileharness.
package mh

import (
	"os"
)

var (
	// Store cache of decision if that is MH or not.
	isHHDecision = ""
)

const (
	// Path to ADB in MH container.
	ADBPath = "/usr/local/google/mobileharness/ate/adb"

	// Decision answers.
	decisionYes = "yes"
	decisionNo  = "no"
)

// IsMH tells if execution is in MH container.
func IsMH() bool {
	if isHHDecision == "" {
		if _, err := os.Stat(ADBPath); err == nil {
			isHHDecision = decisionYes
		} else {
			isHHDecision = decisionNo
		}
	}
	return isHHDecision == decisionYes
}
