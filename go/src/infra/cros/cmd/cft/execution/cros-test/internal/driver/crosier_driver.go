// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/common/errors"
	"go.chromium.org/infra/cros/cmd/cft/execution/cros-test/internal/device"
)

const (
	testInitScriptPath = "/usr/local/libexec/crosier/init_env.sh"
	testRunScriptPath  = "/usr/local/libexec/crosier/run_tests.sh"
)

// CrosierDriver runs gtest and report its results.
type CrosierDriver struct {
	// logger provides logging service.
	logger *log.Logger
}

// NewCrosierDriver creates a new driver to run tests.
func NewCrosierDriver(logger *log.Logger) *CrosierDriver {
	return &CrosierDriver{
		logger: logger,
	}
}

// Name returns the name of the driver.
func (td *CrosierDriver) Name() string {
	return "crosier"
}

// runCrosierInitCmd executes a script on DUT that initializes Crosier testing environment.
// This script needs to run once before any tests run.
func runCrosierInitCmd(ctx context.Context, logger *log.Logger, dut api.DutServiceClient) error {
	var err error

	cmdExec := api.ExecCommandRequest{
		Command: testInitScriptPath,
		Args:    []string{},
	}

	var client api.DutService_ExecCommandClient
	if client, err = dut.ExecCommand(ctx, &cmdExec); err != nil {
		return fmt.Errorf("failed to execute init command on DUT: %w", err)
	}

	var resp *api.ExecCommandResponse
	if resp, err = client.Recv(); err != nil {
		return fmt.Errorf("failed to get init command results: %w", err)
	}

	logCmd(logger, &cmdExec, resp)
	if resp.ExitInfo.Status != 0 || len(string(resp.Stdout)) > 0 || len(string(resp.Stderr)) > 0 {
		return fmt.Errorf("non-zero exit code (%d) or non-empty output for init command:\nstderr:%v\nstdout:%v",
			resp.ExitInfo.Status,
			string(resp.Stderr),
			string(resp.Stdout))
	}

	return nil
}

// parseResultXML wraps parsing of XML test results to make it unit testable.
func parseResultXML(res []byte, gtestResults *gtestResult) error {
	if err := xml.Unmarshal(res, &gtestResults); err != nil {
		return err
	}
	return nil
}

// runCrosierCmd executes a test on the DUT.
// reasons for failure, if any. Empty reasons means command passed.
func runCrosierCmd(ctx context.Context, logger *log.Logger, dut api.DutServiceClient, test *api.TestCaseMetadata) *executionData {
	var err error

	startTime := time.Now()
	outFileName := fmt.Sprintf("/tmp/%v-%d.xml", test.TestCase.Id.Value, startTime.Unix())

	// Execute the gtest on the DUT.
	cmdArgs := []string{
		fmt.Sprintf("--gtest_output=xml:%v", outFileName),
		fmt.Sprintf("--gtest_filter=%v", test.TestCase.Name),
	}
	cmdExec := api.ExecCommandRequest{
		Command: testRunScriptPath,
		Args:    cmdArgs,
	}

	var client api.DutService_ExecCommandClient
	if client, err = dut.ExecCommand(ctx, &cmdExec); err != nil {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("failed to exec command on DUT: %v", err)})
	}

	var resp *api.ExecCommandResponse
	if resp, err = client.Recv(); err != nil {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("failed to get command results: %v", err)})
	}

	logCmd(logger, &cmdExec, resp)

	if resp.ExitInfo.Status != 0 {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("unexpected failure: stderr: %v, err: %v", string(resp.Stderr), resp.ExitInfo.ErrorMessage)})
	}

	// Test has passed, now get the results.
	cmdExec = api.ExecCommandRequest{
		Command: "cat",
		Args:    []string{outFileName},
	}

	if client, err = dut.ExecCommand(ctx, &cmdExec); err != nil {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("failed to exec command on DUT: %v", err)})
	}

	if resp, err = client.Recv(); err != nil {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("failed to get command results: %v", err)})
	}

	logCmd(logger, &cmdExec, resp)

	if resp.ExitInfo.Status != 0 {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("non-zero exit code (%d) reading test results:\nstderr:%v\nerr:%v",
			resp.ExitInfo.Status,
			string(resp.Stderr),
			resp.ExitInfo.ErrorMessage)})
	}

	// Build the test results struct.
	var gtestResults gtestResult
	if err = parseResultXML(resp.Stdout, &gtestResults); err != nil {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("failed to parse gtest xml data: %v", err)})
	}

	return testResult(test.TestCase.Name, startTime, &gtestResults)
}

// RunTests drives a test framework to execute tests.
func (td *CrosierDriver) RunTests(ctx context.Context, resultsDir string, req *api.CrosTestRequest, tlwAddr string, tests []*api.TestCaseMetadata) (*api.CrosTestResponse, error) {
	var err error
	var testCaseResults []*api.TestCaseResult

	// Setup dut connection to be able to run the tests and get results.
	var dutInfo *device.DutInfo
	if dutInfo, err = device.FillDUTInfo(req.Primary, ""); err != nil {
		return nil, errors.NewStatusError(errors.InvalidArgument,
			fmt.Errorf("cannot get address from primary device: %v", dutInfo))
	}

	var primaryDutConn *grpc.ClientConn
	if primaryDutConn, err = grpc.NewClient(dutInfo.DutServer, grpc.WithTransportCredentials(insecure.NewCredentials())); err != nil {
		return nil, errors.NewStatusError(errors.InvalidArgument,
			fmt.Errorf("cannot create connection with primary device: %v, address: %v", req.Primary, dutInfo.DutServer))
	}
	defer primaryDutConn.Close()

	dut := api.NewDutServiceClient(primaryDutConn)

	// Initialize the DUT for Crosier testing.
	if err = runCrosierInitCmd(ctx, td.logger, dut); err != nil {
		return nil, errors.NewStatusError(errors.InvalidArgument, fmt.Errorf("failed to initialize DUT: %w", err))
	}

	// Run tests and parse results.
	harness := &api.TestHarness{TestHarnessType: &api.TestHarness_Gtest_{Gtest: &api.TestHarness_Gtest{}}}
	for _, test := range tests {
		results := runCrosierCmd(ctx, td.logger, dut, test)

		tcResult := buildTestCaseResults(test.TestCase.Id.Value, harness, results)
		testCaseResults = append(testCaseResults, tcResult)
	}
	return &api.CrosTestResponse{TestCaseResults: testCaseResults}, nil
}
