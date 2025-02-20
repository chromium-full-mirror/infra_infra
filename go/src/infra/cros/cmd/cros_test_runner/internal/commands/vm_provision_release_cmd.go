// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// VMProvisionReleaseCmd represents vm-provision service release cmd.
type VMProvisionReleaseCmd struct {
	*interfaces.SingleCmdByExecutor

	//Deps
	LeaseVMResponse *api.LeaseVMResponse
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *VMProvisionReleaseCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		if sk.LeaseVMResponse == nil {
			return fmt.Errorf("cmd %q missing dependency: LeaseVMResponse", cmd.GetCommandType())
		}
		if sk.LeaseVMResponse.GetLeaseId() == "" {
			return fmt.Errorf("cmd %q missing dependency: LeaseVMResponse.LeaseID", cmd.GetCommandType())
		}
		if sk.LeaseVMResponse.GetVm().GetGceRegion() == "" {
			return fmt.Errorf("cmd %q missing dependency: LeaseVMResponse.GceRegion", cmd.GetCommandType())
		}
		cmd.LeaseVMResponse = sk.LeaseVMResponse
	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *VMProvisionReleaseCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		sk.DutTopology = nil
	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}
	return nil
}

func NewVMProvisionReleaseCmd(executor interfaces.ExecutorInterface) *VMProvisionReleaseCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(VMProvisionReleaseCmdType, executor)
	cmd := &VMProvisionReleaseCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
