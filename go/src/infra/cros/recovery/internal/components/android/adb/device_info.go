// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package adb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/logger"
)

type State int

const (
	Unknown State = iota
	Offline
	Bootloader
	Device
	Unauthorized
)

func (e State) String() string {
	switch e {
	case Device:
		return "device"
	case Unauthorized:
		return "unauthorized"
	case Offline:
		return "offline"
	case Bootloader:
		return "bootloader"
	case Unknown:
		return "not found"
	default:
		return fmt.Sprintf("%d", int(e))
	}
}

// GetDeviceState reads device state (offline | bootloader | device).
func GetDeviceState(ctx context.Context, run components.Runner, log logger.Logger, serialNumber string) (State, error) {
	const adbGetStateCmd = "adb devices | grep -sw '%s' | awk '{print $2}'"
	cmd := fmt.Sprintf(adbGetStateCmd, serialNumber)
	state, err := run(ctx, time.Minute, cmd)
	if err != nil {
		return Unknown, errors.Annotate(err, "get attached device state").Err()
	}
	log.Debugf("Attached device state: %q", state)
	switch state {
	case "device":
		return Device, nil
	case "offline":
		return Offline, nil
	case "bootloader":
		return Bootloader, nil
	case "unauthorized":
		return Unauthorized, nil
	default:
		return Unknown, nil
	}
}

// GetDevicePath reads device path.
func GetDevicePath(ctx context.Context, run components.Runner, log logger.Logger, serialNumber string) (string, error) {
	const adbGetDevPathCmd = "adb -s %s get-devpath"
	cmd := fmt.Sprintf(adbGetDevPathCmd, serialNumber)
	path, err := run(ctx, time.Minute, cmd)
	if err != nil {
		return "", errors.Annotate(err, "get attached device path").Err()
	}
	log.Debugf("Attached device path: %q", path)
	return path, nil
}

// GetDeviceUSBPath returns device USB path (e.g. usb:1-6.1.1.3.3).
func GetDeviceUSBPath(ctx context.Context, run components.Runner, log logger.Logger, serialNumber string) (string, error) {
	const adbGetUsbPathCmd = "adb devices -l | grep %s | awk '{print $3}'"
	cmd := fmt.Sprintf(adbGetUsbPathCmd, serialNumber)
	path, err := run(ctx, 30*time.Second, cmd)
	if err != nil {
		return "", errors.Annotate(err, "get device usb path").Err()
	}
	if !strings.HasPrefix(path, "usb:") {
		return "", errors.Reason("invalid attached device USB path: %q", path).Err()
	}
	log.Debugf("Attached device USB path: %q", path)
	return path, nil
}

// GetDeviceUSBFilename returns device USB filename (e.g. /dev/bus/usb/001/030).
func GetDeviceUSBFilename(ctx context.Context, run components.Runner, log logger.Logger, serialNumber string) (string, error) {
	usbPath, err := GetDeviceUSBPath(ctx, run, log, serialNumber)
	if err != nil {
		return "", errors.Annotate(err, "get attached device USB filename").Err()
	}
	const adbGetDevUsbFilenameCmd = "lsusb -tvv | grep '%s' | head -1 | awk '{print $2}'"
	cmd := fmt.Sprintf(adbGetDevUsbFilenameCmd, usbPath[len("usb:"):])
	fn, err := run(ctx, 30*time.Second, cmd)
	if err != nil {
		return "", errors.Annotate(err, "get attached device USB filename").Err()
	}
	if !strings.HasPrefix(fn, "/dev/bus/usb/") {
		return "", errors.Reason("invalid attached device USB filename: %s", fn).Err()
	}
	log.Debugf("Attached device USB filename: %q", fn)
	return fn, nil
}

// IsDeviceAccessible verifies that DUT is accessible through the associated host.
func IsDeviceAccessible(ctx context.Context, run components.Runner, log logger.Logger, serialNumber string) error {
	state, err := GetDeviceState(ctx, run, log, serialNumber)
	if err != nil {
		return errors.Annotate(err, "dut is accessible").Err()
	}
	log.Debugf("Attached DUT %q: state %q", serialNumber, state)
	if state != Device {
		return errors.Reason("invalid attached dut %q state: %q", serialNumber, state).Err()
	}
	return nil
}

// IsDeviceRooted validates whether device is rooted. Refer to go/abp-security/rooted-devices for info on device rooting.
func IsDeviceRooted(ctx context.Context, run components.Runner, log logger.Logger, serialNumber string) error {
	const adbIsDeviceRootedCmd = "adb -s %s shell su root whoami>/dev/null 2>&1; echo $?"
	cmd := fmt.Sprintf(adbIsDeviceRootedCmd, serialNumber)
	output, err := run(ctx, time.Minute, cmd)
	if err != nil {
		return errors.Annotate(err, "is device rooted").Err()
	}
	if strings.TrimSpace(output) != "0" {
		return errors.Reason("attached device %q is not rooted", serialNumber).Err()
	}
	log.Debugf("Attached device %q is rooted", serialNumber)
	return nil
}

// IsDebuggableBuildOnDevice checks whether device has application debugging enabled or not.
func IsDebuggableBuildOnDevice(ctx context.Context, run components.Runner, log logger.Logger, serialNumber string) error {
	const adbIsDebuggableBuildCmd = "adb -s %s shell getprop ro.debuggable"
	cmd := fmt.Sprintf(adbIsDebuggableBuildCmd, serialNumber)
	output, err := run(ctx, time.Minute, cmd)
	if err != nil {
		return errors.Annotate(err, "is debuggable build").Err()
	}
	if strings.TrimSpace(output) != "1" {
		return errors.Reason("build on device %q is not debuggable", serialNumber).Err()
	}
	log.Debugf("Attached device %q has debugging enabled", serialNumber)
	return nil
}

// IsSecureBuildOnDevice checks whether device has userdebug or user build.
// More details on https://source.android.com/source/add-device.html#build-variants
func IsSecureBuildOnDevice(ctx context.Context, run components.Runner, log logger.Logger, serialNumber string) error {
	const adbIsSecureBuildCmd = "adb -s %s shell getprop ro.secure"
	cmd := fmt.Sprintf(adbIsSecureBuildCmd, serialNumber)
	output, err := run(ctx, time.Minute, cmd)
	if err != nil {
		return errors.Annotate(err, "is secure build").Err()
	}
	if strings.TrimSpace(output) != "1" {
		return errors.Reason("build on device %q is not secure", serialNumber).Err()
	}
	log.Debugf("Attached device %q has a secure build", serialNumber)
	return nil
}
