// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	hashstructure "github.com/mitchellh/hashstructure/v2"

	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

func makeCtx() context.Context {
	return context.Background()
}

type testVars struct {
	HwDef001 *api.SwarmingDefinition
	HwDef002 *api.SwarmingDefinition
	HwDef003 *api.SwarmingDefinition
	HwDef004 *api.SwarmingDefinition

	hwDef001WDeps     *api.SwarmingDefinition
	HwDef001WVariant  *api.SwarmingDefinition
	HwDef001WVariant2 *api.SwarmingDefinition
	HwDef001WVariant3 *api.SwarmingDefinition

	HwDef001SU          *api.SchedulingUnit
	HwDef001SUWVariant  *api.SchedulingUnit
	HwDef001SUWVariant2 *api.SchedulingUnit
	HwDef001SUWVariant3 *api.SchedulingUnit

	HwDef001SUWDeps *api.SchedulingUnit
	HwDef002SU      *api.SchedulingUnit
	HwDef003SU      *api.SchedulingUnit
	HwDef004SU      *api.SchedulingUnit

	HwDef001SUWHW1Companion *api.SchedulingUnit

	SU1          *api.SchedulingUnitOptions
	SU1WDeps     *api.SchedulingUnitOptions
	SU1WVariant  *api.SchedulingUnitOptions
	SU1WVariant2 *api.SchedulingUnitOptions
	SU1WVariant3 *api.SchedulingUnitOptions

	SU2 *api.SchedulingUnitOptions
	SU3 *api.SchedulingUnitOptions
	SU4 *api.SchedulingUnitOptions

	SU1Or2    *api.SchedulingUnitOptions
	SU2Or3    *api.SchedulingUnitOptions
	SU1Or2Or3 *api.SchedulingUnitOptions

	SU1W2Vars *api.SchedulingUnitOptions
	SU1W3Vars *api.SchedulingUnitOptions

	HwEqMap   map[uint64][]uint64
	HwUUIDMap map[uint64]*api.SchedulingUnitOptions

	HwHash0 uint64
	HwHash1 uint64
	HwHash2 uint64
}

func buildTestVars() *testVars {
	// All of these are common stuff used across different tests; rater than having to reconstruct them in each.
	emptyCompanions := []*api.SwarmingDefinition{}

	hwDef001 := buildTestProto("foo", "bar")
	hwDef002 := buildTestProto("foo", "bar2")
	hwDef003 := buildTestProto("foo", "bar3")
	hwDef004 := buildTestProto("foo", "bar4")

	swarmingLabels := []string{"foo"}

	hwDef001WDeps := buildTestProtoDeps("foo", "bar", swarmingLabels)

	hwDef001SU := schedulingUnitFromSwarmingDefs(hwDef001, emptyCompanions)
	hwDef002SU := schedulingUnitFromSwarmingDefs(hwDef002, emptyCompanions)
	hwDef003SU := schedulingUnitFromSwarmingDefs(hwDef003, emptyCompanions)
	hwDef004SU := schedulingUnitFromSwarmingDefs(hwDef004, emptyCompanions)

	hwDef001SUWDeps := schedulingUnitFromSwarmingDefs(hwDef001WDeps, emptyCompanions)

	companionsWHwDef001 := []*api.SwarmingDefinition{hwDef001}
	hwDef001SUWHW1Companion := schedulingUnitFromSwarmingDefs(hwDef001, companionsWHwDef001)

	hwDef001WVariant := buildTestProtoWVariant("foo", "bar", "amd-256-kernelnext")
	hwDef001WVariant2 := buildTestProtoWVariant("foo", "bar", "amd-9001-kernelnext")
	hwDef001WVariant3 := buildTestProtoWVariant("foo", "bar", "dma-256-kernelnext")

	hw1VariantSchedulingUnit := schedulingUnitFromSwarmingDefs(hwDef001WVariant, []*api.SwarmingDefinition{})
	hw1Variant2SchedulingUnit := schedulingUnitFromSwarmingDefs(hwDef001WVariant2, []*api.SwarmingDefinition{})
	hw1Variant3SchedulingUnit := schedulingUnitFromSwarmingDefs(hwDef001WVariant3, []*api.SwarmingDefinition{})

	suFor1_2 := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hwDef001SU, hwDef002SU},
	}
	suFor2_3 := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hwDef003SU, hwDef002SU},
	}
	suFor1_2_3 := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hwDef003SU, hwDef002SU, hwDef001SU},
	}
	sU1W2Vars := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hw1VariantSchedulingUnit, hw1Variant2SchedulingUnit},
	}
	sU1W3Vars := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hw1VariantSchedulingUnit, hw1Variant2SchedulingUnit, hw1Variant3SchedulingUnit},
	}
	suFor1 := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hwDef001SU},
	}
	suFor1WDeps := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hwDef001SUWDeps},
	}
	suFor1WVariant := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hw1VariantSchedulingUnit},
	}
	suFor1WVariant2 := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hw1Variant2SchedulingUnit},
	}
	suFor1WVariant3 := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hw1Variant3SchedulingUnit},
	}
	suFor2 := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hwDef002SU},
	}
	suFor3 := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hwDef003SU},
	}
	suFor4 := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{hwDef004SU},
	}

	hwEqMap := make(map[uint64][]uint64)
	hwUUIDMap := make(map[uint64]*api.SchedulingUnitOptions)

	hwHash0 := uint64(0)
	hwHash1 := uint64(1)
	hwHash2 := uint64(2)

	hwEqMap[hwHash0] = []uint64{hwHash0, hwHash1}
	hwEqMap[hwHash1] = []uint64{hwHash1}
	hwUUIDMap[hwHash0] = suFor1_2
	hwUUIDMap[hwHash1] = suFor1_2_3
	hwUUIDMap[hwHash2] = suFor4
	hwEqMap[hwHash0] = []uint64{hwHash0, hwHash1, hwHash2}
	hwEqMap[hwHash2] = []uint64{hwHash2}

	return &testVars{
		HwDef001: hwDef001,
		HwDef002: hwDef002,
		HwDef003: hwDef003,
		HwDef004: hwDef004,

		hwDef001WDeps: hwDef001WDeps,

		HwDef001SU: hwDef001SU,
		HwDef002SU: hwDef002SU,
		HwDef003SU: hwDef003SU,
		HwDef004SU: hwDef004SU,

		HwDef001SUWDeps:     hwDef001SUWDeps,
		HwDef001SUWVariant:  hw1VariantSchedulingUnit,
		HwDef001SUWVariant2: hw1Variant2SchedulingUnit,
		HwDef001SUWVariant3: hw1Variant3SchedulingUnit,

		HwDef001SUWHW1Companion: hwDef001SUWHW1Companion,
		HwDef001WVariant:        hwDef001WVariant,
		HwDef001WVariant2:       hwDef001WVariant2,
		HwDef001WVariant3:       hwDef001WVariant3,

		SU1:          suFor1,
		SU2:          suFor2,
		SU3:          suFor3,
		SU4:          suFor4,
		SU1WDeps:     suFor1WDeps,
		SU1WVariant:  suFor1WVariant,
		SU1WVariant2: suFor1WVariant2,
		SU1WVariant3: suFor1WVariant3,

		SU1Or2:    suFor1_2,
		SU2Or3:    suFor2_3,
		SU1Or2Or3: suFor1_2_3,

		SU1W2Vars: sU1W2Vars,
		SU1W3Vars: sU1W3Vars,

		HwEqMap:   hwEqMap,
		HwUUIDMap: hwUUIDMap,

		HwHash0: hwHash0,
		HwHash1: hwHash1,
		HwHash2: hwHash2,
	}

}

