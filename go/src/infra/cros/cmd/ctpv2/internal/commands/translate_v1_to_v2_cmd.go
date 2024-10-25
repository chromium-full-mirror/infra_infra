// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"strings"

	build_api "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"
	"google.golang.org/protobuf/types/known/durationpb"

	androidapi "infra/cros/cmd/common_lib/android_api"
	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/common_builders"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/ctpv2/data"
)

// TranslateV1ToV2Cmd represents v1 to v2 translation cmd.
type TranslateV1ToV2Cmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	CtpV1Requests map[string]*test_platform.Request
	CtpV2Request  *api.CTPv2Request // This will be updated if isn't set by deps
	BuildState    *build.State

	// Updates
	RequestToTargetChainMap map[string]map[string]string
	CtpV2RequestMap         map[string]*api.CTPRequest
	DddTrackerMap           map[string]bool

	AlStateInfo *data.AlStateInfo
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *TranslateV1ToV2Cmd) ExtractDependencies(
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
func (cmd *TranslateV1ToV2Cmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.PrePostFilterStateKeeper:
		err = cmd.updateLocalTestStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *TranslateV1ToV2Cmd) extractDepsFromFilterStateKeepr(
	ctx context.Context,
	sk *data.PrePostFilterStateKeeper) error {

	cmd.BuildState = sk.BuildState
	if sk.CtpV2Request == nil || len(sk.CtpV2Request.GetRequests()) == 0 {
		if sk.CtpV1Requests == nil || len(sk.CtpV1Requests) == 0 {
			return fmt.Errorf("Cmd %q missing dependency: Either v1 or v2 request is required!", cmd.GetCommandType())
		} else {
			cmd.CtpV1Requests = sk.CtpV1Requests
		}
	} else {
		cmd.CtpV2Request = sk.CtpV2Request
	}

	cmd.AlStateInfo = sk.AlStateInfo
	if cmd.AlStateInfo == nil {
		cmd.AlStateInfo = &data.AlStateInfo{
			IsAlRun:       false,
			WorkUnitTrees: map[string]*androidapi.WorkUnitNode{},
		}
	}

	return nil
}

func (cmd *TranslateV1ToV2Cmd) updateLocalTestStateKeeper(
	ctx context.Context,
	sk *data.PrePostFilterStateKeeper) error {

	if cmd.CtpV2Request != nil {
		sk.CtpV2Request = cmd.CtpV2Request
	}

	if cmd.CtpV2RequestMap != nil && len(cmd.CtpV2RequestMap) > 0 {
		sk.V1KeyToCTPv2Req = cmd.CtpV2RequestMap
	}

	if cmd.RequestToTargetChainMap != nil && len(cmd.RequestToTargetChainMap) > 0 {
		sk.RequestToTargetChainMap = cmd.RequestToTargetChainMap
	}

	if cmd.DddTrackerMap != nil && len(cmd.DddTrackerMap) > 0 {
		sk.DddTrackerMap = cmd.DddTrackerMap
	}

	if cmd.AlStateInfo != nil {
		sk.AlStateInfo = cmd.AlStateInfo
	}

	return nil
}

// getATPDeps reaches into the request to extract the parentWUID and invocation
// ID of the ATP run.
//
// NOTE: CTPv2 currently only supports one ATP request per builder. If we want
// to support multiple requests then we need to adjust this function to handle
// that.
func getATPDeps(req *api.CTPv2Request) (parentWUID, invocationID string) {
	// If the current request is an AL run then fetch the Invocation ID and the
	// ATP WU ID so that we can build our tree.
	for _, request := range req.GetRequests() {
		if args := request.GetSuiteRequest().GetTestSuite().GetExecutionMetadata().GetArgs(); args != nil {
			for _, arg := range args {
				if arg.GetFlag() == "ants_invocation_id" {
					invocationID = arg.GetValue()
				}
				if arg.GetFlag() == "ants_work_unit_id" {
					parentWUID = arg.GetValue()
				}
			}

		}
	}

	return
}

// Execute executes the command.
func (cmd *TranslateV1ToV2Cmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Translate v1 to v2 request (if required)")
	defer func() { step.End(err) }()

	// exit if v2 is already set
	if cmd.CtpV2Request != nil && len(cmd.CtpV2Request.GetRequests()) > 0 {
		step.SetSummaryMarkdown("V2 request already set so skipping translation...")

		// populate reqs
		for _, ctpReq := range cmd.CtpV2Request.GetRequests() {
			// do ATP msg to ctp request translation
			if ctpReq.GetEncodedAtpTestJobMsg() != "" {
				// if atp encoded test job msg is present, then decode it & construct CTP req from it
				err = cmd.constructCtpReqFromEncodedTestJobMsg(ctx, ctpReq)
				if err != nil {
					logging.Infof(ctx, "err while constructing ctp req from encoded atp test job msg: %s", err.Error())
					return err
				}
			}
		}

		// If the current run is an AL run then pull the ATP details out of the
		// request arguments.
		if parentWUID, invocationID := getATPDeps(cmd.CtpV2Request); parentWUID != "" && invocationID != "" {
			// Set true if not done already.
			cmd.AlStateInfo.IsAlRun = true

			// Generate the top of the tree node to begin the ATP WU tree.
			top, err := androidapi.NewWorkUnitNode(parentWUID, invocationID, androidapi.TestJob, nil, common.GetCTPEnvironment(cmd.BuildState.Build().GetBuilder()))
			if err != nil {
				return err
			}

			// TODO: Set the cached tree inside of the State keeper rather than
			// inside of the API. We also likely want to pass along which node
			// dependant steps should be using rather than giving them the full
			// tree to traverse.
			cmd.AlStateInfo.WorkUnitTrees["test"] = top

			fmt.Printf("top %s: %+v\n", top.GetWorkUnit().Id, top)
		}

		return nil
	}
	common.WriteAnyObjectToStepLog(ctx, step, cmd.CtpV1Requests, "Received CtpV1 Request")
	v1KeysMap := cmd.CreateKeysForEachV1Request()
	common.WriteAnyObjectToStepLog(ctx, step, v1KeysMap, "RequestToBMVTargetKeyMap")

	v2RequestMap, requestChainMap, dddTrackerMap := common_builders.NewCTPV2FromV1(ctx, cmd.CtpV1Requests, cmd.BuildState).BuildRequest()
	common.WriteAnyObjectToStepLog(ctx, step, requestChainMap, "RequestChainMap")
	common.WriteAnyObjectToStepLog(ctx, step, dddTrackerMap, "DddTrackerMap")
	cmd.CtpV2RequestMap = v2RequestMap // will be used to propagate the request key to each invocation
	cmd.DddTrackerMap = dddTrackerMap  // will be used to process test results differently in summarize step

	finalMap := cmd.CreateKeyToBMVTargetChain(v1KeysMap, requestChainMap)
	common.WriteAnyObjectToStepLog(ctx, step, finalMap, "FinalMap")
	cmd.RequestToTargetChainMap = finalMap
	step.SetSummaryMarkdown("Translation succeeded")
	common.WriteAnyObjectToStepLog(ctx, step, cmd.CtpV2RequestMap, "Translated CtpV2 Request Map")

	return err
}

func (cmd *TranslateV1ToV2Cmd) CreateKeyToBMVTargetChain(v1KeysMap map[string]string, requestChainMap map[string][]string) map[string]map[string]string {
	finalMap := map[string]map[string]string{}
	for key, chainedKeys := range requestChainMap {
		bmvToKeyMap := map[string]string{}
		for _, chainedKey := range chainedKeys {
			bmvToKeyMap[v1KeysMap[chainedKey]] = chainedKey
		}
		finalMap[key] = bmvToKeyMap
	}
	return finalMap
}

func (cmd *TranslateV1ToV2Cmd) CreateKeysForEachV1Request() map[string]string {
	if cmd.CtpV1Requests == nil || len(cmd.CtpV1Requests) == 0 {
		return nil
	}
	reqToTargetMap := map[string]string{}
	for reqName, req := range cmd.CtpV1Requests {
		// At this point, there should be one target for each request.
		reqToTargetMap[reqName] = createBoardModelVariantKeyForRequest(req)
	}

	return reqToTargetMap
}

func createBoardModelVariantKeyForRequest(req *test_platform.Request) string {
	board := req.GetParams().GetSoftwareAttributes().GetBuildTarget().GetName()
	model := req.GetParams().GetHardwareAttributes().GetModel()
	variant := common_builders.GetVariant(req.GetParams().GetSoftwareDependencies())

	return common.ConstructKey(board, model, variant)
}

func (cmd *TranslateV1ToV2Cmd) constructCtpReqFromEncodedTestJobMsg(ctx context.Context, ctpReq *api.CTPRequest) error {
	var err error
	step, ctx := build.StartStep(ctx, "Ctp Req From TestJobMsg")
	defer func() { step.End(err) }()
	common.WriteStringToStepLog(ctx, step, ctpReq.GetEncodedAtpTestJobMsg(), "received encoded atp test job msg")

	// Decode the Base64 string
	testJobMsg, err := common.DecodeTestJobMsg(ctx, ctpReq.GetEncodedAtpTestJobMsg())
	if err != nil {
		return err
	}
	logging.Infof(ctx, "successfully decoded test job msg!")
	common.WriteAnyObjectToStepLog(ctx, step, testJobMsg, "decoded atp test job msg")

	// populate the fields from received atp test job msg
	populateCtpRequest(ctpReq, testJobMsg)

	common.WriteProtoToStepLog(ctx, step, ctpReq, "populated_ctp_req")

	return nil
}

func populateCtpRequest(ctpReq *api.CTPRequest, testJobMsg *common.TestJobMessage) {
	ctpReq.SuiteRequest = buildSuiteRequest(testJobMsg)
	ctpReq.ScheduleTargets = buildScheduleTargets(testJobMsg)
	ctpReq.SchedulerInfo = buildSchedulerInfo(testJobMsg)
	ctpReq.Pool = getSchedulingPool(testJobMsg)
	ctpReq.KarbonFilters = getKarbonFilters()
	ctpReq.RunDynamic = true
}

func buildSuiteRequest(testJobMsg *common.TestJobMessage) *api.SuiteRequest {
	// Default values
	suiteName := "adhoc"
	testCaseTagCriteria := &api.TestSuite_TestCaseTagCriteria{}
	antsInvId := ""
	antsWuId := ""
	buildEnv := ""
	totalShards := 0
	retryCount := 0
	maxDuration := &durationpb.Duration{Seconds: 40 * 3600}
	maxInShard := 10
	dddSuite := false

	// build related
	buildId := ""
	branch := ""
	buildFlavor := ""
	buildType := ""
	buildTarget := ""

	extraBuildId := ""
	extraBranch := ""
	extraBuildFlavor := ""
	extraBuildType := ""
	extraBuildTarget := ""

	if testJobMsg.Test != nil {
		suiteName = testJobMsg.Test.Name
		for _, arg := range testJobMsg.Test.Args {
			if arg.Key == "tag_include_list" {
				testCaseTagCriteria.Tags = append(testCaseTagCriteria.Tags, arg.Values...)
			} else if arg.Key == "tag_exclude_list" {
				testCaseTagCriteria.TagExcludes = append(testCaseTagCriteria.TagExcludes, arg.Values...)
			} else if arg.Key == "test_names_include_list" {
				testCaseTagCriteria.TestNames = append(testCaseTagCriteria.TestNames, arg.Values...)
			} else if arg.Key == "test_names_exclude_list" {
				testCaseTagCriteria.TestNameExcludes = append(testCaseTagCriteria.TestNameExcludes, arg.Values...)
			}
		}

		totalShards = int(testJobMsg.Test.Shards)
		retryCount = int(testJobMsg.Test.RunCount) - 1 // RunCount represents total count
	}

	for _, data := range testJobMsg.PluginData {
		if data.Key == "ants_invocation_id" {
			antsInvId = data.Values[0] // if the key is present, there should be only one value
		} else if data.Key == "ants_work_unit_id" {
			antsWuId = data.Values[0] // if the key is present, there should be only one value
		}
	}

	for _, option := range testJobMsg.RunnerOptions {
		if option.Key == "build_environment" {
			buildEnv = option.Values[0] // if the key is present, there should be only one value
		}
	}

	if testJobMsg.Build != nil {
		buildId = testJobMsg.Build.BuildId
		branch = testJobMsg.Build.Branch
		buildFlavor = testJobMsg.Build.BuildFlavor
		buildTarget = testJobMsg.Build.BuildTarget
		buildType = testJobMsg.Build.BuildType
	}

	if len(testJobMsg.ExtraBuilds) != 0 {
		extraBuild := testJobMsg.ExtraBuilds[0] // TODO (azrahman): add multiple extra build support

		extraBuildId = extraBuild.BuildId
		extraBranch = extraBuild.Branch
		extraBuildFlavor = extraBuild.BuildFlavor
		extraBuildTarget = extraBuild.BuildTarget
		extraBuildType = extraBuild.BuildType
	}

	executionMetadata := &api.ExecutionMetadata{
		Args: []*api.Arg{
			{Flag: "ants_invocation_id", Value: antsInvId},
			{Flag: "ants_work_unit_id", Value: antsWuId},
			{Flag: "android_build_environment", Value: buildEnv},
			{Flag: "branch", Value: branch},
			{Flag: "build_flavor", Value: buildFlavor},
			{Flag: "build_id", Value: buildId},
			{Flag: "build_target", Value: buildTarget},
			{Flag: "build_type", Value: buildType},
			{Flag: "extra_branch", Value: extraBranch},
			{Flag: "extra_build_flavor", Value: extraBuildFlavor},
			{Flag: "extra_build", Value: extraBuildId},
			{Flag: "extra_target", Value: extraBuildTarget},
			{Flag: "extra_build_type", Value: extraBuildType},
			{Flag: "skip_ants_upload", Value: "true"}, // TODO(srinivas/james): move this logic to ants-publish-filter
		},
	}
	testSuite := &api.TestSuite{
		Name:              suiteName,
		Spec:              &api.TestSuite_TestCaseTagCriteria_{TestCaseTagCriteria: testCaseTagCriteria},
		ExecutionMetadata: executionMetadata,
		TotalShards:       int64(totalShards)}

	return &api.SuiteRequest{
		SuiteRequest:    &api.SuiteRequest_TestSuite{TestSuite: testSuite},
		MaximumDuration: maxDuration,
		MaxInShard:      int64(maxInShard),
		DddSuite:        dddSuite,
		RetryCount:      int64(retryCount)}
}

func buildScheduleTargets(testJobMsg *common.TestJobMessage) []*api.ScheduleTargets {
	primaryBoard := ""
	models := []string{}
	swarmingDims := []string{} // TODO: (azrahman): add swarming dims support

	// build related
	buildId := ""
	buildTarget := ""
	buildType := "ATP"

	if testJobMsg.TestBench != nil {
		for _, attr := range testJobMsg.TestBench.Attributes {
			kv := strings.Split(attr, "=")
			if len(kv) == 2 { // needs to be exactly 2
				key := kv[0]
				value := kv[1]
				if key == "models" {
					models = append(models, value)
				}
			}
		}

		primaryBoard = testJobMsg.TestBench.RunTarget
	}

	if testJobMsg.Build != nil {
		buildId = testJobMsg.Build.BuildId
		buildTarget = testJobMsg.Build.BuildTarget
		buildType = testJobMsg.Build.BuildType
	}

	installPath := fmt.Sprintf(
		common.AndroidBuildPathFormat,
		buildId, buildTarget, primaryBoard, buildId)

	scheduleTargetsList := []*api.ScheduleTargets{}
	for _, model := range models {
		hwTarget := &api.HWTarget{Target: &api.HWTarget_LegacyHw{LegacyHw: &api.LegacyHW{Board: primaryBoard, Model: model, SwarmingDimensions: swarmingDims}}}
		swTarget := &api.SWTarget{SwTarget: &api.SWTarget_LegacySw{LegacySw: &api.LegacySW{Build: buildType, GcsPath: installPath}}}
		target := &api.Targets{HwTarget: hwTarget, SwTarget: swTarget}
		scheduleTargetsList = append(scheduleTargetsList, &api.ScheduleTargets{Targets: []*api.Targets{target}})
	}

	return scheduleTargetsList
}

func getKarbonFilters() []*api.CTPFilter {
	return []*api.CTPFilter{
		{
			ContainerInfo: &api.ContainerInfo{
				Container: &build_api.ContainerImageInfo{
					Name: "al-provision-filter",
				},
			},
		},
		{
			ContainerInfo: &api.ContainerInfo{
				Container: &build_api.ContainerImageInfo{
					Name: "foil-filter",
				},
			},
		},
		{
			ContainerInfo: &api.ContainerInfo{
				Container: &build_api.ContainerImageInfo{
					Name: "test-finder",
				},
			},
		},
		{
			ContainerInfo: &api.ContainerInfo{
				Container: &build_api.ContainerImageInfo{
					Name: "ants-publish-filter",
				},
			},
		},
	}
}

func buildSchedulerInfo(testJobMsg *common.TestJobMessage) *api.SchedulerInfo {
	return &api.SchedulerInfo{Scheduler: api.SchedulerInfo_SCHEDUKE}
}

func getSchedulingPool(testJobMsg *common.TestJobMessage) string {
	pool := ""

	if testJobMsg.TestBench != nil {
		for _, attr := range testJobMsg.TestBench.Attributes {
			kv := strings.Split(attr, "=")
			if len(kv) == 2 { // needs to be exactly 2
				key := kv[0]
				value := kv[1]
				if key == "pool" {
					pool = value
				}
			}
		}
	}

	return pool
}

// NewTranslateV1toV2Cmd returns a new TranslateV1ToV2Cmd
func NewTranslateV1toV2Cmd() *TranslateV1ToV2Cmd {
	abstractCmd := interfaces.NewAbstractCmd(TranslateV1toV2RequestType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &TranslateV1ToV2Cmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}
