// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package executors defines the base executors type.
package executors

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/analytics"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/commontypes"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/streaming"
	ctpv2_data "go.chromium.org/infra/cros/cmd/ctpv2/data"
	"go.chromium.org/infra/cros/cmd/ctpv2/internal/commands"
)

// FilterExecutor represents executor for all filter related commands.
type FilterExecutor struct {
	*interfaces.AbstractExecutor

	FilterServiceClient testapi.GenericFilterServiceClient
	ContainerInfo       *ctpv2_data.ContainerInfo
}

func NewFilterExecutor() *FilterExecutor {
	absExec := interfaces.NewAbstractExecutor(FilterExecutorType)
	return &FilterExecutor{AbstractExecutor: absExec}
}

func (ex *FilterExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.FilterExecutionCmd:
		key := ""
		if cmd.ContainerInfo != nil && cmd.BQClient != nil {
			key = fmt.Sprintf("%s-execute", cmd.ContainerInfo.GetKey())
			analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, &analytics.BqData{Step: key, Status: analytics.Start}, cmd.InputTestPlan, cmd.BuildState)
		}
		start := time.Now()
		status := analytics.Success

		// Execute the Filter
		err := ex.filterExecutionCommandExecution(ctx, cmd)
		if err != nil {
			status = analytics.Fail
		}
		if key != "" {
			analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, &analytics.BqData{Step: key, Status: status, Duration: float32(time.Since(start).Seconds())}, cmd.InputTestPlan, cmd.BuildState)
		}
		return err

	default:
		return fmt.Errorf(
			"command type %s is not supported by %s executor type",
			cmd.GetCommandType(),
			ex.GetExecutorType())
	}
}

// filterExecutionCommandExecution executes filter execution command.
func (ex *FilterExecutor) filterExecutionCommandExecution(
	ctx context.Context,
	cmd *commands.FilterExecutionCmd) error {

	ex.ContainerInfo = cmd.ContainerInfo

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Filter execution: %s", ex.ContainerInfo.GetKey()))
	defer func() { step.End(err) }()

	common.WriteProtoToStepLog(ctx, step, cmd.InputTestPlan, "filter request")

	fitlerResp, err := ex.ExecuteFilter(ctx, step, cmd, cmd.InputTestPlan)
	if err != nil {
		errorLog := step.Log("Filter Error")
		_, _ = errorLog.Write([]byte(err.Error()))
		return errors.Annotate(err, "Filter execution cmd err: ").Err()
	}

	common.WriteProtoToStepLog(ctx, step, fitlerResp, "filter response")
	cmd.OutputTestPlan = fitlerResp

	return err
}

func executeTestFinderAdaptor(ctx context.Context, conn *grpc.ClientConn, filterReq *testapi.InternalTestplan) (*testapi.InternalTestplan, error) {
	// Create new client.
	TFServiceClient := testapi.NewTestFinderServiceClient(conn)
	if TFServiceClient == nil {
		return nil, fmt.Errorf("filterServiceClient is nil")
	}

	logging.Infof(ctx, "Executing Test-Finder Adaptor")

	// 32MB stream size as the internal proot can get somewhat large.
	maxRecvSizeOption := grpc.MaxCallRecvMsgSize(32 * 10e6)
	maxSendSizeOption := grpc.MaxCallSendMsgSize(32 * 10e6)

	req, _ := common.ToTestFinderRequest(filterReq)

	logging.Infof(ctx, "Custom TF Adaptor Request: %s", req)

	// Call the TF client.
	findTestResp, err := TFServiceClient.FindTests(ctx, req, maxRecvSizeOption, maxSendSizeOption)
	if err != nil {
		err = errors.Annotate(err, "filter grpc execution failure: ").Err()
		// log error but don't return it as we want enumeration error happening for this
		logging.Infof(ctx, err.Error())
		return filterReq, nil
	}

	logging.Infof(ctx, "Backfilling results")
	err = common.FillTestCasesIntoTestPlan(filterReq, findTestResp)
	if err != nil {
		return nil, errors.Annotate(err, "Error in translated TestFinder: ").Err()
	}
	return filterReq, nil
}