func makeHwInfo(hw *api.HWRequirements) *hwInfo {
	hashV, _ := hashstructure.Hash(hw.HwDefinition[0].DutInfo, hashstructure.FormatV2, nil)
	provV, _ := hashstructure.Hash(hw.HwDefinition[0].Variant, hashstructure.FormatV2, nil)
	return &hwInfo{
		oldReq:    hw,
		hwValue:   hashV,
		provValue: provV,
	}
}

func makeHwInfoNew(innerHW *api.SchedulingUnit) *hwInfo {
	flattened := &api.SchedulingUnitOptions{
		SchedulingUnits: []*api.SchedulingUnit{innerHW},
	}
	return &hwInfo{
		req:     flattened,
		hwValue: hashForSchedulingUnit(innerHW),
	}

}

func buildTestProto(boardName string, modelName string) *api.SwarmingDefinition {
	dut := &dut_api.Dut{}

	Cros := &dut_api.Dut_ChromeOS{DutModel: &dut_api.DutModel{
		BuildTarget: boardName,
		ModelName:   modelName,
	}}
	dut.DutType = &dut_api.Dut_Chromeos{Chromeos: Cros}

	return &api.SwarmingDefinition{DutInfo: dut}
}

func buildTestProtoWVariant(boardName string, modelName string, variant string) *api.SwarmingDefinition {
	start := buildTestProto(boardName, modelName)
	start.Variant = variant
	return start
}

func buildTestProtoDeps(boardName string, modelName string, deps []string) *api.SwarmingDefinition {
	dut := &dut_api.Dut{}

	Cros := &dut_api.Dut_ChromeOS{DutModel: &dut_api.DutModel{
		BuildTarget: boardName,
		ModelName:   modelName,
	}}
	dut.DutType = &dut_api.Dut_Chromeos{Chromeos: Cros}

	return &api.SwarmingDefinition{DutInfo: dut, SwarmingLabels: deps}
}

func buildTestProtoWithProvisionInfo(boardName string, modelName string, provInfo string) *api.SwarmingDefinition {
	dut := &dut_api.Dut{}

	Cros := &dut_api.Dut_ChromeOS{DutModel: &dut_api.DutModel{
		BuildTarget: boardName,
		ModelName:   modelName,
	}}
	dut.DutType = &dut_api.Dut_Chromeos{Chromeos: Cros}
	return &api.SwarmingDefinition{DutInfo: dut,

		Variant: provInfo,
	}
}
func buildTestProtoWithProvisionInfoAndDeps(boardName string, modelName string, provInfo string, deps []string) *api.SwarmingDefinition {
	dut := &dut_api.Dut{}

	Cros := &dut_api.Dut_ChromeOS{DutModel: &dut_api.DutModel{
		BuildTarget: boardName,
		ModelName:   modelName,
	}}
	dut.DutType = &dut_api.Dut_Chromeos{Chromeos: Cros}
	return &api.SwarmingDefinition{DutInfo: dut,
		SwarmingLabels: deps,
		Variant:        provInfo,
	}
}

func TestAllItemsIn(t *testing.T) {
	if !allItemsIn([]string{"foo"}, []string{"foo"}) {
		t.Fatal("[foo] not found in [foo] when should be")
	}
	if !allItemsIn([]string{"foo"}, []string{"foo", "bar"}) {
		t.Fatal("[foo] not found in [foo, bar] when should be")
	}
	if !allItemsIn([]string{}, []string{}) {
		t.Fatal("[] not found in [] when should be")
	}
	if allItemsIn([]string{"foo", "bar"}, []string{"foo"}) {
		t.Fatal("[foo, bar] found in [foo] when should not be")
	}

}

// TODO; once HWRequirements is deprecated; remove this test.
func TestOldFindMatches(t *testing.T) {
	swarmingLabels := []string{"foo"}

	hwDef0a := buildTestProtoDeps("foo", "bar", swarmingLabels)
	hwDef0 := buildTestProto("foo", "bar")
	hwDef01 := buildTestProto("foo", "bar1")
	hwDef02 := buildTestProto("foo", "bar2")

	hw1 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef0},
	}

	hw1a := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef0a},
	}

	hw1AndHw2 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef0, hwDef02},
	}

	hw1AndHw2AndHw3 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef0, hwDef01, hwDef02},
	}

	// This test assumes flattenList works; which has its own unittest coverage.
	// Note: to ensure proper test coverage of the "findMatches" method, we must use flattenList
	// to generate the data rather than hand crafting.
	eqMap := oldFlattenList(makeCtx(), []*api.HWRequirements{hw1AndHw2AndHw3})
	flatHWUUIDMap := make(map[uint64]*hwInfo)

	for k, v := range eqMap {
		flatHWUUIDMap[k] = v
	}

	if len(oldFindMatches(hw1AndHw2, flatHWUUIDMap)) != 2 {
		t.Fatal("Map did not match both items from test class")
	}
	if len(oldFindMatches(hw1a, flatHWUUIDMap)) != 0 {
		t.Fatal("Map matched items with dependencies when it should not have")
	}

	flatWithDeps := oldFlattenList(makeCtx(), []*api.HWRequirements{hw1a})
	flatHWUUIDMap = make(map[uint64]*hwInfo)

	for k, v := range flatWithDeps {
		flatHWUUIDMap[k] = v
	}

	if len(oldFindMatches(hw1, flatHWUUIDMap)) != 1 {
		t.Fatal("Did not find match when should have")
	}

}

func TestFindMatches(t *testing.T) {
	vars := buildTestVars()

	// This test assumes flattenList works; which has its own unittest coverage.
	// Note: to ensure proper test coverage of the "findMatches" method, we must use flattenList
	// to generate the data rather than hand crafting.
	eqMap := flattenList(makeCtx(), []*api.SchedulingUnitOptions{vars.SU1Or2Or3})
	flatHWUUIDMap := make(map[uint64]*hwInfo)

	for k, v := range eqMap {
		flatHWUUIDMap[k] = v
	}

	// SU1 and SU2 should be found
	if len(findMatches(makeCtx(), vars.SU1Or2, flatHWUUIDMap)) != 2 {
		t.Fatal("Map did not match both items from test class")
	}

	// SU1WDeps should not be found, as its not in the given EqMap above.
	if len(findMatches(makeCtx(), vars.SU1WDeps, flatHWUUIDMap)) != 0 {
		t.Fatal("Map matched items with dependencies when it should not have")
	}

	flatWithDeps := flattenList(makeCtx(), []*api.SchedulingUnitOptions{vars.SU1WDeps})
	flatHWUUIDMap = make(map[uint64]*hwInfo)

	for k, v := range flatWithDeps {
		flatHWUUIDMap[k] = v
	}

	// However; we should find that a test with out deps, can run on HW _with_ deps.
	if len(findMatches(makeCtx(), vars.SU1, flatHWUUIDMap)) != 1 {
		t.Fatal("Did not find match when should have")
	}

	// Variant Matching
	flatWVariant := flattenList(makeCtx(), []*api.SchedulingUnitOptions{vars.SU1WVariant})
	flatHWUUIDMap = make(map[uint64]*hwInfo)

	for k, v := range flatWVariant {
		flatHWUUIDMap[k] = v
	}
	// variant matches variant
	if len(findMatches(makeCtx(), vars.SU1WVariant, flatHWUUIDMap)) != 1 {
		t.Fatal("Did not find match when should have")
	}
	// variant does NOT match NON variant
	if len(findMatches(makeCtx(), vars.SU1, flatHWUUIDMap)) == 1 {
		t.Fatal("found match when should not have")
	}

}

