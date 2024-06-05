// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"regexp"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/execs/cros/amt"
)

// flexAMTPresent returns true if Intel AMT (vPro) is present.
func flexAMTPresent(ctx context.Context, info *execs.ExecInfo) error {
	client := getFlexAMTClient()
	present, err := client.AMTPresent()
	if err != nil {
		return errors.Annotate(err, "flex AMT present").Err()
	}
	if !present {
		return errors.Reason("flex AMT present: not found").Err()
	}
	return nil
}

// flexAMTPowerOff powers the DUT off using Intel AMT (vPro).
func flexAMTPowerOff(ctx context.Context, info *execs.ExecInfo) error {
	client := getFlexAMTClient()
	return errors.Annotate(client.PowerOff(), "flex AMT power-off").Err()
}

// flexAMTPowerOn powers the DUT on using Intel AMT (vPro).
func flexAMTPowerOn(ctx context.Context, info *execs.ExecInfo) error {
	client := getFlexAMTClient()
	return errors.Annotate(client.PowerOn(), "flex AMT power-off").Err()
}

// Configure and return an AMTClient.
func getFlexAMTClient() amt.AMTClient {
	//TODO(josephsussman): Get these from somewhere else.
	return amt.NewAMTClient("192.168.231.218", "admin", "P@ssword1")
}

// Get the device number for the servo-attached USB.
func findDeviceNumInOutput(output string) (string, error) {
	re := regexp.MustCompile(`^Boot([0-9A-F]{4})\* USB HDD`)
	lines := strings.Split(output, "\n")
	for i := 0; i < len(lines); i++ {
		match := re.FindStringSubmatch(lines[i])
		if match[1] == "" {
			continue
		}
		return match[1], nil
	}
	return "", errors.Reason("flex USB device number: not found").Err()
}

// setUSBForNextFlexBoot sets USB-drive as next boot device for the DUT.
func setUSBForNextFlexBoot(ctx context.Context, info *execs.ExecInfo) error {
	timeout := 2 * time.Second
	run := info.DefaultRunner()
	// Get the output of `efibootmgr`.
	out, err := run(ctx, timeout, "efibootmgr")
	if err != nil {
		return errors.Annotate(err, "set USB as next boot: fail to read efibootmgr").Err()
	}
	devnum, err := findDeviceNumInOutput(out)
	if err != nil {
		return err
	}
	_, err = run(ctx, timeout, "efibootmgr", "--bootnext", devnum)
	return errors.Annotate(err, "set USB as next boot: fail to set efibootmgr bootnext").Err()
}

func init() {
	execs.Register("cros_flex_amt_present", flexAMTPresent)
	execs.Register("cros_flex_amt_power_off", flexAMTPowerOff)
	execs.Register("cros_flex_amt_power_on", flexAMTPowerOn)
	execs.Register("cros_flex_usb_nextboot", setUSBForNextFlexBoot)
}
