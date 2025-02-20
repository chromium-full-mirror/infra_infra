// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/components/urlpath"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
	"go.chromium.org/infra/cros/recovery/version"
)

// provisionExec performs provisioning of the device.
//
// To prevent reboot of device please provide action exec argument 'no_reboot'.
// To provide custom image data please use 'os_name', 'os_bucket', 'os_image_path'.
func provisionExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	osImageName := argsMap.AsString(ctx, "os_name", "")
	if osImageName == "" {
		sv, err := version.ByResource(
			ctx,
			version.Type(argsMap.AsString(ctx, "device_type", "")),
			info.GetDut(),
			info.GetActiveResource())
		if err != nil {
			return errors.Annotate(err, "cros provision").Err()
		}
		osImageName = sv.GetOsImagePath()
	}
	if osImageName == "" {
		return errors.Reason("cros provision: os image not provided").Err()
	}
	log.Debugf(ctx, "Used OS image name: %s", osImageName)
	osImageBucket := argsMap.AsString(ctx, "os_bucket", gsCrOSImageBucket)
	log.Debugf(ctx, "Used OS bucket name: %s", osImageBucket)
	osImagePath := argsMap.AsString(ctx, "os_image_path", fmt.Sprintf("%s/%s", osImageBucket, osImageName))
	log.Debugf(ctx, "Used OS image path: %s", osImagePath)
	req := &tlw.ProvisionRequest{
		Resource:        info.GetActiveResource(),
		PreventReboot:   false,
		SystemImagePath: osImagePath,
	}
	if _, ok := argsMap["no_reboot"]; ok {
		req.PreventReboot = true
		log.Debugf(ctx, "Cros provision will be perform without reboot.")
	}
	log.Debugf(ctx, "Cros provision OS image path: %s", req.SystemImagePath)
	err := info.GetAccess().Provision(ctx, req)
	return errors.Annotate(err, "cros provision").Err()
}

// Download image to the USB-drive.
//
// To provide custom image data please use 'os_name', 'os_bucket', 'os_image_path'.
func downloadImageToUSBExec(ctx context.Context, info *execs.ExecInfo) error {
	sv, err := version.ByDut(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "download image to usb-drive").Err()
	}
	servo := info.GetDut().GetChromeos().GetServo()
	if servo == nil {
		return errors.Reason("download image to usb-drive: setup does not have servo").Err()
	}
	info.AddObservation(metrics.NewStringObservation("usbkey_model", servo.GetUsbDrive().GetManufacturer()))
	info.AddObservation(metrics.NewStringObservation("usbkey_state", servo.GetUsbkeyState().String()))
	argsMap := info.GetActionArgs(ctx)
	osImageName := argsMap.AsString(ctx, "os_name", sv.GetOsImagePath())
	log.Debugf(ctx, "Used OS image name: %s", osImageName)
	osImageBucket := argsMap.AsString(ctx, "os_bucket", gsCrOSImageBucket)
	log.Debugf(ctx, "Used OS bucket name: %s", osImageBucket)
	osImagePath := argsMap.AsString(ctx, "os_image_path", fmt.Sprintf("%s/%s", osImageBucket, osImageName))
	log.Debugf(ctx, "Used OS image path: %s", osImagePath)
	// Requesting convert GC path to caches service path.
	// Example: `http://Addr:8082/download/chromeos-image-archive/board-release/R99-XXXXX.XX.0/`
	downloadPath, err := info.GetAccess().GetCacheUrl(ctx, info.GetDut().Name, osImagePath)
	if err != nil {
		return errors.Annotate(err, "download image to usb-drive").Err()
	}
	// Path provided by TLS cannot be used for downloading and/or extracting the image file.
	// But we can utilize the address of caching service and apply some string manipulation to construct the URL that can be used for this.
	// Example: `http://Addr:8082/extract/chromeos-image-archive/board-release/R99-XXXXX.XX.0/chromiumos_test_image.tar.xz?file=chromiumos_test_image.bin`
	extractPath := strings.Replace(downloadPath, "/download/", "/extract/", 1)
	pathWithIds, err := urlpath.EnrichWithTrackingIds(ctx, extractPath)
	if err != nil {
		return errors.Annotate(err, "download image to usb-drive").Err()
	}
	image := fmt.Sprintf("%s/chromiumos_test_image.tar.xz?file=chromiumos_test_image.bin", pathWithIds)
	log.Debugf(ctx, "Download image for USB-drive: %s", image)
	val, err := info.NewServod().Call(ctx, "set", info.GetExecTimeout(), "download_image_to_usb_dev", image)
	log.Debugf(ctx, "Received reponse: %v", val)

	// If we fail we can detect issues with USB-drive, so we can mark it for replacement.
	// Example:
	if err != nil && strings.Contains(err.Error(), "Read-only file system:") {
		log.Debugf(ctx, "USB-drive is read-only, it is recommended to replace the device.")
		metrics.DefaultActionAddObservations(ctx, metrics.NewStringObservation("servo_usb_replacement_reason", "read-only"))
		servo.UsbkeyState = tlw.HardwareState_HARDWARE_NEED_REPLACEMENT
	}
	return errors.Annotate(err, "download image to usb-drive").Err()
}

