// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/bigquery"
	hashstructure "github.com/mitchellh/hashstructure/v2"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/analytics"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/ctpv2/data"
)

// MiddleOutRequestCmd represents middle out command.
type MiddleOutRequestCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	InternalTestPlan *api.InternalTestplan

	// Updates
	MiddledOutResp *data.MiddleOutResponse

	// BQ client for logging
	BQClient *bigquery.Client
	// BuildState
	BuildState *build.State

	ExecutionError error

	DurationBasedSharding bool
}

const (
	TautoTastPrefix = "tauto.tast"
	NoDevicesInt    = math.MinInt32 + 1
	TastPrefix      = "tast"
	minIterations   = 1
	maxIterations   = 10
)

var dursCache = make(map[string]map[string]float64)

// ExtractDependencies (Boiler plate)
func (cmd *MiddleOutRequestCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper (Boiler plate)
func (cmd *MiddleOutRequestCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.updateFilterStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}
	// set cmd to nil, to release memory
	cmd = nil

	return nil
}

func (cmd *MiddleOutRequestCmd) extractDepsFromFilterStateKeeper(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	// If no plan...
	if len(sk.TestPlanStates) == 0 {
		if sk.InitialInternalTestPlan != nil {
			// Set the first state from initial test plan
			sk.TestPlanStates = append(sk.TestPlanStates, sk.InitialInternalTestPlan)
			// Set the cmd input test plan
			cmd.InternalTestPlan = proto.Clone(sk.InitialInternalTestPlan).(*api.InternalTestplan)
		} else {
			return fmt.Errorf("cmd %q missing dependency: InputTestPlan", cmd.GetCommandType())
		}
	} else {
		// Get the last test plan state and set it as input test plan for current filter
		cmd.InternalTestPlan = proto.Clone(sk.TestPlanStates[len(sk.TestPlanStates)-1]).(*api.InternalTestplan)
	}

	if sk.BQClient != nil {
		cmd.BQClient = sk.BQClient
	}

	// By default, we do not want to do this, as even shard duration is often at the cost of total runtime
	// due to the nature of how fixtures work in Tast. So only enable this for the ChromeOS Cq flow.
	cmd.DurationBasedSharding = false
	if sk.CtpReq.GetSchedulerInfo().GetQsAccount() == common.PcqQsAccount && !sk.IsAlRun {
		cmd.DurationBasedSharding = true
	}

	cmd.BuildState = sk.BuildState

	cmd.ExecutionError = sk.ExecutionError
	return nil
}

func (cmd *MiddleOutRequestCmd) updateFilterStateKeeper(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	if cmd.MiddledOutResp != nil {
		sk.MiddledOutResp = cmd.MiddledOutResp
	}

	sk.ExecutionError = cmd.ExecutionError
	return nil
}

// light wrapper around the BQ call, to make unittests easier.
func getDurationMetrics(harness string, ctx context.Context) (map[string]float64, error) {
	durs, ok := dursCache[harness]
	if !ok {
		logging.Infof(ctx, "getting durations for harness %s", harness)
		bqClient := analytics.CtpAnalyticsBQClient(context.Background())
		d, err := analytics.ReadDurationMetrics(bqClient, harness, ctx)
		dursCache[harness] = d
		return d, err
	} else {
		logging.Infof(ctx, "CacheHit for metrics")
		return durs, nil
	}
}

// Execute executes the command.
func (cmd *MiddleOutRequestCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Middle Out")
	defer func() { step.End(err) }()

	defer func(err error) {
		cmd.ExecutionError = err
	}(err)

	key := "middleout-execute"

	if cmd.BQClient != nil {
		analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, &analytics.BqData{Step: key, Status: analytics.Start}, cmd.InternalTestPlan, cmd.BuildState)
	}
	start := time.Now()
	status := analytics.Success

	// TODO (dbeckett/Aziz) figure out how to properly make this
	pool := cmd.InternalTestPlan.GetSuiteInfo().GetSuiteMetadata().GetPool()
	if pool == "" {
		pool = "DUT_POOL_QUOTA"
	}
	cfg := distroCfg{
		maxInShard: func() int {
			maxInShard := int(cmd.InternalTestPlan.GetSuiteInfo().GetSuiteRequest().GetMaxInShard())
			if maxInShard == 0 {
				return 150
			}
			return maxInShard
		}(),
		pool:                             pool,
		durationBasedSharding:            cmd.DurationBasedSharding,
		requestsPerSchedulingUnitOptions: validateIterationsCfg(ctx, cmd.InternalTestPlan.GetSuiteInfo().GetSuiteRequest().GetIterations()),
		// TODO: (thandakas) Update input with new InternalTestPlan field when available
		multiEqcByModel: true,
	}

	trReqs, err := middleOut(ctx, cmd.InternalTestPlan, cfg)
	if err != nil {
		status = analytics.Fail
		if cmd.BQClient != nil {
			analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, &analytics.BqData{Step: key, Status: status, Duration: float32(time.Since(start).Seconds())}, cmd.InternalTestPlan, cmd.BuildState)
		}

		return errors.Annotate(err, "Failed to execute MiddleOPut: ").Err()
	}
	if cmd.BQClient != nil {
		analytics.SoftInsertStepWInternalPlan(ctx, cmd.BQClient, &analytics.BqData{Step: key, Status: status, Duration: float32(time.Since(start).Seconds())}, cmd.InternalTestPlan, cmd.BuildState)
	}
	logging.Infof(
		ctx,
		"len of data: %d",
		len(trReqs))

	cmd.MiddledOutResp = &data.MiddleOutResponse{TrReqs: trReqs, SuiteInfo: cmd.InternalTestPlan.GetSuiteInfo()}

	middleOutData, err := json.MarshalIndent(cmd.MiddledOutResp, "", "  ")
	if err != nil {
		logging.Infof(
			ctx,
			"error during writing MO response to log: %s",
			err.Error())
	}
	_, _ = step.Log("Middle out output").Write(middleOutData)

	return nil
}

