// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/execution/errors"

	"infra/cros/cmd/cft/execution/cros-test/internal/device"
)

// GtestDriver runs gtest and report its results.
type GtestDriver struct {
	// logger provides logging service.
	logger *log.Logger
}

// NewGtestDriver creates a new driver to run tests.
func NewGtestDriver(logger *log.Logger) *GtestDriver {
	return &GtestDriver{
		logger: logger,
	}
}

// Name returns the name of the driver.
func (td *GtestDriver) Name() string {
	return "gtest"
}

// gtestFailure represents gtest failure information
// ...
//
//	<failure message="error message" type=""></failure>
type gtestFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
}

// gtestCase represents a test case execution
// ...
//
//	<testcase name="Positive" status="run" time="0.782"
//				timestamp="2024-01-17T23:11:33" classname="StubTest">
//		...
//	</testcase>
type gtestCase struct {
	Name      string         `xml:"name,attr"`
	Status    string         `xml:"status,attr"`
	Time      float64        `xml:"time,attr"`
	Timestamp string         `xml:"timestamp,attr"`
	Classname string         `xml:"classname,attr"`
	Failures  []gtestFailure `xml:"failure"`
}

// gtestSuite represents a gtest suite
// ...
//
//	<testsuite name="StubTest" tests="1" failures="0" disabled="0" errors="0"
//				time="0.782" timestamp="2024-01-17T23:11:33">
//		...
//	</testsuite>
type gtestSuite struct {
	Name      string      `xml:"name,attr"`
	Tests     int         `xml:"tests,attr"`
	Failures  int         `xml:"failures,attr"`
	Disabled  int         `xml:"disabled,attr"`
	Errors    int         `xml:"errors,attr"`
	Time      float64     `xml:"time,attr"`
	Timestamp string      `xml:"timestamp,attr"`
	TestCases []gtestCase `xml:"testcase"`
}

// gtestResult represents the entire gtest execution
// ...
//
//	<testsuites name="AllTests" tests="1" failures="0" disabled="0" errors="0"
//				time="0.782" timestamp="2024-01-17T23:11:33">
//		...
//	</testsuites>
type gtestResult struct {
	XMLName    xml.Name     `xml:"testsuites"`
	Name       string       `xml:"name,attr"`
	Tests      int          `xml:"tests,attr"`
	Failures   int          `xml:"failures,attr"`
	Disabled   int          `xml:"disabled,attr"`
	Errors     int          `xml:"errors,attr"`
	Time       float64      `xml:"time,attr"`
	Timestamp  string       `xml:"timestamp,attr"`
	TestSuites []gtestSuite `xml:"testsuite"`
}

// Data needed to convert a gtestResult into ResultMetadata
type executionData struct {
	startTime time.Time
	duration  int64
	reasons   []string
}

func newExecutionData(startTime time.Time, duration int64, reasons []string) *executionData {
	return &executionData{
		startTime: startTime,
		duration:  duration,
		reasons:   reasons,
	}
}

// logCmd logs a remote command run through a DUT server
func logCmd(logger *log.Logger, cmd *api.ExecCommandRequest, resp *api.ExecCommandResponse) {
	logger.Printf("cmd '%v', args '%v'", cmd.Command, cmd.Args)
	logger.Printf("[status]:\n%v", resp.ExitInfo.Status)
	logger.Printf("[stdout]:\n%v", string(resp.Stdout))
	logger.Printf("[stderr]:\n%v", string(resp.Stderr))
	logger.Printf("[error]:\n%v", string(resp.ExitInfo.ErrorMessage))
}

const invalidDuration int64 = 0