func TestAssignHardware(t *testing.T) {
	vars := buildTestVars()
	selectedDevice := uint64(1)
	selectedDevice2 := uint64(2)

	expandCurrentShard := false
	flatUUIDLoadingMap := make(map[uint64]*hwInfo)

	l := &loading{freeDevices: 2, totalDevicesUnAssigned: 2, staticBotCount: int64(2)}
	flatUUIDLoadingMap[selectedDevice] = &hwInfo{
		req:               vars.SU1,
		labLoading:        l,
		numInCurrentShard: 0,
	}
	flatUUIDLoadingMap[selectedDevice2] = &hwInfo{
		req:               vars.SU2,
		labLoading:        l,
		numInCurrentShard: 0,
	}

	shardedtc := []string{"1"}
	shardedtc2 := []string{"2"}
	shardedtc3 := []string{"3"}

	cfg := distroCfg{
		maxInShard:                       2,
		requestsPerSchedulingUnitOptions: 1,
	}

	solverData := newMiddleOutData()
	solverData.cfg = cfg
	solverData.flatHWUUIDMap = flatUUIDLoadingMap

	assignHardware(solverData, selectedDevice, expandCurrentShard, shardedtc, selectedDevice)
	if flatUUIDLoadingMap[selectedDevice].labLoading.freeDevices != 1 {
		t.Fatalf("Assigning a device did not reduce its lab loading :%v", flatUUIDLoadingMap[selectedDevice])
	}
	if flatUUIDLoadingMap[selectedDevice].numInCurrentShard != 1 {
		t.Fatalf("Assigning an empty shard 1 test did not increase its num in shard count")
	}

	assignHardware(solverData, selectedDevice, true, shardedtc2, selectedDevice)
	if flatUUIDLoadingMap[selectedDevice].labLoading.freeDevices != 1 {
		t.Fatalf("Assigning a device did not reduce its lab loading")
	}
	if flatUUIDLoadingMap[selectedDevice].numInCurrentShard != 0 {
		t.Fatalf("Filling a shard did not reset the count")
	}

	assignHardware(solverData, selectedDevice, false, shardedtc3, selectedDevice)
	if flatUUIDLoadingMap[selectedDevice].labLoading.freeDevices != 0 {
		t.Fatalf("Assigning a device did not reduce its lab loading")
	}
	if flatUUIDLoadingMap[selectedDevice].numInCurrentShard != 1 {
		t.Fatalf("Filling a shard did not reset the count")
	}

	assignHardware(solverData, selectedDevice2, false, shardedtc, selectedDevice)
	if flatUUIDLoadingMap[selectedDevice].labLoading.totalDevicesUnAssigned != -1 {
		t.Fatalf("Same HW; different groupping should share the same lab resource but didn't")
	}
	if flatUUIDLoadingMap[selectedDevice].numInCurrentShard != 1 {
		t.Fatalf("new grouping should only show 1 in shard")
	}

	expected1 := [][]string{
		{"1", "2"},
		{"3"},
	}

	expected2 := [][]string{
		{"1"},
	}

	if !reflect.DeepEqual(solverData.finalAssignments[selectedDevice], expected1) {
		t.Fatalf("incorrect hw assignments1")
	}

	if !reflect.DeepEqual(solverData.finalAssignments[selectedDevice2], expected2) {
		t.Fatalf("incorrect hw assignments2")
	}

}

// TODO; once HWRequirements is deprecated; remove this test.
func TestFindMatchesProv(t *testing.T) {
	// Test that a hwDef with provision info is not matched unless the provision info is also the same.
	swarmingLabels := []string{"foo"}

	hwDef0 := buildTestProto("foo", "bar")
	hwDef01 := buildTestProto("foo", "bar1")

	hwDef02 := buildTestProto("foo", "bar2")

	hwDef03 := buildTestProtoWithProvisionInfo("foo", "bar2", "prov")

	hwDef03Deps := buildTestProtoWithProvisionInfoAndDeps("foo", "bar2", "prov", swarmingLabels)

	hwDef0WDeps := buildTestProtoDeps("foo", "bar", swarmingLabels)
	testWithDeps := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef0WDeps},
	}

	hw2 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef03},
	}

	testWithProvAndDeps := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef03Deps},
	}
	testClass := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef0, hwDef02},
	}

	classWithNoDeps := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef0, hwDef01, hwDef02, hwDef03},
	}

	// This test assumes flattenList works; which has its own unittest coverage.
	// Note: to ensure proper test coverage of the "findMatches" method, we must use flattenList
	// to generate the data rather than hand crafting.
	flatMap := oldFlattenList(makeCtx(), []*api.HWRequirements{classWithNoDeps})
	flatHWUUIDMap := make(map[uint64]*hwInfo)

	for k, v := range flatMap {
		flatHWUUIDMap[k] = v
	}

	if len(oldFindMatches(testClass, flatHWUUIDMap)) != 2 {
		t.Fatal("Map did not match both items from test class")
	}
	if len(oldFindMatches(testWithDeps, flatHWUUIDMap)) != 0 {
		t.Fatal("Map matched items with dependencies when it should not have")
	}

	// Test we get a match with provision and subdeps.
	flatMap = oldFlattenList(makeCtx(), []*api.HWRequirements{testWithProvAndDeps})
	flatHWUUIDMap = make(map[uint64]*hwInfo)

	for k, v := range flatMap {
		flatHWUUIDMap[k] = v
	}
	if len(oldFindMatches(hw2, flatHWUUIDMap)) != 1 {
		t.Fatal("Map matched items with dependencies when it should not have")
	}
}

