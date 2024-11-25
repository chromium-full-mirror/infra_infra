// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"regexp"
	"strings"
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	labApi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/servo"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/internal/retry"
	"infra/cros/recovery/logger/metrics"
	"infra/cros/recovery/tlw"
	"infra/cros/recovery/version"
)

func servoUSBHasCROSStableImageExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	// This is the max number of times we'll try reading the ChromeOS image
	servodReadCROSImageRetryLimit := argsMap.AsInt(ctx, "retry_count", 1)
	// retryInterval is the timeout between retries for reading the ChromeOS image
	retryInterval := argsMap.AsDuration(ctx, "retry_interval", 1, time.Second)
	usbFileCheck := argsMap.AsBool(ctx, "usb_file_check", false)
	sv, err := version.ByDut(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "servo usb-key has cros stable image").Err()
	}
	expectedImage := sv.GetOsImagePath()
	if expectedImage == "" {
		return errors.Reason("servo usb-key has cros stable image: stable image is not specified").Err()
	}
	sh := info.GetChromeos().GetServo()
	if sh.GetName() == "" {
		return errors.Reason("servo usb-key has cros stable image: servo is not present as part of dut info").Err()
	}
	run := info.NewRunner(sh.GetName())
	servod := info.NewServod()
	logger := info.NewLogger()
	usbPath, newUSBState, err := servo.USBDrivePath(ctx, usbFileCheck, run, servod, logger)
	if err != nil {
		log.Debugf(ctx, "Servo USB Has CROS Stable Image: could not read usb key path")
		// Skip state if it is not specified.
		if newUSBState != tlw.HardwareState_HARDWARE_UNSPECIFIED {
			sh.UsbkeyState = newUSBState
			log.Infof(ctx, "New Servo USB-key state: %s", sh.GetUsbkeyState().String())
		}
		return errors.Annotate(err, "servo usb-key has cros stable image").Err()
	}
	var imageName string
	var getImageName = func() (err error) {
		imageName, err = servo.ChromeOSImageNameFromUSBDrive(ctx, usbPath, run, servod, logger)
		return err
	}
	if err := retry.LimitCount(ctx, servodReadCROSImageRetryLimit, retryInterval, getImageName, "get ChromeOS image name from usb"); err != nil {
		return errors.Annotate(err, "servo usb-key has cros stable image").Err()
	}
	if strings.Contains(expectedImage, imageName) {
		log.Infof(ctx, "The image %q found on USB-key and match to stable version", imageName)
		return nil
	}
	return errors.Reason("servo usb-key has cros stable image: expected %q but found %q", expectedImage, imageName).Err()
}

func servoUSBKeyIsDetectedExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	fileCheck := argsMap.AsBool(ctx, "file_check", false)
	servoHost := info.GetChromeos().GetServo()
	if servoHost.GetName() == "" {
		return errors.Reason("servo USB key is detected: servo host is not specified").Err()
	}
	servodRun := info.NewRunner(servoHost.GetName())
	servod := info.NewServod()
	usbPath, newUSBState, err := servo.USBDrivePath(ctx, fileCheck, servodRun, servod, info.NewLogger())
	var usbDetectionObservationValue string
	defer func() {
		if usbDetectionObservationValue != "" {
			info.AddObservation(metrics.NewStringObservation("usb_detection", usbDetectionObservationValue))
		}
	}()
	if err != nil {
		// Skip state if it is not specified.
		if newUSBState != tlw.HardwareState_HARDWARE_UNSPECIFIED {
			servoHost.UsbkeyState = newUSBState
			log.Infof(ctx, "New Servo USB-key state: %s", servoHost.GetUsbkeyState().String())
		}
		usbDetectionObservationValue = "usbkey_detection_failed"
		log.Debugf(ctx, "Fail to detect path to USB key connected to the servo from servo-host.")
		return errors.Annotate(err, "servo USB key is detected").Err()
	}
	// The replacement state can be overwritten by the audit logic.
	if servoHost.GetUsbkeyState() != tlw.HardwareState_HARDWARE_NEED_REPLACEMENT {
		servoHost.UsbkeyState = tlw.HardwareState_HARDWARE_NORMAL
	}
	usbDetectionObservationValue = "usbkey_detected"
	log.Debugf(ctx, "USB key is detected from servo-host as: %q.", usbPath)

	if argsMap.AsBool(ctx, "check_drop_connection", false) {
		timeout := argsMap.AsDuration(ctx, "check_drop_connection_timeout", 120, time.Second)
		log.Debugf(ctx, "Prepare to check if USB-key would drop connection in %v.", timeout)
		time.Sleep(timeout)
		if err := servo.USBDriveReadable(ctx, usbPath, servodRun, info.NewLogger()); err != nil {
			log.Infof(ctx, "USB key lost connection after timeout, so mark it for replacement.", timeout)
			servoHost.UsbkeyState = tlw.HardwareState_HARDWARE_NEED_REPLACEMENT
			return errors.Annotate(err, "servo USB key is detected: connection dropped").Err()
		}
	}
	return nil
}

