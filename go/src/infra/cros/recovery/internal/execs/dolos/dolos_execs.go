// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dolos

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

const (
	dolosCmd             = "/usr/bin/doloscmd "
	dolosSubCmdGetStatus = "get-status"
	dolosSubCmdVersion   = "version"
	dolosSubCmdFwUpdate  = "firmware-update"
	dolosSubCmdFindUart  = "find-uartname"
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
	dolos := info.GetChromeos().GetDolos()

	output, err := runDolosCommand(ctx, info, dolos, dolosSubCmdFindUart)
	if err != nil {
		return errors.Annotate(err, "unable to get dolos UART").Err()
	}

	var decoded FindUartNameResponse
	if err := protojson.Unmarshal([]byte(output), &decoded); err != nil {
		return errors.Annotate(err, "update dolos UART: fail to parse results").Err()
	}
	log.Infof(ctx, "Found dolos uartname %s.", decoded.GetUartname())
	dolos.SerialUsb = decoded.GetUartname()

	return nil
}

// determineAndSetStateExec calculate the current Dolos state and update UFS.
func determineAndSetStateExec(ctx context.Context, info *execs.ExecInfo) error {

	dolos := info.GetChromeos().GetDolos()

	previousState := dolos.GetState()
	dolos.State = tlw.Dolos_DOLOS_UNKNOWN

	output, err := runDolosCommand(ctx, info, dolos, dolosSubCmdGetStatus)
	if err != nil {
		return errors.Annotate(err, "unable to get dolos status").Err()
	}
	var decoded GetStatusResponse
	err = protojson.Unmarshal([]byte(output), &decoded)
	if err != nil {
		return errors.Annotate(err, "determine dolos state").Err()
	}

	newState := decoded.GetStatus().String()
	log.Debugf(ctx, "Previous dolos state: %s", previousState)
	if v, ok := tlw.Dolos_State_value[newState]; ok {
		dolos.State = tlw.Dolos_State(v)
		log.Infof(ctx, "Set dolos state to be: %s", newState)
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

// dolosDoesNotNeedsRebootExec look at status and decide if Dolos needs to be rebooted.
func dolosDoesNotNeedsRebootExec(ctx context.Context, info *execs.ExecInfo) error {
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

	dolos := info.GetChromeos().GetDolos()

	output, err := runDolosCommand(ctx, info, dolos, dolosSubCmdVersion)
	if err != nil {
		log.Infof(ctx, "Unable to determine dolos version, so do not try to upgrade")
		return nil
	}

	var decoded GetVersionResponse
	err = protojson.Unmarshal([]byte(output), &decoded)
	if err != nil {
		return errors.Annotate(err, "determine dolos version").Err()
	}

	if dolos.GetFwVersion() == "" {
		dolos.FwVersion = decoded.GetVersion()
		log.Infof(ctx, "Setting UFS dolos firmware version to: %s", dolos.FwVersion)
		// No firmware update required.
		return nil
	}

	if dolos.GetFwVersion() != decoded.GetVersion() {
		return errors.Reason("dolos version does not match ufs").Err()
	}

	return nil
}

// determineAndSetStateExec calculate the current Dolos state and update UFS.
func updateDolosFirmwareExec(ctx context.Context, info *execs.ExecInfo) error {

	dolos := info.GetChromeos().GetDolos()

	log.Infof(ctx, "Update dolos firmware from to %s", dolos.FwVersion)
	_, err := runDolosCommand(ctx, info, dolos, fmt.Sprintf("update-firmware --firmware_version %s ", dolos.FwVersion))
	if err != nil {
		return errors.Annotate(err, "unable to update dolos version").Err()
	}
	return nil
}

func runDolosCommand(ctx context.Context, info *execs.ExecInfo, dolos *tlw.Dolos, dolosSubCommand string) (string, error) {
	run := info.NewRunner(dolos.GetHostname())
	command := dolosCmd

	if dolos == nil {
		return "", errors.Reason("dolos ufs data is missing.").Err()
	}

	if dolos.GetSerialUsb() != "" {
		command += dolosSubCommand + " --uartname " + dolos.GetSerialUsb()
	} else {
		command += dolosSubCommand + " --serial " + dolos.GetSerialCable()
	}

	output, err := run(ctx, info.GetExecTimeout(), command)
	log.Infof(ctx, "Dolos command %s returned %s ", command, output)
	return output, err
}

func init() {
	execs.Register("dolos_does_not_need_reboot", dolosDoesNotNeedsRebootExec)
	execs.Register("dolos_determine_and_set_dolos_state", determineAndSetStateExec)
	execs.Register("dolos_set_dolos_state", setStateExec)
	execs.Register("dolos_is_uartname_cached", isUartnameCachedExec)
	execs.Register("dolos_is_enabled", isEnabledForTestbedExec)
	execs.Register("dolos_update_uartname_cache", updateUartNameExec)
	execs.Register("dolos_update_firmware", updateDolosFirmwareExec)
	execs.Register("dolos_check_firmware_up_to_date", checkFirmwareUpToDateExec)
}
