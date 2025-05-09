// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
)

// GenericServiceExecutor represents executor for all cros-generic related commands.
type GenericServiceExecutor struct {
	*interfaces.AbstractExecutor
}

func NewGenericServiceExecutor() *GenericServiceExecutor {
	absExec := interfaces.NewAbstractExecutor(GenericServiceExecutorType)
	return &GenericServiceExecutor{AbstractExecutor: absExec}
}

func (ex *GenericServiceExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.GenericServiceCmd:
		return ex.GenericServiceHandler(ctx, cmd)
	default:
		return fmt.Errorf(
			"Command type %s is not supported by %s executor type!",
			cmd.GetCommandType(),
			ex.GetExecutorType())
	}
}

// GenericServiceHandler executes the generic start command.
func (ex *GenericServiceExecutor) GenericServiceHandler(
	ctx context.Context,
	cmd *commands.GenericServiceCmd) (err error) {
	stepName := "generic service"
	if cmd.GenericRequest.DynamicIdentifier != "" {
		stepName = fmt.Sprintf("%s: %s", stepName, cmd.GenericRequest.DynamicIdentifier)
	}
	step, ctx := build.StartStep(ctx, stepName)
	defer func() { step.End(err) }()

	common.WriteProtoToStepLog(ctx, step, cmd.GenericRequest, "generic service request")

	client, err := ex.ConnectToService(ctx, cmd.GenericRequest.GetServiceAddress())
	if err != nil {
		err = fmt.Errorf("error connecting to generic service, %w", err)
		return
	}

	startResp, err := ex.Start(ctx, client, cmd.GenericRequest.StartRequest)
	cmd.StartResp = startResp
	if err != nil {
		err = fmt.Errorf("error in generic service for 'Start', %w", err)
		return
	}

	runResp, err := ex.Run(ctx, client, cmd.GenericRequest.RunRequest)
	cmd.RunResp = runResp
	if err != nil {
		err = fmt.Errorf("error in generic service for 'Run', %w", err)
		return
	}

	stopResp, err := ex.Stop(ctx, client, cmd.GenericRequest.StopRequest)
	cmd.StopResp = stopResp
	if err != nil {
		err = fmt.Errorf("error in generic service for 'Stop', %w", err)
		return
	}

	return
}

// ConnectToService connects to the GenericServiceService attached to the server address.
func (ex *GenericServiceExecutor) ConnectToService(
	ctx context.Context,
	endpoint *labapi.IpEndpoint) (testapi.GenericServiceClient, error) {
	var err error
	step, ctx := build.StartStep(ctx, "Establish Connection")
	defer func() { step.End(err) }()

	// Connect with the service.
	address := common.GetServerAddress(endpoint)
	conn, err := common.ConnectWithService(ctx, address)
	if err != nil {
		logging.Infof(
			ctx,
			"error during connecting with generic server at %s: %s",
			address,
			err.Error())
		return nil, err
	}
	logging.Infof(ctx, "Connected with generic service.")

	// Create new client.
	genericServiceClient := testapi.NewGenericServiceClient(conn)
	if genericServiceClient == nil {
		err = fmt.Errorf("ProvisionServiceClient is nil")
		return nil, err
	}

	return genericServiceClient, err
}

// Start invokces the Start endpoint of the GenericServiceClient
func (ex *GenericServiceExecutor) Start(
	ctx context.Context,
	client testapi.GenericServiceClient,
	req *testapi.GenericStartRequest,
) (resp *testapi.GenericStartResponse, err error) {
	step, ctx := build.StartStep(ctx, "Start")
	defer func() { step.End(err) }()

	if req == nil {
		err = fmt.Errorf("GenericStartRequest is nil")
		return
	}

	if client == nil {
		err = fmt.Errorf("GenericServiceClient is nil")
		return
	}

	resp, err = client.Start(ctx, req, grpc.EmptyCallOption{})
	if err != nil {
		return
	}
	common.WriteProtoToStepLog(ctx, step, resp, "start response")

	return
}

// Run invokces the Run endpoint of the GenericServiceClient
func (ex *GenericServiceExecutor) Run(
	ctx context.Context,
	client testapi.GenericServiceClient,
	req *testapi.GenericRunRequest,
) (resp *testapi.GenericRunResponse, err error) {
	step, ctx := build.StartStep(ctx, "Run")
	defer func() { step.End(err) }()

	if req == nil {
		err = fmt.Errorf("GenericRunRequest is nil")
		return
	}

	if client == nil {
		err = fmt.Errorf("GenericServiceClient is nil")
		return
	}

	resp, err = client.Run(ctx, req, grpc.EmptyCallOption{})
	if err != nil {
		return
	}
	common.WriteProtoToStepLog(ctx, step, resp, "run response")

	return
}

// Stop invokces the Stop endpoint of the GenericServiceClient
func (ex *GenericServiceExecutor) Stop(
	ctx context.Context,
	client testapi.GenericServiceClient,
	req *testapi.GenericStopRequest,
) (resp *testapi.GenericStopResponse, err error) {
	step, ctx := build.StartStep(ctx, "Stop")
	defer func() { step.End(err) }()

	if req == nil {
		err = fmt.Errorf("GenericStopRequest is nil")
		return
	}

	if client == nil {
		err = fmt.Errorf("GenericServiceClient is nil")
		return
	}

	resp, err = client.Stop(ctx, req, grpc.EmptyCallOption{})
	if err != nil {
		return
	}
	common.WriteProtoToStepLog(ctx, step, resp, "stop response")

	return
}