func TestSharedDeviceLabLoadingDifferentProvision3(t *testing.T) {
	vars := buildTestVars()
	hwEqMap := make(map[uint64][]uint64)
	hwUUIDMap := make(map[uint64]*api.SchedulingUnitOptions)
	HwHash0 := uint64(0)
	HwHash1 := uint64(1)
	HwHash2 := uint64(2)

	hwEqMap[HwHash0] = []uint64{HwHash0}
	hwEqMap[HwHash1] = []uint64{HwHash1}
	hwEqMap[HwHash2] = []uint64{HwHash2}

	hwUUIDMap[HwHash0] = vars.SU1W2Vars
	hwUUIDMap[HwHash1] = vars.SU1W3Vars
	hwUUIDMap[HwHash2] = vars.SU1WVariant3

	newEq, flatUUIDLoadingMap := flattenEqMap(hwEqMap, hwUUIDMap)

	solverData := newMiddleOutData()
	solverData.cfg = distroCfg{
		unitTestDevices:                  2,
		maxInShard:                       2,
		requestsPerSchedulingUnitOptions: 1}
	solverData.flatHWUUIDMap = flatUUIDLoadingMap
	solverData.hwEquivalenceMap = newEq

	devices := solverData.hwEquivalenceMap[HwHash1]
	for _, device := range devices {
		solverData.flatHWUUIDMap[device].shardHarness = "tast"
	}

	populateLabAvalability(makeCtx(), solverData)

	hwCache := map[uint64]bool{}
	modelCache := map[string]bool{}

	// The goal of this check is to ensure the first test goes into a new shard.
	selectedDevice, expandCurrentShard := getDevices(solverData, 2, HwHash1, "tast", hwCache, modelCache, false)
	if expandCurrentShard {
		t.Fatalf("First test should go into new shard and did not")
	}

	// Set the # of tests currently in the shard to 1
	flatUUIDLoadingMap[selectedDevice].numInCurrentShard = 1
	hwCache = map[uint64]bool{}
	modelCache = map[string]bool{}

	// Adding another test should result in the shard being expanded; and the same device being selected.
	selectedDevice2, expandCurrentShard := getDevices(solverData, 1, HwHash1, "tast", hwCache, modelCache, false)

	if selectedDevice != selectedDevice2 {
		t.Fatalf("Shard was not filled when it should have been")
	}
	if !expandCurrentShard {
		t.Fatalf("Should be in the same shard.")
	}
	// Reset the shard, and reduce the number of devices for this by 1.
	flatUUIDLoadingMap[selectedDevice].numInCurrentShard = 0
	// Since each SchedulingUnit in the map is the same underlying hardware; just with different SW vairants
	// Reducing the count of _one_ device should reduce the count of *ALL* scheduling units.
	flatUUIDLoadingMap[selectedDevice].labLoading.freeDevices--
	for _, v := range flatUUIDLoadingMap {
		if v.labLoading.freeDevices != 1 {
			t.Fatalf("All devices share same hardware thus all should be reduced by 1")
		}
	}
}

func TestGreedyDistro(t *testing.T) {
	vars := buildTestVars()

	hwToTCMap := make(map[uint64][]string)

	hwToTCMap[vars.HwHash0] = []string{"a", "b", "c"}
	hwToTCMap[vars.HwHash1] = []string{"d", "e"}
	hwToTCMap[vars.HwHash2] = []string{"f", "g", "h", "i", "j", "k"}

	cfg := distroCfg{isUnitTest: true, unitTestDevices: 3, maxInShard: 50, requestsPerSchedulingUnitOptions: 1}

	eqMap := flattenList(makeCtx(), []*api.SchedulingUnitOptions{vars.SU1Or2, vars.SU1Or2Or3, vars.SU4})

	flatHWUUIDMap := make(map[uint64]*hwInfo)

	for k, v := range eqMap {
		flatHWUUIDMap[k] = v
	}

	flatEqMap := make(map[uint64][]uint64)

	for key, hwOptions := range vars.HwUUIDMap {
		flatEqMap[key] = findMatches(makeCtx(), hwOptions, flatHWUUIDMap)
	}

	solverData := newMiddleOutData()
	solverData.hwToTCMap = hwToTCMap
	solverData.hwEquivalenceMap = flatEqMap
	solverData.hwUUIDMap = vars.HwUUIDMap
	solverData.cfg = cfg
	solverData.flatHWUUIDMap = flatHWUUIDMap

	finalAssignments := greedyDistro(makeCtx(), solverData)

	expected := []*allowedAssignment{
		{
			tc:   []string{"a", "b", "c", "d", "e"},
			nwHw: vars.SU1,
		},
		{
			tc:   []string{"a", "b", "c", "d", "e"},
			nwHw: vars.SU2,
		},
		{
			tc:   []string{"a", "b", "c", "d", "e"},
			nwHw: vars.SU3,
		},
		{
			tc:   []string{"a", "b", "c", "f", "g", "h", "i", "j", "k"},
			nwHw: vars.SU4,
		},
	}

	expectedTcs := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}
	correct, err := validateDistro(finalAssignments, flatHWUUIDMap, cfg, expectedTcs, expected)
	if !correct {
		t.Fatal(err)
	}

	// Rerun the same test with different settings, ensure things still "work"
	cfg = distroCfg{
		isUnitTest:                       true,
		maxInShard:                       5,
		unitTestDevices:                  5,
		requestsPerSchedulingUnitOptions: 1,
	}
	solverData.cfg = cfg

	solverData.finalAssignments = make(map[uint64][][]string)
	finalAssignments = greedyDistro(makeCtx(), solverData)
	correct, err = validateDistro(finalAssignments, flatHWUUIDMap, cfg, expectedTcs, expected)
	if !correct {
		t.Fatal(err)
	}
}

func TestGreedyDistroIterations(t *testing.T) {
	vars := buildTestVars()

	hwToTCMap := make(map[uint64][]string)

	hwToTCMap[vars.HwHash0] = []string{"a", "b", "c"}
	hwToTCMap[vars.HwHash1] = []string{"d", "e"}
	hwToTCMap[vars.HwHash2] = []string{"f", "g", "h", "i", "j", "k"}

	cfg := distroCfg{isUnitTest: true, unitTestDevices: 3, maxInShard: 50, requestsPerSchedulingUnitOptions: 3}

	eqMap := flattenList(makeCtx(), []*api.SchedulingUnitOptions{vars.SU1Or2, vars.SU1Or2Or3, vars.SU4})

	flatHWUUIDMap := make(map[uint64]*hwInfo)

	for k, v := range eqMap {
		flatHWUUIDMap[k] = v
	}

	flatEqMap := make(map[uint64][]uint64)

	for key, hwOptions := range vars.HwUUIDMap {
		flatEqMap[key] = findMatches(makeCtx(), hwOptions, flatHWUUIDMap)
	}

	solverData := newMiddleOutData()
	solverData.hwToTCMap = hwToTCMap
	solverData.hwEquivalenceMap = flatEqMap
	solverData.hwUUIDMap = vars.HwUUIDMap
	solverData.cfg = cfg
	solverData.flatHWUUIDMap = flatHWUUIDMap

	finalAssignments := greedyDistro(makeCtx(), solverData)

	expected := []*allowedAssignment{
		{
			tc:   []string{"a", "b", "c", "d", "e"},
			nwHw: vars.SU1,
		},
		{
			tc:   []string{"a", "b", "c", "d", "e"},
			nwHw: vars.SU2,
		},
		{
			tc:   []string{"a", "b", "c", "d", "e"},
			nwHw: vars.SU3,
		},
		{
			tc:   []string{"a", "b", "c", "f", "g", "h", "i", "j", "k"},
			nwHw: vars.SU4,
		},
	}

	expectedTcs := []string{"a", "a", "b", "b", "c", "c", "d", "d", "d", "e", "e", "e", "f", "g", "h", "i", "j", "k"}
	correct, err := validateDistro(finalAssignments, flatHWUUIDMap, cfg, expectedTcs, expected)
	if !correct {
		t.Fatal(err)
	}
}

