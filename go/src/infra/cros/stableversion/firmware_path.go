// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"fmt"
	"regexp"
)

var firmwarePathPattern *regexp.Regexp = regexp.MustCompile(`.*/R[0-9]+-(?P<sv>[0-9]+\.[0-9]+\.[0-9]+).*`)

// ValidateFirmwarePath checks that a given firmware path is well-formed
// such as "octopus-firmware/R72-11297.75.0"
// or      "octopus-release/R72-11297.75.0"
func ValidateFirmwarePath(path string) error {
	_, err := parseFirmwarePath(path)
	return err
}

func parseFirmwarePath(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("firmware path cannot be empty")
	}
	if firmwarePathPattern.FindString(s) == "" {
		return "", fmt.Errorf("firmware path is not valid: %q", s)
	}
	m, err := findMatchMap(firmwarePathPattern, s)
	if err != nil {
		return "", err
	}
	version, err := extractString(m, "sv")
	if err != nil {
		return "", err
	}
	return version, nil
}
