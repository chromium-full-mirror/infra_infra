// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/analytics"
	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/ctpv2/data"
)

// GenerateTrv2RequestsCmd represents scheduling task(s) cmd.
type GenerateTrv2RequestsCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	MiddledOutResp   *data.MiddleOutResponse
	BuildState       *build.State
	Scheduler        interfaces.SchedulerInterface
	DynamicRun       bool
	InternalTestPlan *api.InternalTestplan
	Config           *config.Config
	RequestKey       string
	CredentialsFile  string
	IsAlRun          bool
	// EnvVersion denotes whether the environment
	// is prod or something else.
	EnvVersion string

	// Updates
	BuildsMap   map[string]*data.BuildRequest
	TestResults map[string]*data.TestResults
	AlStateInfo *data.AlStateInfo // will be used as dep as well

	// For logging
	BQClient          *bigquery.Client
	StartCmdTime      time.Time
	StartTrReqGenTime time.Time
	DynamicExperiment bool

	// Helper structures
	schedulingUnitsMetadataMap map[string][]*api.SchedulingUnit
	TrReqsStart                []*analytics.TaskData
	TrReqsEnd                  []*analytics.TaskData
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *GenerateTrv2RequestsCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *GenerateTrv2RequestsCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.updateScheduleStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *GenerateTrv2RequestsCmd) extractDepsFromFilterStateKeeper(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	if sk.MiddledOutResp == nil {
		return fmt.Errorf("Cmd %q missing dependency: MiddleOutResponse", cmd.GetCommandType())
	}

	if sk.BuildState == nil {
		return fmt.Errorf("Cmd %q missing dependency: Scheduler", cmd.GetCommandType())
	}

	if sk.Config == nil {
		logging.Warningf(ctx, "cmd %q missing Config", cmd.GetCommandType())
	}

	if sk.BQClient != nil {
		cmd.BQClient = sk.BQClient
	}
	cmd.InternalTestPlan = proto.Clone(sk.TestPlanStates[len(sk.TestPlanStates)-1]).(*api.InternalTestplan)

	if sk.CtpReq == nil {
		return fmt.Errorf("Cmd %q missing dependency: CtpReq", cmd.GetCommandType())
	}

	if sk.RequestKey == "" {
		logging.Warningf(ctx, "cmd %q missing optional dependency: RequestKey", cmd.GetCommandType())
	}

	if sk.AlStateInfo == nil {
		logging.Warningf(ctx, "cmd %q missing optional dependency: AlStateInfo", cmd.GetCommandType())
	}

	cmd.DynamicExperiment = slices.Contains(sk.BuildState.Build().GetInput().GetExperiments(), "chromeos.cros_infra_config.dynamic_trv2")
	cmd.CredentialsFile = sk.DockerKeyFile
	cmd.RequestKey = sk.RequestKey
	cmd.DynamicRun = sk.CtpReq.RunDynamic || cmd.DynamicExperiment
	cmd.MiddledOutResp = sk.MiddledOutResp
	cmd.BuildState = sk.BuildState
	cmd.Config = sk.Config
	cmd.AlStateInfo = sk.AlStateInfo
	cmd.IsAlRun = sk.IsAlRun
	cmd.EnvVersion = sk.CTPversion

	// Convert scheduling units into map for better searching.
	cmd.schedulingUnitsMetadataMap = buildSchedUnitMap(cmd.InternalTestPlan.GetSuiteInfo())

	return nil
}

func (cmd *GenerateTrv2RequestsCmd) updateScheduleStateKeeper(ctx context.Context, sk *data.FilterStateKeeper) error {
	if cmd.BuildsMap != nil && len(cmd.BuildsMap) != 0 {
		sk.BuildsMap = cmd.BuildsMap
	}

	// TestResults will be set here only if there was an enum error for this suite
	// so it's safe to set it directly because we want the result to be carried till
	// summarize step. And in this case, scheduleTasks will not add anything to the
	// testResults.
	if cmd.TestResults != nil && len(cmd.TestResults) != 0 {
		sk.SuiteTestResults = cmd.TestResults
	}

	if cmd.AlStateInfo != nil && cmd.AlStateInfo.CurrentTestJobEvent != nil {
		// update it's state only
		cmd.AlStateInfo.CurrentTestJobEvent.State = "RUNNING"
		cmd.AlStateInfo.CurrentTestJobEvent.TestJob.TestJobState = "RUNNING"
	}
	return nil
}