// TODO; remove this test when the proto migration is completed.
func TestOldIsParent(t *testing.T) {
	parent := &hwInfo{hwValue: uint64(0)}

	helperList := []*helper{}

	if !oldIsParent(parent, helperList) {
		t.Fatalf("Empty parent/child should be true")
	}

	swarmingLabels := []string{"foo"}
	hwDef0 := buildTestProto("foo", "bar")
	hwDef001WDeps := buildTestProtoWithProvisionInfoAndDeps("foo", "bar", "prov1", swarmingLabels)
	hwDef002 := buildTestProtoWithProvisionInfo("foo", "bar", "prov2")

	hw1aprov1 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef001WDeps},
	}
	hw1prov2 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef002},
	}

	hw1aprov1L := []*helper{}

	for _, child := range hw1aprov1.HwDefinition {

		childHash, _ := hashstructure.Hash(child.DutInfo, hashstructure.FormatV2, nil)
		childHashProv, _ := hashstructure.Hash(child.Variant, hashstructure.FormatV2, nil)
		h := &helper{
			hashV:          childHash,
			hashProvV:      childHashProv,
			swarmingLabels: child.GetSwarmingLabels(),
		}
		hw1aprov1L = append(hw1aprov1L, h)
	}

	hw1prov2L := []*helper{}

	for _, child := range hw1prov2.HwDefinition {

		childHash, _ := hashstructure.Hash(child.DutInfo, hashstructure.FormatV2, nil)
		childHashProv, _ := hashstructure.Hash(child.Variant, hashstructure.FormatV2, nil)
		h := &helper{
			hashV:          childHash,
			hashProvV:      childHashProv,
			swarmingLabels: child.GetSwarmingLabels(),
		}
		hw1prov2L = append(hw1prov2L, h)
	}

	// Can 1 run 2?
	if oldIsParent(makeHwInfo(hw1aprov1), hw1prov2L) {
		t.Fatalf("Match found even with different provision info")
	}
	if oldIsParent(makeHwInfo(hw1prov2), hw1aprov1L) {
		t.Fatalf("Match found even with different provision info")
	}

	// In this case, the parent has more swarming labels defined than the child.
	hwDef0WDeps := buildTestProtoDeps("foo", "bar", swarmingLabels)

	hw1a := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef0WDeps},
	}
	hw1 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef0},
	}
	hw1L := []*helper{}

	for _, child := range hw1.HwDefinition {

		childHash, _ := hashstructure.Hash(child.DutInfo, hashstructure.FormatV2, nil)
		childHashProv, _ := hashstructure.Hash(child.Variant, hashstructure.FormatV2, nil)
		h := &helper{
			hashV:          childHash,
			hashProvV:      childHashProv,
			swarmingLabels: child.GetSwarmingLabels(),
		}
		hw1L = append(hw1L, h)
	}

	// Should match
	if !oldIsParent(makeHwInfo(hw1a), hw1L) {
		t.Fatalf("Parent with extra deps should be able to run child")
	}

	hw1ahelper := []*helper{}

	for _, child := range hw1a.HwDefinition {

		childHash, _ := hashstructure.Hash(child.DutInfo, hashstructure.FormatV2, nil)
		childHashProv, _ := hashstructure.Hash(child.Variant, hashstructure.FormatV2, nil)
		h := &helper{
			hashV:          childHash,
			hashProvV:      childHashProv,
			swarmingLabels: child.GetSwarmingLabels(),
		}
		hw1ahelper = append(hw1ahelper, h)
	}

	// // Should not match.
	if oldIsParent(makeHwInfo(hw1), hw1ahelper) {
		t.Fatalf("child with extra deps should not be able to use limited parent")
	}

	// Test different models will not match in either direction.
	hwDef02 := buildTestProto("foo", "bar2")
	hw2 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef02},
	}

	hw2helper := []*helper{}

	for _, child := range hw2.HwDefinition {

		childHash, _ := hashstructure.Hash(child.DutInfo, hashstructure.FormatV2, nil)
		childHashProv, _ := hashstructure.Hash(child.Variant, hashstructure.FormatV2, nil)
		h := &helper{
			hashV:          childHash,
			hashProvV:      childHashProv,
			swarmingLabels: child.GetSwarmingLabels(),
		}
		hw2helper = append(hw2helper, h)
	}
	if oldIsParent(makeHwInfo(hw1a), hw2helper) {
		t.Fatalf("Parent with different model should not be possible")
	}
	if oldIsParent(makeHwInfo(hw2), hw1ahelper) {
		t.Fatalf("child with different model should not be possible.")
	}

	// Test that when given hw1 || hw2, it can be run on hw1a
	hw1OrHw2 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{hwDef0, hwDef02},
	}
	hw1OrHw2L := []*helper{}

	for _, child := range hw1OrHw2.HwDefinition {

		childHash, _ := hashstructure.Hash(child.DutInfo, hashstructure.FormatV2, nil)
		childHashProv, _ := hashstructure.Hash(child.Variant, hashstructure.FormatV2, nil)
		h := &helper{
			hashV:          childHash,
			hashProvV:      childHashProv,
			swarmingLabels: child.GetSwarmingLabels(),
		}
		hw1OrHw2L = append(hw1OrHw2L, h)
	}

	if !oldIsParent(makeHwInfo(hw1a), hw1OrHw2L) {
		t.Fatalf("Case where every parent is in model should be true.")
	}
}

func schedulingUnitFromSwarmingDefs(primaryDef *api.SwarmingDefinition, companions []*api.SwarmingDefinition) *api.SchedulingUnit {
	def := &api.SchedulingUnit{
		PrimaryTarget: &api.Target{
			SwarmingDef: primaryDef,
		},
	}
	for _, c := range companions {
		def.CompanionTargets = append(def.CompanionTargets, &api.Target{SwarmingDef: c})
	}
	return def
}

func eqClassHelperList(units []*api.SchedulingUnit) []*helper {
	class := []*helper{}
	for _, unit := range units {
		h := &helper{
			hashV:          hashForSchedulingUnit(unit),
			swarmingLabels: unit.PrimaryTarget.SwarmingDef.GetSwarmingLabels(),
		}

		class = append(class, h)
	}

	return class
}

