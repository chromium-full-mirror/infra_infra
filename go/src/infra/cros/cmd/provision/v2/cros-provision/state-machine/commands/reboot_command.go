// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"go.chromium.org/chromiumos/test/provision/v2/cros-provision/service"
	"context"
	"fmt"
	"log"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type OptionalRebootArgs struct {
	// force the reboot over a clean reboot.
	Force bool
}

type RebootCommand struct {
	timeout *time.Duration
	ctx     context.Context
	cs      *service.CrOSService
	force   bool
}

func getForce(optRebootArgs ...OptionalRebootArgs) bool {
	if len(optRebootArgs) > 0 {
		return optRebootArgs[0].Force
	}
	// Default.
	return false
}

func NewRebootWithTimeoutCommand(timeout time.Duration, ctx context.Context, cs *service.CrOSService, optRebootArgs ...OptionalRebootArgs) *RebootCommand {
	return &RebootCommand{
		timeout: &timeout,
		ctx:     ctx,
		cs:      cs,
		force:   getForce(optRebootArgs...),
	}
}

func NewRebootCommand(ctx context.Context, cs *service.CrOSService, optRebootArgs ...OptionalRebootArgs) *RebootCommand {
	return &RebootCommand{
		timeout: nil,
		ctx:     ctx,
		cs:      cs,
		force:   getForce(optRebootArgs...),
	}
}

func (c *RebootCommand) Execute(log *log.Logger) error {
	log.Printf("Start RebootCommand Execute")

	ctx := c.ctx
	var cancel context.CancelFunc
	if c.timeout != nil {
		ctx, cancel = context.WithTimeout(context.Background(), *c.timeout)
		defer cancel()
	}
	if c.force {
		log.Printf("Performing forced reboot")
		bootID, err := c.cs.Connection.RunCmd(ctx, "/bin/cat", []string{"/proc/sys/kernel/random/boot_id"})
		if err != nil {
			return err
		}

		// Ignore errors.
		c.cs.Connection.RunCmd(ctx, "/bin/echo", []string{"\"b\"", ">", "/proc/sysrq-trigger"})

		if err = c.cs.Connection.ForceReconnectWithBackoff(ctx); err != nil {
			return err
		}

		// Force waiting for changed boot ID.
		bootIDPostForcedReboot, err := c.cs.Connection.RunCmd(ctx, "/bin/cat", []string{"/proc/sys/kernel/random/boot_id"})
		if err != nil {
			return err
		}

		if bootID == bootIDPostForcedReboot {
			return fmt.Errorf("boot ID did not change from: %s", bootID)
		}
	} else if err := c.cs.Connection.Restart(ctx); err != nil {
		return err
	}
	log.Printf("RebootCommand Success")
	return nil
}

func (c *RebootCommand) Revert() error {
	return nil
}

func (c *RebootCommand) GetErrorMessage() string {
	return "failed to reboot DUT"
}

func (c *RebootCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_DUT_UNREACHABLE_POST_PROVISION
}
