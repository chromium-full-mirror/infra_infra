// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// updateCr50LabelExec will update the DUT's Cr50Phase state into the corresponding Cr50 state.
func updateCr50LabelExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	// Example of the rwVersion: `0.5.40`
	rwVersion, err := GetCr50FwVersion(ctx, r, CR50RegionRW)
	if err != nil {
		info.GetChromeos().Cr50Phase = tlw.ChromeOS_CR50_PHASE_UNSPECIFIED
		return errors.Annotate(err, "update cr50 label").Err()
	}
	rwVersionComponents := strings.Split(rwVersion, ".")
	if len(rwVersionComponents) < 2 {
		info.GetChromeos().Cr50Phase = tlw.ChromeOS_CR50_PHASE_UNSPECIFIED
		return errors.Reason("update cr50 label: the number of version component in the rw version is incorrect.").Err()
	}
	// Check the major version to determine prePVT vs PVT.
	// Ex:
	// rwVersionComponents: ["0", "5", "40"].
	// marjoRwVersion: integer value of 5.
	majorRwVersion, err := strconv.ParseInt(rwVersionComponents[1], 10, 64)
	if err != nil {
		info.GetChromeos().Cr50Phase = tlw.ChromeOS_CR50_PHASE_UNSPECIFIED
		return errors.Annotate(err, "update cr50 label").Err()
	}
	if majorRwVersion%2 != 0 {
		// PVT image has a odd major version number.
		// prePVT image has an even major version number.
		info.GetChromeos().Cr50Phase = tlw.ChromeOS_CR50_PHASE_PVT
	} else {
		info.GetChromeos().Cr50Phase = tlw.ChromeOS_CR50_PHASE_PREPVT
	}
	log.Infof(ctx, "update DUT's Cr50 to be %s", info.GetChromeos().GetCr50Phase())
	return nil
}

// updateCr50KeyIdLabelExec will update the DUT's Cr50KeyEnv state into the corresponding Cr50 key id state.
func updateCr50KeyIdLabelExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	roKeyIDString, err := GetCr50FwKeyID(ctx, r, CR50RegionRO)
	if err != nil {
		info.GetChromeos().Cr50KeyEnv = tlw.ChromeOS_CR50_KEYENV_UNSPECIFIED
		return errors.Annotate(err, "update cr50 key id").Err()
	}
	// Trim "," due to the remaining of the regular expression.
	// Trim "0x" due to the restriction of golang's ParseInt only taking the hex number without "0x".
	// Ex:
	// Before Trim: "0xffffff,"
	// After Trim: "ffffff"
	roKeyIDString = strings.Trim(roKeyIDString, ",0x")
	roKeyID, err := strconv.ParseInt(roKeyIDString, 16, 64)
	if err != nil {
		info.GetChromeos().Cr50KeyEnv = tlw.ChromeOS_CR50_KEYENV_UNSPECIFIED
		return errors.Annotate(err, "update cr50 key id").Err()
	}
	if roKeyID&(1<<2) != 0 {
		info.GetChromeos().Cr50KeyEnv = tlw.ChromeOS_CR50_KEYENV_PROD
	} else {
		info.GetChromeos().Cr50KeyEnv = tlw.ChromeOS_CR50_KEYENV_DEV
	}
	log.Infof(ctx, "update DUT's Cr50 Key Env to be %s", info.GetChromeos().GetCr50KeyEnv())
	return nil
}

