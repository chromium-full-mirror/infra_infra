// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// DISCLAIMER: Bunch of TODOs and lots of commented code in this file.
// Keeping this file as helper for now until this whole file/trv2 request
// generator is stable. The todos will be resolved over time while getting data
// from new input fields and introducing functionalities. The commented out
// codes are from CTPv1 that will help with making sure no data/info are
// getting missed while resolving those functions. Then these functions will be
// moved to common_lib.

package commands

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	goconfig "go.chromium.org/chromiumos/config/go"
	gobuildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/common_builders"
	"infra/cros/cmd/common_lib/dynamic_updates"
	"infra/cros/cmd/ctpv2/data"
	"infra/libs/skylab/inventory"
	"infra/libs/skylab/request"
	"infra/libs/skylab/worker"
)

const (
	// This deadline is constructed from various CTP req params that we do not
	// want to depend on for ctpv2. So hardcoding them for now so that they are in
	// one place. And later may move to new input params for ctpv2 or configs if
	// required.
	// TODO (azrahman): revisit this.
	DefaultTimeout = 8 * time.Hour // Intentionally put a large number for now so
	// that reqs don't timeout due to this.

	CtpRequestUIDTemplate = "TestPlanRuns/%d/%s"
	DutPoolQuota          = "DUT_POOL_QUOTA"
)

type TrV2ReqHelper struct {
	// Control Variables
	dynamicRun bool

	// Top Level Variables
	schedUnit            *testapi.SchedulingUnit
	trReqHWDef           *testapi.SwarmingDefinition // TODO (oldProto-azrahman): remove when new proto fully rolls in
	testCases            []*testapi.TestCase_Id
	suiteInfo            *testapi.SuiteInfo
	shardNum             int
	build                *build.State
	schedUnitMetadataMap map[string][]*testapi.SchedulingUnit
	config               *config.Config
	credentialsFile      string
	// envVersion denotes whether the environment
	// is prod or something else.
	envVersion string

	// Other fields often used several times throughout.
	suiteName        string
	primaryTarget    *HwTarget
	secondaryTargets []*HwTarget
	pool             string
	currBBID         int64
	maxDuration      time.Duration
	lookupTable      map[string]string

	analyticsName    string
	parentRequestUID string
	currSwarmingID   string
	builderStr       string
	isAlRun          bool
}

type HwTarget struct {
	board           string
	model           string
	variant         string
	boardWVaraint   string
	provisionInfo   []*testapi.ProvisionInfo
	gcsArtifactPath string // if cros type
	apiTarget       *api.Target
}

// FakeHwTarget is for testing since fields are private.
func FakeHwTarget(board, model, variant string) *HwTarget {
	return &HwTarget{
		board:   board,
		model:   model,
		variant: variant,
	}
}

// GenerateTrv2Req generates ScheduleBuildRequest.
func GenerateTrv2Req(ctx context.Context, canOutliveParent bool, trHelper *TrV2ReqHelper, isLED bool) (*buildbucketpb.ScheduleBuildRequest, error) {
	populateHelper(ctx, trHelper)
	err := populateHelper(ctx, trHelper)
	if err != nil {
		return nil, errors.Annotate(err, "unable to build up context: ").Err()
	}

	// Create bb request
	reqArgs, err := GenerateArgs(ctx, trHelper)
	if err != nil {
		return nil, errors.Annotate(err, "error while creating req: ").Err()
	}
	testRunnerBuildID := common.TestRunnerBuilderID(trHelper.config)
	// Check for VM.
	runAsVM := common.IsSupportedVMBoard(trHelper.primaryTarget.board)
	if runAsVM {
		testRunnerBuildID.Builder = common.ConvertBuilderNameToVM(testRunnerBuildID.Builder)
	}
	req, err := reqArgs.NewBBRequest(testRunnerBuildID)
	if err != nil {
		return nil, err
	}
	if canOutliveParent {
		req.CanOutliveParent = buildbucketpb.Trinary_YES
	} else {
		req.CanOutliveParent = buildbucketpb.Trinary_NO
	}

	if runAsVM {
		req.Dimensions = []*buildbucketpb.RequestedDimension{
			{
				Key:   "role",
				Value: "vmlab",
			},
		}
	}

	if isLED {
		req.ShadowInput = &buildbucketpb.ScheduleBuildRequest_ShadowInput{
			InheritFromParent: true,
		}
	}

	return req, nil
}

func getPool(suiteInfo *testapi.SuiteInfo) string {
	if suiteInfo.GetSuiteMetadata().GetPool() != "" {
		return suiteInfo.GetSuiteMetadata().GetPool()
	}
	return DutPoolQuota
}

func populateHwTarget(ctx context.Context, target *testapi.Target, suiteInfo *testapi.SuiteInfo) (*HwTarget, error) {
	board := getBuildTargetFromSchedulingTarget(target)
	model := getModelFromSchedulingTarget(target)
	variant := target.GetSwarmingDef().GetVariant()
	return populateHwTargetHelper(ctx, board, model, variant, suiteInfo, target.GetSwarmingDef().GetDutInfo(), target)
}

func populateHelperOldProto(ctx context.Context, trHelper *TrV2ReqHelper) error {
	board := getBuildTargetfromHwDef(trHelper.trReqHWDef)
	model := getModelTargetfromHwDef(trHelper.trReqHWDef)
	variant := trHelper.trReqHWDef.GetVariant()
	target, err := populateHwTargetHelper(ctx, board, model, variant, trHelper.suiteInfo, trHelper.trReqHWDef.GetDutInfo(), nil)
	if err != nil {
		return err
	}
	trHelper.primaryTarget = target
	target.provisionInfo, trHelper.lookupTable = findProvisionInfo(ctx, trHelper)

	return nil
}

