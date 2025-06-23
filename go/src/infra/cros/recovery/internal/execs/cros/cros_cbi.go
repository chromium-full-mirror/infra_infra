// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// CBI corruption detection and repair logic. go/cbi-auto-recovery-dd
package cros

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/components/cros/cbi"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
)

// restoreCBIContentsFromUFS restores CBI contents on the DUT by writing the CBI contents stored
// in UFS to CBI EEPROM on the DUT.
func restoreCBIContentsFromUFS(ctx context.Context, info *execs.ExecInfo) error {
	runner := info.NewRunner(info.GetDut().Name)
	if info.GetChromeos().GetCbi() == nil {
		return errors.Reason("restore CBI contents from UFS: no previous CBI contents were found in UFS").Err()
	}

	cbiLocation, err := cbi.GetCBILocation(ctx, runner)
	if err != nil {
		return errors.Annotate(err, "restore CBI contents from UFS").Err()
	}

	err = cbi.WriteCBIContents(ctx, runner, cbiLocation, info.GetChromeos().GetCbi())
	return errors.WrapIf(err, "restore CBI contents from UFS")
}

// invalidateCBICache clears the current CBI cache to ensure that any existing
// CBI contents are up to date. Will throw an error if something unexpected occurs,
// but should otherwise always return nil.
func invalidateCBICache(ctx context.Context, info *execs.ExecInfo) error {
	runner := info.NewRunner(info.GetDut().Name)
	err := cbi.InvalidateCBICache(ctx, runner)
	return errors.WrapIf(err, "invalidate CBI cache")
}

// ufsContainsCBIContents returns nil if CBI Contents were previously stored for
// this DUT in UFS.
func ufsContainsCBIContents(ctx context.Context, info *execs.ExecInfo) error {
	if len(info.GetChromeos().GetCbi().GetRawContents()) == 0 {
		return errors.Reason("UFS contains CBI contents: no previous CBI contents were found in UFS").Err()
	}
	return nil
}

// ufsDoesNotContainCBIContents returns nil if CBI Contents were NOT previously
// stored for this DUT in UFS.
func ufsDoesNotContainCBIContents(ctx context.Context, info *execs.ExecInfo) error {
	if len(info.GetChromeos().GetCbi().GetRawContents()) != 0 {
		return errors.Reason("UFS does not contain CBI contents: previous CBI contents were found in UFS").Err()
	}
	return nil
}

// cbiIsPresent checks if CBI contents are found on the DUT.
func cbiIsPresent(ctx context.Context, info *execs.ExecInfo) error {
	runner := info.NewRunner(info.GetDut().Name)
	cbiLocation, err := cbi.GetCBILocation(ctx, runner)
	if err != nil {
		return errors.Annotate(err, "CBI is present").Err()
	}
	if cbiLocation == nil {
		return errors.Reason("CBI is present: no CBI contents were found on the DUT, but encountered no error - please submit a bug").Err()
	}
	return nil
}

// backupCBI reads the CBI contents on the DUT and stores them in UFS.
func backupCBI(ctx context.Context, info *execs.ExecInfo) error {
	runner := info.NewRunner(info.GetDut().Name)
	dutCBI, err := cbi.GetCBIContents(ctx, runner)
	if err != nil {
		return errors.Annotate(err, "backup CBI").Err()
	}

	info.GetChromeos().Cbi = dutCBI
	return err
}

// cbiContentsAreValidExec reads the CBI contents on the DUT and returns an
// error if they do not contain valid CBI magic or are missing any of the
// required fields.
func cbiContentsAreValidExec(ctx context.Context, info *execs.ExecInfo) error {
	runner := info.NewRunner(info.GetDut().Name)
	dutCBI, err := cbi.GetCBIContents(ctx, runner)
	if err != nil {
		return errors.Annotate(err, "CBI contents are valid").Err()
	}

	if !cbi.ContainsCBIMagic(dutCBI) {
		log.Debugf(ctx, "CBI contents are valid: CBI contents found on the DUT: %s", dutCBI.GetRawContents())
		return errors.Reason("CBI contents are valid: the CBI contents on the DUT do not contain valid magic").Err()
	}

	if err := cbi.VerifyRequiredFields(ctx, runner); err != nil {
		return errors.Annotate(err, "CBI contents are valid").Err()
	}

	return nil
}

