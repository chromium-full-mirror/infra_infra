// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/ufs"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	"go.chromium.org/infra/cros/dutstate"
	lab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

var (
	poolsDisallowed = []string{
		"foilTest",
		"foilTestBroken",
		"satlab-0wgtfqin1850814b",
	}
)

// UpdateDutStateCmd represents update dut state command.
type UpdateDutStateCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	TestResponses      *api.CrosTestResponse // optional
	UfsNameSpace       string                // optional
	ProvisionResponses map[string][]*api.InstallResponse
	ProvisionDevices   map[string]*api.CrosTestRequest_Device
	SkipReason         string
	IsAlRun            bool

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

	// TODO(cdelagarza): remove commented out code when confident with auto_repair
	// if cmd.SkipReason != "" {
	// 	step.SetSummaryMarkdown(fmt.Sprintf("Skipped: %s", cmd.SkipReason))
	// 	return nil
	// }

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
	hostName := device.GetDut().GetId().GetValue()

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Update Dut: %s", hostName))
	defer func() { step.End(err) }()

	logging.Infof(ctx, "deviceId: %s", deviceId)

	// setup new context if ufs namespace was provided
	if cmd.UfsNameSpace != "" {
		ctx = ufs.SetupContext(ctx, cmd.UfsNameSpace)
	}
	ctx = addNamespaceCtxIfNotPresent(ctx, ufsUtil.OSNamespace)

	ufsClient, err := ufs.NewClient(ctx)
	if err != nil {
		logging.Infof(ctx, "failed to create ufs client, %s", err.Error())
	}
	machine, err := ufsClient.GetMachineLSE(ctx, &ufsAPI.GetMachineLSERequest{
		Name: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, hostName),
	})
	if err != nil {
		err = errors.Annotate(err, "failed to retrieve machineLSE for %s", hostName).Err()
		logging.Infof(ctx, err.Error())
		step.SetSummaryMarkdown(err.Error())
		return nil
	}
	dutID := machine.GetMachines()[0]
	deviceData, err := ufsClient.GetDeviceData(ctx, &ufsAPI.GetDeviceDataRequest{
		DeviceId: machine.GetMachines()[0],
		Hostname: hostName,
	})
	if err != nil {
		err = errors.Annotate(err, "failed to get device data for %s", hostName).Err()
		logging.Infof(ctx, err.Error())
		step.SetSummaryMarkdown(err.Error())
		return nil
	}

	currentDutState, err := ufs.GetDutStateFromUFS(ctx, device.GetDut().GetId().GetValue())
	if err != nil {
		logging.Infof(ctx, "error while getting current dut state: %s", err.Error())
	}
	logging.Infof(ctx, "Dut state before any kind of update: %s", currentDutState)

	deviceState := currentDutState
	repairReason := cmd.determineNeedForRepairAndModify(ctx, deviceData, responses)
	if repairReason != "" {
		logging.Infof(ctx, "Setting %s to NEEDS_REPAIR for failure: %s", hostName, repairReason)
		deviceState = dutstate.NeedsRepair
	}

	_, err = updateDutState(ctx, step, ufsClient, hostName, dutID, deviceState, deviceData)
	if err != nil {
		logging.Infof(ctx, "error while updating dut state: %s", err.Error())
	}

	step.SetSummaryMarkdown(fmt.Sprintf("dut state: %s", deviceState.String()))
	step.AddTagValue("dut_state", deviceState.String())
	return nil
}

func (cmd *UpdateDutStateCmd) determineNeedForRepairAndModify(ctx context.Context, deviceData *ufsAPI.GetDeviceDataResponse, responses []*api.InstallResponse) (repairReason string) {
	repairRequest := lab.DutState_REPAIR_REQUEST_UNKNOWN
	osType := lab.VersionInfo_UNKNOWN
	foundProvisionError := false
	for _, response := range responses {
		logging.Infof(ctx, "Found provision response with status: %s", response.GetStatus().String())

		// NOTE(b/406486832): If we are unable to boot from the USB after
		// multiple reboot/retries, mark the USB state as unknown.
		if response.GetStatus() == api.InstallResponse_STATUS_PRE_PROVISION_USB_BOOT_FAILURE && deviceData.GetChromeOsDeviceData().GetDutState() != nil {
			deviceData.GetChromeOsDeviceData().GetDutState().ServoUsbState = lab.HardwareState_HARDWARE_UNKNOWN
		}

		if response.GetStatus() != api.InstallResponse_STATUS_SUCCESS {
			repairRequest = determineRepairRequest(response.GetStatus())
			repairReason = "provision"
			foundProvisionError = true
			break
		}
	}
	if !foundProvisionError {
		osType = lab.VersionInfo_CHROMEOS
		if cmd.IsAlRun {
			osType = lab.VersionInfo_ANDROID
		}

		if cmd.TestResponses == nil || cmd.TestResponses.TestCaseResults == nil || len(cmd.TestResponses.TestCaseResults) == 0 {
			repairReason = "failed before running test(s)"
		} else if common.IsAnyTestFailure(cmd.TestResponses.TestCaseResults) {
			repairReason = "test(s)"
		}
	}
	if deviceData.GetChromeOsDeviceData() != nil {
		if osType != lab.VersionInfo_UNKNOWN {
			deviceData.GetChromeOsDeviceData().GetDutState().GetVersionInfo().OsType = osType
		}
		if repairRequest != lab.DutState_REPAIR_REQUEST_UNKNOWN {
			if deviceData.GetChromeOsDeviceData().GetDutState().GetRepairRequests() == nil {
				deviceData.GetChromeOsDeviceData().GetDutState().RepairRequests = []lab.DutState_RepairRequest{}
			}
			deviceData.GetChromeOsDeviceData().GetDutState().RepairRequests = append(deviceData.GetChromeOsDeviceData().GetDutState().RepairRequests, repairRequest)
		}
	}

	return
}

