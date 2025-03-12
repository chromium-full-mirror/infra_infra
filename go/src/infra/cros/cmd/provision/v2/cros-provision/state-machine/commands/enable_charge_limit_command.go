// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"go.chromium.org/chromiumos/test/provision/v2/cros-provision/service"
	"context"
	"fmt"
	"log"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type EnableChargeLimitCommand struct {
	ctx context.Context
	cs  *service.CrOSService
}

func NewEnableChargeLimitCommand(ctx context.Context, cs *service.CrOSService) *EnableChargeLimitCommand {
	return &EnableChargeLimitCommand{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *EnableChargeLimitCommand) Execute(log *log.Logger) error {
	log.Printf("Start EnableChargeLimitCommand Execute")

	// If this is run on a DUT without a battery (or even just a backup battery
	// for Chromebases), this will effectively do nothing.
	if _, err := c.cs.Connection.RunCmd(c.ctx, "echo", []string{"\"1\"", ">", "/var/lib/power_manager/charge_limit_enabled"}); err != nil {
		return fmt.Errorf("failed to create charge_limit_enabled pref, %s", err)
	}

	if _, err := c.cs.Connection.RunCmd(c.ctx, "echo", []string{"\"3\"", ">", "/var/lib/power_manager/adaptive_charging_hold_delta_percent"}); err != nil {
		return fmt.Errorf("failed to create adaptive_charging_hold_delta_percent pref, %s", err)
	}

	if stdout, err := c.cs.Connection.RunCmd(c.ctx, "stop", []string{"powerd", "2>&1"}); err != nil {
		// Typical failure is that powerd is not running, which is fine. We
		// only need to ensure that it starts after we wrote the above prefs.
		if !strings.Contains(stdout, "stop: Unknown instance") {
			return fmt.Errorf("failed to stop powerd: %s: %s", err, stdout)
		}
	}

	if stdout, err := c.cs.Connection.RunCmd(c.ctx, "start", []string{"powerd", "2>&1"}); err != nil {
		// If the job is already running, somebody else probably started it between
		// the time we stopped it and when we tried to start it again. In that case
		// the goal has succeeded, because it was restarted.
		if !strings.Contains(stdout, "start: Job is already running") {
			return fmt.Errorf("failed to start powerd: %s: %s", err, stdout)
		}
	}

	log.Printf("EnableChargeLimitCommand Success")

	return nil
}

func (c *EnableChargeLimitCommand) Revert() error {
	log.Printf("Start EnableChargeLimitCommand Revert")

	if _, err := c.cs.Connection.RunCmd(c.ctx, "rm", []string{"/var/lib/power_manager/charge_limit_enabled"}); err != nil {
		return fmt.Errorf("failed to remove charge_limit_enabled pref, %s", err)
	}

	if _, err := c.cs.Connection.RunCmd(c.ctx, "rm", []string{"/var/lib/power_manager/adaptive_charging_hold_delta_percent"}); err != nil {
		return fmt.Errorf("failed to remove adaptive_charging_hold_delta_percent pref, %s", err)
	}

	if _, err := c.cs.Connection.RunCmd(c.ctx, "stop", []string{"powerd"}); err != nil {
		log.Printf("failed to stop powerd, %s", err)
	}

	if _, err := c.cs.Connection.RunCmd(c.ctx, "start", []string{"powerd"}); err != nil {
		return fmt.Errorf("failed to start powerd, %s", err)
	}

	log.Printf("EnableChargeLimitCommand Revert Success")

	return nil
}

func (c *EnableChargeLimitCommand) GetErrorMessage() string {
	return "failed to get enable Charge Limit"
}

func (c *EnableChargeLimitCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PROVISIONING_FAILED
}