func TestIsParent(t *testing.T) {
	parent := &hwInfo{hwValue: uint64(0)}
	helperList := []*helper{}
	vars := buildTestVars()

	if !isParentofAtleastOne(parent, helperList) {
		t.Fatalf("Empty parent/child should be true")
	}

	EqClassListHw1WDeps := eqClassHelperList([]*api.SchedulingUnit{vars.HwDef001SUWDeps})
	EqClassListHw1 := eqClassHelperList([]*api.SchedulingUnit{vars.HwDef001SU})
	EqClassListHw2 := eqClassHelperList([]*api.SchedulingUnit{vars.HwDef002SU})
	EqClassListHw1_2 := eqClassHelperList([]*api.SchedulingUnit{vars.HwDef001SU, vars.HwDef002SU})

	// Identical Device check.
	if !isParentofAtleastOne(makeHwInfoNew(vars.HwDef001SUWDeps), EqClassListHw1WDeps) {
		t.Fatalf("HW1 should be able to run [HW1]")
	}

	// Device with extra labels should be able to run tests with less.
	if !isParentofAtleastOne(makeHwInfoNew(vars.HwDef001SUWDeps), EqClassListHw1) {
		t.Fatalf("HW1 w/ Deps should be able to run [HW1]")
	}

	// However, a parent MISSING labels should not be able to run the EqClass.
	if isParentofAtleastOne(makeHwInfoNew(vars.HwDef001SU), EqClassListHw1WDeps) {
		t.Fatalf("HW1 should NOT be able to run [HW1 w/ Deps]")
	}

	// Device with extra labels should be able to run tests with HW1 or 2 as options.
	if !isParentofAtleastOne(makeHwInfoNew(vars.HwDef001SUWDeps), EqClassListHw1_2) {
		t.Fatalf("HW1 w/ Labels should match [HW1 || HW2].")
	}

	// Device with extra labels should be able to run tests with HW1 or 2 as options.
	if isParentofAtleastOne(makeHwInfoNew(vars.HwDef001SUWDeps), EqClassListHw2) {
		t.Fatalf("HW1 w/ Labels should NOT match [HW2].")
	}

	EqClassListHw1WCompanion := eqClassHelperList([]*api.SchedulingUnit{vars.HwDef001SUWHW1Companion})

	// MultiDut should not ever match non-multdut.
	if isParentofAtleastOne(makeHwInfoNew(vars.HwDef001SUWHW1Companion), EqClassListHw1WDeps) {
		t.Fatalf("{Primary=HW1, Secondary=Hw1} should NOT match [HW1]")
	}

	// MultiDut should not ever match itself.
	if !isParentofAtleastOne(makeHwInfoNew(vars.HwDef001SUWHW1Companion), EqClassListHw1WCompanion) {
		t.Fatalf("{Primary=HW1, Secondary=Hw1} should NOT match [{Primary=HW1, Secondary=Hw1}]")
	}

	EqClassListHw1Variant := eqClassHelperList([]*api.SchedulingUnit{vars.HwDef001SUWVariant})

	// Identical Device check.
	if !isParentofAtleastOne(makeHwInfoNew(vars.HwDef001SUWVariant), EqClassListHw1Variant) {
		t.Fatalf("HW1.variant should be able to run [HW1.variant]")
	}

	// Variant check.
	if isParentofAtleastOne(makeHwInfoNew(vars.HwDef001SU), EqClassListHw1Variant) {

		t.Fatalf("HW1 should NOT be able to run [HW1.variant]")
	}
	// Variant check.
	if isParentofAtleastOne(makeHwInfoNew(vars.HwDef001SUWVariant), EqClassListHw1) {
		t.Fatalf("HW1.variant should NOT be able to run [HW1]")
	}
}

// TODO; remove this test when the proto migration is completed.
func TestOldFlattenList(t *testing.T) {
	SwarmingDef0 := buildTestProto("foo", "bar")
	SwarmingDef1 := buildTestProto("foo", "bar2")
	SwarmingDef2 := buildTestProto("foo2", "bar2")
	hwDef0 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{SwarmingDef0, SwarmingDef1},
	}

	hwDef1 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{SwarmingDef1, SwarmingDef2},
	}
	allHw := []*api.HWRequirements{hwDef0, hwDef1}

	expectedDef0 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{SwarmingDef0},
	}

	expectedDef1 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{SwarmingDef1},
	}
	expectedDef2 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{SwarmingDef2},
	}

	flatList := oldFlattenList(makeCtx(), allHw)
	for _, given := range []*api.HWRequirements{expectedDef0, expectedDef1, expectedDef2} {
		found := false
		for _, hw := range flatList {
			got, _ := hashstructure.Hash(hw.oldReq, hashstructure.FormatV2, nil)
			givenhash, _ := hashstructure.Hash(given, hashstructure.FormatV2, nil)

			if got == givenhash {
				found = true
			}
		}
		if !found {
			for k, v := range flatList {

				fmt.Println(k, v.oldReq)
			}
			t.Fatalf("Swarming def: %s not found in list", given)
		}
	}
	if len(flatList) != 3 {
		t.Fatalf("len prob")
	}
}

func TestFlattenList(t *testing.T) {
	vars := buildTestVars()

	allHw := []*api.SchedulingUnitOptions{vars.SU1Or2, vars.SU2Or3}

	flatList := flattenList(makeCtx(), allHw)
	for _, given := range []*api.SchedulingUnitOptions{vars.SU1, vars.SU2, vars.SU3} {
		found := false
		for _, hw := range flatList {
			got, _ := hashstructure.Hash(hw.req, hashstructure.FormatV2, nil)
			givenhash, _ := hashstructure.Hash(given, hashstructure.FormatV2, nil)
			if got == givenhash {
				found = true
			}
		}
		if !found {
			t.Fatalf("Swarming def: %s not found in list", given)
		}
	}
	if len(flatList) != 3 {
		t.Fatalf("len prob %v", len(flatList))
	}
}

// TODO; remove this test when the proto migration is completed.
func TestOldFlattenEqMap(t *testing.T) {
	SwarmingDef0 := buildTestProto("foo", "bar")
	SwarmingDef1 := buildTestProto("foo", "bar2")
	SwarmingDef2 := buildTestProto("foo2", "bar2")
	SwarmingDef3 := buildTestProto("foo3", "bar3")

	hwDef0 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{SwarmingDef0, SwarmingDef1},
	}
	hwDef1 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{SwarmingDef0, SwarmingDef1, SwarmingDef2},
	}
	hwDef2 := &api.HWRequirements{
		HwDefinition: []*api.SwarmingDefinition{SwarmingDef3},
	}

	hwEqMap := make(map[uint64][]uint64)
	hwUUIDMap := make(map[uint64]*api.HWRequirements)
	HwHash0 := uint64(0)
	HwHash1 := uint64(1)

	hwEqMap[HwHash0] = []uint64{HwHash0, HwHash1}
	hwEqMap[HwHash1] = []uint64{HwHash1}

	hwUUIDMap[HwHash0] = hwDef0
	hwUUIDMap[HwHash1] = hwDef1

	newEq, newUUID := oldFlattenEqMap(hwEqMap, hwUUIDMap)

	if len(newEq[HwHash0]) != 3 {
		t.Fatalf("Did not find the expected # of classes in the new Equiv map.")
	}
	for _, k := range newEq[HwHash0] {
		value, ok := newUUID[k]

		if !ok {
			t.Fatalf("class missing from lookup map")
		}
		found := false

		for _, given := range []*api.SwarmingDefinition{SwarmingDef0, SwarmingDef1, SwarmingDef2} {
			if reflect.DeepEqual(value.oldReq.HwDefinition[0], given) {
				found = true
			}
		}
		if !found {
			t.Fatalf("HW definition missing from flattened eq map")
		}
	}

	// Test a third class properly flattens into the first class, but not the second
	HwHash2 := uint64(2)
	hwUUIDMap[HwHash2] = hwDef2
	hwEqMap[HwHash0] = []uint64{HwHash0, HwHash1, HwHash2}
	hwEqMap[HwHash2] = []uint64{HwHash2}
	newEq, newUUID = oldFlattenEqMap(hwEqMap, hwUUIDMap)

	if len(newEq[HwHash0]) != 4 {
		t.Fatalf("Did not find the expected # of classes in the new Equiv map.")
	}

	if len(newEq[1]) != 3 {
		t.Fatalf("Did not find the expected # of classes in the new Equiv map.")
	}

	if len(newEq[HwHash2]) != 1 {
		t.Fatalf("Did not find the expected # of classes in the new Equiv map.")
	}

	for _, k := range newEq[HwHash0] {
		value, ok := newUUID[k]

		if !ok {
			t.Fatalf("class missing from lookup map")
		}
		found := false

		for _, given := range []*api.SwarmingDefinition{SwarmingDef0, SwarmingDef1, SwarmingDef2, SwarmingDef3} {
			if reflect.DeepEqual(value.oldReq.HwDefinition[0], given) {
				found = true
			}
		}
		if !found {
			t.Fatalf("HW definition missing from flattened eq map")
		}
	}

}

