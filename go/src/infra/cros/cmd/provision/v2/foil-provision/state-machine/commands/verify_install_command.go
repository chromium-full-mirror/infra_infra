// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"log"
	"strings"

	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/service"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type VerifyInstall struct {
	ctx            context.Context
	cs             *service.FoilService
	RebootRequired bool
}

func NewVerifyInstallCommand(ctx context.Context, cs *service.FoilService) *VerifyInstall {
	return &VerifyInstall{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *VerifyInstall) Execute(log *log.Logger) error {
	log.Printf("Start VerifyInstall Execute")
	log.Println("Checking S vs V", c.cs.CurrentBuild, c.cs.TargetBuild)

	if !strings.Contains(c.cs.CurrentBuild, c.cs.TargetBuild) {
		return fmt.Errorf("Build post install does not match target")
	}

	return nil
}
func (c *VerifyInstall) Revert() error {
	return nil
}

func (c *VerifyInstall) GetErrorMessage() string {
	return "Image incorrect post OTA reboot"
}

func (c *VerifyInstall) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_IMAGE_MISMATCH_POST_PROVISION_UPDATE
}
