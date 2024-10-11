// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/timestamp"
	"go.chromium.org/chromiumos/config/go/test/api"
	commonpb "go.chromium.org/chromiumos/infra/proto/go/test_platform/common"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/dutstate"
)

// ProcessResultsCmd represents process results command.
type ProcessResultsCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps (all are optional)
	CftTestRequest  *skylab_test_runner.CFTTestRequest
	GcsURL          string
	TesthausURL     string
	ProvisionResps  map[string][]*api.InstallResponse
	TestResponses   *api.CrosTestResponse
	CurrentDutState dutstate.State // optional
	buildState      *build.State

	// Updates
	SkylabResult           *skylab_test_runner.Result
	TestExecutionStartTime *timestamp.Timestamp
	TestExecutionEndTime   *timestamp.Timestamp
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *ProcessResultsCmd) ExtractDependencies(ctx context.Context, ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *ProcessResultsCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.updateHwTestStateKeeper(ctx, sk)
	case *data.LocalTestStateKeeper:
		err = cmd.updateHwTestStateKeeper(ctx, &sk.HwTestStateKeeper)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// Execute executes the command.
func (cmd *ProcessResultsCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Results")
	defer func() { step.End(build.AttachStatus(err, bbpb.Status_FAILURE, nil)) }()

	common.AddLinksToStepSummaryMarkdown(step, cmd.TesthausURL, common.GetGcsClickableLink(cmd.GcsURL))

	// Default values
	prejobVerdict := skylab_test_runner.Result_Prejob_Step_VERDICT_UNDEFINED
	prejobReason := ""
	logData := getLogData(cmd.TesthausURL, cmd.GcsURL)

	// Parse provision info
	var prejob *skylab_test_runner.Result_Prejob = nil
	if cmd.ProvisionResps != nil && len(cmd.ProvisionResps) > 0 {
		for dutName, provisionResps := range cmd.ProvisionResps {
			for _, provisionResp := range provisionResps {
				if provisionResp.GetStatus() == api.InstallResponse_STATUS_SUCCESS && prejobVerdict != skylab_test_runner.Result_Prejob_Step_VERDICT_FAIL {
					prejobVerdict = skylab_test_runner.Result_Prejob_Step_VERDICT_PASS
				} else {
					prejobVerdict = skylab_test_runner.Result_Prejob_Step_VERDICT_FAIL
					if prejobReason == "" {
						prejobReason = provisionResp.GetStatus().String()
					}
				}
				stepName := fmt.Sprintf("Provision of %s", dutName)
				provErr := common.CreateStepWithStatus(ctx, stepName, provisionResp.GetStatus().String(), provisionResp.GetStatus() != api.InstallResponse_STATUS_SUCCESS, true)
				// Propogate error status to parent step
				if err == nil {
					err = provErr
				}
			}
		}
		prejob = &skylab_test_runner.Result_Prejob{
			Step: []*skylab_test_runner.Result_Prejob_Step{
				{
					Name:                 "provision",
					Verdict:              prejobVerdict,
					HumanReadableSummary: prejobReason,
				},
			},
		}
	}

	if cmd.TestResponses != nil && cmd.TestResponses.GetGivenTestResults() != nil {
		err = cmd.generateSkylabResultForAluminium(ctx, step, prejob, logData, err)
	} else {
		err = cmd.generateSkylabResultForClassic(ctx, step, prejob, logData, err)
	}

	return nil
}

// extractDepsFromHwTestStateKeeper extracts cmd deps from hw test state keeper.
func (cmd *ProcessResultsCmd) extractDepsFromHwTestStateKeeper(ctx context.Context, sk *data.HwTestStateKeeper) error {
	if sk.CftTestRequest == nil {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: CftTestRequest", cmd.GetCommandType())
	}
	if sk.TestResponses == nil {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: TestResponses", cmd.GetCommandType())
	}
	if sk.GcsURL == "" {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: GcsURL", cmd.GetCommandType())
	}
	if sk.TesthausURL == "" {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: TesthausURL", cmd.GetCommandType())
	}
	if sk.CurrentDutState == "" {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: CurrentDutState", cmd.GetCommandType())
	}

	for _, id := range sk.DeviceIdentifiers {
		responses, ok := sk.ProvisionResponses[id]
		if ok && len(responses) > 0 {
			if device, ok := sk.Devices[id]; ok {
				cmd.ProvisionResps[device.GetDut().GetId().GetValue()] = responses
			}
		}
	}

	if len(cmd.ProvisionResps) < 1 {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: ProvisionResps", cmd.GetCommandType())
	}
	cmd.TestExecutionStartTime = sk.TestExecutionStartTime
	cmd.TestExecutionEndTime = sk.TestExecutionEndTime
	cmd.CftTestRequest = sk.CftTestRequest
	cmd.TestResponses = sk.TestResponses
	cmd.GcsURL = sk.GcsURL
	cmd.TesthausURL = sk.TesthausURL
	cmd.CurrentDutState = sk.CurrentDutState
	cmd.buildState = sk.BuildState
	return nil
}

func (cmd *ProcessResultsCmd) updateHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {

	if cmd.SkylabResult != nil {
		sk.SkylabResult = cmd.SkylabResult
	}

	return nil
}

// getTestVerdict converts testcase result to testcase verdict.
func getTestVerdict(ctx context.Context, testResult *api.TestCaseResult) (skylab_test_runner.Result_Autotest_TestCase_Verdict, bool) {
	// Default values
	isTestFailure := true
	var testVerdict skylab_test_runner.Result_Autotest_TestCase_Verdict

	// Convert testcase result to testcase verdict
	switch testResult.Verdict.(type) {
	case *api.TestCaseResult_Pass_:
		isTestFailure = false
		testVerdict = skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS
	case *api.TestCaseResult_Fail_:
		testVerdict = skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL
	case *api.TestCaseResult_Abort_:
		testVerdict = skylab_test_runner.Result_Autotest_TestCase_VERDICT_ABORT
	case *api.TestCaseResult_Crash_:
		testVerdict = skylab_test_runner.Result_Autotest_TestCase_VERDICT_ERROR
	case *api.TestCaseResult_Skip_:
		testVerdict = skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS
	case *api.TestCaseResult_NotRun_:
		testVerdict = skylab_test_runner.Result_Autotest_TestCase_VERDICT_FAIL
	default:
		logging.Infof(ctx, "No valid test case result status found for %s.", testResult.GetTestCaseId().GetValue())
		testVerdict = skylab_test_runner.Result_Autotest_TestCase_VERDICT_NO_VERDICT
	}

	return testVerdict, isTestFailure
}

// getDefaultAutotestTestCasesResult constructs default result from input.
func getDefaultAutotestTestCasesResult(ctx context.Context, req *skylab_test_runner.CFTTestRequest) []*skylab_test_runner.Result_Autotest_TestCase {
	autotestTestCases := []*skylab_test_runner.Result_Autotest_TestCase{}
	for _, testSuite := range req.GetTestSuites() {
		for _, testCaseId := range testSuite.GetTestCaseIds().GetTestCaseIds() {
			autotestTestCase := &skylab_test_runner.Result_Autotest_TestCase{
				Name:    testCaseId.GetValue(),
				Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_NO_VERDICT,
			}
			autotestTestCases = append(autotestTestCases, autotestTestCase)

			_ = common.CreateStepWithStatus(ctx, testCaseId.GetValue(), common.TestDidNotRunErr, true, false)
		}
	}

	return autotestTestCases
}

// getDefaultAndroidGenericTestCasesResult constructs default result from input.
func getDefaultAndroidGenericTestCasesResult(ctx context.Context, req *skylab_test_runner.CFTTestRequest) []*skylab_test_runner.Result_AndroidGeneric_GivenTestCase {
	givenTestCases := []*skylab_test_runner.Result_AndroidGeneric_GivenTestCase{}
	for _, testSuite := range req.GetTestSuites() {
		for _, testCaseID := range testSuite.GetTestCaseIds().GetTestCaseIds() {
			givenTestCase := &skylab_test_runner.Result_AndroidGeneric_GivenTestCase{
				ParentTest: testCaseID.GetValue(),
				Incomplete: true,
			}
			givenTestCases = append(givenTestCases, givenTestCase)

			_ = common.CreateStepWithStatus(ctx, testCaseID.GetValue(), common.TestDidNotRunErr, true, false)
		}
	}

	return givenTestCases
}

// getLogData constructs tasklogdata from provided links.
func getLogData(testhausURL string, gcsURL string) *commonpb.TaskLogData {
	logData := &commonpb.TaskLogData{}
	if testhausURL != "" {
		logData.TesthausUrl = testhausURL
	}
	if gcsURL != "" {
		logData.GsUrl = gcsURL
	}

	return logData
}

func NewProcessResultsCmd() *ProcessResultsCmd {
	abstractCmd := interfaces.NewAbstractCmd(ProcessResultsCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &ProcessResultsCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor, ProvisionResps: map[string][]*api.InstallResponse{}}
}

func (cmd *ProcessResultsCmd) generateSkylabResultForClassic(ctx context.Context, step *build.Step, prejob *skylab_test_runner.Result_Prejob, logData *commonpb.TaskLogData, err error) error {
	// Parse test results
	autotestTestCases := []*skylab_test_runner.Result_Autotest_TestCase{}
	var testErr error
	isIncomplete := true
	testCaseCount := len(cmd.TestResponses.GetTestCaseResults())
	if cmd.TestResponses != nil && testCaseCount > 0 {
		isIncomplete = false
		isPastCaseLimit := testCaseCount > 2000
		passCount := 0
		failCount := 0
		for _, testResult := range cmd.TestResponses.GetTestCaseResults() {
			testVerdict, isTestFailure := getTestVerdict(ctx, testResult)
			testResultReason := testResult.GetReason()
			autotestTestCase := &skylab_test_runner.Result_Autotest_TestCase{
				Name:                 testResult.GetTestCaseId().GetValue(),
				Verdict:              testVerdict,
				HumanReadableSummary: testResultReason,
			}
			autotestTestCases = append(autotestTestCases, autotestTestCase)
			if isTestFailure {
				failCount++
			} else {
				passCount++
			}

			if !isPastCaseLimit {
				// Set test steps
				testErr = common.CreateStepWithStatus(ctx, testResult.GetTestCaseId().GetValue(), testResultReason, isTestFailure, true)
				// Propagate error status to parent step
				if err == nil && isTestFailure {
					err = testErr
				}
			}
		}
		if isPastCaseLimit {
			if passCount > 0 {
				common.CreateStepWithStatus(ctx, fmt.Sprintf("%d tests passed", passCount), "", false, false)
			}
			if failCount > 0 {
				common.CreateStepWithStatus(ctx, fmt.Sprintf("%d tests failed", failCount), "", true, false)
			}
		}
	}

	// If no test results, add default results from input.
	if len(autotestTestCases) == 0 {
		autotestTestCases = getDefaultAutotestTestCasesResult(ctx, cmd.CftTestRequest)
	}

	autotestResult := &skylab_test_runner.Result_Autotest{
		TestCases:  autotestTestCases,
		Incomplete: isIncomplete,
	}
	skylabResult := &skylab_test_runner.Result{
		Harness: &skylab_test_runner.Result_AutotestResult{
			AutotestResult: autotestResult,
		},
		Prejob: prejob,
		AutotestResults: map[string]*skylab_test_runner.Result_Autotest{
			"original_test": autotestResult,
		},
		StateUpdate: &skylab_test_runner.Result_StateUpdate{
			DutState: cmd.CurrentDutState.String(),
		},
		LogData: logData,
	}

	cmd.SkylabResult = skylabResult
	common.WriteProtoToStepLog(ctx, step, skylabResult, "skylab_result")

	return err

}

func (cmd *ProcessResultsCmd) generateSkylabResultForAluminium(ctx context.Context, step *build.Step, prejob *skylab_test_runner.Result_Prejob, logData *commonpb.TaskLogData, err error) error {
	// Parse test results
	givenTestCases := []*skylab_test_runner.Result_AndroidGeneric_GivenTestCase{}
	var testErr error
	isIncomplete := true
	testCaseCount := len(cmd.TestResponses.GetTestCaseResults())
	if cmd.TestResponses != nil && testCaseCount > 0 {
		isIncomplete = false
		isPastCaseLimit := testCaseCount > 2000
		passCount := 0
		failCount := 0
		for _, givenTestResult := range cmd.TestResponses.GetGivenTestResults() {
			childTestCases := []*skylab_test_runner.Result_Autotest_TestCase{}
			for _, childTestResult := range givenTestResult.GetChildTestCaseResults() {
				testVerdict, isTestFailure := getTestVerdict(ctx, childTestResult)
				testResultReason := childTestResult.GetReason()
				childTestCase := &skylab_test_runner.Result_Autotest_TestCase{
					Name:                 childTestResult.GetTestCaseId().GetValue(),
					Verdict:              testVerdict,
					HumanReadableSummary: testResultReason,
				}
				if isTestFailure {
					failCount++
				} else {
					passCount++
				}
				childTestCases = append(childTestCases, childTestCase)
				if !isPastCaseLimit {
					// Set test steps
					testErr = common.CreateStepWithStatus(ctx, childTestResult.GetTestCaseId().GetValue(), testResultReason, isTestFailure, true)
					// Propagate error status to parent step
					if err == nil && isTestFailure {
						err = testErr
					}
				}
			}
			givenTestCases = append(givenTestCases, &skylab_test_runner.Result_AndroidGeneric_GivenTestCase{
				ParentTest:     givenTestResult.GetParentTest(),
				ChildTestCases: childTestCases,
				Incomplete:     isIncomplete,
			})

		}
		if isPastCaseLimit {
			if passCount > 0 {
				common.CreateStepWithStatus(ctx, fmt.Sprintf("%d tests passed", passCount), "", false, false)
			}
			if failCount > 0 {
				common.CreateStepWithStatus(ctx, fmt.Sprintf("%d tests failed", failCount), "", true, false)
			}
		}
	}

	// If no test results, add default results from input.
	if len(givenTestCases) == 0 {
		givenTestCases = getDefaultAndroidGenericTestCasesResult(ctx, cmd.CftTestRequest)
	}

	skylabResult := &skylab_test_runner.Result{
		Harness: &skylab_test_runner.Result_AndroidGenericResult{
			AndroidGenericResult: &skylab_test_runner.Result_AndroidGeneric{
				GivenTestCases: givenTestCases,
			},
		},
		Prejob: prejob,
		StateUpdate: &skylab_test_runner.Result_StateUpdate{
			DutState: cmd.CurrentDutState.String(),
		},
		LogData:   logData,
		StartTime: cmd.TestExecutionStartTime,
		EndTime:   cmd.TestExecutionEndTime,
		ResourceUrls: []*skylab_test_runner.Result_Links{
			{
				Name: skylab_test_runner.Result_Links_TEST_HAUS,
				Url:  cmd.TesthausURL,
			},
			{
				Name: skylab_test_runner.Result_Links_GOOGLE_STORAGE,
				Url:  cmd.GcsURL,
			},
		},
	}

	cmd.SkylabResult = skylabResult
	common.WriteProtoToStepLog(ctx, step, skylabResult, "skylab_result")

	return err

}
