// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

import (
	"context"

	"go.chromium.org/infra/cros/recovery/internal/components/android/fastboot"
	"go.chromium.org/infra/cros/recovery/internal/execs"
)

// deviceInFastbootMode check if the device is in fastboot mode and returns error if not.
func deviceInFastbootMode(ctx context.Context, info *execs.ExecInfo) error {
	run := newRunner(info)
	serialNumber := info.GetAndroid().GetSerialNumber()
	return fastboot.IsDeviceInFastbootMode(ctx, run, serialNumber)
}

// fastbootReboot reboot an Android device that's in fastboot mode.
func fastbootReboot(ctx context.Context, info *execs.ExecInfo) error {
	run := newRunner(info)
	serialNumber := info.GetAndroid().GetSerialNumber()
	return fastboot.FastbootReboot(ctx, run, serialNumber)
}

func init() {
	execs.Register("android_device_in_fastboot_mode", deviceInFastbootMode)
	execs.Register("android_reboot_device_via_fastboot", fastbootReboot)
}