func populateHelperNewProto(ctx context.Context, trHelper *TrV2ReqHelper) error {
	target, err := populateHwTarget(ctx, trHelper.schedUnit.GetPrimaryTarget(), trHelper.suiteInfo)
	if err != nil {
		return err
	}
	companionTargets := []*HwTarget{}
	for _, companion := range trHelper.schedUnit.GetCompanionTargets() {
		target, err := populateHwTarget(ctx, companion, trHelper.suiteInfo)
		if err != nil {
			return err
		}
		companionTargets = append(companionTargets, target)
	}

	companionTargets = strictestTargetsFirst(companionTargets)
	schedUnit := FindSchedulingUnit(target, companionTargets, trHelper.schedUnitMetadataMap)
	if schedUnit == nil {
		return fmt.Errorf("failed to find scheduling unit match")
	}
	trHelper.lookupTable = schedUnit.GetDynamicUpdateLookupTable()
	trHelper.primaryTarget = target
	trHelper.secondaryTargets = companionTargets

	return nil
}

// FindSchedulingUnit matches scheduling targets by board-variant, then attempts to find by
// model. If the requested target has a model then try to match on model, fall back to match on
// no model. If no request model, choose first match.
func FindSchedulingUnit(primary *HwTarget, companions []*HwTarget, schedUnitMetadataMap map[string][]*testapi.SchedulingUnit) *api.SchedulingUnit {
	primaryKey := primary.board + "-" + primary.variant
	// If no scheduling unit is found in the map which matched the board, exit immediately.
	schedulingUnitCandidates, ok := schedUnitMetadataMap[primaryKey]
	if !ok {
		return nil
	}
	var matchedSchedUnit *api.SchedulingUnit
	for _, schedulingUnitCandidate := range schedulingUnitCandidates {
		_, model, _ := targetToBoardModelVariant(schedulingUnitCandidate.GetPrimaryTarget())
		// If not requested model, models must match.
		/*
			Example:
				Candidates = [octopus, octopus.phaser]
				Requested = octopus.phaser

				Loop:
				- i = 0 (octopus)
					* Fails initial model match on octopus as phaser != "" and phaser != "" (model).
					* Passes fall through check as model == "" but no match has been found yet.
					* Match gets set to octopus.
				- i = 1 (octopus.phaser)
					* Passes initial model match as phaser != "" and phaser == phaser (model).
					* Match gets set to octopus.phaser.
				Returns octopus.phaser as match.
		*/
		if primary.model != "" && primary.model != model {
			// However, if candidate has no model, allow through
			// if we haven't found any match.
			if model != "" || matchedSchedUnit != nil {
				continue
			}
		}
		primary.provisionInfo = schedulingUnitCandidate.GetPrimaryTarget().GetSwarmingDef().GetProvisionInfo()

		// Check if companions all have a match.
		companionsPool := []*HwTarget{}
		companionsPool = append(companionsPool, companions...)
		for _, companionCandidate := range schedulingUnitCandidate.GetCompanionTargets() {
			matchIndex := findCompanionMatch(companionsPool, companionCandidate)
			if matchIndex == -1 {
				continue
			}
			companionsPool[matchIndex].provisionInfo = companionCandidate.GetSwarmingDef().GetProvisionInfo()
			companionsPool = slices.Delete(companionsPool, matchIndex, matchIndex+1)
		}
		// Continue if not all companions found a match.
		if len(companionsPool) > 0 {
			continue
		}
		matchedSchedUnit = schedulingUnitCandidate
	}
	return matchedSchedUnit
}

// buildSchedUnitMap converts the metadata scheduling units into
// a map keyed by board-variant.
func buildSchedUnitMap(suiteInfo *api.SuiteInfo) map[string][]*api.SchedulingUnit {
	schedMap := map[string][]*api.SchedulingUnit{}

	for _, schedUnit := range suiteInfo.GetSuiteMetadata().GetSchedulingUnits() {
		board, _, variant := targetToBoardModelVariant(schedUnit.PrimaryTarget)
		key := board + "-" + variant
		if _, ok := schedMap[key]; !ok {
			schedMap[key] = []*api.SchedulingUnit{}
		}
		schedMap[key] = append(schedMap[key], schedUnit)
	}

	return schedMap
}

// findCompanionMatch matches companions to the candidate based
// on board-variant, and same model matching as primary.
func findCompanionMatch(companions []*HwTarget, candidate *api.Target) int {
	board, model, variant := targetToBoardModelVariant(candidate)
	matchedIndex := -1
	for i, companion := range companions {
		// Must match on board and variant.
		if companion.board != board && companion.variant != variant {
			continue
		}
		// If companion request has model, must match model.
		if companion.model != "" && companion.model != model {
			// Fallback onto empty model if no match has been found yet.
			if model != "" || matchedIndex != -1 {
				continue
			}
		}
		matchedIndex = i
	}

	return matchedIndex
}

// strictestTargetsFirst orders the targets list by their board/model/variant
// provided. Targets with all three provided should be matched first. Prioritize
// variant, then model, then board.
//
// Scores:
//
//	No model -> 0
//	Model -> 1
//
// Bucket sort seems appropriate.
func strictestTargetsFirst(targets []*HwTarget) []*HwTarget {
	strictnessBuckets := [][]*HwTarget{
		{}, {},
	}
	for _, target := range targets {
		score := 0
		if target.model != "" {
			score += 1
		}
		strictnessBuckets[score] = append(strictnessBuckets[score], target)
	}
	res := []*HwTarget{}
	for _, bucket := range strictnessBuckets {
		res = append(bucket, res...)
	}
	return res
}

func targetToBoardModelVariant(target *api.Target) (string, string, string) {
	return strings.ToLower(getBuildTargetFromSchedulingTarget(target)),
		strings.ToLower(getModelFromSchedulingTarget(target)),
		strings.ToLower(target.GetSwarmingDef().GetVariant())
}

