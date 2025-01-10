// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	common_proto "go.chromium.org/chromiumos/infra/proto/go/test_platform/common"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/suitelimits"
	"infra/cros/cmd/ctpv2/data"
)

// SummarizeCmd represents summarizing of all test results cmd.
type SummarizeCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	AllTestResults          map[string][]*data.TestResults
	RequestToTargetChainMap map[string]map[string]string
	DddTrackerMap           map[string]bool

	// Updates
	ExecuteResponses *steps.ExecuteResponses

	AlStateInfo *data.AlStateInfo
	BuildState  *build.State
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *SummarizeCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.PrePostFilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeepr(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *SummarizeCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.PrePostFilterStateKeeper:
		err = cmd.updatePrePostStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *SummarizeCmd) extractDepsFromFilterStateKeepr(
	ctx context.Context,
	sk *data.PrePostFilterStateKeeper) error {

	if sk.AllTestResults == nil || len(sk.AllTestResults) == 0 {
		return fmt.Errorf("Cmd %q missing dependency: AllTestResults", cmd.GetCommandType())
	}

	cmd.AllTestResults = sk.AllTestResults

	if sk.RequestToTargetChainMap != nil && len(sk.RequestToTargetChainMap) > 0 {
		cmd.RequestToTargetChainMap = sk.RequestToTargetChainMap
	}

	if sk.DddTrackerMap != nil && len(sk.DddTrackerMap) > 0 {
		cmd.DddTrackerMap = sk.DddTrackerMap
	}

	cmd.AlStateInfo = sk.AlStateInfo

	cmd.BuildState = sk.BuildState

	return nil
}

func (cmd *SummarizeCmd) updatePrePostStateKeeper(
	ctx context.Context,
	sk *data.PrePostFilterStateKeeper) error {
	if cmd.ExecuteResponses != nil {
		sk.ExecuteResponses = cmd.ExecuteResponses
	}

	return nil
}

func GetResultsMapFromList(resultsList []*data.TestResults) map[string][]*data.TestResults {
	keyToResultsMap := map[string][]*data.TestResults{}

	for _, result := range resultsList {
		if _, ok := keyToResultsMap[result.Key]; !ok {
			keyToResultsMap[result.Key] = []*data.TestResults{result}
		} else {
			keyToResultsMap[result.Key] = append(keyToResultsMap[result.Key], result)
		}
	}

	return keyToResultsMap
}

// Execute executes the command.
func (cmd *SummarizeCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Summarize")
	defer func() { step.End(err) }()

	common.WriteAnyObjectToStepLog(ctx, step, cmd.AllTestResults, "all test results")

	// Add the suite limits metrics to the summarization step.
	for requestName, csvData := range suitelimits.GetIncrementalLogs() {
		common.WriteAnyObjectToStepLog(ctx, step, csvData, fmt.Sprintf("%s execution logs", requestName))
	}
	common.WriteAnyObjectToStepLog(ctx, step, suitelimits.GetTotalsLogs(), "Total Per Suite Execution Statistics")

	// sort suite keys first
	suiteKeys := make([]string, 0, len(cmd.AllTestResults))
	for k := range cmd.AllTestResults {
		suiteKeys = append(suiteKeys, k)
	}

	sort.Strings(suiteKeys)

	// If direct v2, the results will be organized by each separate request(suite).
	// Otherwise, results will be linked back to the original request name.
	resultsMap := cmd.AllTestResults
	if cmd.RequestToTargetChainMap != nil && len(cmd.RequestToTargetChainMap) > 0 {
		resultsMap, err = cmd.RestructureResultsMap(cmd.AllTestResults)
		if err != nil {
			return err
		}
	}

	cmd.ExecuteResponses = ToExecuteResponses(resultsMap)
	common.WriteProtoToStepLog(ctx, step, cmd.ExecuteResponses, "output_properties")

	for _, suite := range suiteKeys {
		// If any failure found, set it to err to fail summarize step
		if suiteErr := cmd.DisplaySuite(ctx, suite); suiteErr != nil {
			err = suiteErr
		}
	}

	// we don't want the build to fail for this step
	return nil
}