// reflashCr50FwExec reflashes CR50 firmware and reboot AP from DUT side to wake it up.
//
// @params: actionArgs should be in the format of:
// Ex: ["flash_timeout:x", "wait_timeout:x"]
func reflashCr50FwExec(ctx context.Context, info *execs.ExecInfo) (rErr error) {
	argsMap := info.GetActionArgs(ctx)
	// Timeout for executing the cr50 fw flash command on the DUT. Default to be 120s.
	flashTimeout := argsMap.AsDuration(ctx, "flash_timeout", 120, time.Second)
	// Delay to wait for the fw flash command to be efftive. Default to be 30s.
	waitTimeout := argsMap.AsDuration(ctx, "wait_timeout", 30, time.Second)
	// Command to update cr50 firmware with post-reset and reboot the DUT.
	updateCmd := `gsctool -ap /opt/google/cr50/firmware/cr50.bin.%s`
	if info.GetChromeos().GetCr50Phase() == tlw.ChromeOS_CR50_PHASE_PREPVT {
		updateCmd = fmt.Sprintf(updateCmd, "prepvt")
	} else {
		updateCmd = fmt.Sprintf(updateCmd, "prod")
	}
	karteAction := info.NewMetric(metrics.Cr50FwReflashKind)
	// TODO(b/248635230): When karte' Search API is capable of taking in asset tag,
	// change the query to use asset tag instead of using hostname.
	defer func() {
		// Recoding cr 50 fw reflash to Karte.
		log.Debugf(ctx, "Updating cr 50 fw reflash record in Karte.")
		karteAction.StopTime = time.Now()
		karteAction.UpdateStatus(rErr)
	}()
	run := info.NewRunner(info.GetDut().Name)
	runBackground := info.NewBackgroundRunner(info.GetDut().Name)
	// For "gsctool", we use the traditional runner because the exit code of both 0 and 1
	// indicates successful execution of the command.
	//
	// r.ExitCode == 0: All up to date, no update needed.
	// r.ExitCode == 1: Update completed, reboot required (errors includes GsctoolRequireRebootError tag).
	_, err := run(ctx, flashTimeout, updateCmd)
	if err != nil {
		errorCode, ok := execs.ErrCodeTag.Value(err)
		if !ok {
			return errors.Annotate(err, "reflash cr50 fw: cannot find error code").Err()
		}
		if errorCode != 1 {
			return errors.Annotate(err, "reflash cr50 fw: fail to flash %q", info.GetChromeos().GetCr50Phase()).Err()
		}
	}
	log.Debugf(ctx, "cr50 fw update successfully.")
	// reboot the DUT for the reflash of the cr50 fw to be effective.
	if out, err := runBackground(ctx, 30*time.Second, "reboot"); err != nil {
		// Client closed connected as rebooting.
		log.Debugf(ctx, "Client exit as device rebooted: %s", err)
		return errors.Annotate(err, "reflash cr50 fw").Err()
	} else {
		log.Debugf(ctx, "Stdout: %s", out)
	}
	log.Debugf(ctx, "waiting for %d seconds to let cr50 fw reflash be effective.", waitTimeout)
	time.Sleep(waitTimeout)
	return nil
}

// isFwWriteProtectionEnabled check if WP is forced to be enabled.
// Check if fwmp shows up in the gsctool -aw output as WP being forced enabled.
// Example output:
//
//	Flash WP: fwmp forced enabled
//	 at boot: fwmp enabled
func isFWMPForceWPEnabled(ctx context.Context, info *execs.ExecInfo) error {
	ha := info.NewHostAccess(info.GetDut().Name)
	res, err := ha.Run(ctx, info.GetExecTimeout(), "gsctool -aw")
	if err != nil {
		return errors.Annotate(err, "if FWMP force WP enabled").Err()
	}
	out := res.GetStdout()
	log.Debugf(ctx, "FW WP output from gsctool: \n%s", res.GetStdout())
	expectedMessages := []string{
		"Flash WP: fwmp forced enabled",
		"at boot: fwmp enabled",
	}
	for _, msg := range expectedMessages {
		if !strings.Contains(out, msg) {
			return errors.Reason("if FWMP force WP enabled: fwmp is not enforce it").Err()
		}
	}
	log.Infof(ctx, "FWMP force WP being enabled!")
	return nil
}

func init() {
	execs.Register("cros_update_cr50_label", updateCr50LabelExec)
	execs.Register("cros_update_cr50_key_id_label", updateCr50KeyIdLabelExec)
	execs.Register("cros_reflash_cr50_fw", reflashCr50FwExec)
	execs.Register("cros_is_fwmp_force_wp_enabled", isFWMPForceWPEnabled)
}
