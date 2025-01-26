// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/jsonpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/auth"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/luciexe/build"
	resultpb "go.chromium.org/luci/resultdb/proto/v1"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/ctpv2/data"
)

// FilterExecutionCmd represents test execution cmd.
type TranslateRequestCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	CtpReq     *testapi.CTPRequest
	RequestKey string
	IsAlRun    bool

	// Updates
	InternalTestPlan *testapi.InternalTestplan
	TestResults      map[string]*data.TestResults

	ExecutionError error
	IsPartnerRun   bool
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *TranslateRequestCmd) ExtractDependencies(
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
func (cmd *TranslateRequestCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.updateLocalTestStateKeeper(sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *TranslateRequestCmd) extractDepsFromFilterStateKeeper(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	if sk.CtpReq == nil {
		return fmt.Errorf("Cmd %q missing dependency: CtpReq", cmd.GetCommandType())
	}

	if sk.AlStateInfo == nil {
		logging.Warningf(ctx, "cmd %q missing optional dependency: AlStateInfo", cmd.GetCommandType())
	}

	if sk.RequestKey == "" {
		logging.Warningf(ctx, "cmd %q missing optional dependency: RequestKey", cmd.GetCommandType())
	}

	cmd.CtpReq = sk.CtpReq
	cmd.RequestKey = sk.RequestKey
	cmd.IsAlRun = sk.IsAlRun
	cmd.IsPartnerRun = sk.IsPartnerRun
	cmd.ExecutionError = sk.ExecutionError
	return nil
}

func (cmd *TranslateRequestCmd) updateLocalTestStateKeeper(sk *data.FilterStateKeeper) error {
	if cmd.InternalTestPlan != nil {
		sk.InitialInternalTestPlan = cmd.InternalTestPlan
	}

	if len(cmd.TestResults) > 0 {
		for k, v := range cmd.TestResults {
			sk.SuiteTestResults[k] = v
		}
	}

	sk.ExecutionError = cmd.ExecutionError
	return nil
}

// Execute executes the command.
func (cmd *TranslateRequestCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Translate request")
	defer func() { step.End(err) }()

	defer func(err error) {
		cmd.ExecutionError = err
	}(err)

	req := step.Log("request received")
	marsh := jsonpb.Marshaler{Indent: "  "}
	if err = marsh.Marshal(req, cmd.CtpReq); err != nil {
		err = errors.Annotate(err, "failed to marshal proto").Err()
	}

	internalStruct := &testapi.InternalTestplan{}
	suitemd := &testapi.SuiteMetadata{
		Pool:              cmd.CtpReq.GetPool(),
		ExecutionMetadata: executionMetadata(cmd.CtpReq, cmd.IsPartnerRun),
		DynamicUpdates:    []*api.UserDefinedDynamicUpdate{},
	}

	// bmvToDimsMap holds the map to {key -> dims[]} for all the targets that are dropped.
	// this will be used to create bot params rejected result that will be reflected in summarize and output properties.
	bmvToDimsMap := updateSchedulingTargetsBasedOnBotAvailability(ctx, cmd.CtpReq)

	// new field that supports multi-dut
	suitemd.SchedulingUnits = schedTargetstoSchedUnits(cmd.CtpReq.GetScheduleTargets())
	suitemd.SchedulingUnitOptions = groupedSchedTargetsToSchedUnitOptions(cmd.CtpReq.GetGroupedScheduleTargets())

	// non-multi-dut legacy flow to support backwards compatibility
	// TODO(azrahman): remove this when not needed any more
	// suitemd.TargetRequirements = targetRequirements(cmd.CtpReq)

	suitemd.SchedulerInfo = generateSchedulerInfo(cmd.CtpReq)

	internalStruct.SuiteInfo = &testapi.SuiteInfo{
		SuiteMetadata: suitemd,
		SuiteRequest:  cmd.CtpReq.GetSuiteRequest(),
	}

	translated_req := step.Log("translated request")
	if err = marsh.Marshal(translated_req, internalStruct); err != nil {
		err = errors.Annotate(err, "failed to marshal proto").Err()
	}

	cmd.InternalTestPlan = internalStruct

	suiteName := cmd.CtpReq.GetSuiteRequest().GetTestSuite().GetName()

	// If there are targets that were dropped, we need to report them as bot params rejected.
	if len(bmvToDimsMap) > 0 {
		cmd.TestResults = map[string]*data.TestResults{}
		for bmvKey, dims := range bmvToDimsMap {
			botParamsRejectedErr := &data.BotParamsRejectedError{Key: bmvKey, RejectedDims: dims}
			testResult := &data.TestResults{Suite: suiteName, Key: bmvKey, TopLevelError: botParamsRejectedErr, RequestKey: cmd.RequestKey, Name: fmt.Sprintf("%s_%s", suiteName, bmvKey), IsALRun: cmd.IsAlRun}
			cmd.TestResults[bmvKey] = testResult
		}
	}

	// Validations
	if len(suitemd.GetSchedulingUnits()) == 0 && len(suitemd.GetSchedulingUnitOptions()) == 0 {
		logging.Infof(ctx, fmt.Sprintf("no scheduling units found at the end of translation. check logs to see if all of the targets are dropped."))
		err = fmt.Errorf("No device targets found at the end of translation. Perhaps invalid targets provided in request.")
		step.SetSummaryMarkdown(err.Error())
	}

	return err
}

func updateSchedulingTargetsBasedOnBotAvailability(ctx context.Context, ctpReq *testapi.CTPRequest) map[string][]string {
	bmvToDimsMap := map[string][]string{}
	swarmingServ, err := common.CreateNewSwarmingService(context.Background())
	if err != nil {
		logging.Infof(ctx, fmt.Sprintf("error found while creating new swarming service: %s", err))
		return bmvToDimsMap
	}
	pool := ctpReq.GetPool()
	// skip swarming bot count check if pool is vmlab
	if pool == "vmlab" {
		return bmvToDimsMap
	}
	botAvailabilityCache := make(map[string]bool)

	if len(ctpReq.GetGroupedScheduleTargets()) > 0 {
		// New proto flow
		// Example: input: [(a, b, c), (d, e, f), (g)]
		// b, f, g - these are invalid
		// output: [(a, c), (d, e)]
		newGroupedScheduleTargets := []*testapi.GroupedScheduleTargets{}
		for _, groupedSchedTargets := range ctpReq.GetGroupedScheduleTargets() {
			localBmvToDimsMap := map[string][]string{}
			newSchedTargets := getNewSchedulingTargetsBasedOnBotAvailability(ctx, groupedSchedTargets.GetGroupedTargets(), localBmvToDimsMap, pool, botAvailabilityCache, swarmingServ)
			if len(newSchedTargets) > 0 {
				// at least one valid target is there so we don't need to report the other targets as bot params rejected.
				// if the current valid target result into bot params rejected later (MO, request_generator), they will report it as bot params rejected.
				newGroupedScheduleTargets = append(newGroupedScheduleTargets, &testapi.GroupedScheduleTargets{GroupedTargets: newSchedTargets})
			} else {
				// all potential targets were removed
				// so this will mean that a required target will not get scheduled due to all targets in the list resulted in bot params rejected.
				// example: input: [(a, b), (c, d)]
				// c, d - these are invalid
				// output: [(a, b)]; so we should generate bot params rejected for (c,d)
				mergeMaps(localBmvToDimsMap, bmvToDimsMap)
			}
		}
		ctpReq.GroupedScheduleTargets = newGroupedScheduleTargets
	} else {
		// TODO (oldProto-azrahman): remove when makes sense
		ctpReq.ScheduleTargets = getNewSchedulingTargetsBasedOnBotAvailability(ctx, ctpReq.GetScheduleTargets(), bmvToDimsMap, pool, botAvailabilityCache, swarmingServ)
	}

	return bmvToDimsMap
}

func getNewSchedulingTargetsBasedOnBotAvailability(ctx context.Context, schedTargets []*testapi.ScheduleTargets, bmvToDimsMap map[string][]string, pool string, botAvailabilityCache map[string]bool, swarmingServ *swarming.Service) []*testapi.ScheduleTargets {
	// this will hold all new available schedule targets
	newSchedulingTargets := []*testapi.ScheduleTargets{}

	for _, schedulingTargets := range schedTargets {
		newTargets := []*testapi.Targets{}
		for _, target := range schedulingTargets.GetTargets() {
			model := target.HwTarget.GetLegacyHw().GetModel()
			board := target.HwTarget.GetLegacyHw().GetBoard()
			// vmlab fork has been historically based on supported boards and not solely based on pool value, this allows to handle cases where users are still sending pool value as DUT_POOL_QUOTA and expecting vm run.
			if common.IsSupportedVMBoard(board) {
				return schedTargets
			}
			variant := target.GetHwTarget().GetLegacyHw().GetVariant()
			bmvKey := common.ConstructKey(board, model, variant)
			dimsForCache := fmt.Sprintf("%s-%s-%s", model, board, pool)
			if _, ok := botAvailabilityCache[dimsForCache]; !ok {
				dims := []string{}
				dims = append(dims, fmt.Sprintf("label-pool:%s", pool))
				dims = append(dims, fmt.Sprintf("label-board:%s", strings.ToLower(target.HwTarget.GetLegacyHw().GetBoard())))
				if target.HwTarget.GetLegacyHw().GetModel() != "" {
					dims = append(dims, fmt.Sprintf("label-model:%s", strings.ToLower(target.HwTarget.GetLegacyHw().GetModel())))
				}
				botCount, err := common.GetBotCount(ctx, dims, swarmingServ)
				if err != nil {
					logging.Infof(ctx, fmt.Sprintf("error found while getting bot count: %s", err))
					// add target instead of stopping execution
					newTargets = append(newTargets, target)
				}
				// only add if bots available
				if botCount > 0 {
					botAvailabilityCache[dimsForCache] = true
					newTargets = append(newTargets, target)
				} else {
					botAvailabilityCache[dimsForCache] = false
					logging.Infof(ctx, fmt.Sprintf("dropping : %s", dimsForCache))
					bmvToDimsMap[bmvKey] = dims
				}
			} else {
				if botAvailabilityCache[dimsForCache] {
					newTargets = append(newTargets, target)
				}
			}
		}

		if len(newTargets) > 0 {
			newSchedulingTarget := &testapi.ScheduleTargets{Targets: newTargets}
			newSchedulingTargets = append(newSchedulingTargets, newSchedulingTarget)
		}
	}

	return newSchedulingTargets
}

func mergeMaps(inputMap map[string][]string, outputMap map[string][]string) {
	for k, v := range inputMap {
		outputMap[k] = v
	}
}

func newBBClient(ctx context.Context) (buildbucketpb.BuildsClient, error) {
	hClient, err := httpClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "create buildbucket client").Err()
	}
	pClient := &prpc.Client{
		C:    hClient,
		Host: "cr-buildbucket.appspot.com",
	}
	return buildbucketpb.NewBuildsPRPCClient(pClient), nil
}

