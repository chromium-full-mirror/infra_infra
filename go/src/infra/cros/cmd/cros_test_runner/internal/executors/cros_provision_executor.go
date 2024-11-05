// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/internal/commands"
)

// CrosProvisionExecutor represents executor for all cros-provision related commands.
type CrosProvisionExecutor struct {
	*interfaces.AbstractExecutor

	Container                  interfaces.ContainerInterface
	CrosProvisionServiceClient testapi.GenericProvisionServiceClient
	ServerAddress              string
}

func NewCrosProvisionExecutor(container interfaces.ContainerInterface) *CrosProvisionExecutor {
	absExec := interfaces.NewAbstractExecutor(CrosProvisionExecutorType)
	return &CrosProvisionExecutor{AbstractExecutor: absExec, Container: container}
}

func (ex *CrosProvisionExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.ProvisionServiceStartCmd:
		if err := ex.provisionStartCommandExecution(ctx, cmd); err != nil {
			return err
		}
		return ex.provisionStartupCommandExecution(ctx, cmd)
	case *commands.ProvisionInstallCmd:
		return ex.provisionInstallCommandExecution(ctx, cmd)
	default:
		return fmt.Errorf(
			"Command type %s is not supported by %s executor type!",
			cmd.GetCommandType(),
			ex.GetExecutorType())
	}
}

// provisionStartCommandExecution executes the provision start command.
func (ex *CrosProvisionExecutor) provisionStartCommandExecution(
	ctx context.Context,
	cmd *commands.ProvisionServiceStartCmd) error {
	var err error
	step, ctx := build.StartStep(ctx, "Provision service start")
	defer func() { step.End(err) }()

	provReq := &testapi.CrosProvisionRequest{
		Dut:            cmd.PrimaryDut,
		ProvisionState: cmd.ProvisionState,
		DutServer:      cmd.DutServerAddress}

	err = ex.Start(ctx, provReq)
	logErr := common.WriteContainerLogToStepLog(ctx, ex.Container, step, "cros-provision log")
	if err != nil {
		return errors.Annotate(err, "Start provision cmd err: ").Err()
	}
	if logErr != nil {
		logging.Infof(ctx, "error during writing cros-provision log contents: %s", err)
	}

	return err
}

func (ex *CrosProvisionExecutor) provisionStartupCommandExecution(
	ctx context.Context,
	cmd *commands.ProvisionServiceStartCmd) error {
	step, ctx := build.StartStep(ctx, "Provision StartUp")
	defer func() { step.End(nil) }()

	startupReq := &testapi.ProvisionStartupRequest{
		Dut:            cmd.PrimaryDut,
		DutServer:      cmd.DutServerAddress,
		ServoNexusAddr: cmd.ServoNexusAddress,
	}
	common.WriteProtoToStepLog(ctx, step, startupReq, "startup request")
	resp, err := ex.StartUp(ctx, startupReq)
	if err != nil {
		step.SetSummaryMarkdown(err.Error())
	}
	common.WriteProtoToStepLog(ctx, step, resp, "startup response")

	return nil
}

