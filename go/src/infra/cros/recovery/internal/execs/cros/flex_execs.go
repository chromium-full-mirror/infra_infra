// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/cros/uefi"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/execs/cros/amt"
)

// flexAMTPresentExec returns true if Intel AMT (vPro) is present.
func flexAMTPresentExec(ctx context.Context, info *execs.ExecInfo) error {
	client := getFlexAMTClient()
	present, err := client.AMTPresent(ctx)
	if err != nil {
		return errors.Annotate(err, "flex AMT present").Err()
	}
	if !present {
		return errors.Reason("flex AMT present: not found").Err()
	}
	return nil
}

// flexAMTPowerOffExec powers the DUT off using Intel AMT (vPro).
func flexAMTPowerOffExec(ctx context.Context, info *execs.ExecInfo) error {
	client := getFlexAMTClient()
	return errors.Annotate(client.PowerOff(ctx), "flex AMT power-off").Err()
}

// flexAMTPowerOnExec powers the DUT on using Intel AMT (vPro).
func flexAMTPowerOnExec(ctx context.Context, info *execs.ExecInfo) error {
	client := getFlexAMTClient()
	return errors.Annotate(client.PowerOn(ctx), "flex AMT power-off").Err()
}

// Configure and return an AMTClient.
func getFlexAMTClient() amt.AMTClient {
	//TODO(josephsussman): Get these from somewhere else.
	return amt.NewAMTClient("192.168.231.218", "admin", "P@ssword1")
}

// setUSBForNextFlexBootExec sets USB-drive as next boot device for the DUT.
func setUSBForNextFlexBootExec(ctx context.Context, info *execs.ExecInfo) error {
	return uefi.SetUSBForNextBoot(ctx, info.DefaultRunner())
}

func init() {
	execs.Register("cros_flex_amt_present", flexAMTPresentExec)
	execs.Register("cros_flex_amt_power_off", flexAMTPowerOffExec)
	execs.Register("cros_flex_amt_power_on", flexAMTPowerOnExec)
	execs.Register("cros_flex_usb_nextboot", setUSBForNextFlexBootExec)
}
