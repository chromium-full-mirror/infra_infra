// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/buildbucket/protoutil"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/data"
)

// ParseDutTopologyCmd represents build input validation command.
type ParseDutTopologyCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	DutTopology        *labapi.DutTopology
	PrimaryDutModel    *labapi.DutModel
	CompanionDutModels []*labapi.DutModel

	// Updates
	Devices           map[string]*testapi.CrosTestRequest_Device
	DevicesMetadata   map[string]*skylab_test_runner.CFTTestRequest_Device
	DeviceIdentifiers []string
	CacheServer       *labapi.IpEndpoint
}

type DeviceInfo struct {
	Device   *testapi.CrosTestRequest_Device
	Metadata *skylab_test_runner.CFTTestRequest_Device
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *ParseDutTopologyCmd) ExtractDependencies(ctx context.Context, ski interfaces.StateKeeperInterface) error {
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
func (cmd *ParseDutTopologyCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.updateHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.updateHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// Execute executes the command.
func (cmd *ParseDutTopologyCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Parse DutTopology")
	defer func() { step.End(err) }()

	common.WriteProtoToStepLog(ctx, step, cmd.DutTopology, "DutTopology")
	cmd.Devices = make(map[string]*testapi.CrosTestRequest_Device)
	cmd.DevicesMetadata = make(map[string]*skylab_test_runner.CFTTestRequest_Device)
	cmd.DeviceIdentifiers = []string{}

	// Find cache server.
	cmd.CacheServer = findCacheServer(cmd.DutTopology)

	devicePool := []*DeviceInfo{}
	for _, dut := range cmd.DutTopology.GetDuts() {
		device, deviceMetadata := parseDut(dut)
		devicePool = append(devicePool, &DeviceInfo{
			Device:   device,
			Metadata: deviceMetadata,
		})
	}

	// Match primary board to dut.
	if cmd.PrimaryDutModel != nil {
		info, err := cmd.matchDut(devicePool, cmd.PrimaryDutModel)
		if err != nil {
			return fmt.Errorf("Failed to match primaryDevice, %w", err)
		}
		cmd.appendDevice(common.NewPrimaryDeviceIdentifier().ID, info)
	}

	for _, companionDutModel := range cmd.CompanionDutModels {
		info, err := cmd.matchDut(devicePool, companionDutModel)
		if err != nil {
			return fmt.Errorf("Failed to match companionDevice, %w", err)
		}
		deviceId := common.NewCompanionDeviceIdentifier(companionDutModel.GetBuildTarget())
		if _, ok := cmd.Devices[deviceId.ID]; ok {
			// deviceId already exists, try postfixing
			// Standard within swarming when there are duplicate boards
			// is to postfix with `_2`. (e.g. `brya | brya_2`)
			postfix := 2
			for {
				if _, ok := cmd.Devices[deviceId.AddPostfix(strconv.Itoa(postfix)).ID]; !ok {
					deviceId = deviceId.AddPostfix(strconv.Itoa(postfix))
					break
				}
				postfix += 1
			}
		}
		cmd.appendDevice(deviceId.ID, info)
	}

	return nil
}

// appendDevice handles storing deviceInfo within top-level stores.
func (cmd *ParseDutTopologyCmd) appendDevice(deviceId string, deviceInfo *DeviceInfo) {
	cmd.DeviceIdentifiers = append(cmd.DeviceIdentifiers, deviceId)
	cmd.Devices[deviceId] = deviceInfo.Device
	cmd.DevicesMetadata[deviceId] = deviceInfo.Metadata
}

// matchDut finds a dut within the dutPool that contains a board that matches the requested board.
func (cmd *ParseDutTopologyCmd) matchDut(dutPool []*DeviceInfo, dutModel *labapi.DutModel) (*DeviceInfo, error) {
	foundIndex := -1
	for i, deviceMetadataPair := range dutPool {
		if deviceMetadataPair.Metadata.GetDutModel().GetBuildTarget() == dutModel.GetBuildTarget() {
			if dutModel.GetModelName() != "" && dutModel.GetModelName() != deviceMetadataPair.Metadata.GetDutModel().GetModelName() {
				continue
			}
			foundIndex = i
			break
		}
	}
	if foundIndex == -1 {
		return nil, fmt.Errorf("Failed to find board/model %s/%s within dut_topology", dutModel.GetBuildTarget(), dutModel.GetModelName())
	}
	match := dutPool[foundIndex]
	dutPool = slices.Delete(dutPool, foundIndex, foundIndex+1)
	return match, nil
}

func (cmd *ParseDutTopologyCmd) extractDepsFromHwTestStateKeeper(ctx context.Context, sk *data.HwTestStateKeeper) error {
	if sk.DutTopology == nil {
		return fmt.Errorf("Cmd %q missing dependency: DutTopology", cmd.GetCommandType())
	}
	cmd.DutTopology = sk.DutTopology

	if sk.PrimaryDutModel == nil {
		logging.Infof(ctx, "Cmd %s missing non-required dependency: primaryDutModel", cmd.GetCommandType())
	}
	cmd.PrimaryDutModel = sk.PrimaryDutModel

	if sk.CompanionDutModels == nil || len(sk.CompanionDutModels) == 0 {
		logging.Infof(ctx, "Cmd %s missing non-required dependency: companionDutModels", cmd.GetCommandType())
	}
	cmd.CompanionDutModels = sk.CompanionDutModels

	return nil
}

func (cmd *ParseDutTopologyCmd) updateHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	sk.CompanionDevices = []*testapi.CrosTestRequest_Device{}
	sk.CompanionDevicesMetadata = []*skylab_test_runner.CFTTestRequest_Device{}
	sk.Devices = map[string]*testapi.CrosTestRequest_Device{}

	for deviceId, device := range cmd.Devices {
		deviceMetadata := cmd.DevicesMetadata[deviceId]
		deviceIdentifier := common.DeviceIdentifierFromString(deviceId)
		if strings.HasPrefix(deviceId, common.Primary) {
			sk.PrimaryDevice = device
			sk.PrimaryDeviceMetadata = deviceMetadata
		} else {
			sk.CompanionDevices = append(sk.CompanionDevices, device)
			sk.CompanionDevicesMetadata = append(sk.CompanionDevicesMetadata, deviceMetadata)
		}
		sk.Devices[deviceIdentifier.ID] = device

		// Need to grab botDims for label-pool.
		build := sk.BuildState.Build()
		botDims := protoutil.MustBotDimensions(build)
		labelPools := getTagValues(botDims, "label-pool")
		updateFirmware := shouldUpdateFirmware(sk.CommonConfig, labelPools, deviceMetadata.GetDutModel())
		sk.UpdateFirmwares[deviceMetadata.GetDutModel().GetBuildTarget()] = updateFirmware
		if err := sk.Injectables.Set(deviceIdentifier.GetDevice(), device); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the injectable storage, %s", cmd.GetCommandType(), deviceIdentifier.GetDevice(), err)
		}
		if err := sk.Injectables.Set(deviceIdentifier.GetDeviceMetadata(), deviceMetadata); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the injectable storage, %s", cmd.GetCommandType(), deviceIdentifier.GetDeviceMetadata(), err)
		}
		if err := sk.Injectables.Set(deviceIdentifier.GetUpdateFirmware(), updateFirmware); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the injectable storage, %s", cmd.GetCommandType(), deviceIdentifier.AddPostfix("updateFirmware").ID)
		}
	}

	sk.DeviceIdentifiers = cmd.DeviceIdentifiers
	if err := sk.Injectables.Set(common.CompanionDevices, sk.CompanionDevices); err != nil {
		logging.Infof(ctx, "Warning: cmd %s failed to set companionDevices in the injectable storage, %s", cmd.GetCommandType(), err)
	}
	if err := sk.Injectables.Set(common.CompanionDevicesMetadata, sk.CompanionDevicesMetadata); err != nil {
		logging.Infof(ctx, "Warning: cmd %s failed to set companionDevicesMetadata in the injectable storage, %s", cmd.GetCommandType(), err)
	}
	sk.CacheServer = cmd.CacheServer
	if sk.CacheServer != nil {
		if err := sk.Injectables.Set(common.CacheServer, sk.CacheServer); err != nil {
			logging.Infof(ctx, "Warning: cmd %s failed to set %s in the injectable storage, %s", cmd.GetCommandType(), common.CacheServer, err)
		}
	}

	return nil
}

