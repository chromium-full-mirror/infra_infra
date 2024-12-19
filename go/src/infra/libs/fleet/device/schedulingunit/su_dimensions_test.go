// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package schedulingunit

import (
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/libs/skylab/inventory"
	"infra/libs/skylab/inventory/swarming"
	ufspb "infra/unifiedfleet/api/v1/models"
)

func TestJoinSingleValueLabel(t *testing.T) {
	ftt.Run("Test with no repeat labels", t, func(t *ftt.Test) {
		l := []string{"eve", "nami", "coral"}
		assert.Loosely(t, differentiateLabelValues(l), should.Match([]string{"eve", "nami", "coral"}))
	})

	ftt.Run("Test with repeat labels", t, func(t *ftt.Test) {
		l := []string{"nami", "coral", "nami", "nami"}
		assert.Loosely(t, differentiateLabelValues(l), should.Match([]string{"nami", "coral", "nami_2", "nami_3"}))
	})
}

func TestDutLabelValues(t *testing.T) {
	ftt.Run("Test get DUT's label values.", t, func(t *ftt.Test) {
		dims := []swarming.Dimensions{
			{
				"dut_name":    {"host1"},
				"label-board": {"coral"},
				"label-model": {"babytiger"},
				"dut_state":   {"ready"},
			},
			{
				"dut_name":    {"host2"},
				"label-board": {"nami"},
				"label-model": {"bard"},
				"dut_state":   {"repair_failed"},
			},
			{
				"dut_name":    {"host3"},
				"label-board": {"eve"},
				"label-model": {"eve"},
				"dut_state":   {"ready"},
			},
		}
		assert.Loosely(t, dutLabelValues("dut_name", dims), should.Match([]string{"host1", "host2", "host3"}))
		assert.Loosely(t, dutLabelValues("label-board", dims), should.Match([]string{"coral", "nami", "eve"}))
		assert.Loosely(t, dutLabelValues("label-model", dims), should.Match([]string{"babytiger", "bard", "eve"}))
		assert.Loosely(t, dutLabelValues("dut_state", dims), should.Match([]string{"ready", "repair_failed", "ready"}))
		assert.Loosely(t, dutLabelValues("IM_NOT_EXIST", dims), should.Match([]string(nil)))
	})
}

func TestLabelIntersection(t *testing.T) {
	ftt.Run("Test find intersection from a given label name.", t, func(t *ftt.Test) {
		dims := []swarming.Dimensions{
			{
				"label-device-stable": {"True"},
				"label-foo":           {"common_value1", "common_value2", "common_value3", "special_value1"},
				"label-foo2":          {"value"},
			},
			{
				"label-device-stable": {"True"},
				"label-foo":           {"common_value1", "common_value2", "common_value3", "special_value2"},
				"label-foo2":          {"value"},
			},
			{
				"label-device-stable": {"True"},
				"label-foo":           {"common_value1", "common_value2", "common_value3", "special_value3"},
			},
		}
		assert.Loosely(t, labelIntersection("label-device-stable", dims), should.Match([]string{"True"}))
		assert.Loosely(t, labelIntersection("label-foo", dims), should.Match([]string{"common_value1", "common_value2", "common_value3"}))
		assert.Loosely(t, labelIntersection("label-foo2", dims), should.Match([]string(nil)))
	})
}

