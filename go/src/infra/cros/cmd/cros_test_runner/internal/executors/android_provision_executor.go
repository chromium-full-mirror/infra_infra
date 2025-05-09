// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
)

// AndroidProvisionExecutor represents executor for all android provision related commands.
type AndroidProvisionExecutor struct {
	*interfaces.AbstractExecutor

	Container                     interfaces.ContainerInterface
	AndroidProvisionServiceClient testapi.GenericProvisionServiceClient
	ServerAddress                 string
}

// NewAndroidProvisionExecutor creates a new AndroidProvisionExecutor object.
func NewAndroidProvisionExecutor(container interfaces.ContainerInterface) *AndroidProvisionExecutor {
	absExec := interfaces.NewAbstractExecutor(AndroidProvisionExecutorType)
	return &AndroidProvisionExecutor{AbstractExecutor: absExec, Container: container}
}

// ExecuteCommand will execute relevant steps based on command type.
func (ex *AndroidProvisionExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.AndroidProvisionServiceStartCmd:
		return ex.androidProvisionStartCommandExecution(ctx, cmd)
	case *commands.AndroidProvisionInstallCmd:
		return ex.androidProvisionInstallCommandExecution(ctx, cmd)
	default:
		return fmt.Errorf(
			"Command type %s is not supported by %s executor type!",
			cmd.GetCommandType(),
			ex.GetExecutorType())
	}
}

// androidProvisionStartCommandExecution executes the android-provision server start command.
func (ex *AndroidProvisionExecutor) androidProvisionStartCommandExecution(
	ctx context.Context,
	cmd *commands.AndroidProvisionServiceStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Android provision service start")
	defer func() { step.End(err) }()

	err = ex.Start(ctx)
	logErr := common.WriteContainerLogToStepLog(ctx, ex.Container, step, "android-provision log")
	if err != nil {
		return errors.Annotate(err, "Start android provision service cmd err: ").Err()
	}
	if logErr != nil {
		logging.Infof(ctx, "error during writing android-provision log contents: %s", err)
	}

	return err
}

// androidProvisionInstallCommandExecution executes the android-provision install command.
func (ex *AndroidProvisionExecutor) androidProvisionInstallCommandExecution(
	ctx context.Context,
	cmd *commands.AndroidProvisionInstallCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Android provision install")
	defer func() { step.End(err) }()

	req := &testapi.InstallRequest{
		Metadata: cmd.AndroidProvisionState}

	logsLoc, err := ex.Container.GetLogsLocation()
	if err != nil {
		logging.Infof(ctx, "error during getting container log location: %s", err)
		return err
	}
	containerLog := step.Log("Android-provision Log")

	taskDone, wg, err := common.StreamLogAsync(ctx, logsLoc, containerLog)
	if err != nil {
		logging.Infof(ctx, "Warning: error during reading container log: %s", err)
	}
	provisionStartupRequest := &testapi.ProvisionStartupRequest{
		Dut:       cmd.AndroidCompanionDut,
		DutServer: cmd.AndroidDutServerAddress,
	}
	common.WriteProtoToStepLog(ctx, step, provisionStartupRequest, "android provision startup request")
	provisionStartupResp, startupErr := ex.AndroidProvisionServiceClient.StartUp(ctx, provisionStartupRequest)
	if startupErr != nil {
		logging.Infof(ctx, "error during startup: %s", startupErr)
		return err
	}
	common.WriteProtoToStepLog(ctx, step, provisionStartupResp, "android provision startup response")
	common.WriteProtoToStepLog(ctx, step, req, "android provision install request")
	resp, err := ex.Install(ctx, req)
	if taskDone != nil {
		taskDone <- true // Notify logging process that main task is done
	}
	cmd.AndroidProvisionResponse = resp
	wg.Wait() // Wait for the logging to complete
	if err != nil {
		err = errors.Annotate(err, "Android provision install cmd err: ").Err()
	}

	step.SetSummaryMarkdown(fmt.Sprintf("android provision status: %s", resp.GetStatus().String()))
	step.AddTagValue("provision_status", resp.GetStatus().String())
	common.WriteProtoToStepLog(ctx, step, resp, "android provision install response")

	if resp.GetStatus() != testapi.InstallResponse_STATUS_SUCCESS {
		err = fmt.Errorf("Android provision failure: %s", resp.GetStatus().String())
	}

	return err
}

// Start starts the android-provision server.
func (ex *AndroidProvisionExecutor) Start(ctx context.Context) error {

	template := &testapi.Template{Container: &testapi.Template_Generic{
		Generic: &testapi.GenericTemplate{
			DockerArtifactDir: "/tmp/provision",
			BinaryName:        "android-provision",
			BinaryArgs: []string{
				"server",
				"-port", "0",
			},
			AdditionalVolumes: []string{
				"/creds:/creds",
			},
		},
	}}
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
			"error during connecting with android-provision server at %s: %s",
			serverAddress,
			err.Error())
		return err
	}
	logging.Infof(ctx, "Connected with android-provision service.")

	// Create new client.
	androidProvisionClient := testapi.NewGenericProvisionServiceClient(conn)
	if androidProvisionClient == nil {
		return fmt.Errorf("androidProvisionServiceClient is nil")
	}

	ex.AndroidProvisionServiceClient = androidProvisionClient

	return nil
}

// Install invokes the provision install endpoint of android-provision.
func (ex *AndroidProvisionExecutor) Install(
	ctx context.Context,
	installReq *testapi.InstallRequest) (*testapi.InstallResponse, error) {

	if installReq == nil {
		return nil, fmt.Errorf("Cannot execute android provision install for nil install request")
	}
	if ex.AndroidProvisionServiceClient == nil {
		return nil, fmt.Errorf("AndroidProvisionServiceClient is nil in AndroidProvisionExecutor")
	}

	provisionOp, err := ex.AndroidProvisionServiceClient.Install(ctx, installReq, grpc.EmptyCallOption{})
	if err != nil {
		return nil, errors.Annotate(err, "android provision install failure: ").Err()
	}

	// TODO: Fix this to use common.WaitLro
	opResp, err := common.ProcessDoneLro(ctx, provisionOp)
	if err != nil {
		return nil, errors.Annotate(err, "android provision lro failure: ").Err()
	}

	provisionResp := &testapi.InstallResponse{}
	if err := opResp.UnmarshalTo(provisionResp); err != nil {
		logging.Infof(ctx, "android provision lro response unmarshalling failed: %s", err.Error())
		return nil, errors.Annotate(err, "android provision lro response unmarshalling failed: ").Err()
	}

	return provisionResp, nil
}
