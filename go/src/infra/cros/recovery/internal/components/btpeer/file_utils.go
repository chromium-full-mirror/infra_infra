// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package btpeer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
)

// AddLineToFile appends a single line option to a file if it does not already exist.
func AddLineToFile(ctx context.Context, runner components.Runner, filePath string, option string) error {
	// Check if line already exists.
	existsCmd := fmt.Sprintf("cat %q | grep -Fx %q", filePath, option)
	if out, err := runner(ctx, 30*time.Second, existsCmd); err == nil && strings.TrimSpace(out) != "" {
		// Nothing to do, already exists.
		return nil
	}

	optionCmd := fmt.Sprintf("'echo %q >> %q'", option, filePath)
	if _, err := runner(ctx, 30*time.Second, "bash", "-c", optionCmd); err != nil {
		return errors.Annotate(err, "add config option: failed to write %q to file %q", option, filePath).Err()
	}
	return nil
}

// Model is the btpeer HW model.
type Model int

const (
	// Model4B is a Rasperry Pi 4B
	Model4B Model = 17
)

// GetHWInfo returns the btpeer's model and revision.
func GetHWInfo(ctx context.Context, runner components.Runner) (Model, int, error) {
	const revisionCmd = "cat /proc/cpuinfo | awk '/Revision/ {print $3}'"
	versionStr, err := runner(ctx, 15*time.Second, revisionCmd)
	if err != nil {
		return 0, 0, errors.Annotate(err, "get hw info: failed to get revision number").Err()
	}
	versionStr = strings.TrimSpace(versionStr)

	// Parse the revision from the provided code.
	// The code is in hex with bits 0-3 representing the revision number
	// and bits 4 - 12 representing the model number
	// C03114 -> 1 100 0000 0011 00010001 0100
	//
	// See: https://www.raspberrypi.com/documentation/computers/raspberry-pi.html#old-style-revision-codes
	versionCode, err := strconv.ParseInt(versionStr, 16, 64)
	if err != nil {
		return 0, 0, errors.Annotate(err, "get hw info: failed to parse revision code: %q", versionStr).Err()
	}

	// Discard revision number and mask with: 11111111
	model := (versionCode >> 4) & 0xff

	// Lowest 4 bits are the revision version so mask with 1111.
	revision := (versionCode) & 0xf

	return Model(model), int(revision), err
}

// RemoveLineFromFile removes any matching lines from a config file.
func RemoveLineFromFile(ctx context.Context, runner components.Runner, filePath string, option string) error {
	// Check if line already exists.
	existsCmd := fmt.Sprintf("cat %q | grep -Fx %q", filePath, option)
	if out, err := runner(ctx, 30*time.Second, existsCmd); err != nil || strings.TrimSpace(out) == "" {
		// Line not found, nothing to do.
		return nil
	}

	optionCmd := fmt.Sprintf("sed -i '/^%s$/d' %s", option, filePath)
	if _, err := runner(ctx, 30*time.Second, optionCmd); err != nil {
		return errors.Annotate(err, "remove config option: failed to remove line %q to file %q", option, filePath).Err()
	}
	return nil
}

// BuildInitrd builds a new /boot/initrd.img image.
func BuildInitrd(ctx context.Context, runner components.Runner) error {
	const buildCmd = "update-initramfs -v -c -k $(uname -r)"
	if _, err := runner(ctx, 2*time.Minute, buildCmd); err != nil {
		return errors.Annotate(err, "build initrd: failed to re-create initrd.img").Err()
	}

	bootPath, err := GetCurrentBootPath(ctx, runner)
	if err != nil {
		return errors.Annotate(err, "build initrd: failed to get current boot path").Err()
	}

	copyCmd := fmt.Sprintf(`cp "/boot/initrd.img-$(uname -r)" %sinitrd.img`, bootPath)
	if _, err := runner(ctx, time.Minute, copyCmd); err != nil {
		return errors.Annotate(err, "build initrd: failed to copy initrd.img").Err()
	}
	return nil
}

// createExecutableScript writes an executable script to the btpeer.
func createExecutableScript(ctx context.Context, runner components.Runner, text string, filePath string) error {
	cmd := fmt.Sprintf("cat > %s <<\"EOF\"\n%s\nEOF", filePath, text)
	if _, err := runner(ctx, 30*time.Second, "bash", "-c", cmd); err != nil {
		return errors.Annotate(err, "write script: failed to write script to file: %q", filePath).Err()
	}

	if _, err := runner(ctx, 30*time.Second, "chmod", "+x", filePath); err != nil {
		return errors.Annotate(err, "write script: failed to make script executable: %q", filePath).Err()
	}

	return nil
}
