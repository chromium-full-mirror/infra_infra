// Copyright (c) 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/cros"
	"infra/cros/recovery/internal/components/cros/firmware"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/version"
)

const (
	// Dump the AP firmware.
	dumpAPFirmwareCmd = `mkdir /tmp/verify_firmware;` +
		`flashrom -p internal -r /tmp/verify_firmware/ap.bin`
	// Verify the firmware image.
	verifyFirmwareCmd = `futility verify /tmp/verify_firmware/ap.bin` +
		` --publickey /usr/share/vboot/devkeys/root_key.vbpubk`
	// Remove the firmware related files we created before.
	removeFirmwareFileCmd = `rm -rf /tmp/verify_firmware`
	// Firmware tarball filename in GS.
	firmwareTarSuffix = ".tar.bz2"
	firmwareTarName   = "firmware_from_source" + firmwareTarSuffix
	// Default mode for chromeos-firmwareupdate when install firmware image.
	defaultFirmwareImageUpdateMode = "recovery"
)

// isFirmwareInGoodState confirms that a host's firmware is in a good state.
//
// For DUTs that run firmware tests, it's possible that the firmware on the DUT can get corrupted.
// This verify action checks whether it appears that firmware should be re-flashed using servo.
func isFirmwareInGoodState(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	_, err := r(ctx, 2*time.Minute, dumpAPFirmwareCmd)
	if err != nil {
		return errors.Annotate(err, "firmware in good state").Err()
	}
	defer func() { r(ctx, time.Minute, removeFirmwareFileCmd) }()
	_, err = r(ctx, time.Minute, verifyFirmwareCmd)
	if err != nil {
		return errors.Annotate(err, "firmware in good state: firmware is in a bad state").Err()
	}
	return nil
}

// isOnRWFirmwareStableVersionExec confirms that the current RW firmware on DUT is match with model specific stable version.
func isOnRWFirmwareStableVersionExec(ctx context.Context, info *execs.ExecInfo) error {
	err := isOnStableFirmwareVersion(ctx, info, "fwid")
	return errors.Annotate(err, "on rw firmware stable version").Err()
}

// isOnROFirmwareStableVersionExec confirms that the current RO firmware on DUT is match with model specific stable version.
func isOnROFirmwareStableVersionExec(ctx context.Context, info *execs.ExecInfo) error {
	err := isOnStableFirmwareVersion(ctx, info, "ro_fwid")
	return errors.Annotate(err, "on ro firmware stable version").Err()
}

func isOnStableFirmwareVersion(ctx context.Context, info *execs.ExecInfo, crossystemControl string) error {
	logger := info.NewLogger()
	sv, err := version.ByDut(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "is on stable firmware version").Err()
	}
	actionArgs := info.GetActionArgs(ctx)
	// Only check number part as we have special fw versions and matching can be different.
	// Ex: Google_Model.XXXXX.XXX.0 vs Google_Model_Ufs.XXXXX.XXX.0
	versionNumberOnly := actionArgs.AsBool(ctx, "only_check_numbers", false)
	if !versionNumberOnly {
		// For multiple firmware model, firmware name may change based on hwid data.
		versionNumberOnly = firmware.IsMultiFirmwareHwid(info.GetChromeos().GetHwid())
	}
	if versionNumberOnly {
		delimiter := "."
		logger.Debugf("Multi-firmware hwid detected, will only compare version number for firmware match validation.")
		err = cros.MatchSuffixValueToExpectation(ctx, info.DefaultRunner(), crossystemControl, sv.GetFirmwareRoVersion(), delimiter, logger)
	} else {
		err = cros.MatchCrossystemValueToExpectation(ctx, info.DefaultRunner(), crossystemControl, sv.GetFirmwareRoVersion())
	}
	return errors.Annotate(err, "is on stable firmware version").Err()
}

// isRWFirmwareStableVersionAvailableExec confirms the stable firmware is up to date with the available firmware.
func isRWFirmwareStableVersionAvailableExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	sv, err := version.ByDut(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "rw firmware stable version available").Err()
	}
	modelFirmware, err := ReadFirmwareManifest(ctx, r, info.GetChromeos().GetModel())
	if err != nil {
		return errors.Annotate(err, "rw firmware stable version available").Err()
	}
	availableVersion, err := modelFirmware.AvailableRWFirmware()
	if err != nil {
		return errors.Annotate(err, "rw firmware stable version available").Err()
	}
	stableVersion := sv.GetFirmwareRoVersion()
	if availableVersion != stableVersion {
		return errors.Reason("rw firmware stable version not available, expected %q, found %q", availableVersion, stableVersion).Err()
	}
	return nil
}

