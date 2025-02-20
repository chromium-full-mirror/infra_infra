// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fastboot

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
)

// IsDeviceInFastbootMode checks if an Android device is in fastboot mode.
func IsDeviceInFastbootMode(ctx context.Context, run components.Runner, serialNumber string) error {
	fastbootCheckCmd := fmt.Sprintf("fastboot devices | grep %s", serialNumber)
	out, err := run(ctx, time.Minute, fastbootCheckCmd)
	if err != nil {
		return errors.Annotate(err, "device in fastboot mode").Err()
	}
	if out == "" {
		return errors.Reason("device %s is not in fastboot mode", serialNumber).Err()
	}
	return nil
}

// FastbootReboot reboot an Android device via fastboot, it assumes the device is already in fastboot mode.
func FastbootReboot(ctx context.Context, run components.Runner, serialNumber string) error {
	fastbootRebootCmd := fmt.Sprintf("fastboot -s %s reboot", serialNumber)
	if _, err := run(ctx, time.Minute, fastbootRebootCmd); err != nil {
		return errors.Annotate(err, "fastboot reboot").Err()
	}
	return nil
}
