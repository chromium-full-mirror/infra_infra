// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/cros-provision/service"
)

type CheckFirmwareSlotCommand struct {
	ctx            context.Context
	cs             *service.CrOSService
	RebootRequired bool
}

func NewCheckFirmwareSlotCommand(ctx context.Context, cs *service.CrOSService) *CheckFirmwareSlotCommand {
	return &CheckFirmwareSlotCommand{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *CheckFirmwareSlotCommand) Execute(log *log.Logger) error {
	log.Printf("Start CheckFirmwareSlotCommand Execute")

	current, err := c.cs.Connection.RunCmd(c.ctx, "crossystem", []string{commonutils.CrossystemCurrentFirmwareSlotKey})
	if err != nil {
		return fmt.Errorf("check current firmware slot: %w", err)
	}
	log.Printf("Current firmware slot: %s", current)

	next, err := c.cs.Connection.RunCmd(c.ctx, "crossystem", []string{commonutils.CrossystemNextFirmwareSlotKey})

	if err != nil {
		return fmt.Errorf("check next firmware slot: %w", err)
	}
	log.Printf("Next firmware slot: %s", next)

	c.RebootRequired = current != next

	log.Printf("CheckFirmwareSlotCommand Success")

	return nil
}

func (c *CheckFirmwareSlotCommand) Revert() error {
	return nil
}

func (c *CheckFirmwareSlotCommand) GetErrorMessage() string {
	return "failed to check if firmware slot changed"
}

func (c *CheckFirmwareSlotCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PROVISIONING_FAILED
}

func (c *CheckFirmwareSlotCommand) Retry() bool {
	return false
}
