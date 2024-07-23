// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"slices"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/ufs"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/dutstate"
)

var (
	poolsDisallowed = []string{
		"satlab_internal_automation_test",
	}
)

// UpdateDutStateCmd represents update dut state command.
type UpdateDutStateCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	TestResponses      *testapi.CrosTestResponse // optional
	UfsNameSpace       string                    // optional
	ProvisionResponses map[string][]*testapi.InstallResponse
	ProvisionDevices   map[string]*testapi.CrosTestRequest_Device
	SkipReason         string

	// Updates
	CurrentDutState dutstate.State
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *UpdateDutStateCmd) ExtractDependencies(ctx context.Context, ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *UpdateDutStateCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.updateHwTestStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// Execute executes the command.
func (cmd *UpdateDutStateCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Update dut states if required")
	defer func() { step.End(err) }()

	if cmd.SkipReason != "" {
		step.SetSummaryMarkdown(fmt.Sprintf("Skipped: %s", cmd.SkipReason))
		return nil
	}

	for deviceId := range cmd.ProvisionDevices {
		err := cmd.updateDevice(ctx, deviceId)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cmd *UpdateDutStateCmd) updateDevice(ctx context.Context, deviceId string) error {
	device := cmd.ProvisionDevices[deviceId]
	responses := cmd.ProvisionResponses[deviceId]

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Update Dut: %s", device.GetDut().GetId().GetValue()))
	defer func() { step.End(err) }()

	logging.Infof(ctx, "deviceId: %s", deviceId)
	triedToUpdateState := false

	// setup new context if ufs namespace was provided
	if cmd.UfsNameSpace != "" {
		ctx = ufs.SetupContext(ctx, cmd.UfsNameSpace)
	}

	currentDutState, err := ufs.GetDutStateFromUFS(ctx, device.GetDut().GetId().GetValue())
	if err != nil {
		logging.Infof(ctx, "error while getting current dut state: %s", err.Error())
	}
	logging.Infof(ctx, "Dut state before any kind of update: %s", currentDutState)

	for _, response := range responses {
		logging.Infof(ctx, "Found provision response with status: %s", response.GetStatus().String())
		if response.GetStatus() != api.InstallResponse_STATUS_SUCCESS {
			triedToUpdateState = updateDutState(ctx, device.GetDut().GetId().GetValue(), dutstate.NeedsRepair, "provision")
			break
		}
	}
	if !triedToUpdateState {
		if cmd.TestResponses == nil || cmd.TestResponses.TestCaseResults == nil || len(cmd.TestResponses.TestCaseResults) == 0 {
			triedToUpdateState = updateDutState(ctx, device.GetDut().GetId().GetValue(), dutstate.NeedsRepair, "failed before running test(s)")
		} else if common.IsAnyTestFailure(cmd.TestResponses.TestCaseResults) {
			triedToUpdateState = updateDutState(ctx, device.GetDut().GetId().GetValue(), dutstate.NeedsRepair, "test(s)")
		}
	}

	if triedToUpdateState {
		currentDutState, err = ufs.GetDutStateFromUFS(ctx, device.GetDut().GetId().GetValue())
		if err != nil {
			logging.Infof(ctx, "error while getting current dut state: %s", err.Error())
		}
		logging.Infof(ctx, "Dut state after update: %s", currentDutState)
	}

	step.SetSummaryMarkdown(fmt.Sprintf("dut state: %s", currentDutState.String()))
	step.AddTagValue("dut_state", currentDutState.String())
	return nil
}

func (cmd *UpdateDutStateCmd) extractDepsFromHwTestStateKeeper(ctx context.Context, sk *data.HwTestStateKeeper) error {
	cmd.ProvisionResponses = make(map[string][]*testapi.InstallResponse)
	cmd.ProvisionDevices = make(map[string]*testapi.CrosTestRequest_Device)

	for _, deviceId := range sk.DeviceIdentifiers {
		cmd.ProvisionResponses[deviceId] = sk.ProvisionResponses[deviceId]
		cmd.ProvisionDevices[deviceId] = sk.Devices[deviceId]
	}

	if sk.TestResponses == nil {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: TestResponses", cmd.GetCommandType())
	}
	cmd.TestResponses = sk.TestResponses

	if sk.CommonConfig == nil || sk.CommonConfig.GetUfsConfig().GetUfsNamespace() == "" {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: UfsNameSpace. Default namespace will be used.", cmd.GetCommandType())
	} else {
		cmd.UfsNameSpace = sk.CommonConfig.GetUfsConfig().GetUfsNamespace()
	}

	pool := common.GetValueFromRequestKeyvals(ctx, sk.CftTestRequest, sk.CrosTestRunnerRequest, common.LabelPool)
	if slices.Contains(poolsDisallowed, pool) {
		cmd.SkipReason = fmt.Sprintf("pool %s has been marked disallowed for dut state updates", pool)
	}

	return nil
}

func (cmd *UpdateDutStateCmd) updateHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if cmd.CurrentDutState != "" {
		sk.CurrentDutState = cmd.CurrentDutState
	}

	return nil
}

// updateDutState tries to update dut state
func updateDutState(ctx context.Context, hostName string, dutState dutstate.State, failureType string) bool {
	logging.Infof(ctx, "Trying to update dut state to %s due to %s failure.", dutstate.NeedsRepair, failureType)
	err := ufs.SafeUpdateUFSDUTState(ctx, hostName, dutState)
	if err != nil {
		logging.Infof(ctx, "Error while updating dut state: %s", err)
		return false
	}
	return true
}

func NewUpdateDutStateCmd() *UpdateDutStateCmd {
	abstractCmd := interfaces.NewAbstractCmd(UpdateDutStateCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &UpdateDutStateCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}