func NewMiddleOutRequestCmd() *MiddleOutRequestCmd {
	abstractCmd := interfaces.NewAbstractCmd(MiddleoutExecutionType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &MiddleOutRequestCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}

// loading is used in lab avalability such that devices with the same HW;
// but different SW requirements, still share the same pool of physical devices.
type loading struct {
	// Current count of free devices for this
	freeDevices int
	// Count of total devices, including busy. Used to balance EQC distro once all free devices have been used
	totalDevicesUnAssigned int
	// Specifically a *static* count of all devices in the lab. Used for bot params rejection checks.
	staticBotCount int64
}

type hwInfo struct {
	req *api.SchedulingUnitOptions

	// TODO; when HwRequirements is fully deprecated, remove this.
	oldReq *api.HWRequirements

	labLoading         *loading
	numInCurrentShard  int
	hwValue            uint64
	matchingValue      uint64
	provValue          uint64
	shardHarness       string
	dimsExcludingReady []string
	publishKeys        []*api.PublishKey
}

// Kv structs are useful for gobased sorting.
type kv struct {
	key   uint64
	value int
}

type pkTestCaseData struct {
	publishKey *api.PublishKey
	testCases  []string
}

type distroCfg struct {
	pool                             string
	isUnitTest                       bool
	unitTestDevices                  int
	maxInShard                       int
	durationBasedSharding            bool
	requestsPerSchedulingUnitOptions int
	multiEqcByModel                  bool
}

type middleOutData struct {
	// Originally defined HW to TC
	hwToTCMap map[uint64][]string

	// Map between original HW objects, and the flat HW equivs
	// Example: [sha123]: [sha1, sha2, sha3]
	hwEquivalenceMap map[uint64][]uint64

	// the Sha of the HWRequirements object from a TC, pointing back to the HWRequirements object
	hwUUIDMap map[uint64]*api.SchedulingUnitOptions

	// TODO; when HwRequirements is fully deprecated, remove this.
	oldhwUUIDMap map[uint64]*api.HWRequirements

	cfg distroCfg

	// Flat HWUUID to flat HW
	flatHWUUIDMap map[uint64]*hwInfo

	// The TestCase name pointing to its respective object
	tcUUIDMap map[string]*api.CTPTestCase

	finalAssignments map[uint64][][]string

	lookBack map[uint64]map[string]*api.SchedulingUnitOptions
}

// newMiddleOutData returns a struct of the middleOutData with the data init'd but empty.
func newMiddleOutData() *middleOutData {
	mo := &middleOutData{
		hwToTCMap:        make(map[uint64][]string),
		hwEquivalenceMap: make(map[uint64][]uint64),
		hwUUIDMap:        make(map[uint64]*api.SchedulingUnitOptions),

		// TODO; when HwRequirements is fully deprecated, remove this.
		oldhwUUIDMap: make(map[uint64]*api.HWRequirements),

		cfg:           distroCfg{},
		flatHWUUIDMap: make(map[uint64]*hwInfo),
		tcUUIDMap:     make(map[string]*api.CTPTestCase),

		finalAssignments: make(map[uint64][][]string),
	}
	return mo

}

func getName(tc *api.CTPTestCase) string {
	return tc.GetMetadata().GetTestCase().GetId().GetValue()
}

func getTCId(tc *api.CTPTestCase) *api.TestCase_Id {
	return tc.GetMetadata().GetTestCase().GetId()
}

func oldProto(targs []*api.HWRequirements) bool {
	return len(targs) > 0
}

func validateIterationsCfg(ctx context.Context, iterations *api.Iterations) int {
	numIterations := minIterations
	// Nil validation check of iterations input
	if iterations != nil {
		numIterations = int(iterations.GetNumRetries())
	}
	// Lower limit for requested scheduled iterations per SchedulingUnitOptions
	if numIterations < minIterations {
		logging.Infof(ctx, fmt.Sprintf("Requested # of scheduled iterations per SchedulingUnitOptions is below MIN limit of %d. Setting config to MIN limit.", minIterations))
		return minIterations
	}
	// Upper limit for requested scheduled iterations per SchedulingUnitOptions
	if numIterations > maxIterations {
		logging.Infof(ctx, fmt.Sprintf("Requested # of scheduled iterations per SchedulingUnitOptions is above MAX limit of %d. Setting config to MAX limit.", maxIterations))
		return maxIterations
	}
	logging.Infof(ctx, fmt.Sprintf("Setting config Iterations to %d", numIterations))
	return numIterations
}

// middleOut creates TRRequest(S) from a ctpv2 internal test plan.
func middleOut(ctx context.Context, resp *api.InternalTestplan, cfg distroCfg) ([]*data.TrRequest, error) {
	solverData := newMiddleOutData()
	solverData.cfg = cfg
	for _, tc := range resp.GetTestCases() {
		tcUUID := getName(tc)
		logging.Infof(ctx, fmt.Sprintf("For TC: %s UUID is %s", tc, tcUUID))

		// Drop all of the HW fluff in the TC for memory sakes.
		tcForMap := &api.CTPTestCase{
			Name:     tc.GetName(),
			Metadata: tc.GetMetadata(),
		}
		solverData.tcUUIDMap[tcUUID] = tcForMap

		for _, hw := range tc.SchedulingUnitOptions {
			// Note: Each `hw` is still a repeated list of HW *options* for the test.
			hash := addHWtohwUUIDMap(solverData.hwUUIDMap, hw)
			for k, v := range flattenList(ctx, []*api.SchedulingUnitOptions{hw}) {
				err := addHWtoFlatHWUUIDMap(ctx, solverData.flatHWUUIDMap, k, v)
				if err != nil {
					logging.Infof(ctx, fmt.Sprintf("error found in addHWtoFlatHWUUIDMap: %s", err))
					return nil, err
				}
			}
			solverData.hwToTCMap[hash] = append(solverData.hwToTCMap[hash], tcUUID)

		}
	}

	// TODO; when HwRequirements is fully deprecated, remove this.
	if len(solverData.oldhwUUIDMap) > 0 {
		for key, hwOptions := range solverData.oldhwUUIDMap {
			solverData.hwEquivalenceMap[key] = oldFindMatches(hwOptions, solverData.flatHWUUIDMap)
		}
	} else {
		// Goal here is to make the hwEquivalenceMap; such that when we go to distribute the hwToTCMap
		// We can lookup what HW options are available.
		for key, hwOptions := range solverData.hwUUIDMap {
			solverData.hwEquivalenceMap[key] = findMatches(ctx, hwOptions, solverData.flatHWUUIDMap)
		}
	}

	return createTrRequests(ctx, greedyDistro(ctx, solverData), solverData)
}

// createTrRequests translates a final {hw:[[shard], [shard]]} map into a flat list of TrRequests.
func createTrRequests(ctx context.Context, distro map[uint64][][]string, solverData *middleOutData) ([]*data.TrRequest, error) {
	TrRequests := []*data.TrRequest{}

	for hwHash, shards := range distro {
		for _, tcs := range shards {
			trReq, err := createTrRequest(ctx, solverData, tcs, hwHash)
			if err != nil {
				return []*data.TrRequest{}, err
			}
			TrRequests = append(TrRequests, trReq)
		}
	}
	return TrRequests, nil
}

func createTrRequest(ctx context.Context, solverData *middleOutData, testCases []string, hwHash uint64) (*data.TrRequest, error) {
	testCaseIds, err := getTestCaseIds(testCases, solverData)
	if err != nil {
		return &data.TrRequest{}, err
	}
	mergedPublishKeys, err := mergePublishKeys(ctx, testCases, solverData, hwHash)
	if err != nil {
		return &data.TrRequest{}, err
	}
	newReq := *solverData.flatHWUUIDMap[hwHash].req
	newReq.PublishKeys = mergedPublishKeys

	trReq := &data.TrRequest{
		Req:    solverData.flatHWUUIDMap[hwHash].oldReq,
		NewReq: &newReq,
		Tcs:    testCaseIds,
		DevicesInfo: &data.DevicesInfo{LabDevicesCount: solverData.flatHWUUIDMap[hwHash].labLoading.staticBotCount,
			Dims: solverData.flatHWUUIDMap[hwHash].dimsExcludingReady,
		},
	}
	return trReq, nil
}

func getTestCaseIds(testCases []string, solverData *middleOutData) ([]*api.TestCase_Id, error) {
	testCaseIds := []*api.TestCase_Id{}
	for _, tc := range testCases {
		lltc, ok := solverData.tcUUIDMap[tc]
		if !ok {
			return []*api.TestCase_Id{}, fmt.Errorf("tc assigned but was not given, something critically wrong happened to end up here")
		}
		testCaseIds = append(testCaseIds, getTCId(lltc))
	}
	return testCaseIds, nil
}

func TestCasesGroupByPublishKeys(testCases []string, solverData *middleOutData, hwOptionHash uint64) (map[string]*pkTestCaseData, error) {
	// Goal here is to group a list of tcs to a shard and associate the tests
	// from that group to 3D/EqC data (if applicable) via a EqC hash map.
	eqcToPkTcs := map[string]*pkTestCaseData{}

	for _, testCase := range testCases {
		// First get the SchedulingUnitOptions (hw option) data for a testCase
		tcToHwOptionMap, ok := solverData.lookBack[hwOptionHash]
		if !ok {
			return map[string]*pkTestCaseData{}, fmt.Errorf("Could not find SchedulingUnitOption for hw option %d", hwOptionHash)
		}
		hwOption, ok := tcToHwOptionMap[testCase]
		if !ok {
			return map[string]*pkTestCaseData{}, fmt.Errorf("Could not find SchedulingUnitOption for hw option %d and test case %s", hwOptionHash, testCase)
		}
		// Then filter out the 3D publish keys of hw option
		// and update the Eqc-To-PublishKeysTestData map with key eqcHash
		for _, pk := range hwOption.PublishKeys {
			if pk.Subject == "3D" {
				eqcHash := pk.KeyValues["eqcHash"]
				eqcData, exists := eqcToPkTcs[eqcHash]
				if exists {
					eqcData.testCases = append(eqcData.testCases, testCase)
				} else {
					eqcData = &pkTestCaseData{
						publishKey: pk,
						testCases:  []string{testCase},
					}
				}
				eqcToPkTcs[eqcHash] = eqcData
			}
		}
	}
	return eqcToPkTcs, nil
}

func mergePublishKeys(ctx context.Context, testCases []string, solverData *middleOutData, hwOptionHash uint64) ([]*api.PublishKey, error) {
	// Goal here is to generate a group tests by 3D PublishKeys for a given hw option
	// The end result will be a slice of 3D PublishKeys, each containing the list
	// of tests that map to it (determined by the hw Option).
	mergedPublishKeys := []*api.PublishKey{}

	eqcToPkTcs, err := TestCasesGroupByPublishKeys(testCases, solverData, hwOptionHash)
	if err != nil {
		return []*api.PublishKey{}, err
	}

	for _, value := range eqcToPkTcs {
		testCases, err := json.Marshal(value.testCases)
		if err != nil {
			logging.Infof(ctx, "Error marshaling JSON: %s", err)
		}
		newPk := &api.PublishKey{
			Subject:   value.publishKey.Subject,
			KeyValues: map[string]string{},
		}
		for pkKey, pkVal := range value.publishKey.KeyValues {
			newPk.KeyValues[pkKey] = pkVal
		}
		newPk.KeyValues["eqcTests"] = string(testCases)
		mergedPublishKeys = append(mergedPublishKeys, newPk)
	}
	return mergedPublishKeys, nil
}

// Given the class map, and the tc map, gather the lab loading for the devices, and make our best guess at assigning tests to HW.
func greedyDistro(ctx context.Context, solverData *middleOutData) map[uint64][][]string {

	populateLabAvalability(ctx, solverData)
	orderedHw := hwSearchOrdering(solverData.hwEquivalenceMap)

	// Starting with the HW classes with the least amount of  equivalent classes
	for _, hwS := range orderedHw {
		// Find the TCS that require this class, and assign them devices.
		hwHash := hwS.key
		tcs := solverData.hwToTCMap[hwHash]

		// Currently we will not try anymore than basic sharding.
		// As in, we won't attempt to "fill" a pod, then spill over.
		// Its either "you can take all these tests" or we get a new pod.
		var shards [][]string

		if solverData.cfg.durationBasedSharding {
			var err error
			shards, err = shardWithTimeDistribution(ctx, tcs, solverData.cfg.maxInShard)
			if err != nil {
				logging.Infof(ctx, "Unable to time based shard, moving to regular.")
				shards = shard(tcs, solverData.cfg.maxInShard)
			}
		} else {
			shards = shard(tcs, solverData.cfg.maxInShard)
		}
		for _, shardedtc := range shards {
			// Keep track of the hw and models that are selected to support multi device selection within eqc
			hwCache := map[uint64]bool{}
			modelCache := map[string]bool{}

			harness := ""
			if len(shardedtc) > 0 {
				harness = getHarness(shardedtc[0])
			}
			// Will attempt to select # of devices of a hwHash (EqC) by iterating
			// through the EqC device selection (getDevices) and try to assign
			// devices that have NOT been selected in prior iterations. In a case where
			// size of the EqC devices < the requested # (requestsPerSchedulingUnitOptions), then
			// getDevices will attempt to assign to a previously selected device if
			// device shard capacity allows.
			for range solverData.cfg.requestsPerSchedulingUnitOptions {
				logging.Infof(ctx, fmt.Sprintf("Looking for Tcs: %s", shardedtc))
				selectedDevice, expandCurrentShard := getDevices(solverData, len(shardedtc), hwHash, harness, hwCache, modelCache, solverData.cfg.multiEqcByModel)
				// If a device is not found, then there is no need to keep checking the eqc for devices
				// for multi requests.
				if selectedDevice == 0 {
					break
				}
				logging.Infof(ctx, "Selected: %s Expand : %s", selectedDevice, expandCurrentShard)
				logging.Infof(ctx, "selected expanded: ", solverData.hwUUIDMap[selectedDevice])
				assignHardware(solverData, selectedDevice, expandCurrentShard, shardedtc, hwHash)
			}
		}
	}
	return solverData.finalAssignments
}

// TODO(derek): uncomment when will be used. 01/27/25
// allBoardsSameInEqc is used to determine if an EQC class is comprised of 1 board, or several.
// If several, the returned board will be ""
// func allBoardsSameInEqc(hwHash uint64, solverData *middleOutData) (bool, string) {
// 	board := ""
// 	devices := solverData.hwEquivalenceMap[hwHash]
// 	for _, device := range devices {
// 		// if the shard is empty, we need to use the labloading process block
// 		// not the shard filler.
// 		d := solverData.flatHWUUIDMap[device]
// 		for _, subbUnit := range d.req.GetSchedulingUnits() {
// 			deviceBoard := subbUnit.GetPrimaryTarget().GetSwarmingDef().GetDutInfo().GetChromeos().GetDutModel().GetBuildTarget()
// 			if board == "" {
// 				board = deviceBoard
// 			} else if board != deviceBoard {
// 				return false, ""
// 			}
// 		}
// 	}
// 	return true, board
// }

// assignHardware will add the tests to the selectedDevice, being aware if it should go into a non-filled hard, or a new one.
// assignHardware will also decrement the number of devices remaining every time device is assigned tests.
func assignHardware(solverData *middleOutData, selectedDevice uint64, expandCurrentShard bool, tcShard []string, hash uint64) {
	// A copy of the tcShard is made to ensure changes to the original slice do not affect the finalAssignments
	shardedtc := make([]string, len(tcShard))
	copy(shardedtc, tcShard)

	if expandCurrentShard {
		lastElement := len(solverData.finalAssignments[selectedDevice])
		solverData.finalAssignments[selectedDevice][lastElement-1] = append(solverData.finalAssignments[selectedDevice][lastElement-1], shardedtc...)
		solverData.flatHWUUIDMap[selectedDevice].numInCurrentShard += len(shardedtc) // Show the status of the current shard. Might be wrong.
		if solverData.flatHWUUIDMap[selectedDevice].numInCurrentShard == solverData.cfg.maxInShard {
			solverData.flatHWUUIDMap[selectedDevice].numInCurrentShard = 0
		}
	} else {
		_, ok := solverData.finalAssignments[selectedDevice]
		if !ok {
			solverData.finalAssignments[selectedDevice] = [][]string{}
		}

		solverData.finalAssignments[selectedDevice] = append(solverData.finalAssignments[selectedDevice], shardedtc)

		// Only decrement real devices.
		if solverData.flatHWUUIDMap[selectedDevice].labLoading.staticBotCount > 0 {
			if solverData.flatHWUUIDMap[selectedDevice].labLoading.freeDevices > 0 {
				solverData.flatHWUUIDMap[selectedDevice].labLoading.freeDevices--

			}
			// Always decrement this by 1, even when we sharded to a free device.
			solverData.flatHWUUIDMap[selectedDevice].labLoading.totalDevicesUnAssigned--

		}
		// If the shard is not full, mark it as such.
		if len(shardedtc) != solverData.cfg.maxInShard {
			solverData.flatHWUUIDMap[selectedDevice].numInCurrentShard += len(shardedtc)
			solverData.flatHWUUIDMap[selectedDevice].shardHarness = getHarness(shardedtc[0])
		}
	}
	if solverData.lookBack == nil {
		solverData.lookBack = make(map[uint64]map[string]*api.SchedulingUnitOptions)
	}

	lbd := createLookback(solverData, shardedtc, hash)
	if solverData.lookBack[selectedDevice] == nil || len(solverData.lookBack[selectedDevice]) == 0 {
		solverData.lookBack[selectedDevice] = lbd
	} else {
		for tc, hwid := range lbd {
			solverData.lookBack[selectedDevice][tc] = hwid
		}
	}
}

func createLookback(solverData *middleOutData, shardedtc []string, hash uint64) map[string]*api.SchedulingUnitOptions {
	lookback := make(map[string]*api.SchedulingUnitOptions)
	for _, tc := range shardedtc {
		og := solverData.hwUUIDMap[hash]
		lookback[tc] = og
	}
	return lookback
}

type helper struct {
	hashV uint64
	// TODO remove `hashProvV` once hwRequirements has been fully removed.
	hashProvV      uint64
	swarmingLabels []string
}

// TODO remove this method once hwRequirements has been fully removed.
// oldFindMatches: find options for the given hwOption from the given flatHWUUIDMap. LEGACY PROTO
func oldFindMatches(hwOption *api.HWRequirements, flatHWUUIDMap map[uint64]*hwInfo) []uint64 {
	matches := []uint64{}
	cnt := 0

	helperList := []*helper{}
	for _, child := range hwOption.GetHwDefinition() {
		childHash, _ := hashstructure.Hash(child.DutInfo, hashstructure.FormatV2, nil)
		childHashProv, _ := hashstructure.Hash(child.Variant, hashstructure.FormatV2, nil)
		h := &helper{
			hashV:          childHash,
			hashProvV:      childHashProv,
			swarmingLabels: child.GetSwarmingLabels(),
		}
		helperList = append(helperList, h)
	}

	for sha, foundHW := range flatHWUUIDMap {
		if len(foundHW.oldReq.GetHwDefinition()) > 1 {
			panic("foundHW.req.HwDefinition should not have more than 1 object inside it")
		}

		if oldIsParent(foundHW, helperList) {
			matches = append(matches, sha)
		}
		cnt++

	}
	return matches
}

// TODO remove this method once hwRequirements has been fully removed.
// oldIsParent determines if the first is fully encompassing of the second object. LEGACY PROTO
func oldIsParent(parentSrc *hwInfo, childSrc []*helper) bool {
	/*
		In other words, if the parent has at least all of the fields of the child, it can run the child.
	*/
	// [1]

	// [1... 5000]
	if len(parentSrc.oldReq.GetHwDefinition()) != 1 && len(childSrc) > 0 {
		return false
	}

	// isParent is true by default if there is no child
	correct := true
	for _, child := range childSrc {
		childHash := child.hashV
		childHashProv := child.hashProvV
		// TODO (dbeckett/azrahman): determine if the AllItemsIn check is going to be problematic and needs to be cached.
		if childHash == parentSrc.hwValue && childHashProv == parentSrc.provValue && allItemsIn(child.swarmingLabels, parentSrc.oldReq.GetHwDefinition()[0].GetSwarmingLabels()) {
			return true
		}

		correct = false
	}

	return correct
}

// findMatches: find options for the given hwOption from the given allFlatHWUUIDMap
func findMatches(ctx context.Context, oneOfHws *api.SchedulingUnitOptions, allFlatHWUUIDMap map[uint64]*hwInfo) []uint64 {
	matches := []uint64{}
	cnt := 0

	/* Using this example:
	 {tc.SchedulingUnits = [
		SchedulingUnitOptions = {
			Schedulingunits = [
				SchedulingUnit = {
					Primary = SwarmingDef1
				},
				SchedulingUnit = {
					Primary = SwarmingDef12
				},
			],
			state: ONEOF
		},
		SchedulingUnitOptions = {
			Schedulingunits = [
				SchedulingUnit = {
					Primary = SwarmingDef3
				},
				SchedulingUnit = {
					Primary = SwarmingDef2
				},
			],
			state: ONEOF
		},

	]}


	`hwOption` is going to only _one_ of the `SchedulingUnitOptions` (not the entire []SchedulingUnits)

		SchedulingUnitOptions = {
			Schedulingunits = [
				SchedulingUnit = {
					Primary = SwarmingDef1
				},
				SchedulingUnit = {
					Primary = SwarmingDef12
				},
			],
			state: ONEOF
		}

	`flatHWUUIDMap` is going to be a full list of all flat HW. Eg:

	[
				SchedulingUnit = {
					Primary = SwarmingDef1
				},
				SchedulingUnit = {
					Primary = SwarmingDef2
				},
				SchedulingUnit = {
					Primary = SwarmingDef3
				},
	]


	The goal of the method is to see per option in `SchedulingUnitOptions`; which DUTS in the flat map match it.
	This is useful for bundling tests which have semi-overlapping deps.
	*/
	EqClassList := []*helper{}
	for _, child := range oneOfHws.SchedulingUnits {

		childHash := hashForSchedulingUnit(child)

		h := &helper{
			hashV:          childHash,
			swarmingLabels: child.GetPrimaryTarget().GetSwarmingDef().GetSwarmingLabels(),
		}

		EqClassList = append(EqClassList, h)
	}

	// From the existing HWUIID map, see what DUTs can run what classes.
	// This effectively means that selecting:
	// `HW.1234` is valid for ['HW.1234' || 'HW.1222']
	// `HW.1222` is valid for ['HW.1234' || 'HW.1222']
	for sha, foundHW := range allFlatHWUUIDMap {
		if len(foundHW.req.SchedulingUnits) > 1 {
			panic("foundHW.req.SchedulingUnits should not have more than 1 object inside it")
		}
		ip := isParentofAtleastOne(foundHW, EqClassList)
		if ip {
			matches = append(matches, sha)
		}
		cnt++

	}
	return matches
}

// isParentofOne determines if the first is fully encompassing of the second object.
func isParentofAtleastOne(parentSrc *hwInfo, childSrc []*helper) bool {
	/*
		In other words, if the parent has at least all of the fields of the child, it can run the child.
	*/
	// [1]

	// [1... 5000]

	// isParent is true by default if there is no child
	correct := true
	for _, child := range childSrc {
		childHash := child.hashV

		// TODO (dbeckett/azrahman): determine if the AllItemsIn check is going to be problematic and needs to be cached.
		if len(parentSrc.req.GetSchedulingUnits()) < 1 {
			return false
		}
		if childHash == parentSrc.hwValue && allItemsIn(child.swarmingLabels, parentSrc.req.GetSchedulingUnits()[0].GetPrimaryTarget().GetSwarmingDef().GetSwarmingLabels()) {
			return true
		}
		correct = false
	}

	return correct
}

// Check all items from 1 are in 2
func allItemsIn(item1 []string, item2 []string) bool {
	// TODO (dbeckett/azrahman): squish this such that we are doing set comparisons, rather than list.
	for _, string1 := range item1 {
		found := false
		for _, string2 := range item2 {
			if string1 == string2 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true

}

func filterToRelevantTests(ctx context.Context, testSet map[string]bool, durs map[string]float64) map[string]float64 {
	testsWeCareAbout := make(map[string]float64)
	for test := range testSet {
		logging.Infof(ctx, "checking for ", test)
		value, ok := durs[test]
		if ok {
			logging.Infof(ctx, "checking for %s time %s ", test, value)

			testsWeCareAbout[test] = value
		} else {
			// If we don't know anything, assume 1 second.
			testsWeCareAbout[test] = 1.0
		}
	}
	return testsWeCareAbout
}

// shard will divide the list into a list of lists where each item in the list length not to exceed maxInShard while balance the test count.
// eg: [1,2,3,4], maxInShard=2 --> [[1,2], [3,4]]
// eg: [1,2,3,4,5], maxInShard=4 -> [[1,2,3], [4,5]] not [[1,2,3,4], [5]]
func shard(alltests []string, maxInShard int) (shards [][]string) {
	harnessBuckets := bucketByHarness(alltests)

	for _, tests := range harnessBuckets {

		// Ceiling division without float.
		numShards := (len(tests)-1)/maxInShard + 1

		baseSize := len(tests) / numShards
		remainder := len(tests) % numShards

		start := 0
		for range numShards {
			shardSize := baseSize
			// Put 1 more item on earlier shards.
			if remainder > 0 {
				shardSize++
				remainder--
			}
			end := start + shardSize
			shards = append(shards, tests[start:end])
			start = end
		}
	}

	return shards
}

func bucketByHarness(alltests []string) map[string][]string {
	harnessBuckets := make(map[string][]string)
	for _, test := range alltests {
		h := getHarness(test)
		_, ok := harnessBuckets[h]

		if !ok {
			harnessBuckets[h] = []string{test}
		} else {
			harnessBuckets[h] = append(harnessBuckets[h], test)
		}
	}
	return harnessBuckets
}

// shard will device the list into a list of lists where each item in the list length of maxInShard
// eg: [1,2,3,4], maxInShard=2 --> [[1,2], [3,4]]
func shardWithTimeDistribution(ctx context.Context, alltests []string, maxInShard int) (shards [][]string, err error) {
	logging.Infof(ctx, "shardWithTimeDistribution")
	type testDuration struct {
		name     string
		duration float64
	}

	harnessBuckets := bucketByHarness(alltests)

	for harness, tests := range harnessBuckets {
		testSet := make(map[string]bool)

		// Make a map of tests in the bucket, so that we can search the query result for its result.
		for _, str := range tests {
			testSet[str] = true
		}

		// Determine the number of shards based on the MaxInShard value and the # of tests.
		totalShards := int(math.Ceil(float64(len(tests)) / float64(maxInShard)))

		// If the entire EQC has the same board, lets limit the query on the board or speed/accuracy.

		logging.Infof(ctx, "pre duration")
		durs, err := getDurationMetrics(harness, ctx)
		logging.Infof(ctx, "post duration")
		if err != nil {
			logging.Infof(ctx, "err while getting duration metrics: %s", err)
			return shards, err
		}
		logging.Infof(ctx, "post duration")

		// 0. Descope the total to just the tests we care about:
		testsWeCareAbout := filterToRelevantTests(ctx, testSet, durs)

		// 1. Sort tests by duration in descending order.

		var orderedDurs []testDuration
		for name, duration := range testsWeCareAbout {
			orderedDurs = append(orderedDurs, testDuration{name, duration})
		}

		sort.Slice(orderedDurs, func(i, j int) bool {
			return orderedDurs[i].duration > orderedDurs[j].duration
		})

		// 2. Initialize slices and track their sums.
		slices := make([][]string, totalShards)
		sums := make([]float64, totalShards)

		// 3. Distribute tests to minimize sum differences.
		for _, test := range orderedDurs {
			minSumIndex := 0
			for i := 1; i < totalShards; i++ {
				if sums[i] < sums[minSumIndex] {
					minSumIndex = i
				}
			}
			slices[minSumIndex] = append(slices[minSumIndex], test.name)
			sums[minSumIndex] += test.duration
		}
		logging.Infof(ctx, "Expected distro: ", sums)
		shards = append(shards, slices...)
	}
	return shards, nil
}

// extracts the harness out of the test name.
func getHarness(t string) string {
	if strings.HasPrefix(t, TautoTastPrefix) {
		return TautoTastPrefix
	}
	v := strings.Split(t, ".")
	// if there is less than 2 parts to the name, then its not a known test format.
	// so just return "unknown"
	if len(v) < 2 {
		return "unknown"
	}
	return v[0]
}

func getDeviceHash(value *hwInfo) uint64 {
	hash := uint64(0)
	// TODO remove this if block (keep the else) once hwRequirements has been fully removed.
	if len(value.oldReq.GetHwDefinition()) > 0 {
		hash, _ = hashstructure.Hash(value.oldReq.GetHwDefinition()[0].GetDutInfo(), hashstructure.FormatV2, nil)
	} else {
		// 0 index is safe as this is coming from the flatHWUUIDMap; which garuntees the list to be flat.
		// We have to use the `HwOnly` hashfor; as we should consider a variant as the same underlying hardware.
		// Eg; Brya and brya-kernelnext, are the same underlying DUT.
		hash = hashForSchedulingUnitHwOnly(value.req.GetSchedulingUnits()[0])

	}
	return hash
}

// Will add the amount of devices in the lab to each of the HW items.
func populateLabAvalability(ctx context.Context, solverData *middleOutData) {
	// toComplete will track what hwInfo will need labAvailibility info to be populated.
	toComplete := []*hwInfo{}

	hwFound := make(map[uint64]*loading)

	for _, value := range solverData.flatHWUUIDMap {

		// This is to ensure when we have to physically identical devices; but which have different software/
		// runtime adjustable instructions, we properly view them as "1" physical device come labLoading time.
		hash := hashWithDeps(value.req.GetSchedulingUnits()[0])

		// We do this check to see if a specific hash has already been discovered. In this case, we'd
		// want to point them both to the same pointer such that when one of the EQC's pulls a device
		// its properly removed from all.
		_, exists := hwFound[hash]
		if exists {
			value.labLoading = hwFound[hash]
			// The continue is very intentionally going to skip the `toComplete` amendment.
			continue
		} else {
			ll := &loading{}
			// Point these to the same object so the `if` above will properly assign pointers
			hwFound[hash] = ll
			value.labLoading = ll
		}

		if solverData.cfg.unitTestDevices != 0 {
			// Load items into the pointer.
			value.labLoading.freeDevices = solverData.cfg.unitTestDevices
			value.labLoading.totalDevicesUnAssigned = solverData.cfg.unitTestDevices
			value.labLoading.staticBotCount = int64(solverData.cfg.unitTestDevices)
		} else {
			toComplete = append(toComplete, value)
		}

	}

	if solverData.cfg.unitTestDevices == 0 {
		logging.Infof(ctx, "Looking for lab devices")
		swarmingServ, err := common.CreateNewSwarmingService(context.Background())
		if err != nil {
			logging.Infof(ctx, fmt.Sprintf("error found while creating new swarming service: %s", err))
			return
		}
		// Query swarming asynchronously for device availability.
		wg := sync.WaitGroup{}
		for _, hwInfoObj := range toComplete {
			wg.Add(1)
			go func(hwInfoInput *hwInfo) {
				defer wg.Done()
				dims := CreateDims(ctx, hwInfoInput, solverData.cfg.pool, true)

				dimsExcludingReady := CreateDims(ctx, hwInfoInput, solverData.cfg.pool, false)

				botCount, err := common.GetBotCount(ctx, dims, swarmingServ)
				if err != nil {
					logging.Infof(ctx, fmt.Sprintf("error found in GetBOTcount: %s", err))
				}
				totalBotCount, err := common.GetBotCount(ctx, dimsExcludingReady, swarmingServ)
				if err != nil {
					logging.Infof(ctx, fmt.Sprintf("error found in GetBOTcount: %s", err))
				}
				hwInfoInput.labLoading.freeDevices = int(botCount)
				hwInfoInput.labLoading.totalDevicesUnAssigned = int(totalBotCount)

				// Note: This field is reserved for the `bot_params_rejected` check.
				hwInfoInput.labLoading.staticBotCount = totalBotCount
				hwInfoInput.dimsExcludingReady = dimsExcludingReady

				logging.Infof(ctx, "Found for lab devices: %v", botCount)

			}(hwInfoObj)
		}
		wg.Wait()
	}
}

// CreateDims creates dims list from hwInfo object.
func CreateDims(ctx context.Context, hwInfo *hwInfo, pool string, readycheck bool) []string {
	// TODO replace `units` with just `hwInfo.req.GetSchedulingUnits()` once hwRequirements has been fully removed.
	units := 0
	if hwInfo.oldReq != nil {
		units = len(hwInfo.oldReq.GetHwDefinition())
	} else if hwInfo.req != nil {
		units = len(hwInfo.req.GetSchedulingUnits())
	}

	if hwInfo == nil || (hwInfo.req == nil && hwInfo.oldReq == nil) || units < 1 {
		return []string{}
	}

	dims := []string{}
	if readycheck {
		// dut_state can be ready while tests are running in dynamic TRv2
		// So we need task:idle so we choose proper option based on real state of the lab
		dims = append(dims, "dut_state:ready", "task:idle")
	}

	if len(hwInfo.req.GetSchedulingUnits()) > 0 {
		primary := hwInfo.req.GetSchedulingUnits()[0].GetPrimaryTarget()
		primaryModel := common.DutModelFromDut(primary.GetSwarmingDef().GetDutInfo())
		if common.IsSupportedVMBoard(primaryModel.GetBuildTarget()) {
			// GCE dims
			return []string{"role:vmlab"}
		}
		dims = ConvertSwarmingLabelsToDims(dims, primary.GetSwarmingDef().GetSwarmingLabels())
		dims = append(dims, common.GetBoardModelDims(hwInfo.req.GetSchedulingUnits()[0])...)
	} else if len(hwInfo.oldReq.GetHwDefinition()) == 1 {
		// TODO remove this entire `else` statement when HWRequirements is done.
		dims = ConvertSwarmingLabelsToDims(dims, hwInfo.oldReq.GetHwDefinition()[0].GetSwarmingLabels())
		if hwInfo.oldReq.GetHwDefinition()[0] != nil {
			dutInfo := hwInfo.oldReq.GetHwDefinition()[0].GetDutInfo()
			if dutInfo.GetChromeos().GetDutModel() != nil {
				dutModel := dutInfo.GetChromeos().GetDutModel()
				if dutModel.GetBuildTarget() != "" {
					dims = append(dims, fmt.Sprintf("label-board:%s", strings.ToLower(dutModel.GetBuildTarget())))
				}
				if dutModel.GetModelName() != "" {
					dims = append(dims, fmt.Sprintf("label-model:%s", strings.ToLower(dutModel.GetModelName())))
				}
			}
		}
	} else {
		return []string{}
	}
	if hwidFromHwInfo(hwInfo) != "" {
		dims = append(dims, fmt.Sprintf("hwid:%s", hwidFromHwInfo(hwInfo)))
	}
	if pool != "" {
		dims = append(dims, fmt.Sprintf("label-pool:%s", pool))
	}

	return dims
}

func hwidFromHwInfo(hwInfo *hwInfo) string {
	if hwInfo.oldReq != nil {
		return hwInfo.oldReq.GetHwDefinition()[0].GetDutInfo().GetChromeos().GetHwid()
	} else if hwInfo.req != nil {
		return hwInfo.req.GetSchedulingUnits()[0].GetPrimaryTarget().GetSwarmingDef().GetDutInfo().GetChromeos().GetHwid()
	}
	return ""
}

func modelFromHwInfo(hwInfo *hwInfo) string {
	if hwInfo.oldReq != nil {
		return hwInfo.oldReq.GetHwDefinition()[0].GetDutInfo().GetChromeos().GetDutModel().GetModelName()
	} else if hwInfo.req != nil {
		return hwInfo.req.GetSchedulingUnits()[0].GetPrimaryTarget().GetSwarmingDef().GetDutInfo().GetChromeos().GetDutModel().GetModelName()
	}
	return ""
}

// ConvertSwarmingLabelsToDims converts provided swarming labels to swarming dims.
func ConvertSwarmingLabelsToDims(defaultDims []string, swarmingLabels []string) []string {
	dims := defaultDims
	for _, label := range swarmingLabels {
		if strings.Contains(label, "label-") {
			dims = append(dims, label)
		} else if strings.HasPrefix(label, "dut_name") || strings.HasPrefix(label, "drone") || strings.HasPrefix(label, "bot") {
			dims = append(dims, label)
		} else if strings.Contains(label, ":") {
			dims = append(dims, fmt.Sprintf("label-%s", label))
		} else {
			dims = append(dims, fmt.Sprintf("label-%s:True", label))
		}
	}
	return dims
}

// translate the given hwEquivalenceMap into a 2d map:
// EG [id1=[1 || 2] || id2=[3 || 4]] needs to be [[1, 2, 3, 4]]
// hwEquivalenceMap is like id1 = [id1, id2]
// now needs to be like id1 = [newid1, newid2, newid3, newid4]
func flattenEqMap(hwEquivalenceMap map[uint64][]uint64, hwUUIDMap map[uint64]*api.SchedulingUnitOptions) (map[uint64][]uint64, map[uint64]*hwInfo) {
	newHWUUIDMap := make(map[uint64]*hwInfo)
	newHwEquivalenceMap := make(map[uint64][]uint64)

	for hw, results := range hwEquivalenceMap {
		var allHw []*api.SchedulingUnitOptions
		newHwEquivalenceMap[hw] = []uint64{}

		allHw = append(allHw, hwUUIDMap[hw])
		for _, hash := range results {
			allHw = append(allHw, hwUUIDMap[hash])
		}

		// flattened is now the uuid for [newid1: obj, etc]
		flattened := flattenList(context.Background(), allHw)

		for k, v := range flattened {
			newHWUUIDMap[k] = v
			newHwEquivalenceMap[hw] = append(newHwEquivalenceMap[hw], k)
		}
	}
	return newHwEquivalenceMap, newHWUUIDMap
}

// TODO; when HwRequirements is fully deprecated, remove this.
func oldFlattenEqMap(hwEquivalenceMap map[uint64][]uint64, hwUUIDMap map[uint64]*api.HWRequirements) (map[uint64][]uint64, map[uint64]*hwInfo) {
	newHWUUIDMap := make(map[uint64]*hwInfo)
	newHwEquivalenceMap := make(map[uint64][]uint64)

	for hw, results := range hwEquivalenceMap {
		allHw := []*api.HWRequirements{hwUUIDMap[hw]}

		for _, hash := range results {
			allHw = append(allHw, hwUUIDMap[hash])
		}

		// flattened is now the uuid for [newid1: obj, etc]
		flattened := oldFlattenList(context.Background(), allHw)

		for k, v := range flattened {
			newHWUUIDMap[k] = v
			newHwEquivalenceMap[hw] = append(newHwEquivalenceMap[hw], k)
		}
	}
	return newHwEquivalenceMap, newHWUUIDMap
}

// TODO; when HwRequirements is fully deprecated, remove this.
// oldFlattenList translates [hw.requirements=[1||2], hw.requirements=[3||4]] into [[1],[2],[3],[4]]
func oldFlattenList(ctx context.Context, allHw []*api.HWRequirements) map[uint64]*hwInfo {
	flatHW := make(map[uint64]*hwInfo)

	for _, hw := range allHw {

		for _, innerHW := range hw.GetHwDefinition() {

			flattened := &api.HWRequirements{
				HwDefinition: []*api.SwarmingDefinition{innerHW},
			}
			dutInfoHash, err := hashstructure.Hash(innerHW.DutInfo, hashstructure.FormatV2, nil)
			if err != nil {
				logging.Infof(ctx, fmt.Sprintf("error while creating hash for dutInfo: %s", err))
			}
			provInfoHash, err := hashstructure.Hash(innerHW.Variant, hashstructure.FormatV2, nil)
			if err != nil {
				logging.Infof(ctx, fmt.Sprintf("error while creating hash for provisionInfo: %s", err))
			}

			flattenedHash, err := hashstructure.Hash(flattened, hashstructure.FormatV2, nil)
			if err != nil {
				logging.Infof(ctx, fmt.Sprintf("error while creating hash for flattened: %s", err))
			}

			newHwInfo := &hwInfo{
				oldReq:    flattened,
				hwValue:   dutInfoHash,
				provValue: provInfoHash,
			}
			flatHW[flattenedHash] = newHwInfo
		}
	}

	return flatHW
}

// flattenList translates [hw.requirements=[1||2], hw.requirements=[3||4]] into [[1],[2],[3],[4]]
func flattenList(ctx context.Context, allHw []*api.SchedulingUnitOptions) map[uint64]*hwInfo {

	flatHW := make(map[uint64]*hwInfo)

	for _, hw := range allHw {

		for _, innerHW := range hw.SchedulingUnits {

			flattened := &api.SchedulingUnitOptions{
				SchedulingUnits: []*api.SchedulingUnit{innerHW},
			}

			flattenedHash, err := hashstructure.Hash(flattened, hashstructure.FormatV2, nil)
			if err != nil {
				logging.Infof(ctx, fmt.Sprintf("error while creating hash for flattened: %s", err))
			}

			newHwInfo := &hwInfo{
				req:           flattened,
				hwValue:       hashForSchedulingUnit(innerHW),
				matchingValue: hashForSchedulingUnit(innerHW),
				publishKeys:   hw.GetPublishKeys(),
			}

			flatHW[flattenedHash] = newHwInfo

		}

	}

	return flatHW
}

// requiresNewShard defines logic based on harness when we would require isolated shards
func requiresNewShard(currentPodHarness, currentTestHarness string) bool {
	// if tautoTast harness or harnesses doesn't match, force isolated shard
	return currentTestHarness == TautoTastPrefix || currentPodHarness != currentTestHarness
}

// getDevices finds a device from the devicepool + hwEquivalenceMap to satsify the need for the test
// It will first look for a matching device with a non-full shard that fits,
// otherwise it will look for a device with the most availability in the lab.
func getDevices(solverData *middleOutData, numTests int, hwHash uint64, harness string, hwCache map[uint64]bool, modelCache map[string]bool, multiEqcByModel bool) (selectedDevice uint64, append bool) {
	// This is a pretty expensive approach to sharding:
	// We will always check all devices to see if they have room in a non-empty shard.
	// So even when we fully fill a device, or it hasn't been touched, we still check it.
	// First, try to fill a non-empty shard
	devices := solverData.hwEquivalenceMap[hwHash]
	for _, device := range devices {
		// if the shard is empty, we need to use the labloading process block
		// not the shard filler.
		if solverData.flatHWUUIDMap[device].numInCurrentShard < 1 {
			continue
		}

		// if requires new shard, force it
		if requiresNewShard(solverData.flatHWUUIDMap[device].shardHarness, harness) {
			continue
		}

		// Only assign it into a shard if there is actually devices.
		if solverData.flatHWUUIDMap[device].labLoading.staticBotCount != 0 {
			// There are cases where a test requires a device which doesn't exist (to later be rejected)
			// But in these examples, its viewed as an "open shard", so we toss other tests with overlapping eq classes
			// into the shard; resulting in those tests being skipped.
			if solverData.flatHWUUIDMap[device].numInCurrentShard+numTests <= solverData.cfg.maxInShard {
				if !devicePreviouslySelected(solverData, device, hwCache, modelCache, multiEqcByModel) {
					selectedDevice = device
					updateCaches(solverData, selectedDevice, hwCache, modelCache, multiEqcByModel)
					return selectedDevice, true
				}
			}
		}
	}

	// Keep record of the hwCacheSize prior to bestChoice selection
	hwCacheSizePrev := len(hwCache)
	// If that cannot be done, then just pick the device with the most available.
	selectedDevice = bestChoice(solverData, devices, hwCache, modelCache, multiEqcByModel)

	// If no new devices were selected, then this means all available real devices
	// have been selected.
	if hwCacheSizePrev == len(hwCache) {
		// If the hwCache is populated, we need to check if the multiEqcByModel flag is set
		if len(hwCache) > 0 {
			// If the flag is set, then we have iterated through all possible available device selections
			// by unique model and next we need to iterate through all unique device selections
			// by a recursive call with the mulitEqcByModel set to false.
			if multiEqcByModel {
				return getDevices(solverData, numTests, hwHash, harness, hwCache, modelCache, false)
			}
			// If the flag is not set, we have iterated through all possible available unique device selections
			// and there is nothing left and reset the selectedDevice.
			selectedDevice = uint64(0)
		}
		// If the cache is empty, then there are no other choices left and
		// return the bestChoice selected device, which is one that will be rejected
	}
	return selectedDevice, false
}

// bestChoice is here to try to grab the most free best based on the following:
// Grab a the most free device
// If none are free, grab the one which has the most amount of un-assigned work to it in the queue.
// Its good to note that freeDevices is never expected to go negative, but `totalDevicesUnAssigned` can.
func bestChoice(solverData *middleOutData, devices []uint64, hwCache map[uint64]bool, modelCache map[string]bool, multiEqcByModel bool) (selectedDevice uint64) {
	maxAvailableFound := math.MinInt32

	// Look through the un-allocated devices for a free one.
	for _, device := range devices {
		if solverData.flatHWUUIDMap[device].labLoading.freeDevices > maxAvailableFound {
			if !devicePreviouslySelected(solverData, device, hwCache, modelCache, multiEqcByModel) {
				maxAvailableFound = solverData.flatHWUUIDMap[device].labLoading.freeDevices
				selectedDevice = device
			}
		}
	}

	if maxAvailableFound > 0 {
		updateCaches(solverData, selectedDevice, hwCache, modelCache, multiEqcByModel)
		return selectedDevice
	}

	// Otherwise, lets just get the one with the most in the Lab.
	// This will be a self-balancing function as if CTPs build a queue on the more deployed devices
	// the less deployed devices will start to free up, then other CTPs will grab them in the upper loop.

	// Reset the selectedDevice.
	selectedDevice = uint64(0)
	maxAvailableFound = math.MinInt32
	for _, device := range devices {
		if solverData.flatHWUUIDMap[device].labLoading.totalDevicesUnAssigned > maxAvailableFound {
			// Skip devices which do not actually exist.
			if solverData.flatHWUUIDMap[device].labLoading.staticBotCount == 0 {
				continue
			}
			if !devicePreviouslySelected(solverData, device, hwCache, modelCache, multiEqcByModel) {
				maxAvailableFound = solverData.flatHWUUIDMap[device].labLoading.freeDevices
				selectedDevice = device
			}
		}
	}

	// If a device was found, then return it.
	if selectedDevice != uint64(0) {
		updateCaches(solverData, selectedDevice, hwCache, modelCache, multiEqcByModel)
		return selectedDevice
	}

	// Finally, if we have no free devices, and no busy but real devices, we will select
	// the device that was requested; and thus will reject the task.
	maxAvailableFound = math.MinInt32
	for _, device := range devices {
		if solverData.flatHWUUIDMap[device].labLoading.totalDevicesUnAssigned > maxAvailableFound {
			maxAvailableFound = solverData.flatHWUUIDMap[device].labLoading.totalDevicesUnAssigned
			selectedDevice = device
		}
	}
	return selectedDevice
}

func devicePreviouslySelected(solverData *middleOutData, device uint64, hwCache map[uint64]bool, modelCache map[string]bool, multiEqcByModel bool) bool {
	if multiEqcByModel {
		model := modelFromHwInfo(solverData.flatHWUUIDMap[device])
		if _, ok := modelCache[model]; !ok {
			return false
		}
	} else {
		if _, ok := hwCache[device]; !ok {
			return false
		}
	}
	return true
}

func updateCaches(solverData *middleOutData, device uint64, hwCache map[uint64]bool, modelCache map[string]bool, multiEqcByModel bool) {
	if multiEqcByModel {
		model := modelFromHwInfo(solverData.flatHWUUIDMap[device])
		modelCache[model] = true
	}
	hwCache[device] = true
}

// hwSearchOrdering: given the flatUUIDLoadingMap, return the order of least common to most common boards/eqs.
func hwSearchOrdering(flatEqMap map[uint64][]uint64) []kv {
	/*
		 For example: hwEqMap {hw1: [hw1,hw2,hw3], hw3: [hw3]}

		we want to allow hw3 to find matches first. This is to give a chance for all tests which specifically require hw3
		to take prioty on that hw.

		Example problem we are trying to solve (given the hwEq map above)
			- labLoading provides there are 2 devices for each type
			- There is a maximum of 2 tests per shard allowed
			- The HW to TC Loading:
				hw1: [tc1, tc2, ... 8]
				hw3: [tc1 .. 4]

			If we allowed hw1 to be solved first, it could potentially be assigned HW3
			(as it has the same avalbility as HW1/Hw2). Then when the Hw3 TC come to be assigned, all the HW3 devices have been
			loaded.

	*/

	var sortedStruct []kv
	for k, v := range flatEqMap {
		sortedStruct = append(sortedStruct, kv{key: k, value: len(v)})
	}

	sort.Slice(sortedStruct, func(i, j int) bool {
		return sortedStruct[i].value < sortedStruct[j].value
	})
	return sortedStruct
}

// addHWtohwUUIDMap is a helper method to inject into the map without overwriting the keys existing values.
func addHWtohwUUIDMap(hwUUIDMap map[uint64]*api.SchedulingUnitOptions, hw *api.SchedulingUnitOptions) uint64 {
	hash, _ := hashstructure.Hash(hw, hashstructure.FormatV2, nil)
	if _, hwExists := hwUUIDMap[hash]; !hwExists {
		// Add the HW to the lookup map if not seen before.
		hwUUIDMap[hash] = hw
	}
	return hash
}

// TODO; when HwRequirements is fully deprecated, remove this.
// oldAddHWtohwUUIDMap is a helper method to inject into the map without overwriting the keys existing values.
func oldAddHWtohwUUIDMap(hwUUIDMap map[uint64]*api.HWRequirements, hw *api.HWRequirements) uint64 {
	hash, _ := hashstructure.Hash(hw, hashstructure.FormatV2, nil)
	_, hwExists := hwUUIDMap[hash]
	// Add the HW to the lookup map if not seen before.
	if !hwExists {
		hwUUIDMap[hash] = hw
	}
	return hash
}

// addHWtoFlatHWUUIDMap is a helper method to inject into the map without overwriting the keys existing values.
func addHWtoFlatHWUUIDMap(ctx context.Context, flatHWUUIDMap map[uint64]*hwInfo, k uint64, v *hwInfo) error {
	_, exists := flatHWUUIDMap[k]
	// Add the HW to the lookup map if not seen before.
	if !exists {
		flatHWUUIDMap[k] = v
	} else if !reflect.DeepEqual(flatHWUUIDMap[k].req, v.req) {
		flatHWUUIDMap[k] = v
		return fmt.Errorf("mismatch")
	}
	return nil
}

type hashHelper struct {
	S    *labapi.Dut
	V    string
	Deps []string
}

func hashWithDeps(unit *api.SchedulingUnit) uint64 {
	h := hashHelper{S: unit.GetPrimaryTarget().GetSwarmingDef().GetDutInfo(),
		Deps: unit.GetPrimaryTarget().GetSwarmingDef().GetSwarmingLabels()}

	hasher := []hashHelper{h}
	for _, secondary := range unit.GetCompanionTargets() {

		if secondary.GetSwarmingDef().GetDutInfo() != nil {
			sh := hashHelper{S: secondary.GetSwarmingDef().GetDutInfo()}

			hasher = append(hasher, sh)
		}
	}
	hash, _ := hashstructure.Hash(hasher, hashstructure.FormatV2, nil)
	return hash
}

func hashForSchedulingUnit(unit *api.SchedulingUnit) uint64 {
	return hashSchedulingUnit(unit, false)
}

// This only looks at the actual DUTinfo, thus it ignores variant.
func hashForSchedulingUnitHwOnly(unit *api.SchedulingUnit) uint64 {
	return hashSchedulingUnit(unit, true)
}

func hashSchedulingUnit(unit *api.SchedulingUnit, hwOnly bool) uint64 {
	h := hashHelper{S: unit.GetPrimaryTarget().GetSwarmingDef().GetDutInfo()}
	if !hwOnly {
		h.V = unit.GetPrimaryTarget().GetSwarmingDef().GetVariant()
	}

	hasher := []hashHelper{h}
	for _, secondary := range unit.GetCompanionTargets() {

		if secondary.GetSwarmingDef().GetDutInfo() != nil {
			sh := hashHelper{S: secondary.GetSwarmingDef().GetDutInfo()}
			if !hwOnly {
				h.V = secondary.GetSwarmingDef().GetVariant()
			}
			hasher = append(hasher, sh)
		}
	}
	hash, _ := hashstructure.Hash(hasher, hashstructure.FormatV2, nil)
	return hash
}
