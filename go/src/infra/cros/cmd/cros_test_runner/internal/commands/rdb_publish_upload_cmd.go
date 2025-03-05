// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	testapipb "go.chromium.org/chromiumos/config/go/test/api"
	testapi_metadata "go.chromium.org/chromiumos/config/go/test/api/metadata"
	artifactpb "go.chromium.org/chromiumos/config/go/test/artifact"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/buildbucket/protoutil"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// RdbPublishUploadCmd represents rdb publish upload cmd.
type RdbPublishUploadCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps
	CurrentInvocationId  string
	TesthausURL          string
	Sources              *testapi_metadata.PublishRdbMetadata_Sources
	BaseVariant          map[string]string
	PostProcessResponses *testapi.RunActivitiesResponse

	// Either constructed TestResultForRdb is required,
	TestResultForRdb *artifactpb.TestResult
	// Or all these are required.
	GcsURL        string
	TestResponses *testapipb.CrosTestResponse
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *RdbPublishUploadCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.HwTestStateKeeper:
		err = cmd.extractDepsFromHwTestStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *RdbPublishUploadCmd) extractDepsFromHwTestStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) error {
	var err error

	if sk.CurrentInvocationId == "" {
		return fmt.Errorf("Cmd %q missing dependency: CurrentInvocationId", cmd.GetCommandType())
	}
	if sk.TesthausURL == "" {
		return fmt.Errorf("Cmd %q missing dependency: TesthausURL", cmd.GetCommandType())
	}
	if sk.CftTestRequest == nil {
		return fmt.Errorf("Cmd %q missing dependency: CftTestRequest", cmd.GetCommandType())
	}

	// If TestResultForRdb is not provided, try to construct it.
	if sk.TestResultForRdb == nil {
		logging.Infof(ctx, "Since TestResultForRdb is not provided, cmd will try to construct it.")

		if sk.BuildState == nil {
			return fmt.Errorf("Cmd %q missing dependency: BuildState", cmd.GetCommandType())
		}
		if sk.GcsURL == "" {
			return fmt.Errorf("Cmd %q missing dependency: GcsURL", cmd.GetCommandType())
		}
		if sk.TestResponses == nil {
			return fmt.Errorf("Cmd %q missing dependency: TestResponses", cmd.GetCommandType())
		}

		// Construct testResultProto
		var testResultProtoErr error
		sk.TestResultForRdb, testResultProtoErr = constructTestResultFromStateKeeper(ctx, sk)
		if testResultProtoErr != nil {
			return errors.Annotate(testResultProtoErr, "Cmd %q failed to construct dependency: TestResultForRdb", cmd.GetCommandType()).Err()
		}
	}

	if sk.BaseVariant == nil {
		logging.Infof(ctx, "Since BaseVariant is not provided, cmd will try to construct it.")
		sk.BaseVariant = constructBaseVariantFromStateKeeper(ctx, sk)
	}

	cmd.PostProcessResponses = &testapipb.RunActivitiesResponse{}
	sk.Injectables.LoadInjectables()
	if err = common.Inject(cmd.PostProcessResponses, "", sk.Injectables, common.NewTaskIdentifier(common.PostProcess).GetRPCResponse("runActivities")); err != nil {
		logging.Infof(ctx, "Failed to inject into PostProcessResponses, %s", err)
	}

	cmd.CurrentInvocationId = sk.CurrentInvocationId
	cmd.TestResultForRdb = sk.TestResultForRdb
	cmd.TesthausURL = sk.TesthausURL
	cmd.BaseVariant = sk.BaseVariant

	if sk.CrosTestRunnerRequest != nil {
		cmd.Sources, err = SourcesFromPrimaryDevice(sk)
		if err != nil {
			return errors.Annotate(err, "Cmd %q failed to construct dependency: Sources", cmd.GetCommandType()).Err()
		}
	} else {
		cmd.Sources, err = SourcesFromCFTTestRequest(sk.CftTestRequest)
		if err != nil {
			return errors.Annotate(err, "Cmd %q failed to construct dependency: Sources", cmd.GetCommandType()).Err()
		}
	}

	cmd.GcsURL = sk.GcsURL
	cmd.TestResponses = sk.TestResponses

	return nil
}