func populateHelper(ctx context.Context, trHelper *TrV2ReqHelper) error {
	if trHelper.schedUnit != nil {
		// new proto flow (supports multi-dut)
		err := populateHelperNewProto(ctx, trHelper)
		if err != nil {
			return err
		}

	} else if trHelper.trReqHWDef != nil {
		// TODO(oldProto-azrahman): remove when the new proto is rolled in
		// old proto flow (doesn't support multi dut)
		err := populateHelperOldProto(ctx, trHelper)
		if err != nil {
			return err
		}
	}
	trHelper.suiteName = trHelper.suiteInfo.GetSuiteRequest().GetTestSuite().GetName()
	trHelper.pool = getPool(trHelper.suiteInfo)
	trHelper.currBBID = trHelper.build.Build().GetId()
	trHelper.builderStr = getBuildFromGcsPath(trHelper.primaryTarget.gcsArtifactPath)
	trHelper.parentRequestUID = fmt.Sprintf(CtpRequestUIDTemplate, trHelper.currBBID, trHelper.suiteName)
	trHelper.currSwarmingID = os.Getenv("SWARMING_TASK_ID")
	if trHelper.currSwarmingID == "" {
		logging.Infof(ctx, "SWARMING_TASK_ID NOT FOUND")
	}
	trHelper.analyticsName = trHelper.suiteInfo.GetSuiteRequest().GetAnalyticsName()
	if trHelper.suiteInfo.GetSuiteRequest().GetMaximumDuration() == nil {
		trHelper.maxDuration = DefaultTimeout
	} else {
		trHelper.maxDuration = trHelper.suiteInfo.GetSuiteRequest().GetMaximumDuration().AsDuration()
	}

	return nil
}

func populateHwTargetHelper(ctx context.Context, board string, model string, variant string, suiteInfo *api.SuiteInfo, dutInfo *labapi.Dut, apiTarget *api.Target) (*HwTarget, error) {
	hwTarget := &HwTarget{apiTarget: apiTarget}
	hwTarget.board = strings.ToLower(board)
	hwTarget.model = strings.ToLower(model)
	hwTarget.variant = strings.ToLower(variant)
	hwTarget.boardWVaraint = hwTarget.board
	if variant != "" {
		hwTarget.boardWVaraint = fmt.Sprintf("%s-%s", hwTarget.board, hwTarget.variant)
	}

	hwTarget.gcsArtifactPath = findGcsPath(suiteInfo, board, variant)
	if hwTarget.gcsArtifactPath == "" {
		logging.Infof(ctx, "GcsPath was not found for build target: %s", hwTarget.boardWVaraint)
		// if the type is not cros, then ignore
		switch dutType := dutInfo.GetDutType().(type) {
		case *labapi.Dut_Chromeos:
			return hwTarget, fmt.Errorf("GcsPath was not found for build target: %s", hwTarget.boardWVaraint)
		default:
			logging.Infof(ctx, "Ignoring gcsPath err for non-cros type: %s", dutType)
		}
	}

	return hwTarget, nil
}

// GenerateArgs generates args for the builder request.
func GenerateArgs(ctx context.Context, trHelper *TrV2ReqHelper) (*request.Args, error) {
	if trHelper.build == nil {
		return nil, fmt.Errorf("No Build Object set in helper.")
	}
	args := request.Args{
		Cmd:               *createCommand(ctx, trHelper),
		SwarmingPool:      trHelper.pool,
		Dimensions:        createFreeformDims(trHelper),
		ParentTaskID:      trHelper.currSwarmingID,
		ParentRequestUID:  trHelper.parentRequestUID,
		ParentBuildID:     trHelper.currBBID,
		Priority:          10,
		TestRunnerRequest: nil,  // Always nil for CFT.
		CFTIsEnabled:      true, // Always true
		Timeout:           trHelper.maxDuration,
		Experiments:       trHelper.build.Build().GetInput().Experiments,
		GerritChanges:     trHelper.build.Build().GetInput().GerritChanges,
		ResultsConfig:     nil, // TODO (azrahman): Investigate if we need this.
		IsALRun:           trHelper.isAlRun,
	}

	if trHelper.config.GetTestRunner().GetSwarmingPool() != "" {
		args.SwarmingPool = trHelper.config.GetTestRunner().GetSwarmingPool()
	}

	labels, err := createLabels(trHelper)
	if err != nil {
		return nil, errors.Annotate(err, "error while creating labels: ").Err()
	}
	args.SchedulableLabels = labels

	secondaryLabels, err := createSecondaryLabels(trHelper)
	if err != nil {
		return nil, errors.Annotate(err, "error while creating secondary labels: ").Err()
	}
	args.SecondaryDevicesLabels = secondaryLabels

	if trHelper.dynamicRun {
		dynamicRequest, err := createDynamicTrv2Request(ctx, trHelper)
		if err != nil {
			return nil, err
		}
		args.DynamicTestRunnerRequest = dynamicRequest
	} else {
		cftTestRequest, err := createCftTestRequest(ctx, trHelper)
		if err != nil {
			return nil, err
		}
		args.CFTTestRunnerRequest = cftTestRequest
	}

	tags, err := createSwarmingTags(ctx, trHelper)
	if err != nil {
		return nil, errors.Annotate(err, "error while creating tags: ").Err()
	}
	args.SwarmingTags = tags

	return &args, nil
}

// createCommand creates cmd for the builder request.
func createCommand(ctx context.Context, trHelper *TrV2ReqHelper) *worker.Command {
	keyvals := make(map[string]string)
	keyvals["suite"] = trHelper.suiteName
	keyvals["label"] = trHelper.suiteName

	cmd := &worker.Command{
		ClientTest:      false,
		Deadline:        time.Now().UTC().Add(trHelper.maxDuration),
		Keyvals:         keyvals,
		OutputToIsolate: true,
		TaskName:        trHelper.suiteName,
	}

	// This was retrieved from input in ctpv1 but turns out this was always the same.
	logdogConfig := &config.Config_SkylabWorker{
		LuciProject: "chromeos",
		LogDogHost:  "luci-logdog.appspot.com",
	}
	cmd.Config(data.Wrap(logdogConfig))

	return cmd
}

func getFreeFormDimsForTarget(target *api.Target) []string {
	dims := []string{}
	hwId := target.GetSwarmingDef().GetDutInfo().GetChromeos().GetHwid()
	if hwId != "" {
		dims = append(dims, fmt.Sprintf("hwid:%s", hwId))
	}

	for _, label := range target.GetSwarmingDef().GetSwarmingLabels() {
		dims = append(dims, formatLabel(label))
	}
	return dims
}

