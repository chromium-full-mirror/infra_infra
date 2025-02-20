// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/logger"
)

// FirmwareUpdaterRequest holds request data for running firmware updater.
type FirmwareUpdaterRequest struct {
	// Mode used for updater.
	// Possible values is: autoupdate, recovery, factory.
	Mode string
	// Run updater with force option.
	Force bool
	// Time Specified to run firmware updater.
	UpdaterTimeout time.Duration
	// AP firmware image (image.bin). If provided firmware updater will use this image
	// instead of OS bundled firmware. Cannot be used together with FirmwareArchive.
	ApImage string
	// EC firmware image (i.e, ec.bin). If provided firmware updater will use this image
	// instead of OS bundled firmware. Cannot be used together with FirmwareArchive.
	EcImage string
	// Firmware archive path. If provided firmware updater extract and use AP and EC image
	// from the archieve. Cannot be used together with ApImage or EcImage.
	FirmwareArchive string
	// If provided it will override firmware target model, see --model option of chromeos-firmwareupdate.
	Model string
	// If update should proceed with write protection flag on, means "--wp=1".
	WriteProtection bool
}

// RunFirmwareUpdater run chromeos-firmwareupdate to update firmware on the host.
func RunFirmwareUpdater(ctx context.Context, req *FirmwareUpdaterRequest, run components.Runner, log logger.Logger) error {
	switch req.Mode {
	case "autoupdate":
	case "recovery":
	case "factory":
	default:
		return errors.Reason("run firmware updater: mode %q is not supported", req.Mode).Err()
	}
	if req.FirmwareArchive != "" && (req.ApImage != "" || req.EcImage != "") {
		return errors.Reason("run firmware updater: both FirmwareArchive and ApImage/EcImage are provided").Err()
	}
	log.Debugf("Run firmware updater: use %q mode.", req.Mode)
	args := []string{
		fmt.Sprintf("--mode=%s", req.Mode),
	}
	if req.Force {
		log.Debugf("Run firmware updater: request to run with force.")
		args = append(args, "--force")
	}
	if req.WriteProtection {
		log.Debugf("Run firmware updater: request to run with write protection on.")
		args = append(args, "--wp=1")
	}
	if req.ApImage != "" {
		log.Debugf(fmt.Sprintf("Run firmware updater: request to install from provided AP image %s", req.ApImage))
		args = append(args, fmt.Sprintf("--image=%s", req.ApImage))
	}
	if req.EcImage != "" {
		log.Debugf(fmt.Sprintf("Run firmware updater: request to install from provided EC image %s", req.EcImage))
		args = append(args, fmt.Sprintf("--ec_image=%s", req.EcImage))
	}
	if req.FirmwareArchive != "" {
		log.Debugf(fmt.Sprintf("Run firmware updater: request to extract and install from provided archive %s", req.FirmwareArchive))
		args = append(args, fmt.Sprintf("--archive=%s", req.FirmwareArchive))
	}
	if req.Model != "" {
		log.Debugf(fmt.Sprintf("Run firmware updater: request to override target model to %s", req.Model))
		args = append(args, fmt.Sprintf("--model=%s", req.Model))
	}
	out, err := run(ctx, req.UpdaterTimeout, "chromeos-firmwareupdate", args...)
	log.Debugf("Run firmware updater stdout:\n%s", out)
	return errors.Annotate(err, "run firmware update").Err()
}

// DisableWriteProtect disables software-controlled write-protect for both FPROMs, and install the RO firmware
func DisableWriteProtect(ctx context.Context, run components.Runner, log logger.Logger, timeout time.Duration, fprom string) error {
	switch fprom {
	case "internal", "ec":
	default:
		return errors.Reason("disable write-protect %q: unsupported", fprom).Err()
	}
	out, err := run(ctx, timeout, "flashrom", "-p", fprom, "--wp-disable", "--wp-range=0,0")
	log.Debugf("Disable writeProtection stdout:\n%s", out)
	return errors.Annotate(err, "disable write-protect %q", fprom).Err()
}

// ReadFirmwareKeysFromHost read AP keys from the host.
func ReadFirmwareKeysFromHost(ctx context.Context, run components.Runner, log logger.Logger) ([]string, error) {
	const extractImagePath = "/tmp/bios.bin"
	if out, err := run(ctx, 5*time.Minute, "flashrom", "-p", "internal", "-r", extractImagePath); err != nil {
		return nil, errors.Annotate(err, "has dev signed firmware").Err()
	} else {
		log.Debugf("Extract bios to the host: %s", out)
	}
	if keys, err := readAPKeysFromFile(ctx, extractImagePath, run, log); err != nil {
		return nil, errors.Annotate(err, "read ap info").Err()
	} else {
		return keys, nil
	}
}