// constructBaseVariantFromStateKeeper constructs the base variant of test
// results within an invocation. If there are duplicate keys, the variant value
// given by the test command always wins.
func constructBaseVariantFromStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) map[string]string {
	baseVariant := make(map[string]string)

	// Buildbucket tags
	build := sk.BuildState.Build()
	if build != nil {
		for _, tag := range build.GetTags() {
			if tag.GetKey() == "label-board" {
				baseVariant["board"] = tag.GetValue()
			} else if tag.GetKey() == "label-model" {
				baseVariant["model"] = tag.GetValue()
			}
		}
	}

	// Autotest keyval from CFT test request
	buildTarget := common.GetValueFromRequestKeyvals(ctx, sk.CftTestRequest, sk.CrosTestRunnerRequest, "build_target")
	if buildTarget != "" {
		baseVariant["build_target"] = buildTarget
	}

	return baseVariant
}

func constructTestResultFromStateKeeper(
	ctx context.Context,
	sk *data.HwTestStateKeeper) (*artifactpb.TestResult, error) {
	build := &buildbucketpb.Build{}
	if sk.BuildState != nil {
		build = sk.BuildState.Build()
	}

	botDims := protoutil.MustBotDimensions(build)
	resultProto := &artifactpb.TestResult{}

	// Invocation level info
	populateTestInvocationInfo(ctx, resultProto, sk, botDims, build)

	// Test level info
	populateTestRunsInfo(ctx, resultProto, sk, botDims, build)

	return resultProto, nil
}

// populateTestInvocationInfo populates test invocation info.
func populateTestInvocationInfo(
	ctx context.Context,
	resultProto *artifactpb.TestResult,
	sk *data.HwTestStateKeeper,
	botDims []*buildbucketpb.StringPair,
	build *buildbucketpb.Build) {
	testInv := &artifactpb.TestInvocation{
		IsCftRun:  true,
		IsTrv2Run: true,
	}

	requestedProperties := build.GetInfra().GetBuildbucket().GetRequestedProperties().GetFields()
	if _, ok := requestedProperties["is_al_run"]; ok {
		testInv.IsAlRun = requestedProperties["is_al_run"].GetBoolValue()
	}

	resultProto.TestInvocation = testInv

	// Dut topology
	populateDUTTopology(ctx, testInv, sk)

	// Primary execution info
	populatePrimaryExecutionInfo(ctx, testInv, sk, botDims, build)

	// Secondary execution info
	populateSecondaryExecutionInfo(ctx, testInv, sk, botDims, build)

	// Scheduling metadata
	bbTags := build.GetTags()
	populateSchedulingMetadata(ctx, testInv, bbTags)

	// Project tracker metadata
	populateProjectTrackerMetadata(ctx, testInv, bbTags)

	// Populate Partner related info.
	populatePartnerInfo(ctx, testInv, sk)
}

// populateBuildInfo populates build info.
func populateBuildInfo(
	ctx context.Context,
	executionInfo *artifactpb.ExecutionInfo,
	sk *data.HwTestStateKeeper,
	botDims []*buildbucketpb.StringPair,
	build *buildbucketpb.Build,
	dut *labapi.Dut) {
	buildInfo := &artifactpb.BuildInfo{}
	executionInfo.BuildInfo = buildInfo

	if buildName := common.GetValueFromRequestKeyvals(ctx, sk.CftTestRequest, sk.CrosTestRunnerRequest, "build"); buildName != "" {
		buildInfo.Name = buildName
	}

	// TODO (azrahman): Even though this says build-target, it's always being
	// set to board upstream (since pre trv2). This should be fixed at some point.
	buildTarget := dut.GetChromeos().GetDutModel().GetBuildTarget()
	if buildTarget != "" {
		buildInfo.BuildTarget = buildTarget
		buildInfo.Board = buildTarget
	} else {
		// Falls back to the CTR and CFT requests if it's the primary DUT.
		if dut == sk.PrimaryDevice.GetDut() {
			if buildTarget = common.GetValueFromRequestKeyvals(ctx, nil, sk.CrosTestRunnerRequest, "build_target"); len(buildTarget) == 0 {
				buildTarget = common.GetValueFromRequestKeyvals(ctx, sk.CftTestRequest, sk.CrosTestRunnerRequest, "build_target")
			}

			if len(buildTarget) != 0 {
				buildInfo.BuildTarget = buildTarget
				buildInfo.Board = buildTarget
			}
		}
	}

	// Populate the board type.
	buildInfo.BoardType = "HW"

	// The builder name contains "test_runner_gce" as a substring for all tests
	// running in the VM lab via test_runner_gce.
	builderName := build.GetBuilder().GetBuilder()
	if strings.Contains(builderName, "test_runner_gce") {
		buildInfo.BoardType = "VM"
	}

	populateBuildMetadata(ctx, buildInfo, sk, botDims, dut)
}

