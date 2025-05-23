// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dolos

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/dolos"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

func isEnabledForTestbedExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetDolos() == nil {
		return errors.Reason("dolos not enabled for this testbed.").Err()
	}
	return nil
}

func isUartnameCachedExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetDolos().GetSerialUsb() == "" {
		return errors.Reason("dolos uart not cached for this device.").Err()
	}
	return nil
}

func updateUartNameExec(ctx context.Context, info *execs.ExecInfo) error {
	dolosInfo := info.GetChromeos().GetDolos()
	dutRun := info.NewRunner(dolosInfo.GetHostname())

	uartName, err := dolos.FindUart(ctx, dutRun, dolosInfo, info.GetExecTimeout())
	if err != nil {
		return errors.Annotate(err, "unable to get dolos UART").Err()
	}
	dolosInfo.SerialUsb = uartName

	return nil
}

// determineAndSetStateExec calculate the current Dolos state and update UFS.
func determineAndSetStateExec(ctx context.Context, info *execs.ExecInfo) error {
	dolosInfo := info.GetChromeos().GetDolos()
	dutRun := info.NewRunner(dolosInfo.GetHostname())

	previousState := dolosInfo.GetState()
	dolosInfo.State = tlw.Dolos_DOLOS_UNKNOWN

	status, err := dolos.GetStatus(ctx, dutRun, dolosInfo, info.GetExecTimeout())
	if err != nil {
		return errors.Annotate(err, "unable to get dolos status").Err()
	}

	newState := status
	log.Debugf(ctx, "Previous dolos state: %s", previousState)
	if v, ok := tlw.Dolos_State_value[newState]; ok {
		dolosInfo.State = tlw.Dolos_State(v)
		log.Infof(ctx, "Set dolos state to be: %s", newState)
		if dolosInfo.State == tlw.Dolos_DOLOS_NOT_PRESENT {
			dolosInfo.SerialUsb = ""
		}
		return nil
	}
	return errors.Reason("determine dolos state: state is %q not found", newState).Err()
}

func setStateExec(ctx context.Context, info *execs.ExecInfo) error {
	args := info.GetActionArgs(ctx)
	newState := strings.ToUpper(args.AsString(ctx, "state", ""))
	if newState == "" {
		return errors.Reason("set dolos state: state is not provided").Err()
	}
	// Verify if dolos is supported.
	// If dolos is not supported the report failure.
	if info.GetChromeos().GetDolos() == nil {
		return errors.Reason("set dolos state: Dolos is not supported").Err()
	}
	log.Debugf(ctx, "Previous dolos state: %s", info.GetChromeos().GetDolos().GetState())
	if v, ok := tlw.Dolos_State_value[newState]; ok {
		info.GetChromeos().GetDolos().State = tlw.Dolos_State(v)
		log.Infof(ctx, "Set dolos state to be: %s", newState)
		return nil
	}
	return errors.Reason("set dolos state: state is %q not found", newState).Err()
}

// checkDoesNotNeedsRebootExec look at status and decide if Dolos needs to be rebooted.
func checkDoesNotNeedsRebootExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetDolos() == nil {
		return errors.Reason("dolos is not supported").Err()
	}
	log.Debugf(ctx, "Dolos state: %s", info.GetChromeos().GetDolos().GetState())
	if info.GetChromeos().GetDolos().GetState() != tlw.Dolos_DOLOS_OK {
		return errors.Reason("dolos does need reboot").Err()
	}
	return nil
}

// determineAndSetStateExec calculate the current Dolos state and update UFS.
func checkFirmwareUpToDateExec(ctx context.Context, info *execs.ExecInfo) error {
	dolosInfo := info.GetChromeos().GetDolos()
	dutRun := info.NewRunner(dolosInfo.GetHostname())

	currentVersion, err := dolos.GetVersion(ctx, dutRun, dolosInfo, info.GetExecTimeout())
	if err != nil {
		log.Infof(ctx, "Unable to determine dolos version, so do not try to upgrade")
		return nil
	}

	if dolosInfo.GetFwVersion() == "" {
		dolosInfo.FwVersion = currentVersion
		log.Infof(ctx, "Setting UFS dolos firmware version to: %s", dolosInfo.FwVersion)
		// No firmware update required.
		return nil
	}

	if dolosInfo.GetFwVersion() != currentVersion {
		return errors.Reason("dolos version does not match ufs").Err()
	}

	return nil
}

// updateFirmwareExec update the firmware to the version in UFS if required.
func updateFirmwareExec(ctx context.Context, info *execs.ExecInfo) error {
	dolosInfo := info.GetChromeos().GetDolos()
	dutRun := info.NewRunner(dolosInfo.GetHostname())
	return dolos.UpdateFirmware(ctx, dutRun, dolosInfo, info.GetExecTimeout())
}

// repairExec calculate the current Dolos state and update UFS.
func repairExec(ctx context.Context, info *execs.ExecInfo) error {
	dolosInfo := info.GetChromeos().GetDolos()
	dutRun := info.NewRunner(dolosInfo.GetHostname())
	return dolos.CallRepair(ctx, dutRun, dolosInfo, info.GetExecTimeout())
}

func init() {
	execs.Register("dolos_does_not_need_reboot", checkDoesNotNeedsRebootExec)
	execs.Register("dolos_determine_and_set_dolos_state", determineAndSetStateExec)
	execs.Register("dolos_set_dolos_state", setStateExec)
	execs.Register("dolos_is_uartname_cached", isUartnameCachedExec)
	execs.Register("dolos_is_enabled", isEnabledForTestbedExec)
	execs.Register("dolos_update_uartname_cache", updateUartNameExec)
	execs.Register("dolos_update_firmware", updateFirmwareExec)
	execs.Register("dolos_check_firmware_up_to_date", checkFirmwareUpToDateExec)
	execs.Register("dolos_call_repair", repairExec)
}
