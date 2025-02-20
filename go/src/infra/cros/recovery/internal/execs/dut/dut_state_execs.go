// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// dutStateReadyActionExec sets dut-state as ready.
func dutStateReadyActionExec(ctx context.Context, info *execs.ExecInfo) error {
	info.GetDut().State = dutstate.Ready
	return nil
}

// dutStateRepairFailedActionExec sets dut-state as repair_failed.
func dutStateRepairFailedActionExec(ctx context.Context, info *execs.ExecInfo) error {
	info.GetDut().State = dutstate.RepairFailed
	return nil
}

// dutStateNeedsDeployActionExec sets dut-state as needs_deploy.
func dutStateNeedsDeployActionExec(ctx context.Context, info *execs.ExecInfo) error {
	info.GetDut().State = dutstate.NeedsDeploy
	return nil
}

// dutStateReservedActionExec sets dut-state as reserved.
func dutStateReservedActionExec(ctx context.Context, info *execs.ExecInfo) error {
	info.GetDut().State = dutstate.Reserved
	return nil
}

// dutStateMatchExec checks whether the state of the DUT matches the
// expected state as mentioned in the extra arguments.
func dutStateMatchExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	expectedState := argsMap.AsString(ctx, "state", "")
	invertResult := argsMap.AsBool(ctx, "invert", false)
	log.Debugf(ctx, "Dut State Match Exec: actual DUT state: %s, expected state :%s", string(info.GetDut().State), expectedState)
	matched := info.GetDut().State == dutstate.State(expectedState)
	if matched && invertResult {
		return errors.Reason("dut state match exec: the actual dut state %s matches the expected state %s, and result of comparition is inverted.", info.GetDut().State, expectedState).Err()
	} else if !matched && !invertResult {
		return errors.Reason("dut state match exec: the actual dut state %s does not the expected state %s.", info.GetDut().State, expectedState).Err()
	}
	return nil
}

func init() {
	execs.Register("dut_state_ready", dutStateReadyActionExec)
	execs.Register("dut_state_repair_failed", dutStateRepairFailedActionExec)
	execs.Register("dut_state_needs_deploy", dutStateNeedsDeployActionExec)
	execs.Register("dut_state_reserved", dutStateReservedActionExec)
	execs.Register("dut_state_match", dutStateMatchExec)
}