// createFreeformDims creates free form dims from swarming def.
func createFreeformDims(trv2ReqHelper *TrV2ReqHelper) []string {
	if trv2ReqHelper.schedUnit != nil {
		// new proto flow
		primaryTarget := trv2ReqHelper.schedUnit.GetPrimaryTarget()

		freeformDims := []string{"dut_state:ready"}
		freeformDims = append(freeformDims, getFreeFormDimsForTarget(primaryTarget)...)

		// secondary targets should not have any swarming labels.
		// hence not adding any from them.

		return freeformDims
	}

	// TODO (oldProt-azrahman): remove
	// old proto flow
	tRRequesthwDef := trv2ReqHelper.trReqHWDef
	freeformDims := []string{"dut_state:ready"}
	if tRRequesthwDef.GetDutInfo().GetChromeos().GetHwid() != "" {
		freeformDims = append(freeformDims, fmt.Sprintf("hwid:%s", tRRequesthwDef.GetDutInfo().GetChromeos().GetHwid()))
	}

	for _, v := range tRRequesthwDef.GetSwarmingLabels() {
		freeformDims = append(freeformDims, formatLabel(v))
	}
	return freeformDims
}

func formatLabel(label string) string {
	if strings.HasPrefix(label, "label") || strings.HasPrefix(label, "dut_name") || strings.HasPrefix(label, "drone") || strings.HasPrefix(label, "bot") {
		return label
	} else {
		return fmt.Sprintf("label-%s", label)
	}
}

func getProvisionInfoFromTarget(target *api.Target, board string, variant string) []*testapi.ProvisionInfo {
	if strings.ToLower(getBuildTargetFromSchedulingTarget(target)) == board && strings.ToLower(target.GetSwarmingDef().GetVariant()) == variant {
		return target.GetSwarmingDef().GetProvisionInfo()
	}
	return nil
}

func getGcsPathFromProvisionInfos(provInfos []*testapi.ProvisionInfo) string {
	for _, provInfo := range provInfos {
		if provInfo.GetType() == testapi.ProvisionInfo_CROS {
			return provInfo.GetInstallRequest().GetImagePath().GetPath()
		}
	}

	return ""
}

func findGcsPathFromTarget(target *api.Target, board string, variant string) string {
	provInfos := getProvisionInfoFromTarget(target, board, variant)
	if provInfos != nil {
		return getGcsPathFromProvisionInfos(provInfos)
	}

	return ""
}

// findGcsPath finds gcs path for provided board.
// This is based on the given board + id; then looping through the suite metadata to find
// the target which matched these. We then will return the GCS path from there.
func findGcsPath(suiteInfo *testapi.SuiteInfo, board string, variant string) string {

	schedUnits := suiteInfo.GetSuiteMetadata().GetSchedulingUnits()
	if schedUnits != nil && len(schedUnits) != 0 {
		// new proto flow
		for _, schedUnit := range schedUnits {
			// search primary target first
			if gcsPath := findGcsPathFromTarget(schedUnit.PrimaryTarget, board, variant); gcsPath != "" {
				return gcsPath
			}
			// search secondary targets
			for _, secondary := range schedUnit.CompanionTargets {
				if gcsPath := findGcsPathFromTarget(secondary, board, variant); gcsPath != "" {
					return gcsPath
				}
			}
		}
		return ""
	}

	// TODO (oldproto-azrahman): remove this when new proto rolls in
	for _, suiteTarget := range suiteInfo.GetSuiteMetadata().GetTargetRequirements() {
		// This is [0] indexed because we are ignoring multi-dut today.

		suiteDef := suiteTarget.GetHwRequirements().GetHwDefinition()
		if len(suiteDef) == 0 {
			return ""
		}
		suiteHwDef := suiteDef[0]
		if getBuildTargetfromHwDef(suiteHwDef) == board && suiteHwDef.GetVariant() == variant {
			provInfos := suiteHwDef.GetProvisionInfo()
			for _, provInfo := range provInfos {
				if provInfo.GetType() == testapi.ProvisionInfo_CROS {
					return provInfo.GetInstallRequest().GetImagePath().GetPath()
				}
			}
		}
	}
	return ""
}

func findProvisionInfo(ctx context.Context, trHelper *TrV2ReqHelper) ([]*testapi.ProvisionInfo, map[string]string) {
	logging.Infof(ctx, "looking for provision info for board: %s, variant: %s", trHelper.primaryTarget.board, trHelper.primaryTarget.variant)
	logging.Infof(ctx, "looking for provision info for suiteMD: %s", trHelper.suiteInfo.GetSuiteMetadata())

	for _, suiteTarget := range trHelper.suiteInfo.GetSuiteMetadata().GetTargetRequirements() {
		// This is [0] indexed because we are ignoring multi-dut today.

		suiteDef := suiteTarget.GetHwRequirements().GetHwDefinition()
		if len(suiteDef) == 0 {
			return nil, map[string]string{}
		}

		suiteHwDef := suiteDef[0]
		logging.Infof(ctx, "looking for provision info for suiteInfo: %s", getBuildTargetfromHwDef(suiteHwDef))

		suiteSwDef := suiteTarget.GetSwRequirement()
		logging.Infof(ctx, "looking for provision info for suiteInfo: %s", suiteSwDef)

		if getBuildTargetfromHwDef(suiteHwDef) == trHelper.primaryTarget.board && suiteHwDef.GetVariant() == trHelper.primaryTarget.variant {
			return suiteHwDef.GetProvisionInfo(), suiteHwDef.GetDynamicUpdateLookupTable()
		}
	}
	return nil, map[string]string{}
}

// ----- TODO (oldProt-azrahman): remove oldProto func defs -----
func getBuildTargetfromHwDef(TRRequesthwDef *testapi.SwarmingDefinition) string {
	return TRRequesthwDef.GetDutInfo().GetChromeos().GetDutModel().GetBuildTarget()
}