// populateBuildMetadata populates build metadata.
func populateBuildMetadata(
	ctx context.Context,
	buildInfo *artifactpb.BuildInfo,
	sk *data.HwTestStateKeeper,
	botDims []*buildbucketpb.StringPair,
	dut *labapi.Dut) {

	// Build metadata
	buildMetadata := &artifactpb.BuildMetadata{}
	buildInfo.BuildMetadata = buildMetadata

	// - Firmware info
	firmwareInfo := &artifactpb.BuildMetadata_Firmware{}
	buildMetadata.Firmware = firmwareInfo

	// TODO (b/270230867): add missing properties when available.
	// ro_fwid, rw_fwid [Dependant on new cft logging service]

	// - Chipset info
	chipsetInfo := &artifactpb.BuildMetadata_Chipset{}
	buildMetadata.Chipset = chipsetInfo

	if wifiChip := getSingleTagValue(botDims, "label-wifi_chip"); wifiChip != "" {
		chipsetInfo.WifiChip = wifiChip
	}

	if wifiRouterModels := getSingleTagValue(botDims, "label-wifi_router_models"); wifiRouterModels != "" {
		chipsetInfo.WifiRouterModels = wifiRouterModels
	}

	// - Kernel info
	kernalInfo := &artifactpb.BuildMetadata_Kernel{}
	buildMetadata.Kernel = kernalInfo

	// TODO (b/270230867): add missing properties when available.
	// kernel_version [Dependant on new cft logging service]

	// - Sku info
	skuInfo := &artifactpb.BuildMetadata_Sku{}
	buildMetadata.Sku = skuInfo
	dlmSKUID := getSingleTagValue(botDims, "label-dlm_sku_id")
	if dlmSKUID != "" {
		skuInfo.DlmSkuId = dlmSKUID
	}

	hwid := getSingleTagValue(botDims, "hwid")
	if hwid != "" {
		skuInfo.Hwid = hwid
	}

	// - Cellular info
	cellularInfo := &artifactpb.BuildMetadata_Cellular{}
	buildMetadata.Cellular = cellularInfo

	if carrier := getSingleTagValue(botDims, "label-carrier"); carrier != "" {
		cellularInfo.Carrier = carrier
	}

	// - Lacros info
	lacrosInfo := &artifactpb.BuildMetadata_Lacros{}
	buildMetadata.Lacros = lacrosInfo

	if ashVersion := common.GetValueFromRequestKeyvals(ctx, sk.CftTestRequest, sk.CrosTestRunnerRequest, "ash_version"); ashVersion != "" {
		lacrosInfo.AshVersion = ashVersion
	}

	if lacrosVersion := common.GetValueFromRequestKeyvals(ctx, sk.CftTestRequest, sk.CrosTestRunnerRequest, "lacros_version"); lacrosVersion != "" {
		lacrosInfo.LacrosVersion = lacrosVersion
	}

	// Populate Chameleon type and connection type info.
	chameleonInfo := &artifactpb.BuildMetadata_ChameleonInfo{}
	buildMetadata.ChameleonInfo = chameleonInfo
	if chameleonTypes := getTagValues(botDims, "label-chameleon_type"); chameleonTypes != nil {
		chameleonInfo.ChameleonType = getChameleonTypes(ctx, chameleonTypes)
	}
	if chameleonConnectionTypes := getTagValues(botDims, "label-chameleon_connection_types"); chameleonConnectionTypes != nil {
		chameleonInfo.ChameleonConnectionTypes = getChameleonConnectionTypes(ctx, chameleonConnectionTypes)
	}

	if dut != nil {
		chromeOSInfo := dut.GetChromeos()
		if chromeOSInfo != nil {
			// - Modem info
			buildMetadata.ModemInfo = chromeOSInfo.ModemInfo

			// - SKU info
			buildMetadata.Sku.HwidSku = chromeOSInfo.Sku
		}
	}
}

// isSkylab returns true if the dut is deployed in internal lab.
func isSkylab(dut *labapi.Dut) bool {
	if dut != nil {
		return !strings.HasPrefix(dut.GetId().GetValue(), "satlab-")
	}
	return true
}

