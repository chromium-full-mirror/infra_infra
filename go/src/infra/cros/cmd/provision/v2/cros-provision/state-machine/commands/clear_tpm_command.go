// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"log"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/provision/v2/cros-provision/service"
)

type ClearTPMCommand struct {
	ctx context.Context
	cs  *service.CrOSService
}

func NewClearTPMCommand(ctx context.Context, cs *service.CrOSService) *ClearTPMCommand {
	return &ClearTPMCommand{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *ClearTPMCommand) Execute(log *log.Logger) error {
	log.Printf("Start ClearTPMCommand Execute")

	if c.canClearTPM(c.ctx) {
		if _, err := c.cs.Connection.RunCmd(c.ctx, "crossystem", []string{"clear_tpm_owner_request=1"}); err != nil {
			return err
		}
	}
	log.Printf("ClearTPMCommand Success")

	return nil
}

// CanClearTPM determines whether the current board can clear TPM
func (c *ClearTPMCommand) canClearTPM(ctx context.Context) bool {
	return !strings.HasPrefix(c.cs.MachineMetadata.Board, "reven")
}

func (c *ClearTPMCommand) Revert() error {
	return nil
}

func (c *ClearTPMCommand) GetErrorMessage() string {
	return "failed to clear TPM"
}

func (c *ClearTPMCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_CLEAR_TPM_FAILED
}
