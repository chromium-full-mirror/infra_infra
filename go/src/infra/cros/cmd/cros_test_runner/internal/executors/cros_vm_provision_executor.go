// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/internal/commands"
)

// CrosVMProvisionExecutor represents executor for all vm-provision related commands.
type CrosVMProvisionExecutor struct {
	*interfaces.AbstractExecutor

	Container                    interfaces.ContainerInterface
	CrosVMProvisionServiceClient testapi.GenericProvisionServiceClient
	ServerAddress                string
}

// NewCrosVMProvisionExecutor creates a new CrosVMProvisionExecutor object.
func NewCrosVMProvisionExecutor(container interfaces.ContainerInterface) *CrosVMProvisionExecutor {
	absExec := interfaces.NewAbstractExecutor(CrosVMProvisionExecutorType)
	return &CrosVMProvisionExecutor{AbstractExecutor: absExec, Container: container}
}

// ExecuteCommand will execute relevant steps based on command type.
func (ex *CrosVMProvisionExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.VMProvisionServiceStartCmd:
		return ex.vmProvisionStartCommandExecution(ctx, cmd)
	case *commands.VMProvisionLeaseCmd:
		return ex.vmProvisionLeaseCommandExecution(ctx, cmd)
	case *commands.VMProvisionReleaseCmd:
		return ex.vmProvisionReleaseCommandExecution(ctx, cmd)
	default:
		return fmt.Errorf(
			"Command type %s is not supported by %s executor type!",
			cmd.GetCommandType(),
			ex.GetExecutorType())
	}
}

// vmProvisionStartCommandExecution executes the vm-provision server start command.
func (ex *CrosVMProvisionExecutor) vmProvisionStartCommandExecution(
	ctx context.Context,
	cmd *commands.VMProvisionServiceStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "VM Provision service start")
	defer func() { step.End(err) }()

	err = ex.Start(ctx)
	logErr := common.WriteContainerLogToStepLog(ctx, ex.Container, step, "vm-provision log")
	if err != nil {
		return errors.Annotate(err, "Start vm provision service cmd err: ").Err()
	}
	if logErr != nil {
		logging.Infof(ctx, "error during writing vm-provision log contents: %s", err)
	}

	return err
}

// Start starts the vm-provision server.
func (ex *CrosVMProvisionExecutor) Start(ctx context.Context) error {

	crosvmTemplate := &api.CrosVMProvisionTemplate{}
	template := &api.Template{
		Container: &api.Template_CrosVmProvision{
			CrosVmProvision: crosvmTemplate,
		},
	}
	// Process container.
	serverAddress, err := ex.Container.ProcessContainer(ctx, template)
	if err != nil {
		return errors.Annotate(err, "error processing container: ").Err()
	}

	ex.ServerAddress = serverAddress

	// Connect with the service.
	conn, err := common.ConnectWithService(ctx, serverAddress)
	if err != nil {
		logging.Infof(
			ctx,
			"error during connecting with vm-provision server at %s: %s",
			serverAddress,
			err.Error())
		return err
	}
	logging.Infof(ctx, "Connected with vm-provision service.")

	// Create new client.
	vmProvisionClient := api.NewGenericProvisionServiceClient(conn)
	if vmProvisionClient == nil {
		return fmt.Errorf("crosVMProvisionServiceClient is nil")
	}

	ex.CrosVMProvisionServiceClient = vmProvisionClient

	return nil
}