// populateDutInfo populates dut info.
func populateDutInfo(
	ctx context.Context,
	executionInfo *artifactpb.ExecutionInfo,
	sk *data.HwTestStateKeeper,
	dut *labapi.Dut,
	provisionState *testapipb.ProvisionState) {
	dutInfo := &artifactpb.DutInfo{}
	executionInfo.DutInfo = dutInfo

	if dut != nil {
		dutInfo.Dut = dut
	}

	if provisionState != nil {
		dutInfo.ProvisionState = provisionState
	}
}

// populateEnvInfo populates env info.
func populateEnvInfo(
	ctx context.Context,
	executionInfo *artifactpb.ExecutionInfo,
	botDims []*buildbucketpb.StringPair,
	build *buildbucketpb.Build,
	dut *labapi.Dut) {

	// - Drone info
	droneInfo := &artifactpb.DroneInfo{}

	if drone := getSingleTagValue(botDims, "drone"); drone != "" {
		droneInfo.Drone = drone
	}
	if droneServer := getSingleTagValue(botDims, "drone_server"); droneServer != "" {
		droneInfo.DroneServer = droneServer
	}

	// - Swarming info
	swarmingInfo := &artifactpb.SwarmingInfo{}

	if testTaskId := getTaskRequestId(build.GetInfra().GetSwarming().GetTaskId()); testTaskId != "" {
		swarmingInfo.TaskId = testTaskId
	} else if testTaskId := getTaskRequestId(build.GetInfra().GetBackend().GetTask().GetId().GetId()); testTaskId != "" {
		swarmingInfo.TaskId = testTaskId
	}

	if suiteTaskId := getTaskRequestId(getSingleTagValue(build.Tags, "parent_task_id")); suiteTaskId != "" {
		swarmingInfo.SuiteTaskId = suiteTaskId
	}

	buildID := build.GetId()
	builder := build.GetBuilder()
	if builder != nil {
		swarmingInfo.TaskName = fmt.Sprintf("bb-%d-%s/%s/%s", buildID, builder.GetProject(), builder.GetBucket(), builder.GetBuilder())
	}
	if pool := getSingleTagValue(botDims, "pool"); pool != "" {
		swarmingInfo.Pool = pool
	}

	labelPool := getSingleTagValue(botDims, "label-pool")
	if labelPool == "" {
		labelPool = getSingleTagValue(build.Tags, "label-pool")
	}
	if labelPool != "" {
		swarmingInfo.LabelPool = labelPool
	}

	if botID := getSingleTagValue(botDims, "id"); botID != "" {
		swarmingInfo.BotId = botID
	}

	// - BuildBucket info
	bbInfo := &artifactpb.BuildbucketInfo{Id: buildID}

	if builder != nil {
		bbInfo.Builder = &artifactpb.BuilderID{Project: builder.GetProject(), Bucket: builder.GetBucket(), Builder: builder.GetBuilder()}
	}
	if len(build.AncestorIds) > 0 {
		bbInfo.AncestorIds = build.AncestorIds
	} else {
		// Falls back to the bb tag for tests coming from Scheduke because
		// they're not scheduled via a parent BBID call.
		parentBBID := getSingleTagValue(build.Tags, "parent_buildbucket_id")
		if parentBBID != "" {
			if id, err := strconv.ParseInt(parentBBID, 10, 64); err == nil {
				bbInfo.AncestorIds = []int64{id}
			}
		}
	}

	if isSkylab(dut) {
		// Skylab
		skylabInfo := &artifactpb.SkylabInfo{DroneInfo: droneInfo, SwarmingInfo: swarmingInfo, BuildbucketInfo: bbInfo}
		executionInfo.EnvInfo = &artifactpb.ExecutionInfo_SkylabInfo{SkylabInfo: skylabInfo}
	} else {
		// Satlab
		satlabInfo := &artifactpb.SatlabInfo{DroneInfo: droneInfo, SwarmingInfo: swarmingInfo, BuildbucketInfo: bbInfo}
		executionInfo.EnvInfo = &artifactpb.ExecutionInfo_SatlabInfo{SatlabInfo: satlabInfo}
	}
}

// populateInventoryInfo populates inventory info.
func populateInventoryInfo(
	ctx context.Context,
	executionInfo *artifactpb.ExecutionInfo,
	sk *data.HwTestStateKeeper,
	botDims []*buildbucketpb.StringPair) {
	inventoryInfo := &artifactpb.InventoryInfo{}
	executionInfo.InventoryInfo = inventoryInfo

	if ufsZone := getSingleTagValue(botDims, "ufs_zone"); ufsZone != "" {
		inventoryInfo.UfsZone = ufsZone
	}
}

