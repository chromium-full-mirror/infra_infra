// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usb

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	// The prefix of the badblocks command for verifying USB
	// drives. The USB-drive path will be attached to it when
	// badblocks needs to be executed on a drive.
	badBlocksCommandPrefix = "badblocks -w -e 300 -b 4096 -t random %s"
	// The prefix of the smartctl command for running the health test
	// for USB drives that support SMART. The USB drive path will be attached
	// to it when the command needs to be executed on a drive.
	smartHealthCommandPrefix = "smartctl -H %s | awk '/SMART overall-health self-assessment test result:/ {print $6}'"
	// Expected output of a passing SMART health test.
	smartPass = "PASSED"
)

// UsbReset resets USB devices. usbId is one of the following
// - PPPP:VVVV - product and vendor id
// - BBB/DDD   - bus and device number
// - "Product" - product name
func UsbReset(ctx context.Context, run components.Runner, log logger.Logger, usbId string) error {
	usbResetCmd := "usbreset " + usbId
	_, err := run(ctx, time.Minute, usbResetCmd)
	if err != nil {
		return errors.Annotate(err, "usb reset").Err()
	}
	log.Debugf("USB is successfully reset: %s", usbId)
	return nil
}

// RunCheckOnHost generates new state for USB-drive by running check on DUT.
func RunCheckOnHost(ctx context.Context, run components.Runner, usbPath string, isSmartDevice bool, timeout time.Duration) (tlw.HardwareState, error) {
	command := fmt.Sprintf(badBlocksCommandPrefix, usbPath)
	if isSmartDevice {
		command = fmt.Sprintf(smartHealthCommandPrefix, usbPath)
	}
	// If error has a message like `it's not safe to run badblocks!` then we have some problems and better to retry.
	isBadblockIssue := func(err error) bool {
		return err != nil && strings.Contains(err.Error(), "safe") && strings.Contains(err.Error(), "badblocks")
	}
	log.Debugf(ctx, "Run Check On Host: Executing %q", command)
	// The execution timeout for this audit job is configured at the
	// level of the action. So the execution of this command will be
	// bound by that.
	out, err := run(ctx, timeout, command)
	if !isSmartDevice && isBadblockIssue(err) {
		log.Debugf(ctx, "Check fail due system find USB-drive used by it. Let's retry!")
		metrics.DefaultActionAddObservations(ctx, metrics.NewStringObservation("usbkey_audit_restarted", "yes"))
		// Sometime it happening, so we can retry.
		time.Sleep(2 * time.Second)
		out, err = run(ctx, timeout, command)
	}
	// Register error for following analysis.
	if err != nil {
		metrics.DefaultActionAddObservations(ctx, metrics.NewStringObservation("usbkey_audit_failure", err.Error()))
	}
	switch {
	case err == nil:
		if isSmartDevice {
			if strings.Contains(out, smartPass) {
				return tlw.HardwareState_HARDWARE_NORMAL, nil
			}
			return tlw.HardwareState_HARDWARE_NEED_REPLACEMENT, nil
		}
		// TODO(vkjoshi@): recheck if this is required, or does stderr need to be examined.
		if len(out) > 0 {
			return tlw.HardwareState_HARDWARE_NEED_REPLACEMENT, nil
		}
		return tlw.HardwareState_HARDWARE_NORMAL, nil
	case !isSmartDevice && isBadblockIssue(err):
		log.Debugf(ctx, "Check fail due system find USB-drive used by it! Skip as something stramge with this DUT.")
		fallthrough
	case components.SSHErrorLinuxTimeout.In(err): // 124 timeout
		fallthrough
	case components.SSHErrorCLINotFound.In(err): // 127 badblocks
		return tlw.HardwareState_HARDWARE_UNSPECIFIED, errors.Annotate(err, "run check on host: could not successfully complete check").Err()
	default:
		return tlw.HardwareState_HARDWARE_NEED_REPLACEMENT, nil
	}
}

// FindUSBDrivePathOnDut finds and returns the path of USB drive on a DUT.
func FindUSBDrivePathOnDut(ctx context.Context, run components.Runner, s components.Servod) (string, error) {
	// Switch USB on servo multiplexer to the DUT-side.
	if err := s.Set(ctx, "image_usbkey_direction", "dut_sees_usbkey"); err != nil {
		return "", errors.Annotate(err, "find usb drive path on dut: could not switch USB to DUT").Err()
	}
	// A detection delay is required when attaching this USB drive to DUT.
	time.Sleep(5 * time.Second)
	out, err := run(ctx, time.Minute, "ls /dev/sd[a-z]")
	if err != nil {
		return "", errors.Annotate(err, "get usb drive path on dut").Err()
	}
	for _, p := range strings.Split(out, "\n") {
		dtOut, dtErr := run(ctx, time.Minute, fmt.Sprintf(". /usr/share/misc/chromeos-common.sh; get_device_type %s", p))
		if dtErr != nil {
			log.Debugf(ctx, "Skip because device type check for %q failed, error: %s", p, dtErr)
			continue
		}
		if dtOut != "USB" {
			log.Debugf(ctx, "Skip as %q device, not as USB device!", p)
			continue
		}
		if _, err := run(ctx, time.Minute, fmt.Sprintf("fdisk -l %s", p)); err != nil {
			log.Debugf(ctx, "Skip because device %q could not be read, error: %s", p, err)
			continue
		}
		log.Debugf(ctx, "The path %q is good to assume is USB-drive.", p)
		return p, nil
	}
	log.Debugf(ctx, "Failed to find any valid USB-drive on DUT!")
	return "", errors.Reason("find usb drive path on dut: did not find any valid USB Drive").Err()
}

// The map of USB drive vendor:model pairings that currently support SMART.
var smartUSBs = map[string]string{
	"Corsair": "Voyager GTX",
}

// IsSmartUSBDrive checks if USB is smart device.
// TODO: replace with better logic if possible.
func IsSmartUSBDrive(ctx context.Context, run components.Runner, usbPath string) (bool, error) {
	if usbPath == "" {
		return false, errors.Reason("is smart USB drive: path is not provided").Err()
	}
	usbBasename := path.Base(usbPath)
	vendorPath := fmt.Sprintf("/sys/block/%s/device/vendor", usbBasename)
	modelPath := fmt.Sprintf("/sys/block/%s/device/model", usbBasename)
	vendor, err := run(ctx, time.Minute, fmt.Sprintf("cat %s", vendorPath))
	if err != nil {
		return false, errors.Annotate(err, "is smart USB drive").Err()
	}
	model, err := run(ctx, time.Minute, fmt.Sprintf("cat %s", modelPath))
	if err != nil {
		return false, errors.Annotate(err, "is smart USB drive").Err()
	}
	v := strings.TrimSpace(vendor)
	if val, exists := smartUSBs[v]; exists {
		if strings.Contains(model, val) {
			return true, nil
		}
	}
	return false, nil
}