func TestFlattenEqMap(t *testing.T) {
	vars := buildTestVars()

	// Helper used to validate
	validate := func(newEq map[uint64][]uint64, newUUID map[uint64]*hwInfo, HwHash0 uint64, truth []*api.SwarmingDefinition) {
		for _, k := range newEq[HwHash0] {
			value, ok := newUUID[k]

			if !ok {
				t.Fatalf("class missing from lookup map")
			}
			found := false

			for _, given := range truth {
				if reflect.DeepEqual(value.req.SchedulingUnits[0].PrimaryTarget.SwarmingDef, given) {
					found = true
				}
			}
			if !found {
				t.Fatalf("HW definition missing from flattened eq map")
			}
		}
	}

	hwEqMap := make(map[uint64][]uint64)
	hwUUIDMap := make(map[uint64]*api.SchedulingUnitOptions)
	HwHash0 := uint64(0)
	HwHash1 := uint64(1)

	hwEqMap[HwHash0] = []uint64{HwHash0, HwHash1}
	hwEqMap[HwHash1] = []uint64{HwHash1}

	hwUUIDMap[HwHash0] = vars.SU1Or2
	hwUUIDMap[HwHash1] = vars.SU1Or2Or3

	newEq, newUUID := flattenEqMap(hwEqMap, vars.HwUUIDMap)

	// Because HwHash0 is == [HwHash && HwHash1]; and `HwHash1` is `SU1_or_2_or_3`; we should see 3 devices in the map.
	if len(newEq[HwHash0]) != 3 {
		t.Fatalf("Did not find the expected # of classes in the new Equiv map.")
	}

	validate(newEq, newUUID, HwHash0, []*api.SwarmingDefinition{vars.HwDef001, vars.HwDef002, vars.HwDef003})

	// Test a third class properly flattens into the first class, but not the second
	HwHash2 := uint64(2)

	hwUUIDMap[HwHash2] = vars.SU4
	hwEqMap[HwHash0] = []uint64{HwHash0, HwHash1, HwHash2}
	hwEqMap[HwHash2] = []uint64{HwHash2}
	newEq, newUUID = flattenEqMap(hwEqMap, hwUUIDMap)

	if len(newEq[HwHash0]) != 4 {
		t.Fatalf("Did not find the expected # of classes in the new Equiv map.")
	}

	if len(newEq[1]) != 3 {
		t.Fatalf("Did not find the expected # of classes in the new Equiv map.")
	}

	if len(newEq[HwHash2]) != 1 {
		t.Fatalf("Did not find the expected # of classes in the new Equiv map.")
	}
	validate(newEq, newUUID, HwHash0, []*api.SwarmingDefinition{vars.HwDef001, vars.HwDef002, vars.HwDef003, vars.HwDef004})

}

func TestGetDevices(t *testing.T) {
	vars := buildTestVars()

	newEq, flatUUIDLoadingMap := flattenEqMap(vars.HwEqMap, vars.HwUUIDMap)

	solverData := newMiddleOutData()
	solverData.cfg = distroCfg{
		unitTestDevices:                  2,
		maxInShard:                       2,
		requestsPerSchedulingUnitOptions: 1}
	solverData.flatHWUUIDMap = flatUUIDLoadingMap
	solverData.hwEquivalenceMap = newEq

	populateLabAvalability(makeCtx(), solverData)

	devices := solverData.hwEquivalenceMap[vars.HwHash1]
	for _, device := range devices {
		solverData.flatHWUUIDMap[device].shardHarness = "tast"
	}
	hwCache := map[uint64]bool{}
	modelCache := map[string]bool{}

	selectedDevice, expandCurrentShard := getDevices(solverData, 1, vars.HwHash1, "tast", hwCache, modelCache, false)

	hwCache = map[uint64]bool{}
	modelCache = map[string]bool{}

	flatUUIDLoadingMap[selectedDevice].numInCurrentShard = 1
	if expandCurrentShard {
		t.Fatalf("First test should go into new shard and did not")
	}
	selectedDevice2, _ := getDevices(solverData, 1, vars.HwHash1, "tast", hwCache, modelCache, false)

	if selectedDevice != selectedDevice2 {
		t.Fatalf("Shard was not filled when it should have been")
	}

	// Shard is full, so reset it and remove 1 from lab loading.
	flatUUIDLoadingMap[selectedDevice].labLoading.freeDevices--

	selectedDevice3, expandCurrentShard := getDevices(solverData, 1, vars.HwHash1, "tauto", hwCache, modelCache, false)

	if selectedDevice == selectedDevice3 || expandCurrentShard {
		// fmt.Print(selectedDevice3)
		t.Fatalf("Device which had tast tests was assigned a tauto.")
	}

	// Shard is full, so reset it and remove 1 from lab loading.
	flatUUIDLoadingMap[selectedDevice].numInCurrentShard = 0
	flatUUIDLoadingMap[selectedDevice].labLoading.freeDevices--

	selectedDevice4, expandCurrentShard := getDevices(solverData, 1, vars.HwHash1, "tast", hwCache, modelCache, false)
	if selectedDevice4 == selectedDevice2 {
		t.Fatalf("New shard should be on different device for balancing")
	}
	if expandCurrentShard {
		t.Fatalf("First test in shard should not mark to expand current.")
	}

}

func TestSorting(t *testing.T) {
	hwEqMap := make(map[uint64][]uint64)
	HwHash0 := uint64(0)
	HwHash1 := uint64(1)
	HwHash2 := uint64(2)
	HwHash3 := uint64(3)

	hwEqMap[HwHash0] = []uint64{HwHash0, HwHash1, HwHash2}
	hwEqMap[HwHash1] = []uint64{HwHash1, HwHash2}
	hwEqMap[HwHash2] = []uint64{HwHash2}
	hwEqMap[HwHash3] = []uint64{HwHash0, HwHash1, HwHash2, HwHash3}

	f := hwSearchOrdering(hwEqMap)

	expected := []uint64{HwHash2, HwHash1, HwHash0, HwHash3}
	for i, k := range f {
		if expected[i] != k.key {
			t.Fatalf("HW did not order least to most common. Expected: %v, Got: %v", expected, f)
		}
	}
}

func TestHarness(t *testing.T) {
	hv := getHarness("tauto.1.3.4.5.6.sdfs")
	if hv != "tauto" {
		t.Fatalf("incorrect harness found :%s expected: tauto", hv)
	}
	hv = getHarness("sdfs")
	if hv != "unknown" {
		t.Fatalf("incorrect harness found :%s expected: unknown", hv)
	}
}

func TestSharding(t *testing.T) {
	tests := []string{"tast.1", "tast.2", "tast.3", "tast.4", "tast.5", "tauto.1", "tauto.2", "tauto.3", "tauto.4", "tauto.5", "gtest.1", "gtest.2", "gtest.3", "gtest.4", "gtest.5"}
	maxShardLength := 3
	shards := shard(tests, maxShardLength)

	if len(shards) != 6 {
		t.Fatalf("expected 6 shard, got: %v", len(shards))
	}
	for _, shard := range shards {

		hName := ""
		if len(shard) > maxShardLength {
			t.Fatalf("shard length exceeded max specified, got: %v", len(shard))
		}
		if len(shard) < 2 {
			t.Fatalf("Should be atleast items in each shard, got: %v", len(shard))
		}
		for _, test := range shard {
			harnessFound := getHarness(test)
			if hName == "" {
				hName = harnessFound
			} else {
				if hName != harnessFound {
					t.Fatalf("mixed harnesses found in shard: %s", shard)
				}
			}

		}

	}
}