func bootIntoRecoveryModeAndResetFirmwareConfig(ctx context.Context, info *execs.ExecInfo) error {
	am := info.GetActionArgs(ctx)
	dut := info.GetDut()
	runner := info.NewRunner(dut.Name)
	dutHa := info.NewHostAccess(dut.Name)
	dutBackgroundRun := info.NewBackgroundRunner(dut.Name)
	dutPing := info.NewPinger(dut.Name)
	servod := info.NewServod()
	cmdRunTimeout := time.Second * 10
	callback := func(_ context.Context) error {
		fwConfig, err := runner(ctx, cmdRunTimeout, "cros_config /firmware firmware-config")
		if err != nil {
			return errors.Annotate(err, "boot in recovery mode and reset firmware config").Err()
		}
		log.Debugf(ctx, "firmware config from os image: %s", fwConfig)
		fwConfigFromCbi, err := runner(ctx, cmdRunTimeout, "ectool cbi get 6 | head -n1 | cut -d' ' -f3")
		if err != nil {
			return errors.Annotate(err, "boot in recovery mode and reset firmware config").Err()
		}
		log.Debugf(ctx, "firmware config from CBI: %s", fwConfigFromCbi)
		if fwConfig != fwConfigFromCbi {
			log.Debugf(ctx, "Setting fw-config from %s to %s", fwConfigFromCbi, fwConfig)
			if _, err := runner(ctx, cmdRunTimeout, fmt.Sprintf("ectool cbi set 6 %s 4 0", fwConfig)); err != nil {
				return errors.Annotate(err, "boot in recovery mode and reset firmware config").Err()
			}
			info.AddObservation(metrics.NewStringObservation("resetFirmwareConfigCbi", fwConfig))
		} else {
			log.Debugf(ctx, "firmware config match between OS and CBI, no action needed")
		}
		return nil
	}
	req := &cros.BootInRecoveryRequest{
		DUT:             dut,
		BootRetry:       am.AsInt(ctx, "boot_retry", 1),
		BootTimeout:     am.AsDuration(ctx, "boot_timeout", 480, time.Second),
		BootInterval:    am.AsDuration(ctx, "boot_interval", 10, time.Second),
		PreventPowerSnk: am.AsBool(ctx, "prevent_power_snk", false),
		// Register that device booted and sshable.
		Callback:            callback,
		AddObservation:      info.AddObservation,
		IgnoreRebootFailure: am.AsBool(ctx, "ignore_reboot_failure", false),
		// After reboot action settings.
		AfterRebootVerify:             am.AsBool(ctx, "after_reboot_check", false),
		AfterRebootTimeout:            am.AsDuration(ctx, "after_reboot_timeout", 150, time.Second),
		AfterRebootAllowUseServoReset: am.AsBool(ctx, "after_reboot_allow_use_servo_reset", false),
	}
	if err := cros.BootInRecoveryMode(ctx, req, runner, dutBackgroundRun, dutPing, dutHa, servod, log.Get(ctx)); err != nil {
		return errors.Annotate(err, "boot in recovery mode and reset firmware config").Err()
	}
	// Time to wait DUT boot up from post installation.
	postResetBootTime := am.AsDuration(ctx, "post_reset_boot_time", 60, time.Second)
	log.Debugf(ctx, "Wait %s for DUT to boot up.", postResetBootTime)
	time.Sleep(postResetBootTime)
	return nil
}

func init() {
	execs.Register("cros_restore_cbi_contents_from_ufs", restoreCBIContentsFromUFS)
	execs.Register("cros_ufs_contains_cbi_contents", ufsContainsCBIContents)
	execs.Register("cros_ufs_does_not_contain_cbi_contents", ufsDoesNotContainCBIContents)
	execs.Register("cros_cbi_is_present", cbiIsPresent)
	execs.Register("cros_backup_cbi", backupCBI)
	execs.Register("cros_invalidate_cbi_cache", invalidateCBICache)
	execs.Register("cros_cbi_contents_are_valid", cbiContentsAreValidExec)
	execs.Register("cros_reset_firmware_config_in_cbi", bootIntoRecoveryModeAndResetFirmwareConfig)
}
