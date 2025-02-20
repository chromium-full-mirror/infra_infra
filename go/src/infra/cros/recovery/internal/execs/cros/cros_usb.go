// Copyright 2022 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros/usb"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// auditUSBFromDUTSideKeyExec initiates an audit of the servo USB key strictly from the DUT side.
func auditUSBFromDUTSideKeyExec(ctx context.Context, info *execs.ExecInfo) error {
	actionArgs := info.GetActionArgs(ctx)
	timeout := actionArgs.AsDuration(ctx, "audit_timeout_min", 120, time.Minute)

	dut := info.GetDut()
	servoHost := info.GetChromeos().GetServo()
	log.Infof(ctx, "Begin servo audit USB from DUT side for %q %q", dut.Name, servoHost.GetName())

	dutRunner := info.NewRunner(dut.Name)
	dutUSB, err := usb.FindUSBDrivePathOnDut(ctx, dutRunner, info.NewServod())
	if err != nil {
		log.Errorf(ctx, "Failed to determine dut USB path: %s", err.Error())
		return errors.Annotate(err, "audit USB from DUT side").Err()
	}
	smartSupport, err := usb.IsSmartUSBDrive(ctx, dutRunner, dutUSB)
	if err != nil {
		log.Errorf(ctx, "Failed to determine if dut USB supports SMART: %s", err.Error())
		return errors.Annotate(err, "audit USB from DUT side").Err()
	}
	state, err := usb.RunCheckOnHost(ctx, dutRunner, dutUSB, smartSupport, timeout)
	if err != nil {
		log.Errorf(ctx, "DUT check failed")
		return errors.Reason("audit USB from DUT side: could not check DUT usb path %q", dutUSB).Err()
	}
	servoHost.UsbkeyState = state
	log.Infof(ctx, "Successfully end servo audit USB. DUT: %q:  %q", dut.Name, servoHost.GetUsbkeyState())
	return nil
}

func init() {
	execs.Register("audit_usb_from_dut_side", auditUSBFromDUTSideKeyExec)
}