// Execute executes the command.
func (cmd *GenerateTrv2RequestsCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Generate Trv2 Requests")
	defer func() { step.End(err) }()

	cmd.ObserveCmdStart(ctx)
	cmd.ObserveEnumerationStart(ctx)

	// Generate trv2 requests
	if len(cmd.MiddledOutResp.TrReqs) == 0 {
		cmd.ObserveCmdEndFailure(ctx)
		err = cmd.ProcessEnumError(ctx, step)
		return err
	}

	buildMap, err := cmd.GenerateRequests(ctx, step)
	if len(buildMap) == 0 {
		cmd.ObserveCmdEndFailure(ctx)
		err = cmd.ProcessEnumError(ctx, step)
		return err
	}

	cmd.BuildsMap = buildMap
	cmd.ObserveEnumerationSuccess(ctx)
	cmd.ObserveCmdEndSuccess(ctx)
	// returning nil coz if there is at least one successful request generation,
	// we should go to next cmd for handling it.
	return nil
}

// GenerateRequests generates trv2 requests
func (cmd *GenerateTrv2RequestsCmd) GenerateRequests(ctx context.Context, step *build.Step) (map[string]*data.BuildRequest, error) {
	var err error

	// Set step summary for dynamic experiment
	if cmd.DynamicExperiment {
		step.SetSummaryMarkdown("Test Runner request set to dynamic mode by experiment")
	}

	// Generate reqs
	errCount := 0
	buildMap := map[string]*data.BuildRequest{}
	shardMap := map[string]int{}
	for _, trReq := range cmd.MiddledOutResp.TrReqs {
		key, err := GetBoardModelVariantKey(ctx, trReq)
		if err != nil {
			logging.Infof(ctx, fmt.Sprintf("error while generating board-model-variant key: %s", err))
			errCount++
			continue
		}
		// figure out shards properly to represent them well in UI
		if _, ok := shardMap[key]; !ok {
			shardMap[key] = 0
		} else {
			shardMap[key] = shardMap[key] + 1
		}

		modifiedKey := fmt.Sprintf("%s-shard-%d", key, shardMap[key])
		buildReq := &data.BuildRequest{Key: modifiedKey, OriginalTrReq: trReq, ShardNum: shardMap[key], SuiteInfo: cmd.MiddledOutResp.SuiteInfo}
		req, err := cmd.GenerateReq(ctx, trReq, modifiedKey, shardMap[key])
		if err != nil {
			buildReq.Err = err
			errCount++
			logging.Infof(ctx, "error while generating trv2 req for %s: %s", modifiedKey, err)
		} else {
			buildReq.ScheduleBuildRequest = req
		}

		buildMap[modifiedKey] = buildReq
	}

	if errCount == 0 {
		step.SetSummaryMarkdown("all test requests were generated successfully")
	} else {
		step.SetSummaryMarkdown(fmt.Sprintf("error found in %d out of %d requests", errCount, len(cmd.MiddledOutResp.TrReqs)))
		err = fmt.Errorf("error found in %d out of %d", errCount, len(cmd.MiddledOutResp.TrReqs))
	}
	cmd.SealTrReqGenStart(ctx)
	cmd.SealTrReqGenEnd(ctx)
	return buildMap, err
}
func (cmd *GenerateTrv2RequestsCmd) GenerateReq(ctx context.Context, trReq *data.TrRequest, key string, shardNum int) (*buildbucketpb.ScheduleBuildRequest, error) {
	var err error

	var TrReqhwDef *api.SwarmingDefinition
	TrReqhwDef = nil

	var schedUnit *api.SchedulingUnit
	schedUnit = nil
	if trReq.NewReq != nil && len(trReq.NewReq.GetSchedulingUnits()) != 0 {
		// '0'ed index because we should always have one hw here. It supports multiple
		// MO should reduce it down to 1 always. The len check is done at MO step.
		schedUnit = trReq.NewReq.GetSchedulingUnits()[0]
	} else {
		// '0'ed index because we should always have one hw here. It supports multiple
		// MO should reduce it down to 1 always. The len check is done at MO step.
		TrReqhwDef = trReq.Req.GetHwDefinition()[0]
	}

	testCases := trReq.Tcs

	cmd.ObserveTrReqGenStart(ctx, trReq, key)

	// Input validations
	if len(testCases) == 0 {
		errStr := "no test is found so, rejecting task"
		logging.Infof(ctx, errStr)
		err = fmt.Errorf(errStr)
		cmd.ObserveTrReqGenFail(ctx, trReq, key, errStr)
		return nil, err

	}

	if trReq.DevicesInfo.LabDevicesCount == 0 {
		logging.Infof(ctx, "no suitable device found to run tests so, rejecting task")
		cmd.ObserveTrReqGenFail(ctx, trReq, key, "rejected, no bots found")
		err := &data.BotParamsRejectedError{Key: key, RejectedDims: trReq.DevicesInfo.Dims}
		return nil, err
	}

	helper := &TrV2ReqHelper{
		schedUnit:            schedUnit,
		trReqHWDef:           TrReqhwDef,
		testCases:            testCases,
		build:                cmd.BuildState,
		suiteInfo:            cmd.InternalTestPlan.SuiteInfo,
		shardNum:             shardNum,
		dynamicRun:           cmd.DynamicRun,
		schedUnitMetadataMap: cmd.schedulingUnitsMetadataMap,
		config:               cmd.Config,
		credentialsFile:      cmd.CredentialsFile,
		isAlRun:              cmd.IsAlRun,
		envVersion:           cmd.EnvVersion,
	}

	req, err := GenerateTrv2Req(ctx, true, helper)
	if err != nil {
		logging.Infof(ctx, "error while generating req: %s", err)
		cmd.ObserveTrReqGenFail(ctx, trReq, key, "unable to build task")
		return nil, errors.Annotate(err, "error while generating req:").Err()
	}

	cmd.ObserveTrReqGenSuccess(ctx, trReq, key)

	return req, nil
}