// vmProvisionLeaseCommandExecution executes the lease dut vm command.
func (ex *CrosVMProvisionExecutor) vmProvisionLeaseCommandExecution(
	ctx context.Context,
	cmd *commands.VMProvisionLeaseCmd) error {
	var err error
	step, ctx := build.StartStep(ctx, "VM-Provision lease dut vm")
	defer func() { step.End(err) }()

	//create request
	img := fmt.Sprintf("projects/%v/global/images/%v", cmd.DutVmGceImage.GetProject(), cmd.DutVmGceImage.GetName())
	d, _ := time.ParseDuration("24h")
	leaseVMRequest := &api.LeaseVMRequest{
		HostReqs: &api.VMRequirements{
			GceImage:                 img,
			GceProject:               common.GceProject,
			GceNetwork:               common.GceNetwork,
			GceMachineType:           common.GceMachineTypeN14,
			GceMinCpuPlatform:        common.GceMinCPUPlatform,
			SubnetModeNetworkEnabled: true,
			GceDiskSize:              getDiskSizeByBoard(img),
		},
		LeaseDuration: durationpb.New(d),
	}

	metadata := &anypb.Any{}
	if err := metadata.MarshalFrom(leaseVMRequest); err != nil {
		logging.Infof(ctx, "Failed to marshal request, %s", err)
		return err
	}

	req := &testapi.InstallRequest{
		Metadata: metadata}

	common.WriteProtoToStepLog(ctx, step, req, "vm provision lease request")

	logsLoc, err := ex.Container.GetLogsLocation()
	if err != nil {
		logging.Infof(ctx, "error during getting container log location: %s", err)
		return err
	}
	containerLog := step.Log("vm-provision log")

	taskDone, wg, err := common.StreamLogAsync(ctx, logsLoc, containerLog)
	if err != nil {
		logging.Infof(ctx, "Warning: error during reading container log: %s", err)
	}
	// lease DUT VM
	resp, err := ex.LeaseDutVM(ctx, req)
	if taskDone != nil {
		taskDone <- true // Notify logging process that main task is done
	}
	wg.Wait() // Wait for the logging to complete
	if err != nil {
		return errors.Annotate(err, "VM Provision lease cmd err: ").Err()
	}
	common.WriteProtoToStepLog(ctx, step, resp, "vm provision lease response")
	leaseVMResponse := &api.LeaseVMResponse{}
	if err := resp.Metadata.UnmarshalTo(leaseVMResponse); err != nil {
		logging.Infof(ctx, "Failed to unmarshal response:, %s", err)
		return err
	}
	if err := ex.validateLeaseVMResponse(leaseVMResponse); err != nil {
		logging.Infof(ctx, "Invalid response from vm leaser:, %s", err)
		return err
	}
	cmd.LeaseVMResponse = leaseVMResponse

	logging.Infof(ctx, "wait for SSH to become available")
	common.WaitDutVmBoot(ctx, leaseVMResponse.GetVm().GetAddress().GetHost())

	logging.Infof(ctx, "completed wait for SSH")

	return err
}

// LeaseDutVM invokes the provision install endpoint of vm-provision.
func (ex *CrosVMProvisionExecutor) LeaseDutVM(
	ctx context.Context,
	installReq *testapi.InstallRequest) (*testapi.InstallResponse, error) {

	if installReq == nil {
		return nil, fmt.Errorf("Cannot execute vm-provision lease for nil lease request.")
	}
	if ex.CrosVMProvisionServiceClient == nil {
		return nil, fmt.Errorf("CrosVMProvisionServiceClient is nil in CrosVMProvisionExecutor")
	}
	vmProvisionOp, err := ex.CrosVMProvisionServiceClient.Install(ctx, installReq, grpc.EmptyCallOption{})
	if err != nil {
		logging.Infof(ctx, "purgeAuthToken called after lease failed")
		purgeErr := ex.purgeAuthToken(ctx)
		if purgeErr != nil {
			logging.Infof(ctx, "auth token file purge error after lease failed: %s", purgeErr)
		}
		return nil, errors.Annotate(err, "vm-provision lease failure: ").Err()
	}

	// TODO: Fix this to use common.WaitLro
	opResp, err := common.ProcessDoneLro(ctx, vmProvisionOp)
	if err != nil {
		return nil, errors.Annotate(err, "vm-provision lro failure: ").Err()
	}

	vmProvisionResp := &testapi.InstallResponse{}
	if err := opResp.UnmarshalTo(vmProvisionResp); err != nil {
		logging.Infof(ctx, "vm-provision lro response unmarshalling failed: %s", err.Error())
		return nil, errors.Annotate(err, "vm-provision lro response unmarshalling failed: ").Err()
	}

	return vmProvisionResp, nil
}

// vmProvisionReleaseCommandExecution executes the release dut vm command.
func (ex *CrosVMProvisionExecutor) vmProvisionReleaseCommandExecution(
	ctx context.Context,
	cmd *commands.VMProvisionReleaseCmd) error {
	var err error
	step, ctx := build.StartStep(ctx, "VM-Provision release dut vm")
	defer func() { step.End(err) }()

	if cmd.LeaseVMResponse == nil {
		logging.Infof(ctx, "Skipping release as lease did not happen earlier during execution")
		return nil
	}

	//create request
	releaseVMRequest := &api.ReleaseVMRequest{
		LeaseId:    cmd.LeaseVMResponse.GetLeaseId(),
		GceProject: common.GceProject,
		GceRegion:  cmd.LeaseVMResponse.GetVm().GetGceRegion(),
	}
	metadata := &anypb.Any{}
	if err := metadata.MarshalFrom(releaseVMRequest); err != nil {
		logging.Infof(ctx, "Failed to marshal request, %s", err)
		return err
	}

	req := &testapi.InstallRequest{
		Metadata: metadata}

	common.WriteProtoToStepLog(ctx, step, req, "vm provision release request")

	logsLoc, err := ex.Container.GetLogsLocation()
	if err != nil {
		logging.Infof(ctx, "error during getting container log location: %s", err)
		return err
	}
	containerLog := step.Log("vm-provision log")

	taskDone, wg, err := common.StreamLogAsync(ctx, logsLoc, containerLog)
	if err != nil {
		logging.Infof(ctx, "Warning: error during reading container log: %s", err)
	}

	resp, err := ex.ReleaseDutVM(ctx, req)
	if taskDone != nil {
		taskDone <- true // Notify logging process that main task is done
	}
	wg.Wait() // Wait for the logging to complete
	if err != nil {
		return errors.Annotate(err, "VM Provision release cmd err: ").Err()
	}
	common.WriteProtoToStepLog(ctx, step, resp, "vm provision release response")

	return err
}

