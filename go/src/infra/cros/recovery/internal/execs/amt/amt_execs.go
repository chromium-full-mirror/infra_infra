// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package amt contains execs used by the AMTManager peripheral.
package amt

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros/amt"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// setAMTStateExec sets the state of AMTManager to the value passed in
// the action arguments.
func setAMTStateExec(ctx context.Context, info *execs.ExecInfo) error {
	args := info.GetActionArgs(ctx)
	newState := strings.ToUpper(args.AsString(ctx, "state", ""))
	if newState == "" {
		return errors.Reason("set amt_manager state: state is not provided").Err()
	}
	// Verify AMT is supported.
	if info.GetChromeos().GetAmtManager() == nil {
		return errors.Reason("set amt_manager state: amt_manager is not supported").Err()
	}

	log.Debugf(ctx, "Old amt_manager state: %s", info.GetChromeos().GetAmtManager().GetState())
	if value, exists := tlw.AMTManager_State_value[newState]; exists {
		info.GetChromeos().GetAmtManager().State = tlw.AMTManager_State(value)
		log.Infof(ctx, "New amt_manager state: %s", newState)
		return nil
	}
	return errors.Reason("set amt_manager state: state %q not found", newState).Err()
}

// healthCheckExec checks if we can retrieve the power state.
func healthCheckExec(ctx context.Context, info *execs.ExecInfo) error {
	dut := info.GetDut()
	if dut.GetChromeos().GetAmtManager() == nil {
		return errors.Reason("check amt_manager health: amt_manager is not supported").Err()
	}
	hostname := dut.GetChromeos().GetAmtManager().GetHostname()
	if hostname == "" {
		return errors.Reason("check amt_manager health: hostname is empty").Err()
	}
	useTLS := dut.GetChromeos().GetAmtManager().GetUseTls()
	// b/353671548: Store the AMT password somewhere else.
	client := amt.NewAMTClient(ctx, hostname, "admin", "P@ssword1", useTLS)
	//TODO(b/353283943): Implement a more granular AMT health check.
	//
	// Use this as a health check for now, since it implicity verifies that we can
	// authenticate and get responses to our requests.
	_, err := client.GetPowerState(ctx)
	return errors.Annotate(err, "flex AMT is not healthy").Err()
}

// amtManagerNotPresentExec checks if the AMT manager is absent.
// returns an error if the AMT manager is present.
func amtManagerNotPresentExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetChromeos().GetAmtManager().GetHostname() != "" {
		return errors.Reason("amt_manager not present: hostname exists").Err()
	}
	return nil
}

func init() {
	execs.Register("amt_manager_is_healthy", healthCheckExec)
	execs.Register("amt_manager_set_state", setAMTStateExec)
	execs.Register("amt_manager_not_present", amtManagerNotPresentExec)
}
