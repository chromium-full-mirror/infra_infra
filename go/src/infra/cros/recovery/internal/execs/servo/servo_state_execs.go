// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// setServoStateExec sets the servo state of the servo of the DUT from the actionArgs argument.
//
// @actionArgs: the list of the string that contains the servo state information.
// It should only contain one string in the format of: "state:x"
// x must be all capatalized and matched one of the record in the predefined servo state.
func setServoStateExec(ctx context.Context, info *execs.ExecInfo) error {
	args := info.GetActionArgs(ctx)
	newState := strings.ToUpper(args.AsString(ctx, "state", ""))
	if newState == "" {
		return errors.Reason("set servo state: state is not provided").Err()
	}
	// Verify if servo is supported.
	// If servo is not supported the report failure.
	if info.GetChromeos().GetServo() == nil {
		return errors.Reason("set servo state: servo is not supported").Err()
	}
	log.Debugf(ctx, "Previous servo state: %s", info.GetChromeos().GetServo().GetState())
	if v, ok := tlw.ServoHost_State_value[newState]; ok {
		info.GetChromeos().GetServo().State = tlw.ServoHost_State(v)
		log.Infof(ctx, "Set servo state to be: %s", newState)
		return nil
	}
	return errors.Reason("set servo state: state is %q not found", newState).Err()
}

// matchStateExec confirms the servo state is the same as the passed in argument from the configuration.
//
// format of the actionArgs: ["state:xxx"] where xxx is one of the predefined servo state.
func matchStateExec(ctx context.Context, info *execs.ExecInfo) error {
	args := info.GetActionArgs(ctx)
	expectedState := strings.ToLower(args.AsString(ctx, "state", ""))
	if expectedState == "" {
		return errors.Reason("match state: state not provided").Err()
	}
	if sh := info.GetChromeos().GetServo(); sh != nil {
		currentState := strings.ToLower(sh.GetState().String())
		if currentState != expectedState {
			return errors.Reason("match state: state mismatch, expected: %q, but got %q", expectedState, currentState).Err()
		}
		return nil
	}
	return errors.Reason("match state: current servo state is unknown").Err()
}

func init() {
	execs.Register("servo_set_servo_state", setServoStateExec)
	execs.Register("servo_match_state", matchStateExec)
}
