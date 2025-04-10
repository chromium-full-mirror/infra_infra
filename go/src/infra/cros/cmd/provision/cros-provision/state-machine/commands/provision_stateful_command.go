// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"log"
	"path"

	conf "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"

	common_utils "go.chromium.org/infra/cros/cmd/provision/common-utils"
	"go.chromium.org/infra/cros/cmd/provision/cros-provision/service"
)

type ProvisionStatefulCommand struct {
	ctx context.Context
	cs  *service.CrOSService
}

func NewProvisionStatefulCommand(ctx context.Context, cs *service.CrOSService) *ProvisionStatefulCommand {
	return &ProvisionStatefulCommand{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *ProvisionStatefulCommand) Execute(log *log.Logger) error {
	log.Printf("Start ProvisionStatefulCommand Execute")

	if c.cs.ImagePath.HostType == conf.StoragePath_LOCAL || c.cs.ImagePath.HostType == conf.StoragePath_HOSTTYPE_UNSPECIFIED {
		return fmt.Errorf("only GS copying is implemented")
	}

	if _, err := c.cs.Connection.RunCmd(c.ctx, "rm", []string{
		"-rf", common_utils.UpdateStatefulFilePath, path.Join(common_utils.StatefulPath, "var_new"), path.Join(common_utils.StatefulPath, "dev_image_new"),
	}); err != nil {
		log.Printf("ProvisionStatefulCommand rm FAILED")
		return err
	}
	log.Printf("ProvisionStatefulCommand rm Completed")

	if err := c.cs.Connection.PipeData(c.ctx,
		common_utils.BucketJoin(c.cs.ImagePath.GetPath(), "stateful.zst"),
		fmt.Sprintf("tar --ignore-command-error --overwrite --directory=%s --selinux --zstd -xf -", common_utils.StatefulPath)); err != nil {
		log.Printf("ProvisionStatefulCommand PipeData zst failed CONTINUING")
		// Continue here, we need the fallback below.
		if err := c.cs.Connection.PipeData(c.ctx,
			common_utils.BucketJoin(c.cs.ImagePath.GetPath(), "stateful.tgz"),
			fmt.Sprintf("tar --ignore-command-error --overwrite --directory=%s --selinux -xzf -", common_utils.StatefulPath)); err != nil {
			log.Printf("ProvisionStatefulCommand PipeData FAILED")
			return err
		}
	}

	log.Printf("ProvisionStatefulCommand PipeData Completed")

	if _, err := c.cs.Connection.RunCmd(c.ctx, "echo", []string{"-n", "clobber", ">", common_utils.UpdateStatefulFilePath}); err != nil {
		log.Printf("ProvisionStatefulCommand UpdateStatefulFilePath FAILED")
		return err
	}
	log.Printf("ProvisionStatefulCommand UpdateStatefulFilePath Completed")
	log.Printf("ProvisionStatefulCommand Success")
	return nil
}

func (c *ProvisionStatefulCommand) Revert() error {
	varNewPath := path.Join(common_utils.StatefulPath, "var_new")
	devImageNewPath := path.Join(common_utils.StatefulPath, "dev_image_new")
	_, err := c.cs.Connection.RunCmd(c.ctx, "rm", []string{"-rf", varNewPath, devImageNewPath, common_utils.UpdateStatefulFilePath})
	if err != nil {
		log.Printf("revert stateful install: failed to revert stateful installation, %s", err)
	}
	return nil
}

func (c *ProvisionStatefulCommand) GetErrorMessage() string {
	return "failed to provision stateful"
}

func (c *ProvisionStatefulCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PROVISIONING_FAILED
}