func (cmd *SummarizeCmd) DisplaySuite(ctx context.Context, suite string) error {
	var err error
	testResults := cmd.AllTestResults[suite]
	step, ctx := build.StartStep(ctx, suite)
	defer func() { step.End(err) }()

	if len(testResults) == 0 {
		err = fmt.Errorf("No test results found for suite %s.", suite)
		step.SetSummaryMarkdown(err.Error())
		return err
	}

	// Get map from list and group error/non-error separately
	testResultsMap := GetResultsMapFromList(testResults)
	// We divide the results into two groups
	// (1) Results that don't have test results (failed before hitting test_runner; i.e. bot params rejected, enum error, others)
	// (2) Results that do have test results
	// #1 is displayed first for better clarity and also grouped together
	// Then #2 is displayed with actual test results
	nonErrorResultKeys, nonErrorResultMap, errorResultKeys, errorResultMap := GroupErrAndNonErrResults(testResultsMap)

	errResultErr := ProcessResultsMap(ctx, errorResultKeys, errorResultMap)
	nonErrResultErr := ProcessResultsMap(ctx, nonErrorResultKeys, nonErrorResultMap)

	// If one of the result is type AL, that means the suite is AL run.
	// AL and non-AL results mix isn't possible inside a single suite run.
	// size check is done above so skipping here.
	if testResults[0].IsALRun {
		cmd.AttachATILinkIfApplicable(step)
	}

	// Assign non nil err (if any) so that this step fails
	if errResultErr != nil {
		err = errResultErr
	} else if nonErrResultErr != nil {
		err = nonErrResultErr
	}

	return err
}

// AttachATILinkIfApplicable adds an ATI link to step, If we are in an AL run and have an invocation attached link to the
// ATI page.
func (cmd *SummarizeCmd) AttachATILinkIfApplicable(step *build.Step) {
	if cmd.AlStateInfo != nil {
		tree := cmd.AlStateInfo.GetWorkUnitTree()
		if tree != nil && tree.Head != nil {
			var apiStack string
			// Append the url field in staging.
			if !common.IsProd(cmd.BuildState.Build().GetBuilder()) {
				apiStack = "?api-stack=atp"
			}

			link := fmt.Sprintf("* [ATI Results](%s/%s/%s)", common.ATILink, tree.Head.GetWorkUnit().InvocationId, apiStack)

			step.SetSummaryMarkdown(link)
		}
	}
}

func (cmd *SummarizeCmd) RestructureResultsMap(testResultMap map[string][]*data.TestResults) (map[string][]*data.TestResults, error) {
	ret := map[string][]*data.TestResults{}
	for _, results := range testResultMap {
		for _, result := range results {
			reqChain := cmd.RequestToTargetChainMap[result.RequestKey]
			if reqChain == nil || len(reqChain) == 0 {
				// should not happen
				return nil, fmt.Errorf("empty request map found")
			}
			if cmd.DddTrackerMap[result.RequestKey] {
				// Processing 3d results
				for _, chainedKey := range reqChain {
					if _, ok := ret[chainedKey]; !ok {
						ret[chainedKey] = []*data.TestResults{}
					}
					ret[chainedKey] = append(ret[chainedKey], result)
				}
			} else {
				// Processing non-3d results
				resultBMVkey := common.ExtractPrefixUntilDelimiter(result.Key, "-shard")
				if DoesResultBelongToAllRequestsInChain(reqChain, resultBMVkey) {
					for _, chainedKey := range reqChain {
						if _, ok := ret[chainedKey]; !ok {
							ret[chainedKey] = []*data.TestResults{}
						}
						ret[chainedKey] = append(ret[chainedKey], result)
					}
				} else {
					// if no enum error, let's try to match with each BMV.
					originalKey := reqChain[resultBMVkey]
					if _, ok := ret[originalKey]; !ok {
						ret[originalKey] = []*data.TestResults{}
					}
					ret[originalKey] = append(ret[originalKey], result)
				}
			}
		}
	}

	return ret, nil
}

func DoesResultBelongToAllRequestsInChain(reqChain map[string]string, resultBMVkey string) bool {
	// resultBMVkey not found means the target was changed by one of the filter.
	// right now, this is done by autovm_test_shifter_filter in non-3D scenario.
	// in that case, the result should apply to all the requests in the chain (as same board).

	// for Enum error, the key won't be in request chain either.
	// in this case, there won't be any individual test results for each BMVs.
	// so we need to copy the same result of each (similar to 3d procressing).
	_, isBmvInReqChain := reqChain[resultBMVkey]
	return !isBmvInReqChain
}

func ProcessResultsMap(ctx context.Context, keys []string, resultMap map[string][]*data.TestResults) error {
	var err error
	for _, key := range keys {

		resultsList := resultMap[key]
		// sort by attempt
		sort.Sort(data.ByAttempt(resultsList))
		if resultErr := DisplayResult(ctx, key, resultsList); resultErr != nil {
			err = resultErr
		}
	}

	return err
}

