// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/analytics"
	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	ctpv2_data "infra/cros/cmd/ctpv2/data"
	"infra/cros/cmd/ctpv2/internal/commands"
	"infra/libs/skylab/inventory/autotest/labels"
	s "infra/libs/skylab/inventory/swarming"
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
			analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, &analytics.BqData{Step: key, Status: status, Duration: float32(time.Now().Sub(start).Seconds())}, cmd.InputTestPlan, cmd.BuildState)
		}
		return err

	default:
		return fmt.Errorf(
			"Command type %s is not supported by %s executor type!",
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

	fitlerResp, err := ex.ExecuteFilter(ctx, cmd.InputTestPlan)
	if err != nil {
		return errors.Annotate(err, "Filter execution cmd err: ").Err()
	}

	common.WriteProtoToStepLog(ctx, step, fitlerResp, "filter response")
	cmd.OutputTestPlan = fitlerResp

	return err
}

func executeTestFinderAdaptor(ctx context.Context, conn *grpc.ClientConn, filterReq *testapi.InternalTestplan) (*testapi.InternalTestplan, error) {
	// Create new client.
	TFServiceClient := api.NewTestFinderServiceClient(conn)
	if TFServiceClient == nil {
		return nil, fmt.Errorf("filterServiceClient is nil")
	}

	logging.Infof(ctx, "Executing Test-Finder Adaptor")

	// 32MB stream size as the internal proot can get somewhat large.
	maxRecvSizeOption := grpc.MaxCallRecvMsgSize(32 * 10e6)
	maxSendSizeOption := grpc.MaxCallSendMsgSize(32 * 10e6)

	req, _ := toTestFinderRequest(filterReq)

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
	err = fillTestCasesIntoTestPlan(ctx, filterReq, findTestResp)
	if err != nil {
		return nil, errors.Annotate(err, "Error in translated TestFinder: ").Err()
	}
	return filterReq, nil
}

func toTestFinderRequest(testPlan *api.InternalTestplan) (*api.CrosTestFinderRequest, error) {
	centralizedSuitesPrefix := "centralizedsuite:"
	// TODO... switch
	requestedSuite, ok := testPlan.GetSuiteInfo().GetSuiteRequest().GetSuiteRequest().(*api.SuiteRequest_TestSuite)
	if !ok {
		return nil, errors.New("SuiteRequest is not TestSuite")
	}
	testSuite := requestedSuite.TestSuite
	if testSuite != nil && strings.HasPrefix(testSuite.Name, centralizedSuitesPrefix) {
		return &api.CrosTestFinderRequest{
			CentralizedSuite: strings.TrimPrefix(testSuite.Name, centralizedSuitesPrefix),
			MetadataRequired: true,
		}, nil
	}
	return &api.CrosTestFinderRequest{
		TestSuites:       []*api.TestSuite{testSuite},
		MetadataRequired: true,
	}, nil
}

func fillTestCasesIntoTestPlan(ctx context.Context, testPlan *api.InternalTestplan, resp *api.CrosTestFinderResponse) error {
	if len(resp.GetTestSuites()) == 0 {
		return nil
	}

	// Only need to check the [0] index; as test-finder only populates that.
	metadataList, ok := resp.GetTestSuites()[0].Spec.(*api.TestSuite_TestCasesMetadata)
	if !ok {
		return errors.New("no test cases metadata in the response")
	}

	for _, metadata := range metadataList.TestCasesMetadata.GetValues() {
		testPlan.TestCases = append(testPlan.TestCases, tfToCTPTestCase(metadata))
	}
	return nil
}

func tfToCTPTestCase(metadata *api.TestCaseMetadata) *api.CTPTestCase {
	tc := &api.CTPTestCase{
		Name:     metadata.GetTestCase().GetId().GetValue(),
		Metadata: metadata,
	}

	deps := Converter(tc.GetMetadata().GetTestCase().GetDependencies())
	if len(deps) != 0 {
		tc.Metadata.TestCase.Dependencies = deps
	}
	return tc
}

func Converter(deps []*api.TestCase_Dependency) []*api.TestCase_Dependency {
	convertedDeps := []string{}
	for _, dep := range deps {
		f := dep.GetValue()
		converted := convertDep(f)
		// If the dep can't be converted, let it flow through naturally. Bot params should handel the case where its invalid
		if len(converted) == 0 {
			convertedDeps = append(convertedDeps, f)
		} else {
			convertedDeps = append(convertedDeps, converted...)
		}
	}
	finalDeps := []*api.TestCase_Dependency{}
	for _, dep := range convertedDeps {
		tcD := &api.TestCase_Dependency{
			Value: dep,
		}
		finalDeps = append(finalDeps, tcD)
	}
	return finalDeps
}

func convertDep(dep string) []string {
	deps := []string{dep}
	parsedDeps := labels.Revert(deps)

	depsf := []string{}
	for k, v := range s.Convert(parsedDeps) {
		for _, innerv := range v {
			depsf = append(depsf, fmt.Sprintf("%s:%s", k, innerv))

		}
	}
	return depsf
}

// ExecuteTests invokes the run tests endpoint of cros-test.
func (ex *FilterExecutor) ExecuteFilter(
	ctx context.Context,
	filterReq *testapi.InternalTestplan) (*testapi.InternalTestplan, error) {

	if filterReq == nil {
		return nil, fmt.Errorf("Cannot execute filter for nil filter request.")
	}
	if ex.ContainerInfo == nil {
		return nil, fmt.Errorf("Cannot execute filter with nil container info.")
	}
	if ex.ContainerInfo.ServiceEndpoint == nil {
		return nil, fmt.Errorf("Cannot execute filter for nil service endpoint.")
	}

	filterEndpointStr, err := ex.ContainerInfo.GetEndpointString()
	if err != nil {
		return nil, errors.Annotate(err, "error while getting filter endpoint str: ").Err()
	}

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
	logging.Infof(ctx, "Connected with filter service.")

	// If the filter is test-finder, build a test-finder command and run that instead. Translate both ways.
	// This is to ensure full backwards compatibility with everything including LTS.
	filter := ex.ContainerInfo.Request.GetContainer().GetContainer().(*testapi.Template_Generic)
	if filter.Generic.GetBinaryName() == "cros-test-finder" {
		filterResp, err := executeTestFinderAdaptor(ctx, conn, filterReq)
		if err != nil {
			return nil, errors.Annotate(err, "test finder adaptor filter err: ").Err()
		}
		logging.Infof(ctx, "Filter Adaptor Success?")
		return filterResp, nil
	}

	// Create new client.
	filterServiceClient := api.NewGenericFilterServiceClient(conn)
	if filterServiceClient == nil {
		return nil, fmt.Errorf("filterServiceClient is nil")
	}
	maxRecvSizeOption := grpc.MaxCallRecvMsgSize(32 * 10e6)
	maxSendSizeOption := grpc.MaxCallSendMsgSize(32 * 10e6)
	// Call filter grpc endpoint
	findTestResp, err := filterServiceClient.Execute(ctx, filterReq, maxRecvSizeOption, maxSendSizeOption)
	if err != nil {
		return nil, errors.Annotate(err, "filter grpc execution failure: ").Err()
	}

	return findTestResp, nil
}
