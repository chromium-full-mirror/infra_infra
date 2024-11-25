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

	"infra/cros/recovery/internal/components/cros/firmware"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
	"infra/cros/recovery/version"
)

func readGbbFlagsByServoExec(ctx context.Context, info *execs.ExecInfo) error {
	servod := info.NewServod()
	run := info.NewRunner(info.GetChromeos().GetServo().GetName())
	rawGBB, err := firmware.ReadGBBByServo(ctx, info.GetExecTimeout(), run, servod)
	if err != nil {
		return errors.Annotate(err, "read gbb flags").Err()
	}
	log.Debugf(ctx, "Device has GBB flags: %v", rawGBB)
	return nil
}

func setGbbFlagsByServoExec(ctx context.Context, info *execs.ExecInfo) error {
	actionArgs := info.GetActionArgs(ctx)
	servod := info.NewServod()
	run := info.NewRunner(info.GetChromeos().GetServo().GetName())
	gbbHex := actionArgs.AsString(ctx, "gbb_flags", "")
	if err := firmware.SetGBBByServo(ctx, gbbHex, info.GetExecTimeout(), run, servod); err != nil {
		return errors.Annotate(err, "set gbb flags").Err()
	}
	if actionArgs.AsBool(ctx, "force_reboot", false) {
		log.Debugf(ctx, "Proceed to reboot by servo")
		if err := servod.Set(ctx, "power_state", "reset"); err != nil {
			return errors.Annotate(err, "set gbb flags").Err()
		}
	}
	return nil
}

func updateFwWithFwImageByServo(ctx context.Context, info *execs.ExecInfo) error {
	sv, err := version.ByDut(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "cros provision").Err()
	}
	mn := "update fw with fw-image by servo"
	am := info.GetActionArgs(ctx)
	imageName := am.AsString(ctx, "version_name", sv.GetFirmwareRoImagePath())
	log.Debugf(ctx, "Used fw image name: %s", imageName)
	gsBucket := am.AsString(ctx, "gs_bucket", gsCrOSImageBucket)
	log.Debugf(ctx, "Used gs bucket name: %s", gsBucket)
	gsImagePath := am.AsString(ctx, "gs_image_path", fmt.Sprintf("%s/%s", gsBucket, imageName))
	log.Debugf(ctx, "Used fw image path: %s", gsImagePath)
	fwDownloadDir := am.AsString(ctx, "fw_download_dir", defaultFwFolderPath(info.GetDut()))
	log.Debugf(ctx, "Used fw image path: %s", gsImagePath)
	// Requesting convert GC path to caches service path.
	// Example: `http://Addr:8082/download/my-bucket/board-firmware/RXX-XXXXX.XX.0/xyz.tar.bz2`
	downloadFilename, err := info.GetAccess().GetCacheUrl(ctx, info.GetDut().Name, gsImagePath)
	if err != nil {
		return errors.Annotate(err, mn).Err()
	}
	log.Debugf(ctx, "Adapted fw path for cache service: %s", downloadFilename)
	if !strings.HasSuffix(downloadFilename, firmwareTarSuffix) {
		fwFileName := am.AsString(ctx, "fw_filename", firmwareTarName)
		downloadFilename = fmt.Sprintf("%s/%s", downloadFilename, fwFileName)
		log.Debugf(ctx, "New path with specified filepath: %s", downloadFilename)
	}
	servod := info.NewServod()
	req := &firmware.InstallFirmwareImageRequest{
		DownloadImagePath:           downloadFilename,
		DownloadImageTimeout:        am.AsDuration(ctx, "download_timeout", 240, time.Second),
		DownloadDir:                 fwDownloadDir,
		Board:                       am.AsString(ctx, "dut_board", info.GetChromeos().GetBoard()),
		Model:                       am.AsString(ctx, "dut_model", info.GetChromeos().GetModel()),
		Hwid:                        am.AsString(ctx, "hwid", info.GetChromeos().GetHwid()),
		ForceUpdate:                 am.AsBool(ctx, "force", false),
		UpdateEcAttemptCount:        am.AsInt(ctx, "update_ec_attempt_count", 0),
		UpdateEcUseBoard:            am.AsBool(ctx, "update_ec_use_board", true),
		UpdateApAttemptCount:        am.AsInt(ctx, "update_ap_attempt_count", 0),
		GBBFlags:                    am.AsString(ctx, "gbb_flags", ""),
		CandidateFirmwareTarget:     am.AsString(ctx, "candidate_fw_target", ""),
		UseSerialTargets:            am.AsBool(ctx, "use_serial_fw_target", false),
		FlashThroughServo:           true,
		Servod:                      servod,
		ServoHostRunner:             info.NewRunner(info.GetChromeos().GetServo().GetName()),
		UseCacheToExtractor:         am.AsBool(ctx, "use_cache_extractor", false),
		DownloadImageReattemptCount: am.AsInt(ctx, "reattempt_count", 3),
		DownloadImageReattemptWait:  am.AsDuration(ctx, "reattempt_wait", 5, time.Second),
	}
	if am.AsBool(ctx, "use_fw_targets_from_inventory", false) {
		req.APTarget = info.GetChromeos().GetFirmwareInfo().GetApTarget()
		req.ECTarget = info.GetChromeos().GetFirmwareInfo().GetEcTarget()
	}
	err = firmware.InstallFirmwareImage(ctx, req, info.NewLogger())
	return errors.Annotate(err, mn).Err()
}

// defaultFwFolderPath provides default path to directory used for firmware extraction.
func defaultFwFolderPath(d *tlw.Dut) string {
	return fmt.Sprintf("/mnt/stateful_partition/tmp/fw_%v", d.Name)
}

// disableSoftwareWriteProtection disable software write protection by servo.
func disableSoftwareWriteProtectionByServo(ctx context.Context, info *execs.ExecInfo) error {
	runner := info.NewRunner(info.GetChromeos().GetServo().GetName())
	servodPort := int(info.GetChromeos().GetServo().GetServodPort())
	err := firmware.DisableSoftwareWriteProtectionByServo(ctx, runner, servodPort, info.GetExecTimeout())
	return errors.Annotate(err, "disable software write protection").Err()
}

func init() {
	execs.Register("cros_read_gbb_by_servo", readGbbFlagsByServoExec)
	execs.Register("cros_set_gbb_by_servo", setGbbFlagsByServoExec)
	execs.Register("cros_update_fw_with_fw_image_by_servo", updateFwWithFwImageByServo)
	execs.Register("cros_disable_software_write_protection_by_servo", disableSoftwareWriteProtectionByServo)
}