// provisionInstallCommandExecution executes the provision install command.
func (ex *CrosProvisionExecutor) provisionInstallCommandExecution(
	ctx context.Context,
	cmd *commands.ProvisionInstallCmd) error {
	var err error
	step, ctx := build.StartStep(ctx, "Provision install")
	defer func() { step.End(err) }()

	req := &testapi.InstallRequest{
		ImagePath:     cmd.OsImagePath,
		PreventReboot: cmd.PreventReboot,
		Metadata:      cmd.InstallMetadata}
	common.WriteProtoToStepLog(ctx, step, req, "provision request")

	logsLoc, err := ex.Container.GetLogsLocation()
	if err != nil {
		logging.Infof(ctx, "error during getting container log location: %s", err)
		return err
	}
	containerLog := step.Log("Cros-provision Log")

	taskDone, wg, err := common.StreamLogAsync(ctx, logsLoc, containerLog)
	if err != nil {
		logging.Infof(ctx, "Warning: error during reading container log: %s", err)
	}

	resp, err := ex.Install(ctx, req)
	if taskDone != nil {
		taskDone <- true // Notify logging process that main task is done
	}
	wg.Wait() // Wait for the logging to complete
	if err != nil {
		err = errors.Annotate(err, "Provision install cmd err: ").Err()
		return err
	}

	step.SetSummaryMarkdown(fmt.Sprintf("provision status: %s", resp.GetStatus().String()))
	step.AddTagValue("provision_status", resp.GetStatus().String())
	cmd.ProvisionResp = resp
	common.WriteProtoToStepLog(ctx, step, resp, "provision response")

	if resp.GetStatus() != api.InstallResponse_STATUS_SUCCESS {
		err = fmt.Errorf("Provision failure: %s", resp.GetStatus().String())
		common.GlobalNonInfraError = err
	}

	return err
}

// Start starts the cros-provision server.
func (ex *CrosProvisionExecutor) Start(
	ctx context.Context,
	provisionInputReq *testapi.CrosProvisionRequest) error {

	if provisionInputReq == nil {
		return fmt.Errorf("Cannot start provision service with nil provision request.")
	}

	provisionTemplate := &testapi.CrosProvisionTemplate{InputRequest: provisionInputReq}
	template := &api.Template{Container: &api.Template_CrosProvision{CrosProvision: provisionTemplate}}

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
			"error during connecting with provision server at %s: %s",
			serverAddress,
			err.Error())
		return err
	}
	logging.Infof(ctx, "Connected with provision service.")

	// Create new client.
	provisionClient := api.NewGenericProvisionServiceClient(conn)
	if provisionClient == nil {
		return fmt.Errorf("ProvisionServiceClient is nil")
	}

	ex.CrosProvisionServiceClient = provisionClient

	return nil
}

func (ex *CrosProvisionExecutor) StartUp(
	ctx context.Context,
	startupReq *testapi.ProvisionStartupRequest) (*testapi.ProvisionStartupResponse, error) {

	if startupReq == nil {
		return nil, fmt.Errorf("Cannot execution provision startup for nil startup request")
	}
	if ex.CrosProvisionServiceClient == nil {
		return nil, fmt.Errorf("CrosProvisionServiceClient is nil in CrosProvisionExecutor")
	}

	startupResp, err := ex.CrosProvisionServiceClient.StartUp(ctx, startupReq, grpc.EmptyCallOption{})
	if err != nil {
		return nil, errors.Annotate(err, "provision startup failure: ").Err()
	}

	return startupResp, nil
}

// Install invokes the provision install endpoint of cros-provision.
func (ex *CrosProvisionExecutor) Install(
	ctx context.Context,
	installReq *testapi.InstallRequest) (*testapi.InstallResponse, error) {

	if installReq == nil {
		return nil, fmt.Errorf("Cannot execute provision install for nil install request.")
	}
	if ex.CrosProvisionServiceClient == nil {
		return nil, fmt.Errorf("CrosProvisionServiceClient is nil in CrosProvisionExecutor")
	}

	provisionOp, err := ex.CrosProvisionServiceClient.Install(ctx, installReq, grpc.EmptyCallOption{})
	if err != nil {
		return nil, errors.Annotate(err, "provision install failure: ").Err()
	}

	// TODO: Fix this to use common.WaitLro
	opResp, err := common.ProcessDoneLro(ctx, provisionOp)
	if err != nil {
		return nil, errors.Annotate(err, "provision lro failure: ").Err()
	}

	provisionResp := &testapi.InstallResponse{}
	if err := opResp.UnmarshalTo(provisionResp); err != nil {
		logging.Infof(ctx, "provision lro response unmarshalling failed: %s", err.Error())
		return nil, errors.Annotate(err, "provision lro response unmarshalling failed: ").Err()
	}

	return provisionResp, nil
}