// ProcessEnumError processes enum error
func (cmd *GenerateTrv2RequestsCmd) ProcessEnumError(ctx context.Context, step *build.Step) error {
	errString := "enumeration error: no test found"
	logging.Infof(ctx, errString)
	step.SetSummaryMarkdown(errString)

	suiteName := suiteName(cmd.MiddledOutResp.SuiteInfo)
	err := &data.EnumerationError{SuiteName: suiteName}

	cmd.TestResults = map[string]*data.TestResults{}
	cmd.TestResults[common.EnumerationErrKey] = &data.TestResults{Suite: suiteName, Key: common.EnumerationErrKey, TopLevelError: err, RequestKey: cmd.RequestKey, Name: suiteName}

	cmd.ObserveEnumerationFailure(ctx)
	return err
}

func GetBoardModelVariantKey(ctx context.Context, trReq *data.TrRequest) (string, error) {
	if trReq.NewReq != nil && len(trReq.NewReq.GetSchedulingUnits()) != 0 {
		// new proto flow
		// '0'ed index because we should always have one unit here. It supports multiple
		// MO should reduce it down to 1 always. The len check is done at MO step.
		schedUnit := trReq.NewReq.GetSchedulingUnits()[0]
		key := GetBoardModelVariantKeyFromTarget(schedUnit.GetPrimaryTarget())
		for _, secondaryTarget := range schedUnit.GetCompanionTargets() {
			secondaryKey := GetBoardModelVariantKeyFromTarget(secondaryTarget)
			key = fmt.Sprintf("%s-%s", key, secondaryKey)
		}

		return key, nil
	}

	// old proto flow
	// TODO (oldProto-azrahman): remove old proto stuffs when schedulingUnits are fully rolled in.
	if trReq.Req == nil || len(trReq.Req.GetHwDefinition()) == 0 {
		// This should not happen. If this happens, we should have flagged input
		// error earlier. Still kept this as a sanity check.
		logging.Infof(ctx, "no hw def is found in req")
		return "", fmt.Errorf("no hw def is found so, rejecting task")
	}

	// '0'ed index because we should always have one hw here. It supports multiple
	// MO should reduce it down to 1 always. The len check is done at MO step.
	TrReqhwDef := trReq.Req.GetHwDefinition()[0]
	board := strings.ToLower(getBuildTargetfromHwDef(TrReqhwDef))
	variant := strings.ToLower(TrReqhwDef.GetVariant())
	model := strings.ToLower(getModelTargetfromHwDef(TrReqhwDef))

	return common.ConstructKey(board, model, variant), nil
}

func GetBoardModelVariantKeyFromTarget(target *api.Target) string {
	board := strings.ToLower(getBuildTargetFromSchedulingTarget(target))
	model := strings.ToLower(getModelFromSchedulingTarget(target))
	variant := strings.ToLower(target.GetSwarmingDef().GetVariant())

	return common.ConstructKey(board, model, variant)
}

// ----- analytics funcs --------
func (cmd *GenerateTrv2RequestsCmd) ObserveCmdStart(ctx context.Context) {
	cmd.StartCmdTime = time.Now()
	bqData := &analytics.BqData{Step: string(cmd.GetCommandType()), Status: analytics.Start}
	analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, bqData, cmd.InternalTestPlan, cmd.BuildState)
}

func (cmd *GenerateTrv2RequestsCmd) ObserveCmdEndSuccess(ctx context.Context) {
	bqData := &analytics.BqData{Step: string(cmd.GetCommandType()), Status: analytics.Success, Duration: float32(time.Since(cmd.StartCmdTime).Seconds())}
	analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, bqData, cmd.InternalTestPlan, cmd.BuildState)
}

