// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros/amt"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// flexSetAMTPowerStateExec sets the specified power state.
func flexSetAMTPowerStateExec(ctx context.Context, info *execs.ExecInfo) error {
	args := info.GetActionArgs(ctx)
	newState := strings.ToLower(args.AsString(ctx, "state", ""))
	if newState == "" {
		return errors.Reason("flex set AMT power state: state is not provided").Err()
	}
	client, err := getFlexAMTClient(ctx, info)
	if err != nil {
		return errors.Reason("flex set AMT power state: failed to create client").Err()
	}
	return errors.Annotate(client.SetPowerState(ctx, newState), "flex set AMT power state").Err()
}

// Configure and return an AMTClient.
func getFlexAMTClient(ctx context.Context, info *execs.ExecInfo) (*amt.AMTClient, error) {
	dut := info.GetDut()
	hostname, err := retrieveAMTHostname(ctx, dut)
	if err != nil {
		return nil, err
	}
	useTLS := dut.GetChromeos().GetAmtManager().GetUseTls()
	// b/353671548: Store the AMT password somewhere else.
	return amt.NewAMTClient(ctx, hostname, "admin", "P@ssword1", useTLS), nil
}

// Retrieve the AMT hostname.
func retrieveAMTHostname(ctx context.Context, dut *tlw.Dut) (string, error) {
	if dut.GetChromeos().GetAmtManager() == nil {
		return "", errors.Reason("flex get AMT client: amt_manager is not supported").Err()
	}
	hostname := dut.GetChromeos().GetAmtManager().GetHostname()
	if hostname == "" {
		return "", errors.Reason("flex get AMT client: hostname is empty").Err()
	}
	return hostname, nil
}

// flexAMTKnownExec checks if AMT management details are present for the DUT.
func flexAMTKnownExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetDut().GetChromeos().GetAmtManager() == nil {
		return errors.Reason("flex AMT present: amt_manager is not supported").Err()
	}
	return nil
}

// Ping addr for up to timeout seconds until it reponds.
func flexPingAMT(addr string, timeout int) error {
	for range timeout {
		// Ping once with a timeout of one second.
		cmd := exec.Command("ping", addr, "-c", "1", "-W", "1")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return errors.Reason("failed to ping AMT").Err()
}

// flexAMTRespondsToPing pings the AMT interface until is responds.
func flexAMTRespondsToPingExec(ctx context.Context, info *execs.ExecInfo) error {
	dut := info.GetDut()
	amtHostname, err := retrieveAMTHostname(ctx, dut)
	if err != nil {
		return err
	}
	return errors.Annotate(flexPingAMT(amtHostname, 30), "flex AMT responds to ping").Err()
}

func init() {
	execs.Register("cros_flex_amt_known", flexAMTKnownExec)
	execs.Register("cros_flex_set_amt_power_state", flexSetAMTPowerStateExec)
	execs.Register("cros_flex_amt_available", flexAMTRespondsToPingExec)
}
