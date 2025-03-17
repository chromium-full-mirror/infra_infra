// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"fmt"
	"regexp"
)

var firmwareVersionPattern *regexp.Regexp = regexp.MustCompile(`Google[-_a-zA-Z0-9]*.(?P<sv>[0-9]+\.[0-9]+\.[_0-9]+).*`)

// ParseFirmwareVersion takes a read-write firmware version and extracts
// semantically meaningful elements.
func ParseFirmwareVersion(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("firmware version cannot be empty")
	}
	if firmwareVersionPattern.FindString(s) == "" {
		return "", fmt.Errorf("firmware version is not valid: %q", s)
	}
	m, err := findMatchMap(firmwareVersionPattern, s)
	if err != nil {
		return "", err
	}
	version, err := extractString(m, "sv")
	if err != nil {
		return "", err
	}
	return version, nil
}

// ValidateFirmwareVersion checks whether a string is a valid read-write
// firmware version. e.g. Google_Rammus.11275.41.0
func ValidateFirmwareVersion(r string) error {
	_, err := ParseFirmwareVersion(r)
	return err
}