// ExecuteFilter invokes the run tests endpoint of cros-test.
func (ex *FilterExecutor) ExecuteFilter(
	ctx context.Context,
	step *build.Step,
	cmd *commands.FilterExecutionCmd,
	filterReq *testapi.InternalTestplan) (resp *testapi.InternalTestplan, err error) {

	if filterReq == nil {
		return nil, fmt.Errorf("cannot execute filter for nil filter request")
	}
	if ex.ContainerInfo == nil {
		return nil, fmt.Errorf("cannot execute filter with nil container info")
	}

	responseChannel := make(chan *commontypes.ContainerManagementResponse)
	containerRequest := commontypes.ContainerManagementRequest{
		Container:            cmd.ContainerInfo.Request,
		ContainerInstruction: commontypes.ProvideContainer,
		ResponseChannel:      responseChannel,
	}
	cmd.ContainerRequestChannel <- containerRequest
	response := <-responseChannel
	if response == nil || response.Address == nil {
		return nil, fmt.Errorf("error while getting filter endpoint, found nil")
	}
	defer func() {
		cmd.ContainerLogsChannel <- &commontypes.ContainerLogInfo{
			Name:        cmd.ContainerInfo.Request.DynamicIdentifier,
			LogLocation: response.LogLocation,
		}
	}()

	filterEndpointStr := fmt.Sprintf("%s:%d", response.Address.GetAddress(), response.Address.GetPort())
	defer func() {
		containerRequest.ContainerInstruction = commontypes.FinishedUsingContainer
		cmd.ContainerRequestChannel <- containerRequest
		<-responseChannel
	}()

	// Connect with the filter service.
	conn, err := common.ConnectWithService(ctx, filterEndpointStr)
	if err != nil {
		logging.Infof(
			ctx,
			"error during connecting with filter server at %s: %s",
			filterEndpointStr,
			err.Error())
		return nil, err
	}
	logging.Infof(ctx, "connected with filter service")

	filter := ex.ContainerInfo.Request.GetContainer().GetContainer().(*testapi.Template_Generic)
	// Create new client.
	filterServiceClient := testapi.NewGenericFilterServiceClient(conn)
	if filterServiceClient == nil {
		return nil, fmt.Errorf("filterServiceClient is nil")
	}
	defer func() {
		if err != nil && filter.Generic.GetBinaryName() == "cros-test-finder" {
			logging.Infof(ctx, "Encountered error when executing cros-test-finder. Falling back to adaptor execution.")
			resp, err = executeTestFinderAdaptor(ctx, conn, filterReq)
			if err != nil {
				err = errors.Annotate(err, "test finder adaptor filter err: ").Err()
			}
			logging.Infof(ctx, "Filter Adaptor Success")
		}
	}()
	stream, streamErr := filterServiceClient.ExecuteWithStream(ctx)
	if streamErr != nil {
		err = streamErr
		logging.Infof(ctx, "ExecuteWithStream returned error: %s", err)
		return
	}

	serverCommuncationHandler := streaming.NewServerCommunicationHandler(stream)
	defer serverCommuncationHandler.Close()
	go serverCommuncationHandler.HandleStreamFromServer()
	go serverCommuncationHandler.HandleStreamToServer()
	go serverCommuncationHandler.StreamLogsToWriter(step.Log("Filter Logs"))
	go serverCommuncationHandler.HandleAuthorizationRequests(ctx)

	err = serverCommuncationHandler.SendInternalTestplan(filterReq)
	if err != nil {
		logging.Infof(ctx, "Failed to send test plan: %s", err)
		return
	}
	resp, err = serverCommuncationHandler.GetInternalTestplan()

	return
}