// runFirmwareUpdaterExec run firmware process on the host to flash firmware from installed OS.
//
// Default mode used is autoupdate.
// To reboot device by the end please provide `reboot:by_servo` or `reboot:by_host`.
func runFirmwareUpdaterExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.NewRunner(info.GetDut().Name)
	am := info.GetActionArgs(ctx)
	logger := info.NewLogger()
	req := &firmware.FirmwareUpdaterRequest{
		// Options for the mode are: autoupdate, recovery, factory.
		Mode:           am.AsString(ctx, "mode", "autoupdate"),
		Force:          am.AsBool(ctx, "force", false),
		UpdaterTimeout: am.AsDuration(ctx, "updater_timeout", 600, time.Second),
	}
	logger.Debugf("Run firmware update: request to run with %q mode.", req.Mode)
	if err := firmware.RunFirmwareUpdater(ctx, req, run, logger); err != nil {
		logger.Debugf("Run firmware update: fail on run updater. Error: %s", err)
		return errors.Annotate(err, "run firmware update").Err()
	}
	return nil
}

// runDisableWriteProtectExec disables software-controlled write-protect.
//
// ChromeOS devices have 'internal' and 'ec' FPROMs, provide by 'fprom:ec'.
func runDisableFPROMWriteProtectExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.NewRunner(info.GetDut().Name)
	am := info.GetActionArgs(ctx)
	fprom := am.AsString(ctx, "fprom", "")
	err := firmware.DisableWriteProtect(ctx, run, info.NewLogger(), info.GetExecTimeout(), fprom)
	return errors.Annotate(err, "disable fprom: %q write-protect", fprom).Err()
}

func hasDevSignedFirmwareExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.NewRunner(info.GetDut().Name)
	if keys, err := firmware.ReadFirmwareKeysFromHost(ctx, run, info.NewLogger()); err != nil {
		return errors.Annotate(err, "has dev signed firmware").Err()
	} else if firmware.IsDevKeys(keys, info.NewLogger()) {
		return nil
	}
	return errors.Reason("has dev signed firmware: dev signed key not found").Err()
}

