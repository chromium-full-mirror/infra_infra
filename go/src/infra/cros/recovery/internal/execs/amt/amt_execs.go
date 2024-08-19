// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package amt contains execs used by the AMTManager peripheral.
package amt

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/cros/amt"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
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
	// b/353671548: Store the AMT password somewhere else.
	client := amt.NewAMTClient(hostname, "admin", "P@ssword1")
	//TODO(b/353283943): Implement a more granular AMT health check.
	//
	// Use this as a health check for now, since it implicity verifies that we can
	// authenticate and get responses to our requests.
	_, err := client.GetPowerState(ctx)
	return errors.Annotate(err, "flex AMT is not healthy").Err()
}

func init() {
	execs.Register("amt_manager_is_healthy", healthCheckExec)
	execs.Register("amt_manager_set_state", setAMTStateExec)
}
