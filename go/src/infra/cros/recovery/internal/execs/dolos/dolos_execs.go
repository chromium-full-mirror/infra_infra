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
	// User the doloscmd to query the Dolos status.
	dolosFindUartCmdGlob       = "/usr/bin/doloscmd find-uartname --serial %s"
	dolosGetStatusCableCmdGlob = "/usr/bin/doloscmd get-status --serial %s"
	dolosGetStatusUsbCmdGlob   = "/usr/bin/doloscmd get-status --uartname %s"
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
	run := info.NewRunner(dolos.GetHostname())
	output, err := run(ctx, info.GetExecTimeout(), fmt.Sprintf(dolosFindUartCmdGlob, dolos.GetSerialCable()))
	if err != nil {
		return errors.Annotate(err, "update dolos UART: fail to read data").Err()
	}

	var decoded FindUartNameResponse
	if err := protojson.Unmarshal([]byte(output), &decoded); err != nil {
		return errors.Annotate(err, "update dolos UART: fail to parse results").Err()
	}
	log.Infof(ctx, "Found dolos uartname %s.", decoded.Uartname)
	dolos.SerialUsb = decoded.Uartname

	return nil
}

// determineAndSetStateExec calculate the current Dolos state and update UFS.
func determineAndSetStateExec(ctx context.Context, info *execs.ExecInfo) error {

	dolos := info.GetChromeos().GetDolos()

	previousState := dolos.GetState()
	dolos.State = tlw.Dolos_DOLOS_UNKNOWN

	run := info.NewRunner(dolos.GetHostname())
	command := ""

	if dolos.GetSerialUsb() != "" {
		command = fmt.Sprintf(dolosGetStatusUsbCmdGlob, dolos.GetSerialUsb())
	} else {
		command = fmt.Sprintf(dolosGetStatusCableCmdGlob, dolos.GetSerialCable())
	}
	output, err := run(ctx, info.GetExecTimeout(), command)
	if err != nil {
		return errors.Annotate(err, "determine dolos state").Err()
	}

	var decoded GetStatusResponse
	err = protojson.Unmarshal([]byte(output), &decoded)
	if err != nil {
		return errors.Annotate(err, "determine dolos state").Err()
	}

	newState := decoded.Status.String()
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

func init() {
	execs.Register("dolos_does_not_need_reboot", dolosDoesNotNeedsRebootExec)
	execs.Register("dolos_determine_and_set_dolos_state", determineAndSetStateExec)
	execs.Register("dolos_set_dolos_state", setStateExec)
	execs.Register("dolos_is_uartname_cached", isUartnameCachedExec)
	execs.Register("dolos_is_enabled", isEnabledForTestbedExec)
	execs.Register("dolos_update_uartname_cache", updateUartNameExec)
}