// populateExecutionInfo populates execution info.
func populateExecutionInfo(
	ctx context.Context,
	executionInfo *artifactpb.ExecutionInfo,
	sk *data.HwTestStateKeeper,
	botDims []*buildbucketpb.StringPair,
	build *buildbucketpb.Build,
	dut *labapi.Dut,
	provisionState *testapipb.ProvisionState) {
	// Build info
	populateBuildInfo(ctx, executionInfo, sk, botDims, build, dut)

	// Dut info
	populateDutInfo(ctx, executionInfo, sk, dut, provisionState)

	// Env info
	populateEnvInfo(ctx, executionInfo, botDims, build, dut)

	// Inventory info
	populateInventoryInfo(ctx, executionInfo, sk, botDims)
}

// populatePrimaryExecutionInfo populates primary execution info.
func populatePrimaryExecutionInfo(
	ctx context.Context,
	testInv *artifactpb.TestInvocation,
	sk *data.HwTestStateKeeper,
	botDims []*buildbucketpb.StringPair,
	build *buildbucketpb.Build) {
	primaryExecInfo := &artifactpb.ExecutionInfo{}
	testInv.PrimaryExecutionInfo = primaryExecInfo

	primaryDut := sk.PrimaryDevice.GetDut()
	requestedPrimaryDut := sk.CftTestRequest.GetPrimaryDut()
	if sk.PrimaryDeviceMetadata != nil {
		requestedPrimaryDut = sk.PrimaryDeviceMetadata
	}
	provisionState := requestedPrimaryDut.GetProvisionState()

	populateExecutionInfo(ctx, primaryExecInfo, sk, botDims, build, primaryDut, provisionState)
}

// populateSecondaryExecutionInfo populates secondary execution info.
func populateSecondaryExecutionInfo(
	ctx context.Context,
	testInv *artifactpb.TestInvocation,
	sk *data.HwTestStateKeeper,
	botDims []*buildbucketpb.StringPair,
	build *buildbucketpb.Build) {
	// TODO (azrahman): check if inventory service actually provides these duts info
	// or not for multi-duts. If not, raise this issue to proper channel.
	companionDevicesMetadata := sk.CompanionDevicesMetadata
	secondaryExecInfos := []*artifactpb.ExecutionInfo{}
	for i, device := range sk.CompanionDevices {
		secondaryExecInfo := &artifactpb.ExecutionInfo{}
		secondaryDUT := device.GetDut()
		secondaryProvisionState := companionDevicesMetadata[i].GetProvisionState()
		populateExecutionInfo(ctx, secondaryExecInfo, sk, botDims, build, secondaryDUT, secondaryProvisionState)

		secondaryExecInfos = append(secondaryExecInfos, secondaryExecInfo)
	}
	testInv.SecondaryExecutionsInfo = secondaryExecInfos
}

// populateSchedulingMetadata populates scheduling metadata.
func populateSchedulingMetadata(
	ctx context.Context,
	testInv *artifactpb.TestInvocation,
	tags []*buildbucketpb.StringPair) {
	schedulingArgs := map[string]string{}
	for _, tag := range tags {
		schedulingArgs[tag.GetKey()] = tag.Value
	}

	if len(schedulingArgs) != 0 {
		testInv.SchedulingMetadata =
			&artifactpb.SchedulingMetadata{
				SchedulingArgs: schedulingArgs,
			}
	}
}

// populateDUTTopology populates DUT topology.
func populateDUTTopology(
	ctx context.Context,
	testInv *artifactpb.TestInvocation,
	sk *data.HwTestStateKeeper) {
	testInv.DutTopology = &labapi.DutTopology{
		Duts: []*labapi.Dut{},
	}
	if sk.DutTopology != nil {
		testInv.DutTopology.Id = sk.DutTopology.GetId()
	}

	// Populates the DUTs with the primary DUT as the first in the list.
	duts := make([]*labapi.Dut, 0, 1+len(sk.CompanionDevices))
	duts = append(duts, sk.PrimaryDevice.GetDut())
	if companionDevices := sk.CompanionDevices; len(companionDevices) > 0 {
		for _, device := range companionDevices {
			duts = append(duts, device.GetDut())
		}
	}
	testInv.DutTopology.Duts = duts
}