// updateFirmwareFromFirmwareImage update RW/RO firmware to a given firmwarm image(stable_version by default).
func updateFirmwareFromFirmwareImage(ctx context.Context, info *execs.ExecInfo) error {
	sv, err := version.ByDut(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "update firmware image").Err()
	}
	actionArgs := info.GetActionArgs(ctx)
	// With b/328679244 the version can contains the file name also.
	imageName := actionArgs.AsString(ctx, "version_name", sv.GetFirmwareRoImagePath())
	log.Debugf(ctx, "Used fw image name: %s", imageName)
	gsBucket := actionArgs.AsString(ctx, "gs_bucket", gsCrOSImageBucket)
	log.Debugf(ctx, "Used gs bucket name: %s", gsBucket)
	gsImagePath := actionArgs.AsString(ctx, "gs_image_path", fmt.Sprintf("%s/%s", gsBucket, imageName))
	log.Debugf(ctx, "Used fw image path: %s", gsImagePath)
	fwDownloadDir := actionArgs.AsString(ctx, "fw_download_dir", defaultFwFolderPath(info.GetDut()))
	log.Debugf(ctx, "Used fw image path: %s", gsImagePath)
	// Requesting convert GC path to caches service path.
	// Example: `http://Addr:8082/download/my-bucket/board-firmware/RXX-XXXXX.XX.0/xyz.tar.bz2`
	downloadFilename, err := info.GetAccess().GetCacheUrl(ctx, info.GetDut().Name, gsImagePath)
	if err != nil {
		return errors.Annotate(err, "update firmware image").Err()
	}
	// Apply file name only if it was not specified before.
	log.Debugf(ctx, "Adapted fw path for cache service: %s", downloadFilename)
	if !strings.HasSuffix(downloadFilename, firmwareTarSuffix) {
		fwFileName := actionArgs.AsString(ctx, "fw_filename", firmwareTarName)
		downloadFilename = fmt.Sprintf("%s/%s", downloadFilename, fwFileName)
		log.Debugf(ctx, "New path with specified filepath: %s", downloadFilename)
	}
	run := info.DefaultRunner()
	req := &firmware.InstallFirmwareImageRequest{
		DownloadImagePath:           downloadFilename,
		DownloadImageTimeout:        actionArgs.AsDuration(ctx, "download_timeout", 240, time.Second),
		DownloadDir:                 fwDownloadDir,
		DutRunner:                   run,
		Board:                       actionArgs.AsString(ctx, "dut_board", info.GetChromeos().GetBoard()),
		Model:                       actionArgs.AsString(ctx, "dut_model", info.GetChromeos().GetModel()),
		Hwid:                        actionArgs.AsString(ctx, "hwid", info.GetChromeos().GetHwid()),
		Servod:                      info.NewServod(),
		ForceUpdate:                 actionArgs.AsBool(ctx, "force", false),
		UpdateEcAttemptCount:        actionArgs.AsInt(ctx, "update_ec_attempt_count", 0),
		UpdateEcUseBoard:            actionArgs.AsBool(ctx, "update_ec_use_board", true),
		UpdateApAttemptCount:        actionArgs.AsInt(ctx, "update_ap_attempt_count", 0),
		GBBFlags:                    actionArgs.AsString(ctx, "gbb_flags", ""),
		CandidateFirmwareTarget:     actionArgs.AsString(ctx, "candidate_fw_target", ""),
		UseSerialTargets:            actionArgs.AsBool(ctx, "use_serial_fw_target", false),
		UpdaterMode:                 actionArgs.AsString(ctx, "mode", defaultFirmwareImageUpdateMode),
		UpdaterTimeout:              actionArgs.AsDuration(ctx, "updater_timeout", 600, time.Second),
		UseCacheToExtractor:         actionArgs.AsBool(ctx, "use_cache_extractor", false),
		DownloadImageReattemptCount: actionArgs.AsInt(ctx, "reattempt_count", 3),
		DownloadImageReattemptWait:  actionArgs.AsDuration(ctx, "reattempt_wait", 5, time.Second),
	}
	if actionArgs.AsBool(ctx, "use_fw_targets_from_inventory", false) {
		req.APTarget = info.GetChromeos().GetFirmwareInfo().GetApTarget()
		req.ECTarget = info.GetChromeos().GetFirmwareInfo().GetEcTarget()
	}
	logger := info.NewLogger()
	if err := firmware.InstallFirmwareImage(ctx, req, logger); err != nil {
		logger.Debugf("Update firmware image: failed to run updater. Error: %s", err)
		return errors.Annotate(err, "update firmware image").Err()
	}
	return nil
}

// isHardwareWriteProtectionDisabled checks if hardware write protection is
// disabled on the DUT. https://chromium.googlesource.com/chromiumos/docs/+/HEAD/write_protection.md
func isHardwareWriteProtectionDisabled(ctx context.Context, info *execs.ExecInfo) error {
	err := cros.MatchCrossystemValueToExpectation(ctx, info.DefaultRunner(), "wpsw_cur", "0")
	return errors.Annotate(err, "is hardware write protection disabled").Err()
}

func init() {
	execs.Register("cros_is_firmware_in_good_state", isFirmwareInGoodState)
	execs.Register("cros_is_on_rw_firmware_stable_version", isOnRWFirmwareStableVersionExec)
	execs.Register("cros_is_on_ro_firmware_stable_version", isOnROFirmwareStableVersionExec)
	execs.Register("cros_is_rw_firmware_stable_version_available", isRWFirmwareStableVersionAvailableExec)
	execs.Register("cros_has_dev_signed_firmware", hasDevSignedFirmwareExec)
	execs.Register("cros_run_firmware_update", runFirmwareUpdaterExec)
	execs.Register("cros_disable_fprom_write_protect", runDisableFPROMWriteProtectExec)
	execs.Register("cros_update_firmware_from_firmware_image", updateFirmwareFromFirmwareImage)
	execs.Register("cros_is_hardware_write_protection_disabled", isHardwareWriteProtectionDisabled)
}
