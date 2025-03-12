// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"log"
	"strings"

	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/service"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type CheckForSkip struct {
	ctx            context.Context
	cs             *service.FoilService
	RebootRequired bool
}

func NewCheckForSkipCommand(ctx context.Context, cs *service.FoilService) *CheckForSkip {
	return &CheckForSkip{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *CheckForSkip) Execute(log *log.Logger) error {
	log.Printf("Start CheckForSkip Execute")
	// Currently hard coded as there is no actual build fetching avaliable yet.
	c.cs.SkipUpdate = strings.Contains(c.cs.CurrentBuild, c.cs.TargetBuild)

	log.Println("SKIP LOGIC CHECKING: Current: vs Target:", c.cs.CurrentBuild, c.cs.TargetBuild)
	return nil
}
func (c *CheckForSkip) Revert() error {
	return nil
}

func (c *CheckForSkip) GetErrorMessage() string {
	return "failed to the checking version pre-provision"
}

func (c *CheckForSkip) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PROVISIONING_FAILED
}
