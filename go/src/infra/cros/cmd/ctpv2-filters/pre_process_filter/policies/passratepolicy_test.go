// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policies

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"

	"infra/cros/cmd/ctpv2-filters/pre_process_filter/structs"
)

func TestDetermineSignalFromQuery(t *testing.T) {
	tcList := make(map[string]struct{})
	tcList["Test1"] = struct{}{}
	tcList["Test2"] = struct{}{}
	tcList["Test3"] = struct{}{}
	tcList["Test4"] = struct{}{}
	tcList["Test6"] = struct{}{}

	log := log.New(os.Stdout, "test", 1)
	qi := PassRatePolicy{requiredPassRate: 99.0, requiredRecentPassRate: 99.5, requiredRecentRuns: 20, missingTcList: tcList, log: log}

	qi.forceMapEnable = listToMap([]string{"Test1"})
	qi.forceMapDisable = listToMap([]string{"Test2"})

	signal, _ := qi.determineSignalFromQuery("Test1", 98.0, 98.0, 30, 20, 5)
	if !signal {
		t.Fatalf("ForceEnableCheck - Test marked as unstable when should be marked as stable.")
	}

	signal, _ = qi.determineSignalFromQuery("Test2", 99.5, 99.0, 30, 20, 1)
	if signal {
		t.Fatalf("ForceDisableCheck - Test marked as stable when should be marked as unstable.")
	}

	signal, _ = qi.determineSignalFromQuery("Test3", 98.0, 99.9, 2, 5, 1)
	if signal {
		t.Fatalf("PassRateCheckUnstable - Test marked as stable when should be marked as unstable.")
	}

	signal, _ = qi.determineSignalFromQuery("Test4", 99.0, 97.0, 30, 30, 1)
	if !signal {
		t.Fatalf("PassRateCheckStable - Test marked as unstable when should be marked as stable.")
	}

	signal, _ = qi.determineSignalFromQuery("Test4", 94.0, 99.5, 30, 30, 1)
	if !signal {
		t.Fatalf("RecentStableCheck - Test marked as unstable when should be marked as stable.")
	}

	// Test max # of filtered reached cases:
	// 1.) Stable Signal, no filters remaining, with a force override to stable.
	signal, _ = qi.determineSignalFromQuery("Test1", 98.0, 98.0, 30, 20, 0)
	if !signal {
		t.Fatalf("ForceEnableCheck - Test marked as unstable when should be marked as stable.")
	}

	// 2.) unstable signal, no filters remaining, should be "stable".
	signal, _ = qi.determineSignalFromQuery("Test3", 99.5, 99.0, 30, 20, 0)
	if !signal {
		t.Fatalf("MaxFilterCheck - Test marked as unstable when should be marked as stable.")
	}

	// 3.) unstable signal, no filters remaining, forcedDisabled, should be "unstable".
	signal, _ = qi.determineSignalFromQuery("Test2", 99.5, 99.0, 30, 20, 0)
	if signal {
		t.Fatalf("MaxFilterForceDisableCheck - Test marked as stable when should be marked as unstable.")
	}

	qi = PassRatePolicy{requiredPassRate: 80.0, requiredRecentPassRate: 90, requiredRecentRuns: 20, missingTcList: tcList, log: log}
	// Test that when the test is < requiredPassRate; and the requiredPassRate > RECENT PASS RATE < requiredRecentPassRate
	// The test is still not enabled (this is added specifically due to a found bug.)
	signal, _ = qi.determineSignalFromQuery("Test2", 70.0, 85.0, 30, 20, 5)
	if signal {
		t.Fatalf("RecentEnableCheck - Test marked as stable when should be marked as unstable.")
	}
	qi = PassRatePolicy{requiredPassRate: 80.0, requiredRecentPassRate: 90, requiredRecentRuns: 20, missingTcList: tcList, log: log}
	// A general recent check
	signal, _ = qi.determineSignalFromQuery("Test2", 70.0, 95.0, 30, 20, 5)
	if !signal {
		t.Fatalf("RecentEnableCheck - Test marked as unstable when should be marked as stable.")
	}
	// A general recent check
	_, filterRemaining := qi.determineSignalFromQuery("Test6", 0.0, 0.0, 30, 20, 5)
	if filterRemaining != 4 {
		fmt.Println(filterRemaining)
		t.Fatalf("filterRemainingCheck - Test in list being filtered should decrement number of remaining filters.")
	}
	_, filterRemaining = qi.determineSignalFromQuery("Test5", 0.0, 0.0, 30, 20, 5)
	if filterRemaining != 5 {
		t.Fatalf("filterRemainingCheck - Test not in requested suite should not count towards filter n.")
	}

}

