// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package uefi contains exec functions for devices with UEFI firmware.
package uefi

import (
	"context"
	"regexp"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/log"
)

// Get the device number for the servo-attached USB.
func findDeviceNumInOutput(output string) (string, error) {
	re := regexp.MustCompile(`^Boot(?P<number>[0-9A-F]{4})\* (UEFI OS\tHD|USB HDD)`)
	lines := strings.Split(output, "\n")
	for i := 0; i < len(lines); i++ {
		matches := re.FindStringSubmatch(lines[i])
		// Continue if the line does not have a match.
		if matches == nil {
			continue
		}
		numberIndex := re.SubexpIndex("number")
		return matches[numberIndex], nil
	}
	return "", errors.Reason("flex USB device number: not found").Err()
}

// SetUSBForNextBoot sets USB-drive as next boot device for the DUT.
func SetUSBForNextBoot(ctx context.Context, run components.Runner) error {
	timeout := 2 * time.Second
	// Get the output of `efibootmgr`.
	out, err := run(ctx, timeout, "efibootmgr")
	if err != nil {
		return errors.Annotate(err, "set USB for next boot: failed to read efibootmgr").Err()
	}
	devnum, err := findDeviceNumInOutput(out)
	log.Debugf(ctx, "USB-drive number is: %s", devnum)
	if err != nil {
		return err
	}
	_, err = run(ctx, timeout, "efibootmgr", "--bootnext", devnum)
	return errors.Annotate(err, "set USB for next boot: failed to set efibootmgr bootnext").Err()
}
