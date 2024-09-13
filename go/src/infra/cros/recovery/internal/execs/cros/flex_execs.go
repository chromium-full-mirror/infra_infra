// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/cros/amt"
	"infra/cros/recovery/internal/execs"
)

// flexSetAMTPowerStateExec sets the specified power state.
func flexSetAMTPowerStateExec(ctx context.Context, info *execs.ExecInfo) error {
	args := info.GetActionArgs(ctx)
	newState := strings.ToLower(args.AsString(ctx, "state", ""))
	if newState == "" {
		return errors.Reason("flex set AMT power state: state is not provided").Err()
	}
	client, err := getFlexAMTClient(info)
	if err != nil {
		return errors.Reason("flex set AMT power state: failed to create client").Err()
	}
	return errors.Annotate(client.SetPowerState(ctx, newState), "flex set AMT power state").Err()
}

// Configure and return an AMTClient.
func getFlexAMTClient(info *execs.ExecInfo) (*amt.AMTClient, error) {
	dut := info.GetDut()
	if dut.GetChromeos().GetAmtManager() == nil {
		return nil, errors.Reason("flex get AMT client: amt_manager is not supported").Err()
	}
	hostname := dut.GetChromeos().GetAmtManager().GetHostname()
	if hostname == "" {
		return nil, errors.Reason("flex get AMT client: hostname is empty").Err()
	}
	// b/353671548: Store the AMT password somewhere else.
	return amt.NewAMTClient(hostname, "admin", "P@ssword1"), nil
}

// flexAMTKnownExec checks if AMT management details are present for the DUT.
func flexAMTKnownExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetDut().GetChromeos().GetAmtManager() == nil {
		return errors.Reason("flex AMT present: amt_manager is not supported").Err()
	}
	return nil
}

func init() {
	execs.Register("cros_flex_amt_known", flexAMTKnownExec)
	execs.Register("cros_flex_set_amt_power_state", flexSetAMTPowerStateExec)
}
