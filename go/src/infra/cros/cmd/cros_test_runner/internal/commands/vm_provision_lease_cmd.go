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
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/data"
	vmlabapi "infra/libs/vmlab/api"
)

// VMProvisionLeaseCmd represents vm-provision service lease cmd.
type VMProvisionLeaseCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	DutVmGceImage *vmlabapi.GceImage
	BuildState    *build.State
	// Updates
	LeaseVMResponse *testapi.LeaseVMResponse
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *VMProvisionLeaseCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		// BuildState is used for accessing experiment flags. If missing, will use defaults.
		cmd.BuildState = sk.BuildState
		if sk.DutVmGceImage == nil {
			return fmt.Errorf("cmd %q missing dependency: DutVmGceImage", cmd.GetCommandType())
		}
		if sk.DutVmGceImage.GetName() == "" {
			return fmt.Errorf("cmd %q missing dependency: DutVmGceImage.Name", cmd.GetCommandType())
		}
		if sk.DutVmGceImage.GetProject() == "" {
			return fmt.Errorf("cmd %q missing dependency: DutVmGceImage.Project", cmd.GetCommandType())
		}
		cmd.DutVmGceImage = sk.DutVmGceImage
	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *VMProvisionLeaseCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.updateVMTestStateKeeper(ctx, sk)
	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// updateVMTestStateKeeper updates
// - DutVm in the state to allow release in a later step.
// - DutTopology in the state to fully mimics the state of hardware tests.
func (cmd *VMProvisionLeaseCmd) updateVMTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if cmd.LeaseVMResponse == nil {
		return fmt.Errorf("empty lease vm response %s", cmd.LeaseVMResponse)
	}
	sk.LeaseVMResponse = cmd.LeaseVMResponse

	var dutModel *labapi.DutModel

	if sk.CftTestRequest != nil {
		dutModel = sk.CftTestRequest.GetPrimaryDut().GetDutModel()
	} else if sk.CrosTestRunnerRequest != nil {
		dutModel = sk.CrosTestRunnerRequest.GetParams().GetPrimaryDut()
	}

	if dutModel == nil {
		return fmt.Errorf("missing dutModel")
	}

	duts := []*labapi.Dut{{
		Id: &labapi.Dut_Id{Value: common.VMLabDutHostName},
		DutType: &labapi.Dut_Chromeos{
			Chromeos: &labapi.Dut_ChromeOS{
				Ssh: &labapi.IpEndpoint{
					Address: cmd.LeaseVMResponse.GetVm().GetAddress().GetHost(),
					Port:    cmd.LeaseVMResponse.GetVm().GetAddress().GetPort(),
				},
				DutModel: dutModel,
			},
		}}}
	sk.DutTopology = &labapi.DutTopology{
		Duts: duts,
	}

	return nil
}

func NewVMProvisionLeaseCmd(executor interfaces.ExecutorInterface) *VMProvisionLeaseCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(VMProvisionLeaseCmdType, executor)
	cmd := &VMProvisionLeaseCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