func newRDBClient(ctx context.Context, host string) (resultpb.RecorderClient, error) {
	hClient, err := httpClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "create recorder client").Err()
	}
	pClient := &prpc.Client{
		C:    hClient,
		Host: host,
	}
	return resultpb.NewRecorderPRPCClient(pClient), nil
}

func httpClient(ctx context.Context) (*http.Client, error) {
	a := auth.NewAuthenticator(ctx, auth.SilentLogin, auth.Options{
		Scopes: []string{auth.OAuthScopeEmail},
	})
	h, err := a.Client()
	if err != nil {
		return nil, errors.Annotate(err, "create http client").Err()
	}
	return h, nil
}

func targetRequirements(req *testapi.CTPRequest) []*testapi.TargetRequirements {
	targs := []*testapi.TargetRequirements{}
	for _, scheduleTarget := range req.GetScheduleTargets() {
		// TODO (azrahman): 0 indexing now for single dut. Add multi-dut support.
		targ := scheduleTarget.GetTargets()[0]
		switch hw := targ.HwTarget.Target.(type) {
		case *testapi.HWTarget_LegacyHw:

			// There will only be one set by the translation; but other filters might
			// expand this as they see fit.
			var hwDefs []*testapi.SwarmingDefinition
			hwDefs = append(hwDefs, buildHwDef(hw.LegacyHw))

			legacysw := legacyswpoper(targ.SwTarget)

			builtTarget := &testapi.TargetRequirements{
				HwRequirements: &testapi.HWRequirements{
					HwDefinition: hwDefs,
				},

				SwRequirement: legacysw,
			}
			targs = append(targs, builtTarget)
		}
	}
	return targs
}

