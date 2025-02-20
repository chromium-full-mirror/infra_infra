// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/internal/commands"
)

// GenericProvisionExecutor represents executor for all cros-provision related commands.
type GenericProvisionExecutor struct {
	*interfaces.AbstractExecutor
}

func NewGenericProvisionExecutor() *GenericProvisionExecutor {
	absExec := interfaces.NewAbstractExecutor(GenericProvisionExecutorType)
	return &GenericProvisionExecutor{AbstractExecutor: absExec}
}

func (ex *GenericProvisionExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {
	var err error
	switch cmd := cmdInterface.(type) {
	case *commands.GenericProvisionCmd:
		err = ex.genericProvisionHandler(ctx, cmd)
	default:
		err = fmt.Errorf(
			"command type %s is not supported by %s executor type",
			cmd.GetCommandType(),
			ex.GetExecutorType())
	}
	if err != nil {
		// Don't wrap err as a TestRunnerError if it already is one.
		var tre *common.TestRunnerError
		if errors.As(err, &tre) {
			return err
		} else {
			return &common.TestRunnerError{
				Type: skylab_test_runner.TestRunnerErrorType_PROVISION,
				Err:  err,
			}
		}
	}
	return nil
}

// provisionStartCommandExecution executes the provision start command.
func (ex *GenericProvisionExecutor) genericProvisionHandler(
	ctx context.Context,
	cmd *commands.GenericProvisionCmd) (err error) {
	stepName := "Provision service"
	if cmd.ProvisionRequest.DynamicIdentifier != "" {
		stepName = fmt.Sprintf("%s: %s", stepName, cmd.ProvisionRequest.DynamicIdentifier)
	}
	step, ctx := build.StartStep(ctx, stepName)
	defer func() { step.End(err) }()

	common.WriteProtoToStepLog(ctx, step, cmd.ProvisionRequest, "provision service request")

	client, lroClient, err := ex.ConnectToService(ctx, cmd.ProvisionRequest.GetServiceAddress())
	if err != nil {
		err = fmt.Errorf("error connecting to provision service, %w", err)
		return
	}

	startUpResp, err := ex.Startup(ctx, client, cmd.ProvisionRequest.StartupRequest)
	if err != nil {
		// Error from Startup should be non-breaking to ensure older
		// builds that don't have this rpc don't require this step.
		logging.Infof(ctx, "error starting up provision service, %s", err)
	}
	cmd.StartUpResp = startUpResp

	installResp, err := ex.Install(ctx, client, lroClient, cmd.ProvisionRequest.GetInstallRequest())
	if err != nil {
		return
	}

	if installResp.GetMessage() != "" {
		step.SetSummaryMarkdown(fmt.Sprintf("provision status: %s provision state: %s", installResp.GetStatus().String(), installResp.GetMessage()))
	} else {
		step.SetSummaryMarkdown(fmt.Sprintf("provision status: %s", installResp.GetStatus().String()))
	}

	step.AddTagValue("provision_status", installResp.GetStatus().String())
	step.AddTagValue("provision_message", installResp.GetMessage())
	cmd.InstallResp = installResp
	common.WriteProtoToStepLog(ctx, step, installResp, "provision response")

	if installResp.GetStatus() != api.InstallResponse_STATUS_SUCCESS {
		err = fmt.Errorf("Provision failure: %s", installResp.GetStatus().String())
		common.GlobalNonInfraError = err
	}

	return
}

// ConnectToService connects to the GenericProvisionService attached to the server address.
func (ex *GenericProvisionExecutor) ConnectToService(
	ctx context.Context,
	endpoint *labapi.IpEndpoint) (api.GenericProvisionServiceClient, longrunning.OperationsClient, error) {
	var err error
	step, ctx := build.StartStep(ctx, "Establish Connection")
	defer func() { step.End(err) }()

	// Connect with the service.
	address := common.GetServerAddress(endpoint)
	conn, err := common.ConnectWithService(ctx, address)
	if err != nil {
		logging.Infof(
			ctx,
			"error during connecting with provision server at %s: %s",
			address,
			err.Error())
		return nil, nil, err
	}
	logging.Infof(ctx, "Connected with provision service.")

	// Create new client.
	provisionClient := api.NewGenericProvisionServiceClient(conn)
	if provisionClient == nil {
		err = fmt.Errorf("ProvisionServiceClient is nil")
		return nil, nil, err
	}

	lroClient := longrunning.NewOperationsClient(conn)
	if lroClient == nil {
		err = fmt.Errorf("OperationsClient is nil")
		return nil, nil, err
	}

	return provisionClient, lroClient, err
}

// Startup invokces the StartUp endpoint of the GenericProvisionServiceClient
func (ex *GenericProvisionExecutor) Startup(
	ctx context.Context,
	client api.GenericProvisionServiceClient,
	req *api.ProvisionStartupRequest,
) (resp *api.ProvisionStartupResponse, err error) {
	step, ctx := build.StartStep(ctx, "Start Up")
	defer func() { step.End(err) }()

	if req == nil {
		err = fmt.Errorf("ProvisionStartupRequest is nil")
		return
	}

	if client == nil {
		err = fmt.Errorf("ProvisionStartupRequest is nil")
		return
	}

	resp, err = client.StartUp(ctx, req, grpc.EmptyCallOption{})
	if err != nil {
		return
	}
	common.WriteProtoToStepLog(ctx, step, resp, "startup response")

	step.SetSummaryMarkdown(fmt.Sprintf("startup status: %s", resp.GetStatus().String()))
	step.AddTagValue("startup_status", resp.GetStatus().String())

	if resp.GetStatus() != api.ProvisionStartupResponse_STATUS_SUCCESS {
		err = fmt.Errorf("Provision Startup failure: %s", resp.GetStatus().String())
		return
	}

	return
}

// Startup invokces the StartUp endpoint of the GenericProvisionServiceClient
func (ex *GenericProvisionExecutor) Install(
	ctx context.Context,
	client api.GenericProvisionServiceClient,
	lroClient longrunning.OperationsClient,
	req *api.InstallRequest,
) (resp *api.InstallResponse, err error) {
	step, ctx := build.StartStep(ctx, "Install")
	defer func() { step.End(err) }()

	if req == nil {
		err = fmt.Errorf("ProvisionStartupRequest is nil")
		return
	}

	if client == nil {
		err = fmt.Errorf("ProvisionStartupRequest is nil")
		return
	}

	common.WriteProtoToStepLog(ctx, step, req, "install request")
	provisionOp, err := client.Install(ctx, req, grpc.EmptyCallOption{})
	if err != nil {
		err = errors.Annotate(err, "provision install failure: ").Err()
		return
	}

	opResp, err := common.WaitLro(ctx, lroClient, provisionOp)
	if err != nil {
		err = errors.Annotate(err, "provision lro failure: ").Err()
		return
	}

	resp = &api.InstallResponse{}
	if err = opResp.UnmarshalTo(resp); err != nil {
		err = errors.Annotate(err, "provision lro response unmarshalling failed: ").Err()
		return
	}

	return
}
