// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"log"

	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/service"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/chromiumos/test/util/adb"
)

type GetVersion struct {
	ctx            context.Context
	cs             *service.FoilService
	RebootRequired bool
}

func NewGetVersionSetup(ctx context.Context, cs *service.FoilService) *GetVersion {
	return &GetVersion{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *GetVersion) Execute(log *log.Logger) error {
	out, err := adb.GetBuildVersion(c.cs.DutIp, log)
	if err != nil {
		return err
	}
	c.cs.CurrentBuild = out
	return nil
}

func (c *GetVersion) Revert() error {
	return nil
}

func (c *GetVersion) GetErrorMessage() string {
	return "failed to GetVersion from device"
}

func (c *GetVersion) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED
}