// parseDut extracts ssh and board/model info from the lab dut and constructs
// a pair of CFT compatible objects.
func parseDut(dut *labapi.Dut) (*testapi.CrosTestRequest_Device, *skylab_test_runner.CFTTestRequest_Device) {
	var ssh *labapi.IpEndpoint
	var model *labapi.DutModel
	var device *testapi.CrosTestRequest_Device
	var deviceMetadata *skylab_test_runner.CFTTestRequest_Device
	switch dutType := dut.DutType.(type) {
	case *labapi.Dut_Chromeos:
		ssh = dutType.Chromeos.GetSsh()
		model = dutType.Chromeos.GetDutModel()
	case *labapi.Dut_Android_:
		// AssociatedHostname points to the ssh address of the
		// labstation associated with the android device, not
		// the android device itself. This value is used in
		// provisioning but testing expects the actual android
		// device address, which will need to be discovered
		// with some pre-test service.
		ssh = dutType.Android.GetAssociatedHostname()
		ssh.Port = 22
		model = dutType.Android.GetDutModel()
	case *labapi.Dut_Devboard_:
		ssh = dutType.Devboard.GetServo().GetServodAddress()
		model = &labapi.DutModel{
			BuildTarget: dutType.Devboard.BoardType,
			ModelName:   dutType.Devboard.BoardType,
		}
	default:
	}
	if dut.CacheServer == nil {
		dut.CacheServer = &labapi.CacheServer{}
	}
	device = &api.CrosTestRequest_Device{
		Dut:       dut,
		DutServer: ssh,
	}
	deviceMetadata = &skylab_test_runner.CFTTestRequest_Device{
		DutModel: model,
	}

	return device, deviceMetadata
}