func TestPopulateMissingTests(t *testing.T) {
	log := log.New(os.Stdout, "test", 1)

	qi := PassRatePolicy{
		data:          make(map[string]structs.SignalFormat),
		otherData:     make(map[string]structs.SignalFormat),
		missingTcList: listToMap([]string{"missing1", "missing2"}),
		log:           log}

	sigData := structs.SignalFormat{Signal: true}

	qi.otherData["missing2"] = sigData
	qi.populateMissingTests()
	data, ok := qi.data["missing2"]
	if !ok {
		t.Fatalf("MissingDataCheck - data from missing not backfiled into primary data")
	}
	if data != sigData {
		t.Fatalf("MissingDataCheck - backfiled missing data incorret")

	}

	_, ok = qi.data["missing"]
	if ok {
		t.Fatalf("MissingDataCheck - Missing data got backfilled (how??)")
	}
}

func TestPolicyFilterTestConfigsToMap(t *testing.T) {
	enabledTest := &api.FilterTestConfig{
		Test:    "test1",
		Board:   []string{"hana"},
		Setting: api.FilterTestConfig_ENABLED,
	}
	disabledTest := &api.FilterTestConfig{
		Test:    "test2",
		Board:   []string{"hana"},
		Setting: api.FilterTestConfig_DISABLED,
	}

	enabled, disabled := policyFilterTestConfigsToMap([]*api.FilterTestConfig{enabledTest, disabledTest}, "hana")
	enabledExpected := listToMap([]string{"test1"})
	disabledExpected := listToMap([]string{"test2"})

	eq := reflect.DeepEqual(enabled, enabledExpected)
	if !eq {
		t.Fatalf("got: %s did not match expected: %s", enabled, enabledExpected)
	}
	eq = reflect.DeepEqual(disabled, disabledExpected)
	if !eq {
		t.Fatalf("got: %s did not match expected: %s", disabled, disabledExpected)
	}
}

func TestPolicyFilterTestConfigsToMapNoBoardMatch(t *testing.T) {
	enabledTest := &api.FilterTestConfig{
		Test:    "test1",
		Board:   []string{"hana"},
		Setting: api.FilterTestConfig_ENABLED,
	}
	disabledTest := &api.FilterTestConfig{
		Test:    "test2",
		Board:   []string{"hana"},
		Setting: api.FilterTestConfig_DISABLED,
	}

	enabled, disabled := policyFilterTestConfigsToMap([]*api.FilterTestConfig{enabledTest, disabledTest}, "other")
	expected := make(map[string]struct{})

	eq := reflect.DeepEqual(enabled, expected)
	if !eq {
		t.Fatalf("got: %s did not match expected: %s", enabled, expected)
	}
	eq = reflect.DeepEqual(disabled, expected)
	if !eq {
		t.Fatalf("got: %s did not match expected: %s", disabled, expected)
	}
}

func TestPolicyFilterTestConfigsAllBoard(t *testing.T) {
	enabledTest := &api.FilterTestConfig{
		Test:    "test1",
		Board:   []string{"*"},
		Setting: api.FilterTestConfig_ENABLED,
	}
	disabledTest := &api.FilterTestConfig{
		Test:    "test2",
		Board:   []string{"*"},
		Setting: api.FilterTestConfig_DISABLED,
	}

	enabled, disabled := policyFilterTestConfigsToMap([]*api.FilterTestConfig{enabledTest, disabledTest}, "other")
	enabledExpected := listToMap([]string{"test1"})
	disabledExpected := listToMap([]string{"test2"})
	eq := reflect.DeepEqual(enabled, enabledExpected)
	if !eq {
		t.Fatalf("got: %s did not match expected: %s", enabled, enabledExpected)
	}
	eq = reflect.DeepEqual(disabled, disabledExpected)
	if !eq {
		t.Fatalf("got: %s did not match expected: %s", disabled, disabledExpected)
	}
}

func TestMaxFiltered(t *testing.T) {
	r := &api.PassRatePolicy{MaxFilteredPercent: 20}
	mf := maxAllowedToBeFiltered(r, 5)
	if mf != 1 {
		t.Fatalf("MaxFiltered Should be 1 when 20 percent of 5 is set, got %v", mf)
	}
}