func getModelTargetfromHwDef(TRRequesthwDef *testapi.SwarmingDefinition) string {
	return TRRequesthwDef.GetDutInfo().GetChromeos().GetDutModel().GetModelName()
}
func getBuildTargetWVariantfromHwDef(TRRequesthwDef *testapi.SwarmingDefinition) string {
	if TRRequesthwDef.GetVariant() == "" {
		return getBuildTargetfromHwDef(TRRequesthwDef)
	}
	return fmt.Sprintf("%s-%s", getBuildTargetfromHwDef(TRRequesthwDef), TRRequesthwDef.GetVariant())
}

// --------------------

func getBuildTargetFromSchedulingTarget(target *testapi.Target) string {
	return common.DutModelFromDut(target.GetSwarmingDef().GetDutInfo()).GetBuildTarget()
}

func getModelFromSchedulingTarget(target *testapi.Target) string {
	return common.DutModelFromDut(target.GetSwarmingDef().GetDutInfo()).GetModelName()
}

func getBuildTargetWVariantFromSchedulingTarget(target *testapi.Target) string {
	if target.GetSwarmingDef().GetVariant() == "" {
		return getBuildTargetFromSchedulingTarget(target)
	}
	return fmt.Sprintf("%s-%s", getBuildTargetFromSchedulingTarget(target), target.GetSwarmingDef().GetVariant())
}

func createDynamicTrv2Request(ctx context.Context, trHelper *TrV2ReqHelper) (*api.CrosTestRunnerDynamicRequest, error) {
	testSuites := []*testapi.TestSuite{
		{
			Name: trHelper.suiteName,
			Spec: &testapi.TestSuite_TestCaseIds{
				TestCaseIds: &testapi.TestCaseIdList{
					TestCaseIds: trHelper.testCases,
				},
			},
			ExecutionMetadata: trHelper.suiteInfo.GetSuiteMetadata().GetExecutionMetadata(),
		},
	}

	keyvals := make(map[string]string)
	keyvals["suite"] = trHelper.suiteName

	// TODO (dbeckett) we need the int of the shard # passed into the gofunc.
	keyvals["label"] = fmt.Sprintf("%s/%s/%s-shard-%d", trHelper.builderStr, trHelper.suiteName, trHelper.suiteName, trHelper.shardNum) // ex: dedede-release/R126-15863.0.0/wifi_cross_device_multidut_flaky/wifi_cross_device_multidut_flaky-shard-0
	keyvals["build"] = trHelper.builderStr                                                                                              // Required for rdb-publish
	keyvals["build_target"] = trHelper.primaryTarget.board
	keyvals["parent_job_id"] = trHelper.currSwarmingID
	keyvals[common.LabelPool] = trHelper.pool

	gsSourcePath := ""
	if path, ok := trHelper.lookupTable["installPath"]; ok {
		gsSourcePath = path + "/metadata/sources.jsonpb"
	}

	primary, companions := createDutModelFromTargets(trHelper.primaryTarget, trHelper.secondaryTargets)
	deadline := time.Now().UTC().Add(trHelper.maxDuration)
	builder := common_builders.DynamicTrv2Builder{
		ParentBuildId:        trHelper.currBBID,
		ParentRequestUid:     trHelper.parentRequestUID,
		GcsArtifactPath:      trHelper.primaryTarget.gcsArtifactPath,
		ContainerMetadataKey: trHelper.primaryTarget.boardWVaraint,
		CredentialsFile:      trHelper.credentialsFile,
		EnvVersion:           trHelper.envVersion,
		BuildString:          trHelper.builderStr,
		Deadline:             timestamppb.New(deadline),
		TestSuites:           testSuites,
		PrimaryDut:           primary,
		CompanionDuts:        companions,
		Keyvals:              keyvals,
		OrderedTaskBuilders: []common_builders.DynamicTaskBuilder{
			common_builders.DefaultDynamicTestTaskWrapper(common.CrosTest),
			common_builders.DefaultDynamicPostProcessTaskWrapper(),
			common_builders.DefaultDynamicRdbPublishTaskWrapper(gsSourcePath, false),
			common_builders.DefaultDynamicGcsPublishTask,
		},
	}

	dynamicRequest, err := builder.BuildRequest(ctx, trHelper.isAlRun)
	if err != nil {
		return nil, errors.Annotate(err, "failed to build base dynamic request").Err()
	}

	if err = dynamic_updates.AddUserDefinedDynamicUpdates(
		dynamicRequest,
		trHelper.suiteInfo.SuiteMetadata.DynamicUpdates,
		trHelper.lookupTable); err != nil {

		return nil, errors.Annotate(err, "failed to add user defined dynamic updates to trv2 request").Err()
	}

	if err = dynamic_updates.AddUserDefinedDynamicUpdates(
		dynamicRequest,
		trHelper.schedUnit.GetSecondaryDynamicUpdates(),
		trHelper.lookupTable); err != nil {

		return nil, errors.Annotate(err, "failed to add secondary user defined dynamic updates to trv2 request").Err()
	}

	return dynamicRequest, err
}