func DisplayResult(ctx context.Context, key string, resultsList []*data.TestResults) error {
	var err error
	step, _ := build.StartStep(ctx, key)
	defer func() { step.End(err) }()

	links := []string{}

	// Here resultsList holds results for a single request(i.e. [jacuzzi]-shard-3).
	// so multiple results in the list means that there are retry attempts for this one.
	// if last retry is green, we should consider this run to be successful. (the list is ordered by attempts).
	for _, result := range resultsList {
		// regardless of error/non-error, log the testCases
		LogTestCasesIfAny(ctx, result, step)
		// override of err is okay since latest green means the whole run is green.
		err = result.GetFailureErr()
		if result.TopLevelError != nil {
			DisplayError(ctx, result, step)
			continue
		}
		buildUrl := result.BuildUrl

		linkStr := "* "
		if result.Attempt > 0 {
			linkStr = fmt.Sprintf("%sretry #%d: ", linkStr, result.Attempt)
		}

		logLink := result.Results.GetLogData().GetTesthausUrl()
		if logLink != "" {
			linkStr = fmt.Sprintf("%s[log link](%s),", linkStr, logLink)
		}

		if buildUrl != "" {
			linkStr = fmt.Sprintf("%s [task link](%s)", linkStr, buildUrl)
		}

		if linkStr != "* " {
			links = append(links, linkStr)
		}

	}
	if len(links) > 0 {
		step.SetSummaryMarkdown(strings.Join(links, "\n"))
	}

	return err
}

func GroupErrAndNonErrResults(inputMap map[string][]*data.TestResults) ([]string, map[string][]*data.TestResults, []string, map[string][]*data.TestResults) {
	nonErrorResultMap := map[string][]*data.TestResults{}
	nonErrorResultKeys := []string{}
	errorResultMap := map[string][]*data.TestResults{}
	errorResultKeys := []string{}

	for _, resultList := range inputMap {
		for _, eachResult := range resultList {
			if eachResult.TopLevelError != nil {
				switch (eachResult.TopLevelError).(type) {
				case *data.EnumerationError:
					errKey := common.EnumerationErrKey
					if _, ok := errorResultMap[errKey]; !ok {
						errorResultKeys = append(errorResultKeys, errKey)
					}
					addToMap(errorResultMap, errKey, eachResult)
				case *data.BotParamsRejectedError:
					errKey := common.BotParamsRejectedErrKey
					if _, ok := errorResultMap[errKey]; !ok {
						errorResultKeys = append(errorResultKeys, errKey)
					}
					addToMap(errorResultMap, errKey, eachResult)
				case *data.SuiteLimitsError:
					errKey := common.SuiteLimitsErrKey
					if _, ok := errorResultMap[errKey]; !ok {
						errorResultKeys = append(errorResultKeys, errKey)
					}
					addToMap(errorResultMap, errKey, eachResult)
				default:
					errKey := common.OtherErrKey
					if _, ok := errorResultMap[errKey]; !ok {
						errorResultKeys = append(errorResultKeys, errKey)
					}
					addToMap(errorResultMap, errKey, eachResult)
				}
			} else {
				if _, ok := nonErrorResultMap[eachResult.Key]; !ok {
					nonErrorResultKeys = append(nonErrorResultKeys, eachResult.Key)
				}
				addToMap(nonErrorResultMap, eachResult.Key, eachResult)
			}
		}
	}

	sort.Strings(nonErrorResultKeys)
	sort.Strings(errorResultKeys)
	return nonErrorResultKeys, nonErrorResultMap, errorResultKeys, errorResultMap
}

func addToMap(inMap map[string][]*data.TestResults, key string, result *data.TestResults) map[string][]*data.TestResults {
	if _, ok := inMap[key]; !ok {
		inMap[key] = []*data.TestResults{}
	}
	inMap[key] = append(inMap[key], result)

	return inMap
}

func LogTestCasesIfAny(ctx context.Context, result *data.TestResults, step *build.Step) {
	testCasesNames := common.GetFlattenedTestCases(result.TestCases)
	if len(testCasesNames) > 0 {
		logNameSuffix := ""
		if result.Attempt > 0 {
			logNameSuffix = fmt.Sprintf("-retry-%d", result.Attempt)
		}
		log := step.Log(fmt.Sprintf("testcases for '%s'%s", result.Key, logNameSuffix))
		log.Write([]byte(fmt.Sprintf("%s", strings.Join(testCasesNames, "\n"))))
	}
}

