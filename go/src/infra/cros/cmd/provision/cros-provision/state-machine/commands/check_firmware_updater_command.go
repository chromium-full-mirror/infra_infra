// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/cros-provision/service"
)

type CheckFirmwareUpdaterCommand struct {
	ctx          context.Context
	cs           *service.CrOSService
	UpdaterExist bool
}

func NewCheckFirmwareUpdaterCommand(ctx context.Context, cs *service.CrOSService) *CheckFirmwareUpdaterCommand {
	return &CheckFirmwareUpdaterCommand{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *CheckFirmwareUpdaterCommand) Execute(log *log.Logger) error {
	log.Printf("Start CheckFirmwareUpdaterCommand Execute")

	exist, err := c.cs.Connection.PathExists(c.ctx, commonutils.FirmwareUpdaterPath)
	if err != nil {
		return err
	}
	c.UpdaterExist = exist

	log.Printf("CheckFirmwareUpdaterCommand Success")

	return nil
}

func (c *CheckFirmwareUpdaterCommand) Revert() error {
	return nil
}

func (c *CheckFirmwareUpdaterCommand) GetErrorMessage() string {
	return "failed to check firmware updater exist"
}

func (c *CheckFirmwareUpdaterCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_UPDATE_FIRMWARE_FAILED
}
