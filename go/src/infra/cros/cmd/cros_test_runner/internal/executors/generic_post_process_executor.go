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
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/internal/commands"
)

// GenericPostProcessExecutor represents executor for all post process related
// commands.
type GenericPostProcessExecutor struct {
	*interfaces.AbstractExecutor
}

// NewGenericPostProcessExecutor creates a new generic post process executor.
func NewGenericPostProcessExecutor() *GenericPostProcessExecutor {
	absExec := interfaces.NewAbstractExecutor(GenericPostProcessExecutorType)
	return &GenericPostProcessExecutor{AbstractExecutor: absExec}
}

func (ex *GenericPostProcessExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {
	switch cmd := cmdInterface.(type) {
	case *commands.GenericPostProcessCmd:
		return ex.GenericPostProcessHandler(ctx, cmd)
	default:
		return fmt.Errorf(
			"Command type: %s is not supported by: %s executor type!",
			cmd.GetCommandType(),
			ex.GetExecutorType())
	}
}

// GenericPostProcessHandler executes the post process commands.
func (ex *GenericPostProcessExecutor) GenericPostProcessHandler(
	ctx context.Context,
	cmd *commands.GenericPostProcessCmd) (err error) {
	stepName := "Post process service"
	postProcessReq := cmd.PostProcessRequest
	if postProcessReq.DynamicIdentifier != "" {
		stepName = fmt.Sprintf("%s: %s", stepName, postProcessReq.DynamicIdentifier)
	}
	step, ctx := build.StartStep(ctx, stepName)
	defer func() { step.End(err) }()

	common.WriteProtoToStepLog(ctx, step, postProcessReq, "post process service request")

	client, err := ex.ConnectToService(ctx, postProcessReq.GetServiceAddress())
	if err != nil {
		return errors.Annotate(err, "connecting to post process service").Err()
	}

	startUpResp, err := ex.StartUp(ctx, client, postProcessReq.GetStartUpRequest())
	if err != nil {
		return errors.Annotate(err, "StartUp cmd").Err()
	}
	cmd.StartUpResp = startUpResp
	common.WriteProtoToStepLog(ctx, step, startUpResp, "StartUp cmd response")

	runActivitiesResp, err := ex.RunActivities(ctx, client, postProcessReq.GetRunActivitiesRequest())
	if err != nil {
		return errors.Annotate(err, "RunActivities cmd").Err()
	}
	cmd.RunActivitiesResp = runActivitiesResp
	common.WriteProtoToStepLog(ctx, step, runActivitiesResp, "RunActivities cmd response")

	return nil
}

// ConnectToService connects to the GenericPostProcessService attached to the
// server address.
func (ex *GenericPostProcessExecutor) ConnectToService(
	ctx context.Context,
	endpoint *labapi.IpEndpoint) (testapi.PostTestServiceClient, error) {
	var err error
	step, ctx := build.StartStep(ctx, "Establish Connection")
	defer func() { step.End(err) }()

	// Connect with the service.
	address := common.GetServerAddress(endpoint)
	conn, err := common.ConnectWithService(ctx, address)
	if err != nil {
		logging.Infof(
			ctx,
			"error during connecting with post process server at: %s: %s",
			address,
			err.Error())
		return nil, err
	}
	logging.Infof(ctx, "Connected with post process service.")

	// Create new client.
	genericPostProcessClient := testapi.NewPostTestServiceClient(conn)
	if genericPostProcessClient == nil {
		err = fmt.Errorf("PostProcessServiceClient is nil")
		return nil, err
	}

	return genericPostProcessClient, err
}

// StartUp invokes the StartUp endpoint of the PostProcessServiceClient.
func (ex *GenericPostProcessExecutor) StartUp(
	ctx context.Context,
	client testapi.PostTestServiceClient,
	req *testapi.PostTestStartUpRequest,
) (resp *testapi.PostTestStartUpResponse, err error) {
	step, ctx := build.StartStep(ctx, "Start up")
	defer func() { step.End(err) }()

	if req == nil {
		err = fmt.Errorf("PostTestStartUpRequest is nil")
		return
	}

	if client == nil {
		err = fmt.Errorf("PostTestServiceClient is nil")
		return
	}

	common.WriteProtoToStepLog(ctx, step, req, "Start up request")
	resp, err = client.StartUp(ctx, req, grpc.EmptyCallOption{})
	if err != nil {
		err = errors.Annotate(err, "Start up failure: ").Err()
		return
	}
	common.WriteProtoToStepLog(ctx, step, resp, "Start up response")

	step.SetSummaryMarkdown(fmt.Sprintf("Startup status: %s", resp.GetStatus().String()))
	step.AddTagValue("startup_status", resp.GetStatus().String())

	if resp.GetStatus() != testapi.PostTestStartUpResponse_STATUS_SUCCESS {
		err = fmt.Errorf("Post process Startup failure: %s", resp.GetStatus().String())
		return
	}
	return
}

// RunActivities invokes the RunActivities endpoint of the
// PostProcessServiceClient.
func (ex *GenericPostProcessExecutor) RunActivities(
	ctx context.Context,
	client testapi.PostTestServiceClient,
	req *testapi.RunActivitiesRequest,
) (resp *testapi.RunActivitiesResponse, err error) {
	step, ctx := build.StartStep(ctx, "Run activities")
	defer func() { step.End(err) }()

	if req == nil {
		err = fmt.Errorf("RunActivitiesRequest is nil")
		return
	}

	if client == nil {
		err = fmt.Errorf("PostTestServiceClient is nil")
		return
	}

	common.WriteProtoToStepLog(ctx, step, req, "RunActivities request")
	resp, err = client.RunActivities(ctx, req, grpc.EmptyCallOption{})
	if err != nil {
		err = errors.Annotate(err, "RunActivities failure: ").Err()
		return
	}
	common.WriteProtoToStepLog(ctx, step, resp, "RunActivities response")

	return
}
