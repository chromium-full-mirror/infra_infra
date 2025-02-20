// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// ProvisionServiceStartCmd represents provision service start cmd.
type ProvisionServiceStartCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	ProvisionState    *testapi.ProvisionState
	DutServerAddress  *labapi.IpEndpoint
	PrimaryDut        *labapi.Dut
	ServoNexusAddress *labapi.IpEndpoint
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *ProvisionServiceStartCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *ProvisionServiceStartCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if sk.CftTestRequest.GetPrimaryDut().GetProvisionState() == nil {
		return fmt.Errorf("Cmd %q missing dependency: ProvisionState", cmd.GetCommandType())
	}

	cmd.ProvisionState = sk.CftTestRequest.GetPrimaryDut().GetProvisionState()

	if sk.PrimaryDevice == nil || sk.PrimaryDevice.Dut == nil {
		return fmt.Errorf("Cmd %q missing dependency: PrimaryDevice", cmd.GetCommandType())
	}

	cmd.PrimaryDut = sk.PrimaryDevice.GetDut()

	if sk.DutServerAddress == nil {
		return fmt.Errorf("Cmd %q missing dependency: DutServerAddress", cmd.GetCommandType())
	}

	cmd.DutServerAddress = sk.DutServerAddress
	cmd.ServoNexusAddress = &labapi.IpEndpoint{}
	common.LogWarningIfErr(ctx, common.InjectDependencies(cmd.ServoNexusAddress, sk.Injectables, []*testapi.DynamicDep{
		{
			Key:   "",
			Value: common.NewTaskIdentifier(common.ServoNexus).AddDeviceID(common.NewPrimaryDeviceIdentifier()).ID,
		},
	}))

	return nil
}

func NewProvisionServiceStartCmd(executor interfaces.ExecutorInterface) *ProvisionServiceStartCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(ProvisionServiceStartCmdType, executor)
	cmd := &ProvisionServiceStartCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