// createMetricsRecordWhenNewUSBDriveFound creates new metrics record when new USB drive detected in setup.
func createMetricsRecordWhenNewUSBDriveFound(ctx context.Context, info *execs.ExecInfo, newDevice *labApi.UsbDrive) {
	metric := info.NewMetric(metrics.USBDriveDetectionKind)
	metric.Status = metrics.ActionStatusSuccess
	metric.Observations = append(metric.Observations,
		metrics.NewStringObservation("serial", newDevice.GetSerial()),
		metrics.NewStringObservation("manufacturer", newDevice.GetManufacturer()),
	)
}

// createMetricsRecordWhenUSBDriveReplaced creates new metrics record when detected that device was replace or removed from setup.
func createMetricsRecordWhenUSBDriveReplaced(ctx context.Context, info *execs.ExecInfo, oldDevice, newDevice *labApi.UsbDrive) {
	newTime := time.Now()
	if newDevice != nil {
		newTime = newDevice.GetFirstSeenTime().AsTime()
	}
	duration := newTime.Sub(oldDevice.GetFirstSeenTime().AsTime())
	metric := info.NewMetric(metrics.USBDriveReplacedKind)
	metric.Status = metrics.ActionStatusSuccess
	metric.Observations = append(metric.Observations,
		metrics.NewStringObservation("serial", oldDevice.GetSerial()),
		metrics.NewStringObservation("manufacturer", oldDevice.GetManufacturer()),
		metrics.NewStringObservation("duration", duration.String()),
	)
}

// servoUpdateUSBKeyHistoryExec will update the inventory record for the servo's usbkey stick with the latest information.
func servoUpdateUSBKeyHistoryExec(ctx context.Context, info *execs.ExecInfo) error {
	sh := info.GetChromeos().GetServo()
	if sh.GetName() == "" {
		return errors.Reason("servo update usbkey history: servo is not present as part of dut info").Err()
	}
	sha := info.NewHostAccess(sh.GetName())
	usbdev, err := servodGetString(ctx, info.NewServod(), "image_usbkey_dev")
	if err != nil {
		return errors.Annotate(err, "servo update usbkey history").Err()
	}
	sysfsPathRes, err := sha.Run(ctx, 30*time.Second, "udevadm", "info", "-q", "path", "-n", usbdev)
	if err != nil {
		return errors.Annotate(err, "servo update usbkey history: read full system path").Err()
	}
	sysfsPath := strings.TrimSpace(sysfsPathRes.GetStdout())
	if sysfsPath == "" {
		return errors.Reason("servo update usbkey history: fail to read full system path").Err()
	}
	// Running udevadm test on a device path would cite all the rules involved,
	// for example, '60-persistent-storage.rules'. It would also print debug
	// information, including model name, serial numbers, vendor ID, etc.
	testRes, err := sha.Run(ctx, 30*time.Second, "udevadm", "test", "--action=add", sysfsPath)
	if err != nil {
		return errors.Annotate(err, "servo update usbkey history: read usb device info").Err()
	}
	testOut := testRes.GetStdout()
	var (
		reModel  = regexp.MustCompile(`(?i)ID_MODEL=(.+)`)
		reSerial = regexp.MustCompile(`(?i)ID_SERIAL_SHORT=(.+)`)
	)
	newDevice := &labApi.UsbDrive{
		FirstSeenTime: timestamppb.New(time.Now()),
	}
	if foundModel := reModel.FindStringSubmatch(testOut); foundModel != nil {
		newDevice.Manufacturer = foundModel[1]
	}
	if foundSerial := reSerial.FindStringSubmatch(testOut); foundSerial != nil {
		newDevice.Serial = foundSerial[1]
	}
	log.Infof(ctx, "Found USB drive info: %q, %q", newDevice.GetManufacturer(), newDevice.GetSerial())
	oldDevice := sh.GetUsbDrive()
	// Two cases where we need to update the record for usbkey.
	if oldDevice.GetSerial() == "" && newDevice.GetSerial() == "" {
		log.Debugf(ctx, "USB drive not found and was not in the setup.")
	} else if oldDevice.GetSerial() == "" {
		log.Debugf(ctx, "New USB drive detected.")
		createMetricsRecordWhenNewUSBDriveFound(ctx, info, newDevice)
		// Updating inventory record.
		sh.UsbDrive = newDevice
	} else if newDevice.GetSerial() == "" {
		log.Debugf(ctx, "USB drive removed from the servo.")
		createMetricsRecordWhenUSBDriveReplaced(ctx, info, oldDevice, newDevice)
	} else if oldDevice.GetSerial() != newDevice.GetSerial() {
		log.Debugf(ctx, "USB drive replaced to new one.")
		createMetricsRecordWhenNewUSBDriveFound(ctx, info, newDevice)
		createMetricsRecordWhenUSBDriveReplaced(ctx, info, oldDevice, newDevice)
		// Updating inventory record.
		sh.UsbDrive = newDevice
	} else if oldDevice.GetSerial() == newDevice.GetSerial() && newDevice.GetManufacturer() != "" {
		oldDevice.Manufacturer = newDevice.GetManufacturer()
	}
	return nil
}

func init() {
	execs.Register("servo_usbkey_has_stable_image", servoUSBHasCROSStableImageExec)
	execs.Register("servo_update_usbkey_history", servoUpdateUSBKeyHistoryExec)
	execs.Register("servo_usbkey_is_detected", servoUSBKeyIsDetectedExec)
}