// populateProjectTrackerMetadata populates project tracker metadata.
func populateProjectTrackerMetadata(
	ctx context.Context,
	testInv *artifactpb.TestInvocation,
	tags []*buildbucketpb.StringPair) {
	projectTrackerMetadata := &artifactpb.ProjectTrackerMetadata{}
	testInv.ProjectTrackerMetadata = projectTrackerMetadata

	for _, tag := range tags {
		if tag.GetKey() == "bug_id" {
			projectTrackerMetadata.BugId = tag.GetValue()

			// Remove the break if adding more fields within the loop.
			break
		}
	}
}

// populatePartnerInfo populates partner info.
func populatePartnerInfo(
	ctx context.Context,
	testInv *artifactpb.TestInvocation,
	sk *data.HwTestStateKeeper) {
	testInv.PartnerInfo = &artifactpb.PartnerInfo{}
	if sk.CommonConfig != nil {
		if partnerInfo := sk.CommonConfig.GetPartnerConfig(); partnerInfo != nil {
			if accountId := partnerInfo.GetAccountId(); accountId > 0 {
				testInv.PartnerInfo.AccountId = accountId
			}
		}
	}
}

// populateTestRunsInfo populates test runs info.
func populateTestRunsInfo(
	ctx context.Context,
	resultProto *artifactpb.TestResult,
	sk *data.HwTestStateKeeper,
	botDims []*buildbucketpb.StringPair,
	build *buildbucketpb.Build) {
	testCaseResults := sk.TestResponses.GetTestCaseResults()
	if testCaseResults == nil {
		return
	}

	testRuns := make([]*artifactpb.TestRun, 0, len(testCaseResults))
	for _, testCaseResult := range testCaseResults {
		testRun := &artifactpb.TestRun{}
		populateTestRun(ctx, testRun, testCaseResult, sk, build)
		testRuns = append(testRuns, testRun)
	}

	resultProto.TestRuns = testRuns
}

// populateTestRun populates test run.
func populateTestRun(
	ctx context.Context,
	testRun *artifactpb.TestRun,
	testCaseResult *testapipb.TestCaseResult,
	sk *data.HwTestStateKeeper,
	build *buildbucketpb.Build,
) {
	testRun.LogsInfo = []*_go.StoragePath{{HostType: _go.StoragePath_GS, Path: sk.GcsURL}}

	populateTestCaseInfo(ctx, testRun, testCaseResult, sk, build)

	populateTimeInfo(testRun, testCaseResult, build)

	populateExecutionMetadata(testRun, sk.CftTestRequest, sk.CrosTestRunnerRequest)
}

// populateTestCaseInfo populates test case info per test run.
func populateTestCaseInfo(
	ctx context.Context,
	testRun *artifactpb.TestRun,
	testCaseResult *testapipb.TestCaseResult,
	sk *data.HwTestStateKeeper,
	build *buildbucketpb.Build,
) {
	testCaseInfo := &artifactpb.TestCaseInfo{}
	testCaseInfo.TestCaseResult = testCaseResult

	suite := common.GetValueFromRequestKeyvals(ctx, sk.CftTestRequest, sk.CrosTestRunnerRequest, "suite")
	if suite == "" {
		suite = getSingleTagValue(build.Tags, "suite")
	}
	if suite != "" {
		testCaseInfo.Suite = suite
	}

	branch := common.GetValueFromRequestKeyvals(ctx, sk.CftTestRequest, sk.CrosTestRunnerRequest, "branch")
	if branch != "" {
		testCaseInfo.Branch = branch
	}

	mainBuilderName := common.GetValueFromRequestKeyvals(ctx, sk.CftTestRequest, sk.CrosTestRunnerRequest, "master_build_config")
	if mainBuilderName != "" {
		testCaseInfo.MainBuilderName = mainBuilderName
	}

	channel := getSingleTagValue(build.Tags, "branch-trigger")
	if channel != "" {
		testCaseInfo.Channel = channel
	}

	displayName := getSingleTagValue(build.Tags, "display_name")
	if displayName != "" {
		testCaseInfo.DisplayName = displayName
	}

	// Fetches the requester from the user tag of the current build first.
	// If not existed, falls back to the parent_created_by tag which indicates
	// the requester for the parent build.
	requester := getSingleTagValue(build.Tags, "user")
	if requester == "" {
		requester = getSingleTagValue(build.Tags, "parent_created_by")
	}
	if requester != "" {
		requester, _ = strings.CutPrefix(requester, "user:")
		testCaseInfo.Requester = requester
	}

	testRun.TestCaseInfo = testCaseInfo
}

