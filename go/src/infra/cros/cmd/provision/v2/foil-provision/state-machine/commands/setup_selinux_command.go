// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"log"

	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/service"
	"go.chromium.org/chromiumos/test/util/adb"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type SetSelinuxCommand struct {
	ctx            context.Context
	cs             *service.FoilService
	RebootRequired bool
}

func NewSetSelinuxCommandSetup(ctx context.Context, cs *service.FoilService) *SetSelinuxCommand {
	return &SetSelinuxCommand{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *SetSelinuxCommand) Execute(log *log.Logger) error {
	log.Printf("Start SetSelinuxCommand Execute, with reboot")
	adb.AdbShellCmd([]string{"setenforce", "0"}, c.cs.DutIp, log)
	adb.AdbShellCmd([]string{"getenforce"}, c.cs.DutIp, log)
	return nil
}

func (c *SetSelinuxCommand) Revert() error {
	return nil
}

func (c *SetSelinuxCommand) GetErrorMessage() string {
	return "failed to SetSelinuxCommand"
}

func (c *SetSelinuxCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PROVISIONING_FAILED
}