// createCftTestRequest creates cft test request.
func createCftTestRequest(ctx context.Context, trHelper *TrV2ReqHelper) (*skylab_test_runner.CFTTestRequest, error) {
	containerGcsPath := trHelper.primaryTarget.gcsArtifactPath + common.ContainerMetadataPath
	containerMetadata, err := common.FetchContainerMetadata(ctx, containerGcsPath)
	if err != nil {
		logging.Infof(ctx, "error while fetching container metadata: %s", err)
		return nil, err
	}

	testSuites := []*testapi.TestSuite{
		{
			Name: trHelper.suiteName,
			Spec: &testapi.TestSuite_TestCaseIds{
				TestCaseIds: &testapi.TestCaseIdList{
					TestCaseIds: trHelper.testCases,
				},
			},
			ExecutionMetadata: trHelper.suiteInfo.GetSuiteMetadata().GetExecutionMetadata(),
		},
	}

	keyvals := make(map[string]string)
	keyvals["suite"] = trHelper.suiteName

	// TODO (dbeckett) we need the int of the shard # passed into the gofunc.
	keyvals["label"] = fmt.Sprintf("%s/%s/%s-shard-%d", trHelper.builderStr, trHelper.suiteName, trHelper.suiteName, trHelper.shardNum) // ex: dedede-release/R126-15863.0.0/wifi_cross_device_multidut_flaky/wifi_cross_device_multidut_flaky-shard-0
	keyvals["build"] = trHelper.builderStr                                                                                              // Required for rdb-publish
	keyvals["build_target"] = trHelper.primaryTarget.board
	keyvals["parent_job_id"] = trHelper.currSwarmingID

	primaryDut, err := createCftDeviceRequestFromTarget(trHelper.primaryTarget)
	if err != nil {
		return nil, err
	}

	transateTrv2Request := false
	if primaryDut.GetProvisionState().Firmware != nil {
		transateTrv2Request = true
	}
	companionDuts := []*skylab_test_runner.CFTTestRequest_Device{}
	for _, secondary := range trHelper.secondaryTargets {
		secondaryDut, err := createCftDeviceRequestFromTarget(secondary)
		if err != nil {
			return nil, err
		}
		companionDuts = append(companionDuts, secondaryDut)
		if secondaryDut.GetProvisionState().Firmware != nil {
			transateTrv2Request = true
		}
	}

	deadline := time.Now().UTC().Add(trHelper.maxDuration)
	cftTestRequest := &skylab_test_runner.CFTTestRequest{
		Deadline:                     timestamppb.New(deadline),
		ParentRequestUid:             trHelper.parentRequestUID,
		ParentBuildId:                trHelper.currBBID,
		PrimaryDut:                   primaryDut,
		CompanionDuts:                companionDuts,
		ContainerMetadata:            common_builders.PatchContainerMetadata(ctx, containerMetadata, trHelper.builderStr, trHelper.credentialsFile, trHelper.envVersion),
		TestSuites:                   testSuites,
		DefaultTestExecutionBehavior: test_platform.Request_Params_NON_CRITICAL,
		AutotestKeyvals:              keyvals,
		RunViaTrv2:                   true,
		TranslateTrv2Request:         transateTrv2Request,
		StepsConfig:                  nil,
	}

	return cftTestRequest, nil
}

// This method is currently incomplete, its basically just taking the gcs path from the given info.
// it will migrate to the dynamic TRv2 stuff in the near future.
// TODO (oldProto-azrahman): remove old proto
func buildProvisionStateOldProto(provInfo []*testapi.ProvisionInfo) (*testapi.ProvisionState, error) {
	if len(provInfo) == 0 {
		return nil, fmt.Errorf("No Provision Info items given")
	}
	gcsPath := provInfo[0].GetInstallRequest().GetImagePath().GetPath()
	if gcsPath == "" {
		return nil, fmt.Errorf("No gcs path found found")
	}

	provisionState := &testapi.ProvisionState{

		SystemImage: &testapi.ProvisionState_SystemImage{
			SystemImagePath: &goconfig.StoragePath{
				HostType: goconfig.StoragePath_GS,
				Path:     gcsPath,
			},
		},
		ProvisionMetadata: nil,
	}
	return provisionState, nil
}

// createDutModelFromTargets forms DutModels for the
// primary and companion targets.
func createDutModelFromTargets(primaryTarget *HwTarget, companionTargets []*HwTarget) (*labapi.DutModel, []*labapi.DutModel) {
	companions := []*labapi.DutModel{}
	for _, companionTarget := range companionTargets {
		companions = append(companions, createDutModelFromTarget(companionTarget))
	}

	return createDutModelFromTarget(primaryTarget), companions
}

// createDutModelFromTarget forms a DutModel for the target.
func createDutModelFromTarget(target *HwTarget) *labapi.DutModel {
	return &labapi.DutModel{
		BuildTarget: target.board,
		ModelName:   target.model,
	}
}

func createCftDeviceRequestFromTarget(target *HwTarget) (*skylab_test_runner.CFTTestRequest_Device, error) {
	var err error
	dutModel := createDutModelFromTarget(target)

	var provisionState *testapi.ProvisionState
	provisionState = nil
	if common.IsCros(target.board) {
		if target.apiTarget == nil {
			// TODO (oldProto-azrahman): remove old proto
			// old proto flow
			provisionState, err = buildProvisionStateOldProto(target.provisionInfo)
		} else {
			// new proto flow
			provisionState, err = buildProvisionStateOldProto(target.provisionInfo)
		}

		if err != nil {
			return nil, err
		}
	} else if common.IsAndroid((target.board)) {
		provisionState, err = buildAndroidProvisionState(target.apiTarget)
		if err != nil {
			return nil, err
		}
	}

	if provisionState == nil {
		return nil, fmt.Errorf("nil provisionState!")
	}

	tryAttachFirmwareConfig(provisionState, target.apiTarget)

	return &skylab_test_runner.CFTTestRequest_Device{
		DutModel:             dutModel,
		ProvisionState:       provisionState,
		ContainerMetadataKey: target.boardWVaraint,
	}, nil
}

func tryAttachFirmwareConfig(provisionState *testapi.ProvisionState, target *api.Target) {
	imageBucket := common_builders.DefaultChromeosBuildGcsBucket
	firmwareRO := ""
	firmwareRW := ""

	kvs := target.GetSwReq().GetKeyValues()

	for _, kv := range kvs {
		if kv.Key == common_builders.RoFirmwareBuild {
			firmwareRO = kv.Value
		}
		if kv.Key == common_builders.RwFirmwareBuild {
			firmwareRW = kv.Value
		}
		if kv.Key == common_builders.ChromeosBuildGcsBucket {
			imageBucket = kv.Value
		}
	}

	provisionState.Firmware = buildFirmwareConfig(firmwareRO, firmwareRW, imageBucket)
}