func (cmd *GenerateTrv2RequestsCmd) ObserveCmdEndFailure(ctx context.Context) {
	bqData := &analytics.BqData{Step: string(cmd.GetCommandType()), Status: analytics.Fail, Duration: float32(time.Since(cmd.StartCmdTime).Seconds())}
	analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, bqData, cmd.InternalTestPlan, cmd.BuildState)
}

func (cmd *GenerateTrv2RequestsCmd) ObserveEnumerationStart(ctx context.Context) {
	bqData := &analytics.BqData{Step: "Enumeration", Status: analytics.Start, Duration: float32(time.Since(cmd.StartCmdTime).Seconds())}
	analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, bqData, cmd.InternalTestPlan, cmd.BuildState)
}

func (cmd *GenerateTrv2RequestsCmd) ObserveEnumerationSuccess(ctx context.Context) {
	bqData := &analytics.BqData{Step: "Enumeration", Status: analytics.Success, Duration: float32(time.Since(cmd.StartCmdTime).Seconds())}
	analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, bqData, cmd.InternalTestPlan, cmd.BuildState)
}

func (cmd *GenerateTrv2RequestsCmd) ObserveEnumerationFailure(ctx context.Context) {
	bqData := &analytics.BqData{Step: "Enumeration", Status: analytics.Fail, Duration: float32(time.Since(cmd.StartCmdTime).Seconds())}
	analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, bqData, cmd.InternalTestPlan, cmd.BuildState)
}

func (cmd *GenerateTrv2RequestsCmd) ObserveTrReqGenStart(ctx context.Context, req *data.TrRequest, key string) {
	cmd.StartTrReqGenTime = time.Now()
	data := &analytics.TaskData{
		Step:          "GenerateReq",
		DisplayName:   key,
		AnalyticsName: cmd.InternalTestPlan.GetSuiteInfo().GetSuiteRequest().GetAnalyticsName(),
		Status:        analytics.Start,
	}

	data = analytics.CreateTrReqData(ctx, cmd.BQClient, data, req, cmd.InternalTestPlan.GetSuiteInfo(), cmd.BuildState)
	cmd.TrReqsStart = append(cmd.TrReqsStart, data)
}

func (cmd *GenerateTrv2RequestsCmd) SealTrReqGenStart(ctx context.Context) {
	analytics.SoftInsertStepWTrReqBulk(ctx, cmd.BQClient, cmd.TrReqsStart)
}

func (cmd *GenerateTrv2RequestsCmd) SealTrReqGenEnd(ctx context.Context) {
	analytics.SoftInsertStepWTrReqBulk(ctx, cmd.BQClient, cmd.TrReqsEnd)
}

func (cmd *GenerateTrv2RequestsCmd) ObserveTrReqGenFail(ctx context.Context, req *data.TrRequest, key string, err string) {
	data := &analytics.TaskData{
		Step:          "GenerateReq",
		DisplayName:   key,
		AnalyticsName: cmd.InternalTestPlan.GetSuiteInfo().GetSuiteRequest().GetAnalyticsName(),
		Duration:      float32(time.Since(cmd.StartTrReqGenTime).Seconds()),
		Status:        analytics.Fail,
		Freeform:      err,
	}
	data = analytics.CreateTrReqData(ctx, cmd.BQClient, data, req, cmd.InternalTestPlan.SuiteInfo, cmd.BuildState)
	cmd.TrReqsEnd = append(cmd.TrReqsEnd, data)
}

func (cmd *GenerateTrv2RequestsCmd) ObserveTrReqGenSuccess(ctx context.Context, req *data.TrRequest, key string) {
	data := &analytics.TaskData{
		Step:          "GenerateReq",
		DisplayName:   key,
		AnalyticsName: cmd.InternalTestPlan.GetSuiteInfo().GetSuiteRequest().GetAnalyticsName(),
		Duration:      float32(time.Since(cmd.StartTrReqGenTime).Seconds()),
		Status:        analytics.Success,
	}
	data = analytics.CreateTrReqData(ctx, cmd.BQClient, data, req, cmd.InternalTestPlan.SuiteInfo, cmd.BuildState)
	cmd.TrReqsEnd = append(cmd.TrReqsEnd, data)
}

// -------- end analytics funcs ------------

func NewGenerateTrv2RequestsCmd() *GenerateTrv2RequestsCmd {
	abstractCmd := interfaces.NewAbstractCmd(GenerateTrv2RequestsCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &GenerateTrv2RequestsCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}