func DisplayError(ctx context.Context, result *data.TestResults, step *build.Step) {
	switch e := (result.TopLevelError).(type) {
	case *data.EnumerationError:
		log := step.Log(fmt.Sprintf("suite `%s`", e.SuiteName))
		log.Write([]byte(e.Error()))
	case *data.BotParamsRejectedError:
		log := step.Log(fmt.Sprintf("bot params rejected for '%s'", e.Key))
		log.Write([]byte(fmt.Sprintf("rejected params: [\n%s\n]", strings.Join(e.RejectedDims, "\n"))))
		result.Key = common.BotParamsRejectedErrKey
	default:
		log := step.Log(fmt.Sprintf("error for '%s'", result.Key))
		log.Write([]byte(e.Error()))
	}
}

// NewSummarizeCmd returns a new SummarizeCmd
func NewSummarizeCmd() *SummarizeCmd {
	abstractCmd := interfaces.NewAbstractCmd(SummarizeCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &SummarizeCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}

func ToExecuteResponses(testResultMap map[string][]*data.TestResults) *steps.ExecuteResponses {
	taggedRes := map[string]*steps.ExecuteResponse{}
	for key, results := range testResultMap {
		consolidatedResults := []*steps.ExecuteResponse_ConsolidatedResult{}
		taskResults := []*steps.ExecuteResponse_TaskResult{}
		verdict := test_platform.TaskState_VERDICT_NO_VERDICT
		sort.Sort(data.ByAttempt(results))
		for _, testResult := range results {
			taskResult := TrResultToErTaskResult(testResult)
			if taskResult != nil {
				taskResults = append(taskResults, taskResult)
			}
			if testResult.GetFailureErr() != nil {
				verdict = test_platform.TaskState_VERDICT_FAILED
			} else {
				verdict = test_platform.TaskState_VERDICT_PASSED
			}
		}
		// If no task result, then use default one where no task level detail will be shared
		if len(taskResults) == 0 {
			taggedRes[key] = &steps.ExecuteResponse{State: &test_platform.TaskState{LifeCycle: test_platform.TaskState_LIFE_CYCLE_COMPLETED, Verdict: verdict}}
		} else {
			// consolidated results
			consolidatedResult := &steps.ExecuteResponse_ConsolidatedResult{
				Attempts: taskResults,
			}
			consolidatedResults = append(consolidatedResults, consolidatedResult)

			taggedRes[key] = &steps.ExecuteResponse{TaskResults: taskResults, ConsolidatedResults: consolidatedResults, State: &test_platform.TaskState{LifeCycle: test_platform.TaskState_LIFE_CYCLE_COMPLETED, Verdict: verdict}}
		}

	}

	return &steps.ExecuteResponses{TaggedResponses: taggedRes}
}

func TrResultToErTaskResult(testResult *data.TestResults) *steps.ExecuteResponse_TaskResult {
	if testResult.TopLevelError != nil {
		switch e := (testResult.TopLevelError).(type) {

		case *data.BotParamsRejectedError:
			r1, r2 := common.GetDims(e.RejectedDims)
			r := &steps.ExecuteResponse_TaskResult{
				Name:                   testResult.Name,
				State:                  &test_platform.TaskState{LifeCycle: test_platform.TaskState_LIFE_CYCLE_REJECTED},
				RejectedTaskDimensions: r1,
				RejectedDimensions:     r2,
			}
			return r
			// TODO: Consider adding enumeration error (v1 does not propagate enumeration error on task level)
		}
		return nil
	}
	r := &steps.ExecuteResponse_TaskResult{
		Name: common.GetTautoTestCaseNameOrDefault(testResult.Results.GetAutotestResult().GetTestCases(), testResult.Name),
		State: &test_platform.TaskState{
			LifeCycle: test_platform.TaskState_LIFE_CYCLE_COMPLETED, // at this point the task must be in completed state
			Verdict:   common.GetTaskStateVerdict(testResult.Results),
		},
		TaskUrl:     testResult.BuildUrl,
		TestCases:   common.TestCasesToTestCaseResult(testResult.Results),
		PrejobSteps: common.PrejobStepsToTestCaseResult(testResult.Results.GetPrejob().GetStep()),
		Attempt:     int32(testResult.Attempt),
	}
	if ld := testResult.Results.GetLogData(); ld != nil {
		r.LogData = proto.Clone(ld).(*common_proto.TaskLogData)
		r.LogUrl = r.LogData.TesthausUrl
	}

	return r
}
