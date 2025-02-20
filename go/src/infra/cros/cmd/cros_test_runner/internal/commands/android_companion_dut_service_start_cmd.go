// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// AndroidCompanionDutServiceStartCmd represents android dut service start cmd.
type AndroidCompanionDutServiceStartCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	CacheServerAddress   *labapi.IpEndpoint
	AndroidDutSshAddress *labapi.IpEndpoint

	// Updates
	AndroidDutServerAddress *labapi.IpEndpoint
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *AndroidCompanionDutServiceStartCmd) ExtractDependencies(
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

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *AndroidCompanionDutServiceStartCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.updateHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.updateHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *AndroidCompanionDutServiceStartCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if sk.PrimaryDevice == nil || sk.PrimaryDevice.Dut == nil {
		return fmt.Errorf("Cmd %q missing dependency: PrimaryDevice", cmd.GetCommandType())
	}
	primaryDut := sk.PrimaryDevice.GetDut()

	if primaryDut.GetCacheServer().GetAddress() == nil {
		return fmt.Errorf("Cmd %q missing dependency: CacheServerAddress", cmd.GetCommandType())
	}
	cmd.CacheServerAddress = primaryDut.GetCacheServer().GetAddress()

	androidBuildTarget := getAndroidBuildTarget(sk.CftTestRequest.GetCompanionDuts())
	androidDut := getAndroidDutFromDutTopology(sk.CompanionDevices, androidBuildTarget)
	if androidDut == nil {
		return fmt.Errorf("Cmd %q missing dependency: Android Dut in Dut topology", cmd.GetCommandType())
	}
	cmd.AndroidDutSshAddress = &labapi.IpEndpoint{
		Address: androidDut.GetAndroid().GetAssociatedHostname().GetAddress(),
		Port:    22,
	}
	return nil
}

func (cmd *AndroidCompanionDutServiceStartCmd) updateHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if cmd.AndroidDutServerAddress != nil {
		sk.AndroidDutServerAddress = cmd.AndroidDutServerAddress
	}

	return nil
}

func NewAndroidCompanionDutServiceStartCmd(executor interfaces.ExecutorInterface) *AndroidCompanionDutServiceStartCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(AndroidCompanionDutServiceStartCmdType, executor)
	cmd := &AndroidCompanionDutServiceStartCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}

func getAndroidBuildTarget(companionDuts []*skylab_test_runner.CFTTestRequest_Device) string {
	if len(companionDuts) < 1 {
		return ""
	}
	for _, companionDut := range companionDuts {
		provisionMetadata := companionDut.GetProvisionState().GetProvisionMetadata()
		if provisionMetadata != nil && provisionMetadata.TypeUrl == androidProvisionRequestMetadata {
			return companionDut.GetDutModel().GetBuildTarget()
		}
	}
	return ""
}

func getAndroidDutFromDutTopology(companionDevices []*api.CrosTestRequest_Device, androidBuildTarget string) *labapi.Dut {
	for _, device := range companionDevices {
		if device.GetDut().GetAndroid() != nil && device.GetDut().GetAndroid().GetDutModel().GetBuildTarget() == androidBuildTarget {
			return device.GetDut()
		}
	}
	return nil
}
