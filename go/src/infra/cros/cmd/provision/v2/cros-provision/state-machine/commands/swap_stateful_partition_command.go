// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	"go.chromium.org/chromiumos/test/provision/v2/cros-provision/service"
	"context"
	"fmt"
	"log"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type SwapStatefulPartitionCommand struct {
	ctx context.Context
	cs  *service.CrOSService
}

func NewSwapStatefulPartitionCommand(ctx context.Context, cs *service.CrOSService) *SwapStatefulPartitionCommand {
	return &SwapStatefulPartitionCommand{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *SwapStatefulPartitionCommand) Execute(log *log.Logger) error {
	log.Printf("Start SwapStatefulPartitionCommand Execute")
	defer log.Printf("SwapStatefulPartitionCommand Complete")

	if c.cs.PreserveStateful {
		log.Printf("Skipping SwapStatefulPartitionCommand due to preservation.")
		return nil
	}

	tmpMnt, err := c.cs.Connection.RunCmd(c.ctx, "/usr/bin/mktemp", []string{"-d"})
	if err != nil {
		return fmt.Errorf("failed to create temporary directory, %s", err)
	}
	tmpMnt = strings.TrimSpace(tmpMnt)

	_, err = c.cs.Connection.RunCmd(c.ctx, "/bin/dd", []string{"if=/dev/zero", fmt.Sprintf("of=%s/fs", tmpMnt), "bs=512M", "count=1"})
	if err != nil {
		return fmt.Errorf("failed to create zero file, %s", err)
	}

	_, err = c.cs.Connection.RunCmd(c.ctx, "/sbin/mkfs.ext4", []string{"-O", "none,has_journal", tmpMnt + "/fs"})
	if err != nil {
		return fmt.Errorf("failed to create powerwash filesystem, %s", err)
	}

	_, err = c.cs.Connection.RunCmd(c.ctx, "/bin/mkdir", []string{tmpMnt + "/mnt"})
	if err != nil {
		return fmt.Errorf("failed to create mount directory, %s", err)
	}

	_, err = c.cs.Connection.RunCmd(c.ctx, "/bin/mount", []string{tmpMnt + "/fs", tmpMnt + "/mnt"})
	if err != nil {
		return fmt.Errorf("failed to mount powerwash filesystem, %s", err)
	}

	_, err = c.cs.Connection.RunCmd(c.ctx, "/bin/echo", []string{"-n", "\"fast safe keepimg\"", ">", tmpMnt + "/mnt/factory_install_reset"})
	if err != nil {
		return fmt.Errorf("failed to write reset file, %s", err)
	}

	_, err = c.cs.Connection.RunCmd(c.ctx, "/bin/umount", []string{tmpMnt + "/mnt"})
	if err != nil {
		return fmt.Errorf("failed to unmount powerwash filesystem, %s", err)
	}

	_, err = c.cs.Connection.RunCmd(c.ctx, "/sbin/fsfreeze", []string{"-f", "/mnt/stateful_partition"})
	if err != nil {
		return fmt.Errorf("failed to freeze stateful filesystem, %s", err)
	}

	pi := common_utils.GetPartitionInfo(c.cs.MachineMetadata.RootInfo.Root, c.cs.MachineMetadata.RootInfo.RootDisk, c.cs.MachineMetadata.RootInfo.RootPartNum)
	_, err = c.cs.Connection.RunCmd(c.ctx, "/bin/dd", []string{fmt.Sprintf("if=%s/fs", tmpMnt), fmt.Sprintf("of=%s", pi.Stateful), "bs=1M", "conv=fsync"})
	if err != nil {
		return fmt.Errorf("failed to unmount powerwash filesystem, %s", err)
	}

	return nil
}

func (c *SwapStatefulPartitionCommand) Revert() error {
	return nil
}

func (c *SwapStatefulPartitionCommand) GetErrorMessage() string {
	return "failed to wipe(swap) stateful"
}

func (c *SwapStatefulPartitionCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PROVISIONING_FAILED
}