func TestGetSchedulingUnitDimensions(t *testing.T) {
	ftt.Run("Test with a non-empty scheduling unit with all devices are stable.", t, func(t *ftt.Test) {
		su := &ufspb.SchedulingUnit{
			Name:       "schedulingunit/test-unit1",
			Pools:      []string{"nearby_sharing"},
			ExposeType: ufspb.SchedulingUnit_DEFAULT,
		}
		dims := []swarming.Dimensions{
			{
				"dut_name":                       {"host1"},
				"label-board":                    {"coral"},
				"label-model":                    {"babytiger"},
				"dut_state":                      {"ready"},
				"random-label1":                  {"123"},
				"label-device-stable":            {"True"},
				"label-peripheral_wifi_state":    {"WORKING"},
				"label-peripheral_btpeer_state":  {"WORKING"},
				"label-working_bluetooth_btpeer": {"1", "2", "3", "4"},
				"label-wifi_router_features": {
					"WIFI_ROUTER_FEATURE_IEEE_802_11_A",
					"WIFI_ROUTER_FEATURE_IEEE_802_11_B",
					"WIFI_ROUTER_FEATURE_IEEE_802_11_G",
					"WIFI_ROUTER_FEATURE_IEEE_802_11_N",
					"999",
				},
				"label-wifi_router_models": {
					"gale",
					"OPENWRT[Ubiquiti_Unifi_6_Lite]",
				},
			},
			{
				"dut_name":            {"host2"},
				"label-board":         {"nami"},
				"label-model":         {"bard"},
				"dut_state":           {"repair_failed"},
				"random-label2":       {"abc"},
				"label-device-stable": {"True"},
			},
			{
				"dut_name":            {"host3"},
				"label-board":         {"eve"},
				"label-model":         {"eve"},
				"dut_state":           {"ready"},
				"random-label2":       {"!@#"},
				"label-device-stable": {"True"},
			},
		}
		expectedResult := map[string][]string{
			"dut_name":                       {"test-unit1"},
			"dut_id":                         {"test-unit1"},
			"label-pool":                     {"nearby_sharing"},
			"label-dut_count":                {"3"},
			"label-multiduts":                {"True"},
			"label-managed_dut":              {"host1", "host2", "host3"},
			"dut_state":                      {"repair_failed"},
			"label-board":                    {"coral", "nami", "eve"},
			"label-model":                    {"babytiger", "bard", "eve"},
			"label-device-stable":            {"True"},
			"label-peripheral_wifi_state":    {"WORKING"},
			"label-peripheral_btpeer_state":  {"WORKING"},
			"label-working_bluetooth_btpeer": {"1", "2", "3", "4"},
			"label-wifi_router_features": {
				"WIFI_ROUTER_FEATURE_IEEE_802_11_A",
				"WIFI_ROUTER_FEATURE_IEEE_802_11_B",
				"WIFI_ROUTER_FEATURE_IEEE_802_11_G",
				"WIFI_ROUTER_FEATURE_IEEE_802_11_N",
				"999",
			},
			"label-wifi_router_models": {
				"gale",
				"OPENWRT[Ubiquiti_Unifi_6_Lite]",
			},
		}
		assert.Loosely(t, GetSchedulingUnitDimensions(su, dims), should.Match(expectedResult))
	})

	ftt.Run("Test with an empty scheduling unit.", t, func(t *ftt.Test) {
		su := &ufspb.SchedulingUnit{
			Name:       "schedulingunit/test-unit1",
			Pools:      []string{"nearby_sharing"},
			ExposeType: ufspb.SchedulingUnit_DEFAULT,
		}
		var dims []swarming.Dimensions
		expectedResult := map[string][]string{
			"dut_name":                      {"test-unit1"},
			"dut_id":                        {"test-unit1"},
			"label-pool":                    {"nearby_sharing"},
			"label-dut_count":               {"0"},
			"label-multiduts":               {"True"},
			"dut_state":                     {"unknown"},
			"label-peripheral_btpeer_state": {"NOT_APPLICABLE"},
			"label-peripheral_wifi_state":   {"NOT_APPLICABLE"},
		}
		assert.Loosely(t, GetSchedulingUnitDimensions(su, dims), should.Match(expectedResult))
	})

	ftt.Run("Test with an scheduling unit that include non-stable device.", t, func(t *ftt.Test) {
		su := &ufspb.SchedulingUnit{
			Name:       "schedulingunit/test-unit1",
			Pools:      []string{"nearby_sharing"},
			ExposeType: ufspb.SchedulingUnit_DEFAULT,
		}
		dims := []swarming.Dimensions{
			{
				"dut_name":            {"host1"},
				"label-board":         {"coral"},
				"label-model":         {"babytiger"},
				"dut_state":           {"ready"},
				"random-label1":       {"123"},
				"label-device-stable": {"True"},
			},
			{
				"dut_name":            {"host2"},
				"label-board":         {"nami"},
				"label-model":         {"bard"},
				"dut_state":           {"repair_failed"},
				"random-label2":       {"abc"},
				"label-device-stable": {"True"},
			},
			{
				"dut_name":      {"host3"},
				"label-board":   {"eve"},
				"label-model":   {"eve"},
				"dut_state":     {"ready"},
				"random-label2": {"!@#"},
			},
		}
		expectedResult := map[string][]string{
			"dut_name":                      {"test-unit1"},
			"dut_id":                        {"test-unit1"},
			"label-pool":                    {"nearby_sharing"},
			"label-dut_count":               {"3"},
			"label-multiduts":               {"True"},
			"label-managed_dut":             {"host1", "host2", "host3"},
			"dut_state":                     {"repair_failed"},
			"label-board":                   {"coral", "nami", "eve"},
			"label-model":                   {"babytiger", "bard", "eve"},
			"label-peripheral_btpeer_state": {"NOT_APPLICABLE"},
			"label-peripheral_wifi_state":   {"NOT_APPLICABLE"},
		}
		assert.Loosely(t, GetSchedulingUnitDimensions(su, dims), should.Match(expectedResult))
	})
	ftt.Run("Test with a strict primary dut dimensions", t, func(t *ftt.Test) {
		su := &ufspb.SchedulingUnit{
			Name:       "schedulingunit/test-unit1",
			Pools:      []string{"nearby_sharing"},
			PrimaryDut: "host1",
			ExposeType: ufspb.SchedulingUnit_STRICTLY_PRIMARY_ONLY,
		}
		dims := []swarming.Dimensions{
			{
				"dut_name":            {"host1"},
				"label-board":         {"coral"},
				"label-model":         {"babytiger"},
				"dut_state":           {"ready"},
				"random-label1":       {"123"},
				"label-device-stable": {"True"},
			},
			{
				"dut_name":            {"host2"},
				"label-board":         {"nami"},
				"label-model":         {"bard"},
				"dut_state":           {"repair_failed"},
				"random-label2":       {"abc"},
				"label-device-stable": {"True"},
			},
			{
				"dut_name":            {"host3"},
				"label-board":         {"eve"},
				"label-model":         {"eve"},
				"dut_state":           {"ready"},
				"random-label2":       {"!@#"},
				"label-device-stable": {"True"},
			},
		}
		expectedResult := map[string][]string{
			"dut_name":                      {"test-unit1"},
			"dut_id":                        {"test-unit1"},
			"label-pool":                    {"nearby_sharing"},
			"label-dut_count":               {"3"},
			"label-multiduts":               {"True"},
			"label-managed_dut":             {"host1", "host2", "host3"},
			"dut_state":                     {"repair_failed"},
			"label-board":                   {"coral"},
			"label-model":                   {"babytiger"},
			"label-device-stable":           {"True"},
			"random-label1":                 {"123"},
			"label-primary_dut":             {"host1"},
			"label-peripheral_btpeer_state": {"NOT_APPLICABLE"},
			"label-peripheral_wifi_state":   {"NOT_APPLICABLE"},
		}
		assert.Loosely(t, GetSchedulingUnitDimensions(su, dims), should.Match(expectedResult))
	})
	ftt.Run("Test with a primary dut default dimensions", t, func(t *ftt.Test) {
		su := &ufspb.SchedulingUnit{
			Name:       "schedulingunit/test-unit1",
			Pools:      []string{"nearby_sharing"},
			PrimaryDut: "host1",
			ExposeType: ufspb.SchedulingUnit_DEFAULT,
		}
		dims := []swarming.Dimensions{
			{
				"dut_name":            {"host1"},
				"label-board":         {"coral"},
				"label-model":         {"babytiger"},
				"dut_state":           {"ready"},
				"random-label1":       {"123"},
				"label-device-stable": {"True"},
			},
			{
				"dut_name":            {"host2"},
				"label-board":         {"nami"},
				"label-model":         {"bard"},
				"dut_state":           {"repair_failed"},
				"random-label2":       {"abc"},
				"label-device-stable": {"True"},
			},
			{
				"dut_name":            {"host3"},
				"label-board":         {"eve"},
				"label-model":         {"eve"},
				"dut_state":           {"ready"},
				"random-label2":       {"!@#"},
				"label-device-stable": {"True"},
			},
		}
		expectedResult := map[string][]string{
			"dut_name":                      {"test-unit1"},
			"dut_id":                        {"test-unit1"},
			"label-pool":                    {"nearby_sharing"},
			"label-dut_count":               {"3"},
			"label-multiduts":               {"True"},
			"label-managed_dut":             {"host1", "host2", "host3"},
			"dut_state":                     {"repair_failed"},
			"label-board":                   {"coral", "nami", "eve"},
			"label-model":                   {"babytiger", "bard", "eve"},
			"label-device-stable":           {"True"},
			"label-primary_dut":             {"host1"},
			"label-peripheral_btpeer_state": {"NOT_APPLICABLE"},
			"label-peripheral_wifi_state":   {"NOT_APPLICABLE"},
		}
		assert.Loosely(t, GetSchedulingUnitDimensions(su, dims), should.Match(expectedResult))
	})
	ftt.Run("Test with a default_plus_primary dimensions", t, func(t *ftt.Test) {
		su := &ufspb.SchedulingUnit{
			Name:       "schedulingunit/test-unit1",
			Pools:      []string{"nearby_sharing"},
			PrimaryDut: "host1",
			ExposeType: ufspb.SchedulingUnit_DEFAULT_PLUS_PRIMARY,
		}
		dims := []swarming.Dimensions{
			{
				"dut_name":            {"host1"},
				"label-board":         {"coral"},
				"label-model":         {"babytiger"},
				"dut_state":           {"ready"},
				"random-label1":       {"123"},
				"label-device-stable": {"True"},
			},
			{
				"dut_name":            {"host2"},
				"label-board":         {"nami"},
				"label-model":         {"bard"},
				"dut_state":           {"repair_failed"},
				"random-label2":       {"abc"},
				"label-device-stable": {"True"},
			},
			{
				"dut_name":            {"host3"},
				"label-board":         {"eve"},
				"label-model":         {"eve"},
				"dut_state":           {"ready"},
				"random-label2":       {"!@#"},
				"label-device-stable": {"True"},
			},
		}
		expectedResult := map[string][]string{
			"dut_name":                      {"test-unit1"},
			"dut_id":                        {"test-unit1"},
			"label-pool":                    {"nearby_sharing"},
			"label-dut_count":               {"3"},
			"label-multiduts":               {"True"},
			"label-managed_dut":             {"host1", "host2", "host3"},
			"dut_state":                     {"repair_failed"},
			"label-board":                   {"coral", "nami", "eve"},
			"label-model":                   {"babytiger", "bard", "eve"},
			"label-device-stable":           {"True"},
			"random-label1":                 {"123"},
			"label-primary_dut":             {"host1"},
			"label-peripheral_btpeer_state": {"NOT_APPLICABLE"},
			"label-peripheral_wifi_state":   {"NOT_APPLICABLE"},
		}
		assert.Loosely(t, GetSchedulingUnitDimensions(su, dims), should.Match(expectedResult))
	})
	ftt.Run("Test schedulingunit with wificell label.", t, func(t *ftt.Test) {
		su := &ufspb.SchedulingUnit{
			Name:       "schedulingunit/test-unit1",
			Pools:      []string{"nearby_sharing"},
			ExposeType: ufspb.SchedulingUnit_DEFAULT,
			Wificell:   true,
		}
		var dims []swarming.Dimensions
		expectedResult := map[string][]string{
			"dut_name":                      {"test-unit1"},
			"dut_id":                        {"test-unit1"},
			"label-pool":                    {"nearby_sharing"},
			"label-dut_count":               {"0"},
			"label-multiduts":               {"True"},
			"label-wificell":                {"True"},
			"dut_state":                     {"unknown"},
			"label-peripheral_btpeer_state": {"NOT_APPLICABLE"},
			"label-peripheral_wifi_state":   {"NOT_APPLICABLE"},
		}
		assert.Loosely(t, GetSchedulingUnitDimensions(su, dims), should.Match(expectedResult))
	})
	ftt.Run("Test schedulingunit with carrier label.", t, func(t *ftt.Test) {
		su := &ufspb.SchedulingUnit{
			Name:       "schedulingunit/test-unit1",
			Pools:      []string{"nearby_sharing"},
			ExposeType: ufspb.SchedulingUnit_DEFAULT,
			Wificell:   true,
			Carrier:    "TEST_CARRIER",
		}
		var dims []swarming.Dimensions
		expectedResult := map[string][]string{
			"dut_name":                      {"test-unit1"},
			"dut_id":                        {"test-unit1"},
			"label-pool":                    {"nearby_sharing"},
			"label-dut_count":               {"0"},
			"label-multiduts":               {"True"},
			"label-wificell":                {"True"},
			"label-carrier":                 {"TEST_CARRIER"},
			"dut_state":                     {"unknown"},
			"label-peripheral_btpeer_state": {"NOT_APPLICABLE"},
			"label-peripheral_wifi_state":   {"NOT_APPLICABLE"},
		}
		assert.Loosely(t, GetSchedulingUnitDimensions(su, dims), should.Match(expectedResult))
	})
}