// testResult returns list of reasons if TC failed, empty list otherwise.
func testResult(testCaseName string, defaultStartTime time.Time, result *gtestResult) *executionData {
	// Example results file (showing a pass and a fail).
	// Right now, only one result is expected, either pass or fail, this is just an
	// example.
	//
	// <?xml version="1.0" encoding="UTF-8"?>
	// <testsuites name="AllTests" tests="2" failures="0" disabled="0" errors="0" time="0.782" timestamp="2024-01-17T23:11:33">
	// 	 <testsuite name="StubTest" tests="2" failures="0" disabled="0" errors="0" time="0.782" timestamp="2024-01-17T23:11:33">
	// 		<testcase name="Positive" status="run" time="0.782" timestamp="2024-01-17T23:11:33" classname="StubTest">
	// 		</testcase>
	// 		<testcase name="Negative" status="run" time="0.782" timestamp="2024-01-17T23:11:33" classname="StubTest">
	// 			<failure message="error message" type=""></failure>
	// 		</testcase>
	// 	 </testsuite>
	// </testsuites>
	//
	// Test case name is "classname"."name", in this case "StubTest.Positive"
	//
	// Var defaultStartTime is used when startTime cannot be read from the gtest output.

	// First make sure we have results, should have one suite and one case.
	if len(result.TestSuites) != 1 || len(result.TestSuites[0].TestCases) != 1 {
		return newExecutionData(defaultStartTime, invalidDuration, []string{"no test results found"})
	}

	// Check that the classname and casename are as expected.
	// testCaseName should be of format '<classname>.<casename>'.
	nameParts := strings.SplitN(testCaseName, ".", 2)
	if len(nameParts) != 2 {
		return newExecutionData(defaultStartTime, invalidDuration, []string{fmt.Sprintf("unexpected testCaseName, got: '%v', want: format '<className>.<caseName>'", testCaseName)})
	}
	className, caseName := nameParts[0], nameParts[1]

	testCase := result.TestSuites[0].TestCases[0]
	if testCase.Classname != className {
		return newExecutionData(defaultStartTime, invalidDuration, []string{fmt.Sprintf("mismatched classname, got: '%v', want: '%v'", testCase.Classname, caseName[0])})
	}

	if testCase.Name != caseName {
		return newExecutionData(defaultStartTime, invalidDuration, []string{fmt.Sprintf("mismatched case name, got: '%v', want: '%v'", testCase.Name, caseName[1])})
	}

	// Check that status and result are as expected.
	status := strings.ToLower(testCase.Status)
	if status != "run" {
		return newExecutionData(defaultStartTime, invalidDuration, []string{fmt.Sprintf("mismatched case status, got: '%v', want: 'run'", status)})
	}

	timeString := result.TestSuites[0].TestCases[0].Timestamp
	startTime := defaultStartTime
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, timeString); err == nil {
			startTime = t
			break
		}
	}

	duration := int64(result.TestSuites[0].TestCases[0].Time)

	// Grab any failures.
	if len(testCase.Failures) > 0 {
		var reasons []string

		for _, failure := range testCase.Failures {
			reasons = append(reasons, fmt.Sprintf("failure: '%v', type: '%v'", failure.Message, failure.Type))
		}

		return newExecutionData(startTime, duration, reasons)
	}

	// Test passed supposedly, make sure no unexpected failures or errors
	if result.Failures != 0 || result.Errors != 0 || result.TestSuites[0].Failures != 0 || result.TestSuites[0].Disabled != 0 {
		return newExecutionData(startTime, duration, []string{"unexpected errors in gtest results"})
	}

	return newExecutionData(startTime, duration, nil)
}

// buildTestCaseResults builds the api.TestCaseResult object for a given test result.
// If err is populated, and ERROR status will be returned.
func buildTestCaseResults(tcID string, harness *api.TestHarness, results *executionData) *api.TestCaseResult {
	tcResult := new(api.TestCaseResult)

	tcResult.TestHarness = harness
	tcResult.TestCaseId = &api.TestCase_Id{Value: tcID}
	tcResult.StartTime = timestamppb.New(results.startTime)
	tcResult.Duration = &durationpb.Duration{Seconds: results.duration}

	// Test passed if no reasons specified
	if len(results.reasons) == 0 {
		tcResult.Verdict = &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}}
	} else {
		tcResult.Verdict = &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}}
		tcResult.Reason = strings.Join(results.reasons, "\n")
	}

	return tcResult
}

// runGtestCmd executes a test on the DUT.
// reasons for failure, if any. Empty reasons means command passed.
func runGtestCmd(ctx context.Context, logger *log.Logger, dut api.DutServiceClient, test *api.TestCaseMetadata) *executionData {
	var err error

	targetBinLocation := test.TestCaseExec.GetTestHarness().GetGtest().GetTargetBinLocation()
	startTime := time.Now()
	outFileName := fmt.Sprintf("/tmp/%v-%d.xml", test.TestCase.Id.Value, startTime.Unix())

	// Execute the gtest on the DUT.
	cmdArgs := []string{
		fmt.Sprintf("--gtest_output=xml:%v", outFileName),
		fmt.Sprintf("--gtest_filter=%v", test.TestCase.Name),
	}
	cmdExec := api.ExecCommandRequest{
		Command: targetBinLocation,
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

	// Gtest should return 0 or 1 if tests ran, anything else should be
	// looked at as an execution/infra failure.
	//
	// Occassionally, gtest will receive a bad arg and return a zero exit
	// code but not actually run the tests. Because some tests might have stdout
	// output, the best way to catch this is to log the command and the driver
	// will fail when it tries to parse the results file, which won't exist.
	if resp.ExitInfo.Status != 0 && resp.ExitInfo.Status != 1 {
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
	if err = xml.Unmarshal(resp.Stdout, &gtestResults); err != nil {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("failed to parse gtest xml data: %v", err)})
	}

	return testResult(test.TestCase.Name, startTime, &gtestResults)
}

// RunTests drives a test framework to execute tests.
func (td *GtestDriver) RunTests(ctx context.Context, resultsDir string, req *api.CrosTestRequest, tlwAddr string, tests []*api.TestCaseMetadata) (*api.CrosTestResponse, error) {
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
	harness := &api.TestHarness{TestHarnessType: &api.TestHarness_Gtest_{Gtest: &api.TestHarness_Gtest{}}}

	for _, test := range tests {
		results := runGtestCmd(ctx, td.logger, dut, test)

		tcResult := buildTestCaseResults(test.TestCase.Id.Value, harness, results)
		testCaseResults = append(testCaseResults, tcResult)
	}
	return &api.CrosTestResponse{TestCaseResults: testCaseResults}, nil
}
