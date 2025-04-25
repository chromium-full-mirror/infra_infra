// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package human_motion_robot

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	errStateNotProvided  = "state is not provided"
	errHMRNotSupported   = "HMR is not supported"
	errTouchHostPiBroken = "cannot get HMR TouchHost errors, TouchHost is broken"
	errHMRBroken         = "HMR is broken"
	errMissingTouchHost  = "missing TouchHost"
)

// setHMRStateExec sets the hmr state of the from the actionArgs argument.
//
// @actionArgs: the list of the string that contains the hmr state information.
// It should only contain one string in the format of: "state:x"
// x must be one of the keys from HumanMotionRobot_State_value
func setHMRStateExec(ctx context.Context, info *execs.ExecInfo) error {
	args := info.GetActionArgs(ctx)
	newState := strings.ToUpper(args.AsString(ctx, "state", ""))
	if newState == "" {
		return errors.Reason("set hmr state: %q", errStateNotProvided).Err()
	}
	if info.GetChromeos().GetHumanMotionRobot() == nil {
		return errors.Reason("set hmr state: %q", errHMRNotSupported).Err()
	}
	log.Debugf(ctx, "Previous hmr state: %s", info.GetChromeos().GetHumanMotionRobot().GetState())
	if v, ok := tlw.HumanMotionRobot_State_value[newState]; ok {
		info.GetChromeos().GetHumanMotionRobot().State = tlw.HumanMotionRobot_State(v)
		log.Infof(ctx, "Set hmr state to be: %s", newState)
		return nil
	}
	return errors.Reason("set hmr state: state is %q not found", newState).Err()
}

// checkHMRStateExec calls XMLRPC api on Touchhost to check the hmr state.
func checkHMRStateExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetHumanMotionRobot() == nil {
		return errors.Reason("check hmr state: %q", errHMRNotSupported).Err()
	}

	if info.GetChromeos().GetHumanMotionRobot().GetTouchhost() != "" {
		res, err := Call(ctx, info.GetAccess(), info.GetChromeos().GetHumanMotionRobot(), "GetErrors")
		if err != nil {
			return errors.Annotate(err, errTouchHostPiBroken).Err()
		}

		vals := res.GetArray().GetValues()
		count := len(vals)

		if count > 0 {
			for _, val := range vals {
				log.Errorf(ctx, "GetErrors: %s", val.GetString_())
			}
			return errors.Reason(errHMRBroken).Err()
		}
	}

	return nil
}

func init() {
	execs.Register("set_hmr_state", setHMRStateExec)
	execs.Register("check_hmr_state", checkHMRStateExec)
}
