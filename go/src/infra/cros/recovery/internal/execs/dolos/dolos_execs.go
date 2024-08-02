// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dolos

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

const (
	// User the doloscmd to query the Dolos status.
	dolosGetStatusCmdGlob = "/usr/bin/doloscmd get-status --serial %s"
)

// Eventually use the files generated from doloscmt.proto.
type dolosStatusResponse struct {
	Status string `json:"status"`
}

// determineAndSetDolosStateExec calculate the current Dolos state and update UFS.
func determineAndSetDolosStateExec(ctx context.Context, info *execs.ExecInfo) error {

	dolos := info.GetChromeos().GetDolos()
	if dolos == nil {
		return errors.Reason("dolos not enabled for this testbed.").Err()
	}
	previousState := info.GetChromeos().GetDolos().GetState()
	info.GetChromeos().GetDolos().State = tlw.Dolos_DOLOS_UNKNOWN

	resource := dolos.GetHostname()
	run := info.NewRunner(resource)
	output, err := run(ctx, info.GetExecTimeout(), fmt.Sprintf(dolosGetStatusCmdGlob, dolos.GetSerialCable()))
	if err != nil {
		return errors.Annotate(err, "set dolos state").Err()
	}

	var decoded dolosStatusResponse
	err = json.Unmarshal([]byte(output), &decoded)
	if err != nil {
		return errors.Annotate(err, "set dolos state").Err()
	}

	newState := decoded.Status
	log.Debugf(ctx, "Previous dolos state: %s", previousState)
	if v, ok := tlw.Dolos_State_value[newState]; ok {
		info.GetChromeos().GetDolos().State = tlw.Dolos_State(v)
		log.Infof(ctx, "Set dolos state to be: %s", newState)
		return nil
	}
	return errors.Reason("set dolos state: state is %q not found", newState).Err()
}

// determineAndSetDolosStateExec calculate the current Dolos state and update UFS.
func setDolosStateExec(ctx context.Context, info *execs.ExecInfo) error {
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
	execs.Register("dolos_determine_and_set_dolos_state", determineAndSetDolosStateExec)
	execs.Register("dolos_set_dolos_state", setDolosStateExec)
	execs.Register("dolos_does_not_need_reboot", dolosDoesNotNeedsRebootExec)
}
