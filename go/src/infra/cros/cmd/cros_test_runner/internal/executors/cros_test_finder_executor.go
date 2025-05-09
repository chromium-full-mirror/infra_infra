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

// CrosTestFinderExecutor represents executor for all cros-test-finder related commands.
type CrosTestFinderExecutor struct {
	*interfaces.AbstractExecutor

	Container                   interfaces.ContainerInterface
	CrosTestFinderServiceClient testapi.TestFinderServiceClient
	ServerAddress               string
}

func NewCrosTestFinderExecutor(container interfaces.ContainerInterface) *CrosTestFinderExecutor {
	absExec := interfaces.NewAbstractExecutor(CrosTestFinderExecutorType)
	return &CrosTestFinderExecutor{AbstractExecutor: absExec, Container: container}
}

func (ex *CrosTestFinderExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.TestFinderServiceStartCmd:
		return ex.testFinderStartCommandExecution(ctx, cmd)
	case *commands.TestFinderExecutionCmd:
		return ex.testFinderExecutionCommandExecution(ctx, cmd)
	default:
		return fmt.Errorf(
			"Command type %s is not supported by %s executor type!",
			cmd.GetCommandType(),
			ex.GetExecutorType())
	}
}

// testStartCommandExecution executes the test server start command.
func (ex *CrosTestFinderExecutor) testFinderStartCommandExecution(
	ctx context.Context,
	cmd *commands.TestFinderServiceStartCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Test Finder service start")
	defer func() { step.End(err) }()

	err = ex.Start(ctx)
	logErr := common.WriteContainerLogToStepLog(ctx, ex.Container, step, "cros-test-finder log")
	if err != nil {
		return errors.Annotate(err, "Start test finder service cmd err: ").Err()
	}
	if logErr != nil {
		logging.Infof(ctx, "error during writing cros-test-finder log contents: %s", err)
	}

	return err
}

// testExecutionCommandExecution executes the test execution command.
func (ex *CrosTestFinderExecutor) testFinderExecutionCommandExecution(
	ctx context.Context,
	cmd *commands.TestFinderExecutionCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Test Finder execution")
	defer func() { step.End(err) }()

	testSuites := []*testapi.TestSuite{}
	var tags []string = nil
	var tagsExclude []string = nil
	if len(cmd.Tags) > 0 && cmd.Tags[0] != "" {
		tags = cmd.Tags
	}
	if len(cmd.TagsExclude) > 0 && cmd.TagsExclude[0] != "" {
		tagsExclude = cmd.TagsExclude
	}
	if tags != nil || tagsExclude != nil {
		testSuites = append(testSuites, &testapi.TestSuite{
			Spec: &testapi.TestSuite_TestCaseTagCriteria_{
				TestCaseTagCriteria: &testapi.TestSuite_TestCaseTagCriteria{
					Tags:        tags,
					TagExcludes: tagsExclude,
				},
			},
		})
	}

	testCaseIds := []*testapi.TestCase_Id{}
	for _, testCaseId := range cmd.Tests {
		if testCaseId != "" {
			testCaseIds = append(testCaseIds, &testapi.TestCase_Id{
				Value: testCaseId,
			})
		}
	}

	if len(testCaseIds) > 0 {
		testSuites = append(testSuites, &testapi.TestSuite{
			Spec: &testapi.TestSuite_TestCaseIds{
				TestCaseIds: &testapi.TestCaseIdList{
					TestCaseIds: testCaseIds,
				},
			},
		})
	}

	testReq := &testapi.CrosTestFinderRequest{
		TestSuites: testSuites,
	}

	common.WriteProtoToStepLog(ctx, step, testReq, "test finder request")

	testResp, err := ex.FindTests(ctx, testReq)

	if err != nil {
		return errors.Annotate(err, "Tests execution cmd err: ").Err()
	}
	cmd.TestSuites = testResp.TestSuites

	common.WriteProtoToStepLog(ctx, step, testResp, "test finder response")

	return err
}

// Start starts the cros-test-finder server.
func (ex *CrosTestFinderExecutor) Start(ctx context.Context) error {
	template := &testapi.Template{
		Container: &testapi.Template_CrosTestFinder{
			CrosTestFinder: &testapi.CrosTestFinderTemplate{},
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
			"error during connecting with cros-test-finder server at %s: %s",
			serverAddress,
			err.Error())
		return err
	}
	logging.Infof(ctx, "Connected with cros-test-finder service.")

	// Create new client.
	testClient := testapi.NewTestFinderServiceClient(conn)
	if testClient == nil {
		return fmt.Errorf("testFinderServiceClient is nil")
	}

	ex.CrosTestFinderServiceClient = testClient

	return nil
}

// ExecuteTests invokes the run tests endpoint of cros-test.
func (ex *CrosTestFinderExecutor) FindTests(
	ctx context.Context,
	testReq *testapi.CrosTestFinderRequest) (*testapi.CrosTestFinderResponse, error) {
	if testReq == nil {
		return nil, fmt.Errorf("Cannot find tests for nil test finder request.")
	}
	if ex.CrosTestFinderServiceClient == nil {
		return nil, fmt.Errorf("CrosTestFinderServiceClient is nil in CrosTestFinderExecutor")
	}
	findTestResp, err := ex.CrosTestFinderServiceClient.FindTests(ctx, testReq, grpc.EmptyCallOption{})
	if err != nil {
		return nil, errors.Annotate(err, "test execution failure: ").Err()
	}

	return findTestResp, nil
}