func TestGetSchedulingUnitBotState(t *testing.T) {
	ftt.Run("Test scheduling unit bot state.", t, func(t *ftt.Test) {
		tm, _ := time.Parse(time.RFC3339, "2021-05-07T11:54:36.225Z")
		su := &ufspb.SchedulingUnit{
			Name:       "schedulingunit/test-unit1",
			UpdateTime: timestamppb.New(tm),
		}
		expectedResult := map[string][]string{
			"scheduling_unit_version_index": {"2021-05-07 11:54:36.225 UTC"},
		}
		assert.Loosely(t, GetSchedulingUnitBotState(su), should.Match(expectedResult))
	})
}

func TestCollectPeripheralDimensions(t *testing.T) {
	tests := []struct {
		name        string
		dutsDimsArg []swarming.Dimensions
		want        swarming.Dimensions
	}{
		{
			"no dut dims",
			nil,
			swarming.Dimensions{
				"label-peripheral_wifi_state":   {"NOT_APPLICABLE"},
				"label-peripheral_btpeer_state": {"NOT_APPLICABLE"},
			},
		},
		{
			"single dut with all peripheral dims",
			[]swarming.Dimensions{
				{
					"label-peripheral_wifi_state":    {"WORKING"},
					"label-peripheral_btpeer_state":  {"WORKING"},
					"label-working_bluetooth_btpeer": {"1", "2", "3", "4"},
					"label-wifi_router_features": {
						"WIFI_ROUTER_FEATURE_IEEE_802_11_A",
						"WIFI_ROUTER_FEATURE_IEEE_802_11_B",
						"WIFI_ROUTER_FEATURE_IEEE_802_11_G",
						"WIFI_ROUTER_FEATURE_IEEE_802_11_N",
						"999",
					},
					"label-wifi_router_models": {
						"gale",
						"OPENWRT[Ubiquiti_Unifi_6_Lite]",
					},
				},
			},
			swarming.Dimensions{
				"label-peripheral_wifi_state":    {"WORKING"},
				"label-peripheral_btpeer_state":  {"WORKING"},
				"label-working_bluetooth_btpeer": {"1", "2", "3", "4"},
				"label-wifi_router_features": {
					"WIFI_ROUTER_FEATURE_IEEE_802_11_A",
					"WIFI_ROUTER_FEATURE_IEEE_802_11_B",
					"WIFI_ROUTER_FEATURE_IEEE_802_11_G",
					"WIFI_ROUTER_FEATURE_IEEE_802_11_N",
					"999",
				},
				"label-wifi_router_models": {
					"gale",
					"OPENWRT[Ubiquiti_Unifi_6_Lite]",
				},
			},
		},
		{
			"Multiple dimensions with working states",
			[]swarming.Dimensions{
				{
					"label-peripheral_wifi_state":   {"WORKING"},
					"label-peripheral_btpeer_state": {"WORKING"},
				},
				{
					"label-peripheral_wifi_state":   {"WORKING"},
					"label-peripheral_btpeer_state": {"WORKING"},
				},
				{
					"label-peripheral_wifi_state":   {"WORKING"},
					"label-peripheral_btpeer_state": {"WORKING"},
				},
			},
			swarming.Dimensions{
				"label-peripheral_wifi_state":   {"WORKING"},
				"label-peripheral_btpeer_state": {"WORKING"},
			},
		},
		{
			"Multiple dimensions with mixed states",
			[]swarming.Dimensions{
				{
					"label-peripheral_wifi_state":   {"WORKING"},
					"label-peripheral_btpeer_state": {"WORKING"},
				},
				{
					"label-peripheral_wifi_state":   {"NOT_APPLICABLE"},
					"label-peripheral_btpeer_state": {"BROKEN"},
				},
				{
					"label-peripheral_wifi_state":   {"WORKING"},
					"label-peripheral_btpeer_state": {"WORKING"},
				},
			},
			swarming.Dimensions{
				"label-peripheral_wifi_state":   {"WORKING"},
				"label-peripheral_btpeer_state": {"BROKEN"},
			},
		},
		{
			"Multiple dimensions with btpeers should collect all",
			[]swarming.Dimensions{
				{
					"label-working_bluetooth_btpeer": {"1"},
				},
				{
					"label-working_bluetooth_btpeer": {"1", "2", "3"},
				},
				{
					"label-working_bluetooth_btpeer": {"1", "2", "3", "4"},
				},
				{}, // Empty dim.
			},
			swarming.Dimensions{
				"label-peripheral_wifi_state":    {"NOT_APPLICABLE"},
				"label-peripheral_btpeer_state":  {"NOT_APPLICABLE"},
				"label-working_bluetooth_btpeer": {"1", "2", "3", "4", "5", "6", "7", "8"},
			},
		},
		{
			"Multiple dimensions with router models should collect all",
			[]swarming.Dimensions{
				{
					"label-wifi_router_models": {"r1"},
				},
				{
					"label-wifi_router_models": {"r2", "r3"},
				},
				{
					"label-wifi_router_models": {"r4"},
				},
				{}, // Empty dim.
			},
			swarming.Dimensions{
				"label-peripheral_wifi_state":   {"NOT_APPLICABLE"},
				"label-peripheral_btpeer_state": {"NOT_APPLICABLE"},
				"label-wifi_router_models":      {"r1", "r2", "r3", "r4"},
			},
		},
		{
			"Multiple dimensions with mixed router features should only include common features",
			[]swarming.Dimensions{
				{
					"label-wifi_router_features": {
						"WIFI_ROUTER_FEATURE_IEEE_802_11_A",
						"WIFI_ROUTER_FEATURE_IEEE_802_11_B",
						"WIFI_ROUTER_FEATURE_IEEE_802_11_G",
						"WIFI_ROUTER_FEATURE_IEEE_802_11_N",
						"999",
					},
				},
				{
					"label-wifi_router_features": {
						"WIFI_ROUTER_FEATURE_IEEE_802_11_A",
						"WIFI_ROUTER_FEATURE_IEEE_802_11_B",
						"WIFI_ROUTER_FEATURE_IEEE_802_11_N",
						"999",
					},
				},
				{
					"label-wifi_router_features": {
						"WIFI_ROUTER_FEATURE_IEEE_802_11_B",
						"WIFI_ROUTER_FEATURE_IEEE_802_11_BE",
						"WIFI_ROUTER_FEATURE_IEEE_802_11_G",
						"WIFI_ROUTER_FEATURE_IEEE_802_11_N",
					},
				},
				{}, // Empty dim should not affect feature list.
			},
			swarming.Dimensions{
				"label-peripheral_wifi_state":   {"NOT_APPLICABLE"},
				"label-peripheral_btpeer_state": {"NOT_APPLICABLE"},
				"label-wifi_router_features": {
					"WIFI_ROUTER_FEATURE_IEEE_802_11_B",
					"WIFI_ROUTER_FEATURE_IEEE_802_11_N",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectPeripheralDimensions(tt.dutsDimsArg)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectPeripheralDimensions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortWifiRouterFeaturesByName(t *testing.T) {
	tests := []struct {
		name            string
		featuresInitial []inventory.Peripherals_WifiRouterFeature
		featuresAfter   []inventory.Peripherals_WifiRouterFeature
	}{
		{
			"empty list",
			[]inventory.Peripherals_WifiRouterFeature{},
			[]inventory.Peripherals_WifiRouterFeature{},
		},
		{
			"nil list",
			nil,
			nil,
		},
		{
			"already sorted",
			[]inventory.Peripherals_WifiRouterFeature{
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				inventory.Peripherals_WIFI_ROUTER_FEATURE_INVALID,
			},
			[]inventory.Peripherals_WifiRouterFeature{
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				inventory.Peripherals_WIFI_ROUTER_FEATURE_INVALID,
			},
		},
		{
			"named sort",
			[]inventory.Peripherals_WifiRouterFeature{
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				inventory.Peripherals_WIFI_ROUTER_FEATURE_INVALID,
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
			},
			[]inventory.Peripherals_WifiRouterFeature{
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				inventory.Peripherals_WIFI_ROUTER_FEATURE_INVALID,
			},
		},
		{
			"unknown name value sort",
			[]inventory.Peripherals_WifiRouterFeature{
				inventory.Peripherals_WifiRouterFeature(99901),
				inventory.Peripherals_WifiRouterFeature(99902),
				inventory.Peripherals_WifiRouterFeature(99900),
			},
			[]inventory.Peripherals_WifiRouterFeature{
				inventory.Peripherals_WifiRouterFeature(99900),
				inventory.Peripherals_WifiRouterFeature(99901),
				inventory.Peripherals_WifiRouterFeature(99902),
			},
		},
		{
			"mixed sort",
			[]inventory.Peripherals_WifiRouterFeature{
				inventory.Peripherals_WifiRouterFeature(99900),
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				inventory.Peripherals_WIFI_ROUTER_FEATURE_INVALID,
				inventory.Peripherals_WifiRouterFeature(99902),
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				inventory.Peripherals_WifiRouterFeature(99901),
			},
			[]inventory.Peripherals_WifiRouterFeature{
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				inventory.Peripherals_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				inventory.Peripherals_WIFI_ROUTER_FEATURE_INVALID,
				inventory.Peripherals_WifiRouterFeature(99900),
				inventory.Peripherals_WifiRouterFeature(99901),
				inventory.Peripherals_WifiRouterFeature(99902),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortWifiRouterFeaturesByName(tt.featuresInitial)
			if !reflect.DeepEqual(tt.featuresInitial, tt.featuresAfter) {
				t.Errorf("SortWifiRouterFeaturesByName() got = %v, want %v", tt.featuresInitial, tt.featuresAfter)
			}
		})
	}
}
