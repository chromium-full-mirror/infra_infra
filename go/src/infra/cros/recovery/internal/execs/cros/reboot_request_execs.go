// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

const (
	// Filter used to find reboot request flags files.
	// Examples: '/var/lib/servod/somebody_reboot'
	// For all the commands below, use `--` for defensiveness and use `-f` so that removing
	// a nonexistent file doesn't fail.
	rebootRequestFindCmd          = "find -- /var/lib/servod/*_reboot"
	rebootRequestRemoveAllCmd     = "rm -f -- /var/lib/servod/*_reboot"
	rebootRequestCreateSingleGlob = "touch -- /var/lib/servod/%d_reboot"
	rebootRequestRemoveSingleGlob = "rm -f -- /var/lib/servod/%d_reboot"
)

// createRebootRequestExec creates reboot flag file request.
func createRebootRequestExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	_, err := run(ctx, time.Minute, fmt.Sprintf(rebootRequestCreateSingleGlob, info.NewServod().Port()))
	if err != nil {
		// Print finish result as we ignore any errors.
		log.Debugf(ctx, "Create the reboot request: %s", err)
	}
	return nil
}

// hasRebootRequestExec checks presence of reboot request flag on the host.
func hasRebootRequestExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	rr, _ := run(ctx, time.Minute, rebootRequestFindCmd)
	if rr == "" {
		return errors.Reason("has reboot request: not request found").Err()
	}
	log.Infof(ctx, "Found reboot requests:\n%s", rr)
	return nil
}

// removeAllRebootRequestsExec removes all reboot flag file requests.
func removeAllRebootRequestsExec(ctx context.Context, info *execs.ExecInfo) error {
	run := info.DefaultRunner()
	if _, err := run(ctx, time.Minute, rebootRequestRemoveAllCmd); err != nil {
		// Print finish result as we ignore any errors.
		log.Debugf(ctx, "Remove all reboot requests: %s", err)
	}
	return nil
}

// removeRebootRequestExec removes reboot flag file request.
func removeRebootRequestExec(ctx context.Context, info *execs.ExecInfo) error {
	servo := info.GetChromeos().GetServo()
	if servo.GetName() == "" {
		return errors.Reason("remove servo in use flag: servo is not present as part of dut info").Err()
	}
	run := info.NewRunner(servo.GetName())
	if _, err := run(ctx, time.Minute, fmt.Sprintf(rebootRequestRemoveSingleGlob, info.NewServod().Port())); err != nil {
		// Print finish result as we ignore any errors.
		log.Debugf(ctx, "Remove the reboot request: %s", err)
	}
	return nil
}

func init() {
	execs.Register("cros_create_reboot_request", createRebootRequestExec)
	execs.Register("cros_has_reboot_request", hasRebootRequestExec)
	execs.Register("cros_remove_all_reboot_request", removeAllRebootRequestsExec)
	execs.Register("cros_remove_reboot_request", removeRebootRequestExec)
}