// populateTestCaseInfo populates time info per test run.
func populateTimeInfo(
	testRun *artifactpb.TestRun,
	testCaseResult *testapipb.TestCaseResult,
	build *buildbucketpb.Build,
) {
	testRun.TimeInfo = &artifactpb.TimingInfo{
		QueuedTime:  build.GetCreateTime(),
		StartedTime: testCaseResult.GetStartTime(),
		Duration:    testCaseResult.GetDuration(),
	}
}

// populateExecutionMetadata populates execution metadata per test run.
func populateExecutionMetadata(testRun *artifactpb.TestRun, cftTestRequest *skylab_test_runner.CFTTestRequest, dynamicRequest *api.CrosTestRunnerDynamicRequest) {
	if dynamicRequest != nil {
		populateExecutionMetadataForTestSuites(testRun, dynamicRequest.GetParams().GetTestSuites())
	}
	if cftTestRequest != nil {
		populateExecutionMetadataForTestSuites(testRun, cftTestRequest.GetTestSuites())
	}
}

func populateExecutionMetadataForTestSuites(testRun *artifactpb.TestRun, testSuites []*testapipb.TestSuite) {
	// Returns early if no test suite is provided.
	if len(testSuites) == 0 {
		return
	}

	// Returns early if the first test suite is empty.
	firstTestSuite := testSuites[0]
	if firstTestSuite == nil || len(firstTestSuite.GetTestCaseIds().GetTestCaseIds()) == 0 {
		return
	}

	// Returns early if no execution metadata is specified.
	suiteExecMetadata := firstTestSuite.GetExecutionMetadata()
	if suiteExecMetadata == nil {
		return
	}

	args := suiteExecMetadata.GetArgs()
	testArgs := make(map[string]string, len(args))
	for _, arg := range args {
		testArgs[arg.GetFlag()] = arg.GetValue()
	}

	testRun.ExecutionMetadata = &artifactpb.ExecutionMetadata{
		TestArgs: testArgs,
	}
}

// getTagValues gets tag values from provided string pairs.
func getTagValues(tags []*buildbucketpb.StringPair, key string) []string {
	values := []string{}
	if len(tags) == 0 {
		return values
	}

	for _, tag := range tags {
		if tag.GetKey() == key {
			values = append(values, tag.GetValue())
		}
	}

	return values
}

// getSingleTagValue gets the first value found from provided string pairs.
func getSingleTagValue(tags []*buildbucketpb.StringPair, key string) string {
	values := getTagValues(tags, key)
	if len(values) > 0 {
		return values[0]
	} else {
		return ""
	}
}

// getChameleonTypes gets the corresponding BuildMetadata_ChameleonTypeType
// based on given string set
func getChameleonTypes(ctx context.Context, chameleonTypeStrings []string) []artifactpb.BuildMetadata_ChameleonType {
	chameleonTypes := make([]artifactpb.BuildMetadata_ChameleonType, 0, len(chameleonTypeStrings))
	for _, str := range chameleonTypeStrings {
		if i, exists := artifactpb.BuildMetadata_ChameleonType_value[str]; exists {
			chameleonType := artifactpb.BuildMetadata_ChameleonType(i)
			chameleonTypes = append(chameleonTypes, chameleonType)
		} else {
			logging.Infof(ctx, "Unrecognized chameleon type: %s", str)
		}
	}
	return chameleonTypes
}

// getChameleonConnectionTypes gets the corresponding BuildMetadata_ChameleonConnectionType
// based on given string set
func getChameleonConnectionTypes(ctx context.Context, chameleonConnectionStrings []string) []artifactpb.BuildMetadata_ChameleonConnectionType {
	chameleonConnectionTypes := make([]artifactpb.BuildMetadata_ChameleonConnectionType, 0, len(chameleonConnectionStrings))
	for _, str := range chameleonConnectionStrings {
		if i, exists := artifactpb.BuildMetadata_ChameleonConnectionType_value[str]; exists {
			chameleonConnectionType := artifactpb.BuildMetadata_ChameleonConnectionType(i)
			chameleonConnectionTypes = append(chameleonConnectionTypes, chameleonConnectionType)
		} else {
			logging.Infof(ctx, "Unrecognized chameleon connection type: %s", str)
		}
	}
	return chameleonConnectionTypes
}