// ReleaseDutVM invokes the provision install endpoint of vm-provision.
func (ex *CrosVMProvisionExecutor) ReleaseDutVM(
	ctx context.Context,
	installReq *testapi.InstallRequest) (*testapi.InstallResponse, error) {

	if installReq == nil {
		return nil, fmt.Errorf("Cannot execute vm-provision release for nil lease request.")
	}
	if ex.CrosVMProvisionServiceClient == nil {
		return nil, fmt.Errorf("CrosVMProvisionServiceClient is nil in CrosVMProvisionExecutor")
	}

	vmProvisionOp, err := ex.CrosVMProvisionServiceClient.Install(ctx, installReq, grpc.EmptyCallOption{})
	if err != nil {
		logging.Infof(ctx, "purgeAuthToken called after release failed")
		purgeErr := ex.purgeAuthToken(ctx)
		if purgeErr != nil {
			logging.Infof(ctx, "auth token file purge error after release failed: %s", purgeErr)
		}
		return nil, errors.Annotate(err, "vm-provision release failure: ").Err()
	}
	logging.Infof(ctx, "purgeAuthToken called after release succeeded")
	purgeErr := ex.purgeAuthToken(ctx)
	if purgeErr != nil {
		logging.Infof(ctx, "auth token file purge error after release succeeded: %s", purgeErr)
	}

	// TODO: Fix this to use common.WaitLro
	opResp, err := common.ProcessDoneLro(ctx, vmProvisionOp)
	if err != nil {
		return nil, errors.Annotate(err, "vm-provision lro failure: ").Err()
	}

	vmProvisionResp := &testapi.InstallResponse{}
	if err := opResp.UnmarshalTo(vmProvisionResp); err != nil {
		logging.Infof(ctx, "vm-provision lro response unmarshalling failed: %s", err.Error())
		return nil, errors.Annotate(err, "vm-provision lro response unmarshalling failed: ").Err()
	}

	return vmProvisionResp, nil
}

func (ex *CrosVMProvisionExecutor) validateLeaseVMResponse(leaseVMResponse *api.LeaseVMResponse) error {

	if leaseVMResponse.GetVm() == nil {
		return fmt.Errorf("Nil VM object in vm leaser response")
	}
	if leaseVMResponse.GetVm().GetAddress() == nil {
		return fmt.Errorf("Nil vm address in vm leaser response")
	}
	if leaseVMResponse.GetVm().GetAddress().GetHost() == "" {
		return fmt.Errorf("Nil vm host address in vm leaser response")
	}
	if leaseVMResponse.GetVm().GetAddress().GetPort() == 0 {
		return fmt.Errorf("Nil vm port address in vm leaser response")
	}
	return nil
}

func getDiskSizeByBoard(image string) int64 {

	if strings.Contains(image, "betty") {
		return 38
	} else if strings.Contains(image, "reven-vmtest") {
		return 20
	} else if strings.Contains(image, "amd64-generic") {
		return 26
	}
	return 13
}

// purgeAuthToken creates a file signalling ctr to stop generating authToken. It then waits to enure that the file has been removed.
func (ex *CrosVMProvisionExecutor) purgeAuthToken(ctx context.Context) error {
	tempDirLoc, err := ex.Container.GetLogsLocation()
	if err != nil {
		return err
	}
	// Construct the full path of the file to be added
	filePath := filepath.Join(tempDirLoc, "removeSignal.txt")
	authFilePath := filepath.Join(tempDirLoc, "authToken.txt")
	logging.Infof(ctx, "Adding auth token remove signal file: %s", filePath)

	_, err = os.Create(filePath)
	if err != nil {
		logging.Infof(ctx, "Failed to create auth token remove signal file: %s", filePath)
		return err
	}
	logging.Infof(ctx, "Auth token remove signal file added: %s", filePath)

	logging.Infof(ctx, "Starting loop to check if auth token file is removed")
	// Check if the file exists in 5 loops with an interval of 0.1 seconds
	for i := 0; i < 5; i++ {
		time.Sleep(100 * time.Millisecond)
		if _, err := os.Stat(authFilePath); os.IsNotExist(err) {
			logging.Infof(ctx, "Auth token file removed from temp dir: %s", authFilePath)
			return nil
		} else if err != nil {
			return err
		}
	}

	return nil
}
