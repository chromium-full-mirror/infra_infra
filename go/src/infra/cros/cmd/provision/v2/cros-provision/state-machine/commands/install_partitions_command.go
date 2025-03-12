// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"go.chromium.org/chromiumos/test/provision/v2/cros-provision/service"
	"context"
	"fmt"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type InstallPartitionsCommand struct {
	ctx context.Context
	cs  *service.CrOSService
}

func NewInstallPartitionsCommand(ctx context.Context, cs *service.CrOSService) *InstallPartitionsCommand {
	return &InstallPartitionsCommand{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *InstallPartitionsCommand) Execute(log *log.Logger) error {
	log.Printf("Start InstallPartitionsCommand Execute")

	var err error

	err = c.cs.InstallZstdCompressedFile(c.ctx, "full_KERN.bin.zst", c.cs.MachineMetadata.RootInfo.PartitionInfo.InactiveKernel)
	if err == nil {
		log.Printf("InstallPartitionsCommand full_KERN.bin.zst COMPLETED")
	} else {
		log.Printf("InstallPartitionsCommand full_KERN.bin.zst failed CONTINUING")
		err = c.cs.InstallZippedImage(c.ctx, "full_dev_part_KERN.bin.gz", c.cs.MachineMetadata.RootInfo.PartitionInfo.InactiveKernel)
		if err != nil {
			return fmt.Errorf("install kernel: %s", err)
		}
		log.Printf("InstallPartitionsCommand full_dev_part_KERN.bin.gz COMPLETED")
	}

	err = c.cs.InstallZstdCompressedFile(c.ctx, "full_ROOT.bin.zst", c.cs.MachineMetadata.RootInfo.PartitionInfo.InactiveRoot)
	if err == nil {
		log.Printf("InstallPartitionsCommand full_ROOT.bin.zst COMPLETED")
	} else {
		log.Printf("InstallPartitionsCommand full_ROOT.bin.zst failed CONTINUING")
		err := c.cs.InstallZippedImage(c.ctx, "full_dev_part_ROOT.bin.gz", c.cs.MachineMetadata.RootInfo.PartitionInfo.InactiveRoot)
		if err != nil {
			return fmt.Errorf("install root: %s", err)
		}
		log.Printf("InstallPartitionsCommand full_dev_part_ROOT.bin.gz COMPLETED")
	}

	log.Printf("InstallPartitionsCommand Success")
	return nil
}

func (c *InstallPartitionsCommand) Revert() error {
	return nil
}

func (c *InstallPartitionsCommand) GetErrorMessage() string {
	return "failed to install partitions"
}

func (c *InstallPartitionsCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_DOWNLOADING_IMAGE_FAILED
}
