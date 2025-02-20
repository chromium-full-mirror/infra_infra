// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

const (
	androidProvisionRequestMetadata = "type.googleapis.com/chromiumos.test.api.AndroidProvisionRequestMetadata"
)

// AndroidProvisionInstallCmd represents android-provision install cmd.
type AndroidProvisionInstallCmd struct {
	*interfaces.SingleCmdByExecutor
	// Deps
	AndroidDutServerAddress *labapi.IpEndpoint
	AndroidProvisionState   *anypb.Any
	AndroidCompanionDut     *labapi.Dut
	// Updates
	AndroidProvisionResponse *testapi.InstallResponse
}

// NewAndroidProvisionInstallCmd returns an object of AndroidProvisionInstallCmd
func NewAndroidProvisionInstallCmd(executor interfaces.ExecutorInterface) *AndroidProvisionInstallCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(AndroidProvisionInstallCmdType, executor)
	cmd := &AndroidProvisionInstallCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *AndroidProvisionInstallCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		companionDuts := sk.CftTestRequest.GetCompanionDuts()
		for _, companionDut := range companionDuts {
			provisionMetadata := companionDut.GetProvisionState().GetProvisionMetadata()
			if provisionMetadata == nil {
				continue
			}
			metadataType := provisionMetadata.TypeUrl
			if metadataType != androidProvisionRequestMetadata {
				continue
			} else {
				cmd.AndroidProvisionState = provisionMetadata
			}
		}
		cmd.AndroidDutServerAddress = sk.AndroidDutServerAddress
		for _, device := range sk.CompanionDevices {
			if device.GetDut().GetAndroid() != nil {
				cmd.AndroidCompanionDut = device.GetDut()
			}
		}
		if cmd.AndroidProvisionState == nil || cmd.AndroidDutServerAddress == nil || cmd.AndroidCompanionDut == nil {
			return fmt.Errorf("missing dependency for cmd type %s", cmd.GetCommandType())
		}

	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *AndroidProvisionInstallCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.updateAndroidHwTestsStateKeeper(ctx, sk)
	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}
	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *AndroidProvisionInstallCmd) updateAndroidHwTestsStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if cmd.AndroidProvisionResponse != nil {
		responses := sk.ProvisionResponses[common.NewCompanionDeviceIdentifier(cmd.AndroidCompanionDut.GetAndroid().GetDutModel().GetBuildTarget()).ID]
		if responses == nil {
			responses = []*testapi.InstallResponse{}
		}
		responses = append(responses, cmd.AndroidProvisionResponse)
		sk.ProvisionResponses[common.NewCompanionDeviceIdentifier(cmd.AndroidCompanionDut.GetAndroid().GetDutModel().GetBuildTarget()).ID] = responses
	}

	return nil
}
