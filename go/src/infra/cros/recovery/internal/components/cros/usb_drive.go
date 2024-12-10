// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/components/cros/storage"
	"infra/cros/recovery/internal/components/servo"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/internal/retry"
)

// BootFromServoUSBDriveInDevMode performs booting device from external storage when DUT is in DEV-mode.
// Device already should to be in DEV-mode and enabled to boot from USB-drive.
// The make to be able boot from USB-drive you need one of followed options:
// 1) Run enable_dev_usb_boot.
// 2) Set GBB with GBB_FLAG_FORCE_DEV_BOOT_USB flag.
//
// Steps:
// 1) Power off the host.
// 2) Trigger reboot by servo.
// 3) Perform ctrl+u by servo to try out boot from external storage.
func BootFromServoUSBDriveInDevMode(ctx context.Context, waitBootTimeout, waitBootInterval time.Duration, dutRun components.Runner, ping components.Pinger, servod components.Servod) error {
	if err := servo.UpdateUSBVisibility(ctx, servo.USBVisibleDUT, servod); err != nil {
		return errors.Annotate(err, "boot from servo usb drive in dev mode").Err()
	}
	if err := servod.Set(ctx, "power_state", "reset"); err != nil {
		return errors.Annotate(err, "boot from servo usb drive in dev mode").Err()
	}
	// Try to boot from UBS-drive so some period of time.
	err := retry.WithTimeout(ctx, waitBootInterval, waitBootTimeout, func() error {
		log.Debugf(ctx, "Pressing ctrl+u")
		if err := servod.Set(ctx, "ctrl_u", "tab"); err != nil {
			return errors.Annotate(err, "wait for device boot").Err()
		}
		// Ping only once to safe time and do not miss the boot time frame.
		if err := IsPingable(ctx, 1, ping); err != nil {
			return errors.Annotate(err, "wait for device boot").Err()
		}
		log.Debugf(ctx, "Device started booting!")
		return nil
	}, "wait to boot")
	if err != nil {
		return errors.Annotate(err, "boot from servo usb drive in dev mode").Err()
	}
	if err := WaitUntilSSHable(ctx, time.Minute, SSHRetryInterval, dutRun); err != nil {
		return errors.Annotate(err, "wait for device boot").Err()
	}
	// List information about block devices.
	// This informcation helps to understand which devices present and visible on the DUT.
	if _, err := dutRun(ctx, 10*time.Second, "lsblk"); err != nil {
		log.Infof(ctx, "Fail to list device of the DUT: %s", err)
	}
	// In some cases the device can boot from internal storage by multiple reasons.
	// Most prevident issues:
	// 1) Image on USB-drive is bad.
	// 2) Booting from USB-drive is not allowed.
	// 3) Device is not in DEV mode.
	if err := storage.IsBootedFromExternalStorage(ctx, dutRun); err != nil {
		return errors.Annotate(err, "boot from servo usb drive in dev mode").Err()
	}
	return nil
}
