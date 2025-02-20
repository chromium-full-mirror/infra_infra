// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
)

// CrosTestExecutor represents executor for all cros-test related commands.
type CrosTestExecutor struct {
	*interfaces.AbstractExecutor

	Container             interfaces.ContainerInterface
	CrosTestServiceClient testapi.ExecutionServiceClient
	ServerAddress         string
}

func NewCrosTestExecutor(container interfaces.ContainerInterface) *CrosTestExecutor {
	absExec := interfaces.NewAbstractExecutor(CrosTestExecutorType)
	return &CrosTestExecutor{AbstractExecutor: absExec, Container: container}
}

func (ex *CrosTestExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.TestServiceStartCmd:
		return ex.testStartCommandExecution(ctx, cmd)
	case *commands.TestsExecutionCmd:
		return ex.testExecutionCommandExecution(ctx, cmd)
	default:
		return fmt.Errorf(
			"Command type %s is not supported by %s executor type!",
			cmd.GetCommandType(),
			ex.GetExecutorType())
	}
}

// testStartCommandExecution executes the test server start command.
func (ex *CrosTestExecutor) testStartCommandExecution(
	ctx context.Context,
	cmd *commands.TestServiceStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Test service start")
	defer func() { step.End(err) }()

	err = ex.Start(ctx)
	logErr := common.WriteContainerLogToStepLog(ctx, ex.Container, step, "cros-test log")
	if err != nil {
		return errors.Annotate(err, "Start test service cmd err: ").Err()
	}
	if logErr != nil {
		logging.Infof(ctx, "error during writing cros-test log contents: %s", err)
	}

	return err
}

// testExecutionCommandExecution executes the test execution command.
func (ex *CrosTestExecutor) testExecutionCommandExecution(
	ctx context.Context,
	cmd *commands.TestsExecutionCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Tests execution")
	defer func() { step.End(err) }()

	var metadata *anypb.Any
	if cmd.TestArgs != nil {
		metadata, _ = anypb.New(cmd.TestArgs)
	} else {
		metadata, _ = anypb.New(cmd.TastArgs)
	}

	testReq := &testapi.CrosTestRequest{
		TestSuites: cmd.TestSuites,
		Primary:    cmd.PrimaryDevice,
		Companions: cmd.CompanionDevices,
		Metadata:   metadata}

	common.WriteProtoToStepLog(ctx, step, testReq, "test request")

	logsLoc, err := ex.Container.GetLogsLocation()
	if err != nil {
		logging.Infof(ctx, "error during getting container log location: %s", err)
		return err
	}
	containerLog := step.Log("Cros-test Log")

	taskDone, wg, err := common.StreamLogAsync(ctx, logsLoc, containerLog)
	if err != nil {
		logging.Infof(ctx, "Warning: error during reading container log: %s", err)
	}

	testResp, err := ex.ExecuteTests(ctx, testReq)
	if taskDone != nil {
		taskDone <- true // Notify logging process that main task is done
	}
	wg.Wait() // wait for the logging to complete

	if err != nil {
		err = errors.Annotate(err, "Tests execution cmd err: ").Err()
	}
	cmd.TestResponses = testResp
	cmd.TkoPublishSrcDir = filepath.Join(logsLoc, "cros-test")

	common.WriteProtoToStepLog(ctx, step, testResp, "test response")

	return err
}

// Start starts the cros-test server.
func (ex *CrosTestExecutor) Start(ctx context.Context) error {
	template := &api.Template{Container: &api.Template_CrosTest{
		CrosTest: &testapi.CrosTestTemplate{},
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
			"error during connecting with cros-test server at %s: %s",
			serverAddress,
			err.Error())
		return err
	}
	logging.Infof(ctx, "Connected with cros-test service.")

	// Create new client.
	testClient := api.NewExecutionServiceClient(conn)
	if testClient == nil {
		return fmt.Errorf("testServiceClient is nil")
	}

	ex.CrosTestServiceClient = testClient

	return nil
}

// ExecuteTests invokes the run tests endpoint of cros-test.
func (ex *CrosTestExecutor) ExecuteTests(
	ctx context.Context,
	testReq *testapi.CrosTestRequest) (*testapi.CrosTestResponse, error) {
	if testReq == nil {
		return nil, fmt.Errorf("Cannot execute tests for nil test request.")
	}
	if ex.CrosTestServiceClient == nil {
		return nil, fmt.Errorf("CrosTestServiceClient is nil in CrosTestExecutor")
	}
	testExecOp, err := ex.CrosTestServiceClient.RunTests(ctx, testReq, grpc.EmptyCallOption{})
	if err != nil {
		return nil, errors.Annotate(err, "test execution failure: ").Err()
	}

	// TODO: Fix this to use common.WaitLro
	opResp, err := common.ProcessDoneLro(ctx, testExecOp)
	if err != nil {
		return nil, errors.Annotate(err, "test execution lro failure: ").Err()
	}

	testResp := &testapi.CrosTestResponse{}

	if err := opResp.UnmarshalTo(testResp); err != nil {
		logging.Infof(ctx, "test execution lro response unmarshalling failed: %s", err.Error())
		return nil, errors.Annotate(err, "test execution lro response unmarshalling failed: ").Err()
	}

	return testResp, nil
}