// getBuildFromGcsPath gets build from gcs path.
func getBuildFromGcsPath(gcsPath string) string {
	dirNames := strings.Split(gcsPath, "/")
	if len(dirNames) < 2 {
		return ""
	}
	return dirNames[len(dirNames)-2] + "/" + dirNames[len(dirNames)-1]
}

// createProvisionableDimensions creates provisionalbe dims.
func createProvisionableDimensions() ([]string, error) {
	dims := []string{}
	// TODO (azrahman): add support post mvp. Should get these info from provision
	// filter.
	// dimChromeOS      = "provisionable-cros-version"
	// dimFirmwareRO    = "provisionable-fwro-version"
	// dimFirmwareRW    = "provisionable-fwrw-version"
	// dimLacrosGCSPath = "provisionable-lacros-gcs-path"

	// if b := builds.ChromeOS; b != "" {
	// 	dims = append(dims, dimChromeOS+":"+b)
	// }
	// if b := builds.FirmwareRO; b != "" {
	// 	dims = append(dims, dimFirmwareRO+":"+b)
	// }
	// if b := builds.FirmwareRW; b != "" {
	// 	dims = append(dims, dimFirmwareRW+":"+b)
	// }
	// if b := builds.LacrosGCSPath; b != "" {
	// 	dims = append(dims, dimLacrosGCSPath+":"+b)

	return dims, nil
}

// createLabels creates labels.
func createLabels(trHelper *TrV2ReqHelper) (*inventory.SchedulableLabels, error) {
	labels := &inventory.SchedulableLabels{}

	// TODO (azrahman): Revisit this.
	// Gotta come back to this:
	// https://logs.chromium.org/logs/chromeos/led/azrahman_google.com/5553cf70b91da45971ba1857ead3fa96fed5297323cf7b084f5a07f9722ade50/+/u/ctpv2/u/step/24/log/2

	// 1. Get test.Dependencies and convert the autotest labels to dut labels
	// deps := g.Invocation.Test.Dependencies
	// flatDims := make([]string, len(deps))
	// for i, dep := range deps {
	// 	flatDims[i] = dep.Label
	// }
	// labels.Revert(flatDims)
	// Sol: 1) handle it ctpv2 via test_finder

	// 2. Add buildTarget and model (Possible from middle out response)
	// inv., inv.Model
	// TODO (azrahman): Handle non chromeos type.

	labels.Board = &trHelper.primaryTarget.board
	labels.Model = &trHelper.primaryTarget.model

	if trHelper.pool == "" || trHelper.pool == DutPoolQuota {
		labels.CriticalPools = append(labels.CriticalPools, inventory.SchedulableLabels_DUT_POOL_QUOTA)
	} else if trHelper.pool != "" {
		labels.SelfServePools = append(labels.SelfServePools, trHelper.pool)
	} else {
		return nil, fmt.Errorf("no pool specified")
	}

	// TODO (azrahman): revisit this.
	// 4. Add device stability?
	// if g.Params.GetHardwareAttributes().GetRequireStableDevice() {
	// 	*inv.Stability = true
	// }

	return labels, nil
}

// createSecondaryLabels creates secondary labels.
func createSecondaryLabels(trHelper *TrV2ReqHelper) ([]*inventory.SchedulableLabels, error) {

	// TODO (azrahman): populate this for multi-dut use-case.
	// 1. Add secondary board and model

	// sds := g.Params.GetSecondaryDevices()
	// var sInvLabels []*inventory.SchedulableLabels
	// for _, sd := range sds {
	// 	il := inventory.NewSchedulableLabels()
	// 	if sd.GetSoftwareAttributes().GetBuildTarget() != nil {
	// 		*il.Board = sd.SoftwareAttributes.BuildTarget.Name
	// 	}
	// 	if sd.GetHardwareAttributes().GetModel() != "" {
	// 		*il.Model = sd.HardwareAttributes.Model
	// 	}
	// 	sInvLabels = append(sInvLabels, il)
	// }
	// return sInvLabels

	if trHelper.schedUnit != nil {
		invLabels := []*inventory.SchedulableLabels{}

		for _, secondary := range trHelper.schedUnit.GetCompanionTargets() {
			il := inventory.NewSchedulableLabels()
			board := getBuildTargetFromSchedulingTarget(secondary)
			model := getModelFromSchedulingTarget(secondary)
			if board != "" {
				*il.Board = board
			}
			if model != "" {
				*il.Model = model
			}
			invLabels = append(invLabels, il)
		}

		return invLabels, nil
	}

	return []*inventory.SchedulableLabels{{}}, nil
}

