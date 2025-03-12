// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/service"
	"go.chromium.org/chromiumos/test/util/adb"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type OptionalRebootArgs struct {
	// force the reboot over a clean reboot.
	Force bool
}

type RebootCommand struct {
	ctx   context.Context
	cs    *service.FoilService
	force bool
}

func NewRebootCommand(ctx context.Context, cs *service.FoilService, optRebootArgs ...OptionalRebootArgs) *RebootCommand {
	return &RebootCommand{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *RebootCommand) Execute(log *log.Logger) error {
	log.Printf("Start RebootCommand Execute")
	outStr, err := adb.AdbCmd([]string{"-s", c.cs.DutIp, "reboot"}, log)
	log.Println(outStr)
	addr := c.cs.DutIp

	if err != nil {
		return fmt.Errorf("ADB Start failed. %s: %s", err, outStr)
	}
	err = adb.RetrySetupAdb(log, addr, 3*time.Minute)
	if err != nil {
		return err
	}

	log.Printf("RebootCommand Success")
	return nil
}

func (c *RebootCommand) Revert() error {
	return nil
}

func (c *RebootCommand) GetErrorMessage() string {
	return "DUT did not boot after OTA"
}

func (c *RebootCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_DUT_UNREACHABLE_POST_PROVISION
}