func downloadProvisionImageToUSBExec(ctx context.Context, info *execs.ExecInfo) error {
	servo := info.GetDut().GetChromeos().GetServo()
	if servo == nil {
		return errors.Reason("download provision image to usb-drive: setup does not have servo").Err()
	}
	board := info.GetDut().GetChromeos().GetBoard()
	if board == "" {
		return errors.Reason("download provision image to usb-drive: board data is missing").Err()
	}
	info.AddObservation(metrics.NewStringObservation("usbkey_model", servo.GetUsbDrive().GetManufacturer()))
	info.AddObservation(metrics.NewStringObservation("usbkey_state", servo.GetUsbkeyState().String()))
	argsMap := info.GetActionArgs(ctx)
	imageVersion := argsMap.AsString(ctx, "image_version", "v4")
	// Example: `gs://chromeos-throw-away-bucket/kimjae/brya-provision-v4.bin`
	imagePath := fmt.Sprintf("gs://chromeos-throw-away-bucket/kimjae/%s-provision-%s.bin", board, imageVersion)
	log.Debugf(ctx, "Used image path: %s", imagePath)

	// Requesting convert GC path to caches service path.
	// Example: `http://Addr:8082/download/chromeos-image-archive/board-release/R99-XXXXX.XX.0/`
	downloadPath, err := info.GetAccess().GetCacheUrl(ctx, info.GetDut().Name, imagePath)
	if err != nil {
		return errors.Annotate(err, "download image to usb-drive").Err()
	}
	log.Debugf(ctx, "Download image for USB-drive: %s", downloadPath)
	val, err := info.NewServod().Call(ctx, "set", info.GetExecTimeout(), "download_image_to_usb_dev", downloadPath)
	log.Debugf(ctx, "Received reponse: %v", val)

	// If we fail we can detect issues with USB-drive, so we can mark it for replacement.
	// Example:
	if err != nil && strings.Contains(err.Error(), "Read-only file system:") {
		log.Debugf(ctx, "USB-drive is read-only, it is recommended to replace the device.")
		metrics.DefaultActionAddObservations(ctx, metrics.NewStringObservation("servo_usb_replacement_reason", "read-only"))
		servo.UsbkeyState = tlw.HardwareState_HARDWARE_NEED_REPLACEMENT
	}
	return errors.Annotate(err, "download image to usb-drive").Err()
}

const (
	// provisionFailed - A flag file to indicate provision failures.
	// The file's location in stateful means that on successful update
	// it will be removed.  Thus, if this file exists, it indicates that
	// we've tried and failed in a previous attempt to update.
	// The file will be created every time a OS provision is kicked off.
	// TODO(b/229309510): Remove old marker file when new marker file is in use.
	provisionFailed       = "/var/tmp/provision_failed"
	provisionFailedMarker = "/mnt/stateful_partition/unencrypted/provision_failed"
)

// isLastProvisionSuccessfulExec confirms that the DUT successfully finished
// its last provision job.
//
// At the start of any update (e.g. for a Provision job), the code creates
// a marker file. The file will be removed if an update finishes successfully.
// Thus, the presence of the file indicates that a prior update failed.
// The verifier tests for the existence of the marker file and fails if
// it still exists.
func isLastProvisionSuccessfulExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	if _, err := run(ctx, 20*time.Second, fmt.Sprintf("test -f %s", provisionFailed)); err == nil {
		return errors.Reason("last provision successful: found fail provision marker on %q", provisionFailed).Err()
	}
	if _, err := run(ctx, 20*time.Second, fmt.Sprintf("test -f %s", provisionFailedMarker)); err == nil {
		return errors.Reason("last provision successful: found fail provision marker on %q", provisionFailedMarker).Err()
	}
	return nil
}

// isCameraboxTabletOnOSVersionExec check if the tablet is on the required os
// version.
func isCameraboxTabletOnOSVersionExec(ctx context.Context, info *execs.ExecInfo) error {
	rv, err := version.ByDut(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "camerabox tablet match os version").Err()
	}
	expectedOS := rv.GetOsImagePath()
	log.Debugf(ctx, "Expected version: %s", expectedOS)
	fromDevice, err := cros.ReleaseBuildPath(ctx, info.DefaultRunner(), info.NewLogger())
	if err != nil {
		return errors.Annotate(err, "camerabox tablet match os version").Err()
	}
	log.Debugf(ctx, "Version on device: %s", fromDevice)
	if fromDevice != expectedOS {
		return errors.Reason("camerabox tablet os version: mismatch, expected %q, found %q", expectedOS, fromDevice).Err()
	}
	return nil
}

// provisionCameraboxTabletExec
func provisionCameraboxTabletExec(ctx context.Context, info *execs.ExecInfo) error {
	rv, err := version.ByDut(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "camerabox tablet match os version").Err()
	}
	chartOSName := rv.GetOsImagePath()
	osImagePath := fmt.Sprintf("%s/%s", gsCrOSImageBucket, chartOSName)
	log.Debugf(ctx, "Used OS image path: %s", osImagePath)
	req := &tlw.ProvisionRequest{
		Resource:        info.GetActiveResource(),
		PreventReboot:   false,
		SystemImagePath: osImagePath,
	}
	log.Debugf(ctx, "Provision camerabox tablet with image path: %s", req.SystemImagePath)
	err = info.GetAccess().Provision(ctx, req)
	return errors.Annotate(err, "provision camerabox tablet").Err()
}

func init() {
	execs.Register("cros_provision", provisionExec)
	execs.Register("servo_download_image_to_usb", downloadImageToUSBExec)
	execs.Register("servo_download_provision_image_to_usb", downloadProvisionImageToUSBExec)
	execs.Register("cros_is_last_provision_successful", isLastProvisionSuccessfulExec)
	execs.Register("is_camerabox_tablet_on_os_version", isCameraboxTabletOnOSVersionExec)
	execs.Register("provision_camerabox_tablet", provisionCameraboxTabletExec)
}