// createSwarmingTags creates swarming tags.
func createSwarmingTags(ctx context.Context, trHelper *TrV2ReqHelper) ([]string, error) {
	tags := []string{}

	// add board, models
	if trHelper.primaryTarget.board != "" {
		tags = append(tags, "label-board:"+trHelper.primaryTarget.board)
		tags = append(tags, "primary_board:"+trHelper.primaryTarget.board)
	}
	if trHelper.primaryTarget.model != "" {
		tags = append(tags, "label-model:"+trHelper.primaryTarget.model)
		tags = append(tags, "primary_model:"+trHelper.primaryTarget.model)
	}

	// add tags for multiDut
	secondaryBooards := []string{}
	secondaryModels := []string{}
	for _, secondary := range trHelper.secondaryTargets {
		if secondary.board != "" {
			secondaryBooards = append(secondaryBooards, secondary.board)
		}
		if secondary.model != "" {
			secondaryModels = append(secondaryModels, secondary.model)
		}
	}

	if len(secondaryBooards) > 0 {
		tags = append(tags, "secondary_boards:"+strings.Join(secondaryBooards, ","))
	}
	if len(secondaryModels) > 0 {
		tags = append(tags, "secondary_models:"+strings.Join(secondaryModels, ","))
	}

	// qs account
	qsAccount := trHelper.suiteInfo.GetSuiteMetadata().GetSchedulerInfo().GetQsAccount()
	if qsAccount == "" {
		qsAccount = "unmanaged_p2"
		logging.Infof(ctx, "no qsAccount given, defaulting to unmanaged_p2.")
	}
	tags = append(tags, "qs_account:"+qsAccount)

	// pool
	tags = append(tags, "label-pool:"+trHelper.pool)

	// suite
	if trHelper.suiteName != "" {
		tags = append(tags, "label-suite:"+trHelper.suiteName)
		tags = append(tags, "suite:"+trHelper.suiteName)
	}

	// parent swarming id
	if trHelper.currSwarmingID != "" {
		tags = append(tags, "parent_task_id:"+trHelper.currSwarmingID)
	}

	// parent created by
	if trHelper.build.Build().GetCreatedBy() != "" {
		tags = append(tags, "parent_created_by:"+trHelper.build.Build().GetCreatedBy())
	}

	// TODO should we un-hardcode this?
	tags = append(tags, "luci_project:"+"chromeos")

	if trHelper.analyticsName != "" {
		tags = append(tags, "analytics_name:"+trHelper.analyticsName)
		tags = append(tags, "ctp-fwd-task-name:"+trHelper.analyticsName)
	}

	tags = append(tags, "build:"+trHelper.builderStr)

	if trHelper.currBBID != 0 {
		tags = append(tags, fmt.Sprintf("parent_buildbucket_id:%v", trHelper.currBBID))
	} else {
		tags = append(tags, "parent_buildbucket_id:0")
	}

	// TODO(dbeckett) THESE BELOW:
	reprName := fmt.Sprintf("shard-%v", trHelper.shardNum)
	tags = append(tags, "display_name:"+makeDisplayName(trHelper.builderStr, trHelper.suiteName, reprName))
	// tags = append(tags, removeReservedTags(g.Params.GetDecorations().GetTags())...)
	// // Add primary/secondary DUTs board/model info in swarming tags for
	// // multi-DUTs result reporting purpose.
	// tags = append(tags, g.multiDutsTags()...)

	return tags, nil
}

func makeDisplayName(buildStr string, suite string, TRName string) string {
	return fmt.Sprintf("%s/%s-%s", buildStr, suite, TRName)
}

func buildCrosProvisionState(target *api.Target) (*testapi.ProvisionState, error) {
	if target == nil {
		return nil, fmt.Errorf("nil target")
	}
	provInfo := target.GetSwarmingDef().GetProvisionInfo()[0]
	if provInfo == nil {
		return nil, fmt.Errorf("No Provision Info items given")
	}
	gcsPath := provInfo.GetInstallRequest().GetImagePath().GetPath()
	if gcsPath == "" {
		return nil, fmt.Errorf("No gcs path found found")
	}

	provisionState := &testapi.ProvisionState{

		SystemImage: &testapi.ProvisionState_SystemImage{
			SystemImagePath: &goconfig.StoragePath{
				HostType: goconfig.StoragePath_GS,
				Path:     gcsPath,
			},
		},
		ProvisionMetadata: nil,
	}
	return provisionState, nil
}

func buildAndroidProvisionState(target *api.Target) (*testapi.ProvisionState, error) {
	if target == nil {
		return nil, fmt.Errorf("nil target")
	}

	androidProvisionRequestMetadata := &testapi.AndroidProvisionRequestMetadata{}
	gmsCorePackage := ""
	androidImageVersion := ""

	kvs := target.GetSwReq().GetKeyValues()

	for _, kv := range kvs {
		if kv.Key == common_builders.GmsCorePackage {
			gmsCorePackage = kv.Value
		}
		if kv.Key == common_builders.AndroidImageVersion {
			androidImageVersion = kv.Value
		}
	}

	if gmsCorePackage != "" {
		androidProvisionRequestMetadata.CipdPackages = []*testapi.CIPDPackage{
			{
				AndroidPackage: 1,
				VersionOneof: &testapi.CIPDPackage_Ref{
					Ref: gmsCorePackage,
				},
			},
		}
	}

	if androidImageVersion != "" {
		androidProvisionRequestMetadata.AndroidOsImage = &testapi.AndroidOsImage{
			LocationOneof: &testapi.AndroidOsImage_OsVersion{
				OsVersion: androidImageVersion,
			},
		}
	}

	provisionMetadata, err := anypb.New(androidProvisionRequestMetadata)
	if err != nil {
		return nil, err
	}

	return &testapi.ProvisionState{ProvisionMetadata: provisionMetadata}, nil
}

func buildFirmwareConfig(firmwareRo, firmwareRw, imageBucket string) *gobuildapi.FirmwareConfig {
	firmwarePathFormat := "gs://%s/%s/firmware_from_source.tar.bz2"
	if (firmwareRo == "" || firmwareRo == "None") && (firmwareRw == "" || firmwareRw == "None") {
		return nil
	}

	var ro *gobuildapi.FirmwarePayload
	var rw *gobuildapi.FirmwarePayload
	if firmwareRo != "" && firmwareRo != "None" {
		ro = &gobuildapi.FirmwarePayload{
			FirmwareImage: &gobuildapi.FirmwarePayload_FirmwareImagePath{
				FirmwareImagePath: &goconfig.StoragePath{
					HostType: goconfig.StoragePath_GS,
					Path:     fmt.Sprintf(firmwarePathFormat, imageBucket, firmwareRo),
				},
			},
		}
	}
	if firmwareRw != "" && firmwareRw != "None" {
		rw = &gobuildapi.FirmwarePayload{
			FirmwareImage: &gobuildapi.FirmwarePayload_FirmwareImagePath{
				FirmwareImagePath: &goconfig.StoragePath{
					HostType: goconfig.StoragePath_GS,
					Path:     fmt.Sprintf(firmwarePathFormat, imageBucket, firmwareRw),
				},
			},
		}
	}

	return &gobuildapi.FirmwareConfig{
		MainRoPayload: ro,
		EcRoPayload:   ro,
		MainRwPayload: rw,
	}
}

func suiteName(suiteInfo *testapi.SuiteInfo) string {
	return suiteInfo.GetSuiteRequest().GetTestSuite().GetName()
}