// shouldUpdateFirmware checks whether or not a dutModel
// should update the firmware during cros-provision
// using the cros firmware update config from the input.
func shouldUpdateFirmware(commonConfig *skylab_test_runner.CommonConfig, labelPools []string, dutModel *labapi.DutModel) bool {
	firmwareConfig := commonConfig.GetCrosFirmwareUpdateConfig()
	if firmwareConfig == nil || !firmwareConfig.Enabled {
		return false
	}

	if slices.Contains(labelPools, "mp_firmware_testing") {
		return false
	}

	allowList := firmwareConfig.GetAllowList()
	if allowList != nil {
		return (slices.Contains(allowList.GetBoards(), dutModel.GetBuildTarget()) ||
			slices.Contains(allowList.GetModels(), dutModel.GetModelName()))
	}
	blockList := firmwareConfig.GetBlockList()
	if blockList != nil {
		return !(slices.Contains(blockList.GetBoards(), dutModel.GetBuildTarget()) ||
			slices.Contains(blockList.GetModels(), dutModel.GetModelName()))
	}

	return false
}

// findCacheServer returns the first cache server found within the
// dut topology. This server can essentially be used for each device.
func findCacheServer(dutTopology *labapi.DutTopology) *labapi.IpEndpoint {
	for _, dut := range dutTopology.GetDuts() {
		if dut.CacheServer != nil && dut.CacheServer.Address != nil {
			return dut.GetCacheServer().GetAddress()
		}
	}
	return nil
}

func NewParseDutTopologyCmd() *ParseDutTopologyCmd {
	abstractCmd := interfaces.NewAbstractCmd(ParseDutTopologyCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &ParseDutTopologyCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}
