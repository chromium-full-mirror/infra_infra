// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/android/adb"
	"go.chromium.org/infra/cros/recovery/internal/execs"
)

// isDutAccessibleExec verifies that DUT is accessible through the associated host.
func isDutAccessibleExec(ctx context.Context, info *execs.ExecInfo) error {
	serialNumber := info.GetAndroid().GetSerialNumber()
	err := adb.IsDeviceAccessible(ctx, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "is dut accessible").Err()
	}
	return nil
}

// isDutOfflineExec verifies that DUT is in offline state.
func isDutOfflineExec(ctx context.Context, info *execs.ExecInfo) error {
	serialNumber := info.GetAndroid().GetSerialNumber()
	state, err := adb.GetDeviceState(ctx, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "is dut offline").Err()
	}
	if state != adb.Offline {
		return errors.Reason("dut %q state: %q", serialNumber, state).Err()
	}
	return nil
}

// isDutRootedExec verifies that DUT is rooted.
func isDutRootedExec(ctx context.Context, info *execs.ExecInfo) error {
	serialNumber := info.GetAndroid().GetSerialNumber()
	err := adb.IsDeviceRooted(ctx, newRunner(info), info.NewLogger(), serialNumber)
	if err != nil {
		return errors.Annotate(err, "is dut rooted").Err()
	}
	return nil
}

// isUserDebugBuildOnDutExec verifies that DUT has userdebug Android build.
// More details on https://source.android.com/source/add-device.html#build-variants
func isUserDebugBuildOnDutExec(ctx context.Context, info *execs.ExecInfo) error {
	run := newRunner(info)
	log := info.NewLogger()
	serialNumber := info.GetAndroid().GetSerialNumber()
	if err := adb.IsDebuggableBuildOnDevice(ctx, run, log, serialNumber); err != nil {
		return errors.Annotate(err, "is user debug build on dut").Err()
	}
	if err := adb.IsSecureBuildOnDevice(ctx, run, log, serialNumber); err != nil {
		return errors.Annotate(err, "is user debug build on dut").Err()
	}
	return nil
}

func init() {
	execs.Register("android_dut_is_accessible", isDutAccessibleExec)
	execs.Register("android_dut_is_offline", isDutOfflineExec)
	execs.Register("android_dut_is_rooted", isDutRootedExec)
	execs.Register("android_dut_has_userdebug_build", isUserDebugBuildOnDutExec)
}