func TestAddBoardModelToDims(t *testing.T) {
	vars := buildTestVars()
	dims := []string{"one"}
	expected := []string{"one", "label-board:foo", "label-model:bar"}
	dims = append(dims, common.GetBoardModelDims(vars.HwDef001SU)...)
	if !reflect.DeepEqual(dims, expected) {
		t.Fatalf("mismatch. Expected %s got %s", dims, expected)
	}

	newDims := []string{"ready"}
	newDims = append(newDims, common.GetBoardModelDims(vars.HwDef001SUWHW1Companion)...)
	expected = []string{"ready", "label-board:foo", "label-model:bar", "label-board:foo_2", "label-model:bar_2"}
	if !reflect.DeepEqual(newDims, expected) {
		t.Fatalf("mismatch. Expected %s got %s", expected, newDims)
	}
}

func TestValidateIterationsCfg(t *testing.T) {
	type IterationsTest struct {
		name           string
		input          *api.Iterations
		expectedResult int
	}
	randomValidIt := rand.Intn(10) + 1
	itTests := []IterationsTest{
		{
			name:           "Empty Iteration",
			input:          nil,
			expectedResult: 1,
		},
		{
			name:           "Valid Range Iteration",
			input:          &api.Iterations{NumRetries: int64(randomValidIt)},
			expectedResult: randomValidIt,
		},
		{
			name:           "Min Edge Iteration",
			input:          &api.Iterations{NumRetries: int64(0)},
			expectedResult: 1,
		},
		{
			name:           "Max Edge Iteration",
			input:          &api.Iterations{NumRetries: int64(100)},
			expectedResult: 10,
		},
	}

	for _, test := range itTests {
		actual := validateIterationsCfg(makeCtx(), test.input)
		if actual != test.expectedResult {
			t.Fatalf("Test %s: EXPECTED - %d , ACTUAL -  %d", test.name, test.expectedResult, actual)
		}
	}
}

// TODO; once HWRequirements is deprecated; remove `target`
func getBuildTarget(target *api.HWRequirements, newtarget *api.SchedulingUnitOptions) string {
	if target != nil {
		return target.HwDefinition[0].GetDutInfo().GetChromeos().DutModel.BuildTarget
	} else {
		return getDutModel(getFirstSwarmingDefFromSchedulingUnitOptions(newtarget)).BuildTarget
	}
}

// TODO; once HWRequirements is deprecated; remove `target`
func getModelName(target *api.HWRequirements, newtarget *api.SchedulingUnitOptions) string {

	if target != nil {
		return target.HwDefinition[0].GetDutInfo().GetChromeos().DutModel.ModelName
	} else {
		return getDutModel(getFirstSwarmingDefFromSchedulingUnitOptions(newtarget)).ModelName
	}
}

func getFirstSwarmingDefFromSchedulingUnitOptions(t *api.SchedulingUnitOptions) *api.SwarmingDefinition {
	return t.GetSchedulingUnits()[0].GetPrimaryTarget().GetSwarmingDef()
}

func getDutModel(t *api.SwarmingDefinition) *dut_api.DutModel {
	return t.GetDutInfo().GetChromeos().GetDutModel()
}

func validateDistro(finalAssignments map[uint64][][]string, flatUUIDLoadingMap map[uint64]*hwInfo, cfg distroCfg, expectedTcs []string, expected []*allowedAssignment) (bool, string) {
	hwCount := make(map[uint64]int)
	foundTcs := []string{}

	for hw, tc := range finalAssignments {

		flatTcs := []string{}
		for _, innerTcs := range tc {
			hwCount[hw]++
			if len(innerTcs) > cfg.maxInShard {
				return false, "Shard size exceeded"
			}
			flatTcs = append(flatTcs, innerTcs...)
		}
		foundTcs = append(foundTcs, flatTcs...)
		if !validateCorrectHwAssignment(flatUUIDLoadingMap[hw], flatTcs, expected) {
			return false, fmt.Sprintf("TC: %s Had wrong hw assignment", flatTcs)
		}
	}

	for hw, hwcount := range hwCount {
		if flatUUIDLoadingMap[hw].labLoading.freeDevices+hwcount != cfg.unitTestDevices {
			return false, fmt.Sprintf("Lab loading incorrect: %v", flatUUIDLoadingMap[hw].labLoading)
		}
	}
	if !listEqual(expectedTcs, foundTcs) {
		return false, fmt.Sprintf("Some Test(s) missing from assignment: got: %v expected: %v", foundTcs, expectedTcs)
	}
	return true, ""
}

func listEqual(expected []string, found []string) bool {
	expectedCounts := make(map[string]int)
	foundCounts := make(map[string]int)

	for _, tc := range expected {
		expectedCounts[tc]++
	}

	for _, tc := range found {
		foundCounts[tc]++
	}

	return reflect.DeepEqual(expectedCounts, foundCounts)
}

func validateCorrectHwAssignment(givenHw *hwInfo, flatTcs []string, expected []*allowedAssignment) bool {
	found := []string{}
	for _, e := range expected {
		// If we find the givenHW in the expected HW list,
		// Look for Tc matches

		if isMatch(givenHw, e) {
			for _, tc := range flatTcs {
				for _, test := range e.tc {
					if tc == test {
						found = append(found, tc)
					}
				}
			}
		}
	}
	return reflect.DeepEqual(found, flatTcs)
}

func isMatch(a *hwInfo, b *allowedAssignment) bool {
	if getBuildTarget(a.oldReq, a.req) != getBuildTarget(b.hw, b.nwHw) {
		return false
	}
	if getModelName(a.oldReq, a.req) != getModelName(b.hw, b.nwHw) {
		return false
	}
	return true
}

type allowedAssignment struct {
	tc   []string
	hw   *api.HWRequirements
	nwHw *api.SchedulingUnitOptions
}

//// Intentionally commented out. This is a helpful method for debugging e2e flow.
//func parseTestPlan(source string) *api.InternalTestplan {
//	unmarshalOptions := protojson.UnmarshalOptions{
//		AllowPartial:   false,
//		DiscardUnknown: false,
//	}
//	testPlan := api.InternalTestplan{}
//	err := unmarshalOptions.Unmarshal([]byte(source), &testPlan)
//	if err != nil {
//		log.Fatal(err)
//	}
//	return &testPlan
//}

//// Intentionally commented out. This is a helpful method for debugging e2e flow.
//func TestExecute(t *testing.T) {
//	data, err := ioutil.ReadFile("test.json")
//	if err != nil {
//		fmt.Println("Error reading file:", err)
//		t.Fatal("err", err)
//	}
//
//	jsonString := string(data)
//	testPlan := parseTestPlan(jsonString)
//
//	cfg := distroCfg{
//		maxInShard:      150,
//		pool:            "wificell",
//		isUnitTest:      true,
//		unitTestDevices: 1,
//		requestsPerSchedulingUnitOptions:   2,
//	}
//
//	out, err := middleOut(makeCtx(), testPlan, cfg)
//	println(out)
//	t.Fatal("o")
//}