func (cmd *UpdateDutStateCmd) extractDepsFromHwTestStateKeeper(ctx context.Context, sk *data.HwTestStateKeeper) error {
	cmd.ProvisionResponses = make(map[string][]*api.InstallResponse)
	cmd.ProvisionDevices = make(map[string]*api.CrosTestRequest_Device)

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

	cmd.IsAlRun = sk.IsAlRun

	// TODO(cdelagarza): remove when confident in auto_repair
	pool := common.GetValueFromRequestKeyvals(ctx, sk.CftTestRequest, sk.CrosTestRunnerRequest, common.LabelPool)
	if slices.Contains(poolsDisallowed, pool) {
		cmd.SkipReason = fmt.Sprintf("pool %s has been marked disallowed for dut state updates", pool)
	}
	if sk.BuildState != nil {
		for _, tag := range sk.BuildState.Build().GetTags() {
			switch tag.GetKey() {
			case "analytics_name", "label-suite", "suite":
				if strings.HasPrefix(tag.GetValue(), "AL.") {
					cmd.SkipReason = fmt.Sprintf("Tag %s=%s has prefix 'AL.' marking disallowed for dut state updates", tag.GetKey(), tag.GetValue())
				}
			default:
			}
		}
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
func updateDutState(ctx context.Context, step *build.Step, ufsClient ufsAPI.FleetClient, hostName, dutID string, dutState dutstate.State, deviceData *ufsAPI.GetDeviceDataResponse) (*ufsAPI.UpdateTestDataResponse, error) {
	request := &ufsAPI.UpdateTestDataRequest{
		DeviceId:      dutID,
		Hostname:      hostName,
		ResourceState: dutstate.ConvertToUFSState(dutState),
		DeviceData: &ufsAPI.UpdateTestDataRequest_ChromeosData{
			ChromeosData: &ufsAPI.UpdateTestDataRequest_ChromeOs{
				DutState: deviceData.GetChromeOsDeviceData().GetDutState(),
			},
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{
				"dut.state",
				"dut_state.version_info",
				"dut_state.repair_requests",
				"dut_state.servo_usb_state",
			},
		},
	}
	common.WriteProtoToStepLog(ctx, step, request, "update request")
	return ufsClient.UpdateTestData(ctx, request)
}

func NewUpdateDutStateCmd() *UpdateDutStateCmd {
	abstractCmd := interfaces.NewAbstractCmd(UpdateDutStateCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &UpdateDutStateCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}

// addNamespaceCtxIfNotPresent checks if a namespace is set in the metadata
// contained in the context. If not, sets it to the default value specified in
// namespace.
func addNamespaceCtxIfNotPresent(ctx context.Context, namespace string) context.Context {
	if existingMetadata, ok := metadata.FromOutgoingContext(ctx); ok {
		// we found a namespace already set in the context, so should just use that
		if _, ok := existingMetadata[ufsUtil.Namespace]; ok {
			return ctx
		}
	}

	newMetadata := metadata.Pairs(ufsUtil.Namespace, namespace)
	return metadata.NewOutgoingContext(ctx, newMetadata)
}

func determineRepairRequest(provisionStatus api.InstallResponse_Status) lab.DutState_RepairRequest {
	switch provisionStatus {
	case
		api.InstallResponse_STATUS_UNSPECIFIED,
		api.InstallResponse_STATUS_INVALID_REQUEST,
		api.InstallResponse_STATUS_DUT_UNREACHABLE_PRE_PROVISION,
		api.InstallResponse_STATUS_DOWNLOADING_IMAGE_FAILED,
		api.InstallResponse_STATUS_GS_UPLOAD_FAILED,
		api.InstallResponse_STATUS_SUCCESS:
		return lab.DutState_REPAIR_REQUEST_UNKNOWN
	default:
		return lab.DutState_REPAIR_REQUEST_PROVISION
	}
}