func schedTargetToSchedUnit(scheduleTarget *testapi.ScheduleTargets) *testapi.SchedulingUnit {
	newSchedUnit := &api.SchedulingUnit{CompanionTargets: []*api.Target{}}
	for i, targ := range scheduleTarget.GetTargets() {
		newTarget := TargetsToNewTarget(targ)
		if i == 0 {
			// primary target
			newSchedUnit.PrimaryTarget = newTarget
		} else {
			// secondary target
			newSchedUnit.CompanionTargets = append(newSchedUnit.CompanionTargets, newTarget)
		}
	}

	return newSchedUnit
}

func schedTargetstoSchedUnits(schedTargets []*testapi.ScheduleTargets) []*testapi.SchedulingUnit {
	schedUnits := []*testapi.SchedulingUnit{}
	for _, scheduleTarget := range schedTargets {
		schedUnits = append(schedUnits, schedTargetToSchedUnit(scheduleTarget))
	}
	return schedUnits
}

func groupedSchedTargetsToSchedUnitOptions(grouped_schedule_targets []*testapi.GroupedScheduleTargets) []*testapi.SchedulingUnitOptions {
	schedUnitOptions := []*testapi.SchedulingUnitOptions{}
	// Grouped targets allow setting and/or relationship between targets.
	// inner i.e. {a, b} is OR relationship: a or b
	// outer i.e. {a}, {b} is AND relationship: a and b
	// [{schedTarget1, schedTarget2}, {schedTarget3, schedTarget4}]
	// --> (schedTarget1 OR schedTarget2) AND (schedTarget3 OR schedTarget4)
	for _, grouped_schedule_targets := range grouped_schedule_targets {
		schedUnits := schedTargetstoSchedUnits(grouped_schedule_targets.GetGroupedTargets())
		schedUnitOptions = append(schedUnitOptions, &testapi.SchedulingUnitOptions{SchedulingUnits: schedUnits, State: api.SchedulingUnitOptions_ONEOF})
	}

	return schedUnitOptions
}