// getTaskRequestId converts the swarming task run id with non "0" suffix to the swarming task
// request id with "0" suffix. Both can be used to point to the same swarming
// task. Swarming supported implicit retry and first task has "1" in suffix and
// retried task has "2" in suffix.
func getTaskRequestId(taskId string) string {
	if taskId == "" {
		return ""
	}

	return fmt.Sprintf("%s0", taskId[:len(taskId)-1])
}

// SourcesFromPrimaryDevice returns the code sources tested in the given
// testing request assuming provision was successful and the version did
// not shcnage during the test.
func SourcesFromPrimaryDevice(sk *data.HwTestStateKeeper) (*testapi_metadata.PublishRdbMetadata_Sources, error) {
	provisionState := sk.PrimaryDeviceMetadata.GetProvisionState()
	if provisionState == nil {
		// Invalid request.
		return nil, errors.Reason("CFTTestRequest: primary_dut.provision_state missing").Err()
	}

	// The path to the system image.
	imagePath := provisionState.SystemImage.GetSystemImagePath()
	if imagePath == nil || imagePath.GetHostType() != _go.StoragePath_GS {
		// For non-GS stored build outputs (e.g. local files),
		// we do not have information about the sources used.
		return nil, nil
	}
	if !strings.HasPrefix(imagePath.GetPath(), "gs://") {
		return nil, errors.Reason("CFTTestRequest: primary_dut.provision_state.system_image.system_image_path.path: must start with gs://").Err()
	}
	if strings.HasSuffix(imagePath.GetPath(), "/") {
		return nil, errors.Reason("CFTTestRequest: primary_dut.provision_state.system_image.system_image_path.path: must not have trailing '/'").Err()
	}
	return &testapi_metadata.PublishRdbMetadata_Sources{
		// Path to the file in Google Cloud Storage that contains
		// information about the code sources built into the build.
		GsPath: imagePath.Path + common.SourceMetadataPath,
		// If custom firmware is used or custom packages are deployed
		// that were not built as part of the Chrome OS image (e.g. Lacros
		// testing or firmware testing), the test is not a pure test
		// of the build sources.
		IsDeploymentDirty: provisionState.GetFirmware() != nil || len(provisionState.GetPackages()) > 0,
	}, nil
}

// SourcesFromCFTTestRequest returns the code sources tested in the given
// CFT testing request.
func SourcesFromCFTTestRequest(request *skylab_test_runner.CFTTestRequest) (*testapi_metadata.PublishRdbMetadata_Sources, error) {
	if request == nil {
		return nil, errors.Reason("CFTTestRequest: missing").Err()
	}
	provisionState := request.GetPrimaryDut().GetProvisionState()
	if provisionState == nil {
		// Invalid request.
		return nil, errors.Reason("CFTTestRequest: primary_dut.provision_state missing").Err()
	}

	// The path to the system image.
	imagePath := provisionState.SystemImage.GetSystemImagePath()
	if imagePath == nil || imagePath.GetHostType() != _go.StoragePath_GS {
		// For non-GS stored build outputs (e.g. local files),
		// we do not have information about the sources used.
		return nil, nil
	}
	if !strings.HasPrefix(imagePath.GetPath(), "gs://") {
		return nil, errors.Reason("CFTTestRequest: primary_dut.provision_state.system_image.system_image_path.path: must start with gs://").Err()
	}
	if strings.HasSuffix(imagePath.GetPath(), "/") {
		return nil, errors.Reason("CFTTestRequest: primary_dut.provision_state.system_image.system_image_path.path: must not have trailing '/'").Err()
	}

	return &testapi_metadata.PublishRdbMetadata_Sources{
		// Path to the file in Google Cloud Storage that contains
		// information about the code sources built into the build.
		GsPath: imagePath.Path + common.SourceMetadataPath,
		// If custom firmware is used or custom packages are deployed
		// that were not built as part of the Chrome OS image (e.g. Lacros
		// testing or firmware testing), the test is not a pure test
		// of the build sources.
		IsDeploymentDirty: provisionState.GetFirmware() != nil || len(provisionState.GetPackages()) > 0,
	}, nil
}

func NewRdbPublishUploadCmd(executor interfaces.ExecutorInterface) *RdbPublishUploadCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(RdbPublishUploadCmdType, executor)
	cmd := &RdbPublishUploadCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
