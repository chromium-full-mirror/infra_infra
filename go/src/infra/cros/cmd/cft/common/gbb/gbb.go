// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gbb implements interactions with GBB flags on the DUT.

package gbb

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"go.chromium.org/infra/cros/cmd/cft/common/adb"
)

// GetGBBFlags returns the DUT's current GBB flags.
func GetGBBFlags(ctx context.Context, log *log.Logger, dutAddress string) (string, error) {
	if err := adb.RetrySetupAdb(log, dutAddress, time.Minute); err != nil {
		return "", fmt.Errorf("connecting to adb: %w", err)
	}
	shellCommand := []string{"su", "root", "futility", "gbb", "--get", "--flash", "--flags"}
	out, err := adb.AdbShellCmd(shellCommand, dutAddress, log, 3, 30)
	if err != nil {
		return "", fmt.Errorf("running futility over adb: %w", err)
	}
	m := regexp.MustCompile(`^flags: (0x[0-9a-f]+)$`).FindStringSubmatch(strings.TrimSpace(out))
	if m == nil {
		return "", fmt.Errorf("unexpected gbb flags response from futility: %s", out)
	}
	return m[1], nil
}

// SetGBBFlags sets the DUT's current GBB flags.
// newFlags can either be an absolute value (like "0x80000001") or a mask (like "+0x00000001" or "-0x00000001").
func SetGBBFlags(ctx context.Context, log *log.Logger, dutAddress, newFlags string) error {
	if !regexp.MustCompile(`^[+-]?0x[0-9a-f]+$`).MatchString(newFlags) {
		return fmt.Errorf("unexpected new gbb flags: %s", newFlags)
	}
	if err := adb.RetrySetupAdb(log, dutAddress, time.Minute); err != nil {
		return fmt.Errorf("connecting to adb: %w", err)
	}
	shellCommand := []string{"su", "root", "futility", "gbb", "--set", "--flash", "--flags", newFlags}
	_, err := adb.AdbShellCmd(shellCommand, dutAddress, log, 3, 30)
	if err != nil {
		return fmt.Errorf("running adb shell command '%+v': %w", shellCommand, err)
	}
	return nil
}