func TargetsToNewTarget(targ *testapi.Targets) *api.Target {
	switch hw := targ.HwTarget.Target.(type) {
	case *testapi.HWTarget_LegacyHw:
		// There will only be one set by the translation; but other filters might
		// expand this as they see fit.
		swDef := buildHwDef(hw.LegacyHw)
		legacysw := legacyswpoper(targ.SwTarget)

		return &api.Target{SwarmingDef: swDef, SwReq: legacysw}
	}
	return nil
}

func legacyswpoper(sws *testapi.SWTarget) *testapi.LegacySW {
	switch sw := sws.SwTarget.(type) {
	case *testapi.SWTarget_LegacySw:
		return sw.LegacySw
	}
	return nil
}

func buildHwDef(hw *testapi.LegacyHW) *testapi.SwarmingDefinition {
	dut := &labapi.Dut{}
	dutModel := &labapi.DutModel{
		BuildTarget: hw.Board,
		ModelName:   hw.Model,
	}
	if common.IsAndroid(hw.GetBoard()) {
		android := &labapi.Dut_Android{DutModel: dutModel}
		dut.DutType = &labapi.Dut_Android_{Android: android}

	} else if common.IsCros(hw.GetBoard()) {
		Cros := &labapi.Dut_ChromeOS{DutModel: dutModel}
		dut.DutType = &labapi.Dut_Chromeos{Chromeos: Cros}

	} else if common.IsDevBoard(hw.GetBoard()) {
		devBoard := &labapi.Dut_Devboard{DutModel: dutModel}
		dut.DutType = &labapi.Dut_Devboard_{Devboard: devBoard}

	}

	return &testapi.SwarmingDefinition{DutInfo: dut, Variant: hw.GetVariant(),
		SwarmingLabels: hw.GetSwarmingDimensions()}
}

func NewTranslateRequestCmd() *TranslateRequestCmd {
	abstractCmd := interfaces.NewAbstractCmd(TranslateRequestType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &TranslateRequestCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}

func generateSchedulerInfo(req *api.CTPRequest) *api.SchedulerInfo {
	return req.GetSchedulerInfo()
}

func executionMetadata(req *api.CTPRequest, isPartnerRun bool) *api.ExecutionMetadata {
	ta := req.GetSuiteRequest().GetTestArgs()
	args := &testapi.ExecutionMetadata{}
	things := []*testapi.Arg{
		{Flag: "is_partner_run", Value: strconv.FormatBool(isPartnerRun)},
	}

	// ta will often be a comma deliminated string such as:
	// "foo=bar,zoo=mar"
	// Split on the comma, then split again on the =
	// Lazy parsing the `=`; as KV support is weak at best.
	// Users are responsible for clean args.
	for _, kv := range strings.Split(ta, " ") {
		k := ""
		v := ""
		for _, innerkv := range strings.Split(kv, "=") {
			if k == "resultdb_settings" || strings.Contains(k, "b64") {
				// force split to 2 (since the value may have multiple '='s)
				rdbKVs := strings.SplitN(kv, "=", 2)
				k = rdbKVs[0]
				v = rdbKVs[1]
			} else if k == "" {
				k = innerkv
			} else if v == "" {
				v = innerkv
			} else {
				fmt.Printf("too many values to unpack, skipping. bad %v from %v\n", innerkv, kv)
				k = ""
				v = ""
			}
		}
		if k != "" && v != "" {
			kvproto := &testapi.Arg{
				Flag:  k,
				Value: v,
			}
			things = append(things, kvproto)
		}
	}

	args.Args = things

	// append any kind of direct metadata provided in suite req
	args.Args = append(args.Args, req.GetSuiteRequest().GetTestSuite().GetExecutionMetadata().GetArgs()...)

	// append is_al_run to execution metadata so that filters can have access to this info
	// NOTE: test_runner doesn't read this and expects upstream(CTP) to set separete flag in the request directly
	args.Args = append(args.Args, &testapi.Arg{Flag: "is_al_run", Value: strconv.FormatBool(req.IsAlRun)})

	return args
}
