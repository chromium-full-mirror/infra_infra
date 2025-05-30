// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package executors defines the base executors type.
package executors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/civil"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/analytics"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/commontypes"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/streaming"
	"go.chromium.org/infra/cros/cmd/ctpv2/internal/commands"
)

// FilterExecutor represents executor for all filter related commands.
type FilterExecutor struct {
	*interfaces.AbstractExecutor

	FilterServiceClient testapi.GenericFilterServiceClient
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
		if cmd.Filter != nil && cmd.BQClient != nil {
			key = fmt.Sprintf("%s-execute", cmd.Filter.GetContainerInfo().GetContainer().GetName())
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

	var err error
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Filter execution: %s", cmd.Filter.GetContainerInfo().GetContainer().GetName()))
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

func CtpFilterToContainerRequest(ctx context.Context, cmd *commands.FilterExecutionCmd) (*testapi.ContainerRequest, error) {
	var containerRequest *testapi.ContainerRequest
	var err error
	// If Filter is already populated, don't fetch anything, just create the container Request.
	if cmd.Filter.GetContainerInfo().GetContainer().GetRepository().GetHostname() != "" {
		if cmd.Filter.GetContainerInfo().GetBinaryName() == "" {
			cmd.Filter.GetContainerInfo().BinaryName = cmd.Filter.GetContainerInfo().GetContainer().GetName()
		}
		containerRequest = common.CreateContainerRequest(cmd.Filter)
		return containerRequest, err
	}

	var filter *testapi.CTPFilter
	filterName := cmd.Filter.GetContainerInfo().GetContainer().GetName()
	// Special logic for cros-test-finder. Continue to pull down from the container MD.
	if filterName == common.TestFinderContainerName {
		filter, err = FetchCrosTestFinder(ctx, cmd)
		if err != nil {
			return containerRequest, err
		}
	} else {
		// Fetch the container info from firestore.
		filter, err = common.FetchFilterFromFirestore(ctx, cmd.FirestoreDB, cmd.CTPversion, filterName, option.WithCredentialsFile(cmd.Creds))
		if err != nil {
			return containerRequest, err
		}
	}

	containerRequest = common.CreateContainerRequest(filter)
	return containerRequest, err
}

func FetchCrosTestFinder(ctx context.Context, cmd *commands.FilterExecutionCmd) (*testapi.CTPFilter, error) {
	board, gcsPath, err := common.GcsInfo(cmd.CtpReq)
	if err != nil {
		return nil, err
	}

	buildContainerMetadata, err := common.FetchImageData(ctx, board, gcsPath)
	if err != nil {
		logging.Infof(ctx, fmt.Sprintf("failed to fetch container image data from %s, will continue without build containers. err: %s", gcsPath, err))
		return nil, errors.Annotate(err, "failed to fetch container image data: ").Err()
	}

	tf, ok := buildContainerMetadata[common.TestFinderContainerName]
	if !ok {
		return nil, fmt.Errorf("could not find %s in build container metadata", common.TestFinderContainerName)
	}

	return &testapi.CTPFilter{
		ContainerInfo: &testapi.ContainerInfo{
			Container: &buildapi.ContainerImageInfo{
				Name:       tf.GetName(),
				Repository: tf.GetRepository(),
				Digest:     tf.GetDigest(),
				Tags:       tf.GetTags(),
			},
			BinaryName: tf.GetName(),
			BinaryArgs: []string{},
		},
	}, nil
}

func (ex *FilterExecutor) getContainer(ctx context.Context, cmd *commands.FilterExecutionCmd) (*commontypes.ContainerManagementResponse, func(), error) {
	container, err := CtpFilterToContainerRequest(ctx, cmd)
	if err != nil {
		return nil, nil, err
	}
	responseChannel := make(chan *commontypes.ContainerManagementResponse)
	containerRequest := commontypes.ContainerManagementRequest{
		Container:            container,
		ContainerInstruction: commontypes.ProvideContainer,
		ResponseChannel:      responseChannel,
	}
	cmd.ContainerRequestChannel <- containerRequest
	response := <-responseChannel
	if response == nil || response.Address == nil {
		return nil, nil, fmt.Errorf("error while getting filter endpoint, found nil")
	}
	go func() {
		cmd.ContainerLogsChannel <- &commontypes.ContainerLogInfo{
			Name:        container.DynamicIdentifier,
			LogLocation: response.LogLocation,
		}
	}()

	return response, func() {
		containerRequest.ContainerInstruction = commontypes.FinishedUsingContainer
		cmd.ContainerRequestChannel <- containerRequest
		<-responseChannel
	}, nil
}

func (ex *FilterExecutor) callExecute(ctx context.Context, cmd *commands.FilterExecutionCmd, conn *grpc.ClientConn, step *build.Step, filterReq *testapi.InternalTestplan) (*testapi.InternalTestplan, error) {
	var resp *testapi.InternalTestplan
	var err error
	// Create new client.
	filterServiceClient := testapi.NewGenericFilterServiceClient(conn)
	if filterServiceClient == nil {
		err = fmt.Errorf("filterServiceClient is nil")
		return nil, err
	}
	defer func() {
		if err != nil && cmd.Filter.GetContainerInfo().GetContainer().GetName() == common.TestFinderContainerName {
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
		return nil, err
	}

	serverCommuncationHandler := streaming.NewServerCommunicationHandler(stream)
	defer serverCommuncationHandler.Close()
	go serverCommuncationHandler.HandleStreamFromServer()
	go serverCommuncationHandler.HandleStreamToServer()
	go serverCommuncationHandler.StreamLogsToWriter(step.Log("Filter Logs"))
	go serverCommuncationHandler.HandleAuthorizationRequests(ctx)

	err = serverCommuncationHandler.SendArgs(cmd.Filter.GetContainerInfo().GetBinaryArgs())

	err = serverCommuncationHandler.SendInternalTestplan(filterReq)
	if err != nil {
		logging.Infof(ctx, "Failed to send test plan: %s", err)
		return nil, err
	}
	resp, err = serverCommuncationHandler.GetInternalTestplan()
	// Send empty testplan as ack that client is done.
	serverCommuncationHandler.SendInternalTestplan(&testapi.InternalTestplan{})

	return resp, err
}

func (ex *FilterExecutor) executeFilterCloudRun(ctx context.Context, cmd *commands.FilterExecutionCmd, step *build.Step, filterReq *testapi.InternalTestplan, serviceName, serviceTag string, isPartnerRun bool) (*testapi.InternalTestplan, error) {
	var resp *testapi.InternalTestplan
	var err error
	// Cloud run disallows underscores. Replace with dashes.
	serviceName = strings.ReplaceAll(serviceName, "_", "-")
	// Tagged revisions use a specific endpoint found on the staging project.
	filterEndpointStr := fmt.Sprintf("%s---%s%s", serviceTag, serviceName, common.TaggedFilterEndpointSuffix)
	audienceEndpointStr := common.StagingFilterEndpointSuffix
	if isPartnerRun {
		filterEndpointStr = fmt.Sprintf("%s---%s%s", serviceTag, serviceName, common.PartnerTaggedFilterEndpointSuffix)
		audienceEndpointStr = common.PartnerFilterEndpointSuffix
	}
	switch serviceTag {
	case common.LabelProd:
		filterEndpointStr = fmt.Sprintf("%s%s", serviceName, common.ProdFilterEndpointSuffix)
		audienceEndpointStr = common.ProdFilterEndpointSuffix
	case common.LabelStaging:
		filterEndpointStr = fmt.Sprintf("%s%s", serviceName, common.StagingFilterEndpointSuffix)
	case common.LabelPartner:
		filterEndpointStr = fmt.Sprintf("%s%s", serviceName, common.PartnerFilterEndpointSuffix)
		audienceEndpointStr = common.PartnerFilterEndpointSuffix
	}
	// Audience must point at base service endpoint, even for tagged revisions.
	audience := fmt.Sprintf("https://%s%s", serviceName, audienceEndpointStr)
	conn, err := common.ConnectWithCloudService(ctx, &labapi.IpEndpoint{
		Address: filterEndpointStr,
		Port:    common.FilterCloudRunPortInt,
	}, audience)
	if err != nil {
		logging.Infof(ctx, "error during connecting with filter server at %s: %s", filterEndpointStr, err.Error())
		return nil, err
	}
	logging.Infof(ctx, "connected with filter service")
	resp, err = ex.callExecute(ctx, cmd, conn, step, filterReq)
	return resp, err
}

// ExecuteFilter invokes the run tests endpoint of cros-test.
func (ex *FilterExecutor) ExecuteFilter(
	ctx context.Context,
	step *build.Step,
	cmd *commands.FilterExecutionCmd,
	filterReq *testapi.InternalTestplan) (*testapi.InternalTestplan, error) {

	var resp *testapi.InternalTestplan
	var err error

	if filterReq == nil {
		return nil, fmt.Errorf("cannot execute filter for nil filter request")
	}
	if cmd.Filter == nil {
		return nil, fmt.Errorf("cannot execute filter with nil filter info")
	}

	startTime := time.Now()
	serviceName := cmd.Filter.GetContainerInfo().GetContainer().GetName()
	disallowCloudRun := false
	usedFallback := false
	defer func() {
		filterDuration := time.Since(startTime)
		success := err == nil
		ObserveFilterData(ctx, cmd, serviceName, filterDuration.Seconds(), !disallowCloudRun, usedFallback, success)
	}()

	// Disallow cros-test-finder.
	if serviceName == common.TestFinderContainerName {
		disallowCloudRun = true
	}
	// Disallow filters with info needed for local container execution.
	if cmd.Filter.GetContainerInfo().GetContainer().GetRepository().GetHostname() != "" {
		disallowCloudRun = true
	}
	if !disallowCloudRun && cmd.CloudRunEnabled {
		serviceTag := common.LabelStaging
		if cmd.CTPversion == common.LabelProd {
			serviceTag = common.LabelProd
		}
		if cmd.IsPartnerRun {
			serviceTag = common.LabelPartner
		}
		// If image info contains tags, hit the tagged revision in cloud run.
		tags := cmd.Filter.GetContainerInfo().GetContainer().GetTags()
		if len(tags) == 1 {
			serviceTag = tags[0]
		}
		// Try to execute with the cloud run instance.
		resp, err = ex.executeFilterCloudRun(ctx, cmd, step, filterReq, serviceName, serviceTag, cmd.IsPartnerRun)
		// If no error found, return. Else will retry with the local container.
		if err == nil {
			return nil, err
		}
		logging.Infof(ctx, "Found Error: %s", err)

		// TODO(cdelagarza): Once rollout experiment for cloud run is finished,
		// remove the fallback option.
		logging.Infof(ctx, "Falling back to local execution")
		usedFallback = true
	}

	response, closer, err := ex.getContainer(ctx, cmd)
	if err != nil {
		return nil, err
	}
	defer closer()
	filterEndpointStr := fmt.Sprintf("%s:%d", response.Address.GetAddress(), response.Address.GetPort())
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

	resp, err = ex.callExecute(ctx, cmd, conn, step, filterReq)

	return resp, err
}

// ObserveFilterData writes analytics data about the filter execution.
//
// TODO(cdelagarza): Remove once cloud run experiment has concluded.
func ObserveFilterData(ctx context.Context, cmd *commands.FilterExecutionCmd, filterName string, duration float64, expirementEnabled, usedFallback, success bool) {
	data := &analytics.CloudRunExperimentData{
		Date:              civil.DateTimeOf(time.Now()),
		ExperimentEnabled: expirementEnabled,
		FilterName:        filterName,
		Duration:          duration,
		SuiteName:         cmd.InputTestPlan.GetSuiteInfo().GetSuiteRequest().GetTestSuite().GetName(),
		UsedFallback:      usedFallback,
		Success:           success,
	}
	analytics.SoftInsertCloudRunExperimentFilterData(ctx, cmd.BQClient, data, cmd.BuildState)
}
