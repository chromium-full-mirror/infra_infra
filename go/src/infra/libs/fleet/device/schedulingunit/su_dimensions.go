// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package schedulingunit

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"go.chromium.org/infra/libs/skylab/inventory"
	"go.chromium.org/infra/libs/skylab/inventory/swarming"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

var dutToSULabelMap = map[string]string{
	"label-board": "label-board",
	"label-model": "label-model",
	"dut_name":    "label-managed_dut",
}

// differentiateLabelValues takes an array of label values and makes them unique.
//
// differentiateLabelValues makes the values unique by adding suffixes to the
// value if they are duplicates.Swarming doesn't allow repeat value of a
// dimension, so we give them a suffix.
//
// E.g. A scheduling unit containing two eve board DUTs will have label-board:
// [eve, eve_2]
func differentiateLabelValues(labelValues []string) []string {
	var res []string
	occurrences := make(map[string]int)
	for _, l := range labelValues {
		occurrences[l] += 1
		suffix := ""
		if occurrences[l] > 1 {
			suffix = fmt.Sprintf("_%d", occurrences[l])
		}
		res = append(res, l+suffix)
	}
	return res
}

// joinDutLabelsToSU joins adds the label values of an SU's DUTs to the SU's
// label dimensions.
func joinDutLabelsToSU(dutLabels []string, dutsDims []swarming.Dimensions, suDims map[string][]string) {
	for _, dutLabelName := range dutLabels {
		if suLabelName, ok := dutToSULabelMap[dutLabelName]; ok {
			joinedLabels := differentiateLabelValues(dutLabelValues(dutLabelName, dutsDims))
			if len(joinedLabels) > 0 {
				suDims[suLabelName] = joinedLabels
			}
		}
	}
}

// dutLabelValues iterates through an array of Swarming dimensions and returns
// all the values of a specified label.
func dutLabelValues(label string, dims []swarming.Dimensions) []string {
	var res []string
	for _, dim := range dims {
		if v, ok := dim[label]; ok {
			if len(v) > 0 {
				res = append(res, v[0])
			}
		}
	}
	return res
}

// labelIntersection takes a label name and a slice of device dimensions, and
// return only values that are common in all devices.
func labelIntersection(label string, dims []swarming.Dimensions) []string {
	valueCount := make(map[string]int)
	for _, dim := range dims {
		if values, ok := dim[label]; ok {
			for _, v := range values {
				valueCount[v] += 1
			}
		}
	}
	// Iterate over the keys of valueCount in lexicographic order.
	var keys []string
	var labels []string
	for label := range valueCount {
		keys = append(keys, label)
	}
	sort.Strings(keys)
	for _, label := range keys {
		count := valueCount[label]
		if count == len(dims) {
			labels = append(labels, label)
		}
	}
	return labels
}

// GetSchedulingUnitDimensions converts UFS SchedulingUnit data to Swarming
// labels and returns it as a map.
func GetSchedulingUnitDimensions(su *ufspb.SchedulingUnit, dutsDims []swarming.Dimensions) map[string][]string {
	// Base labels for a SchedulingUnit
	suDims := map[string][]string{
		"dut_name":        {ufsUtil.RemovePrefix(su.GetName())},
		"dut_id":          {ufsUtil.RemovePrefix(su.GetName())},
		"label-pool":      su.GetPools(),
		"label-dut_count": {fmt.Sprintf("%d", len(dutsDims))},
		"label-multiduts": {"True"},
		"dut_state":       {ufsUtil.SchedulingUnitDutState(dutLabelValues("dut_state", dutsDims))},
	}
	if su.GetPrimaryDut() != "" {
		suDims["label-primary_dut"] = []string{su.GetPrimaryDut()}
	}
	if su.GetWificell() {
		suDims["label-wificell"] = []string{"True"}
	}
	if su.GetCarrier() != "" {
		suDims["label-carrier"] = []string{su.GetCarrier()}
	}

	var dutLabels, conjunctionLabels []string
	var detailedLabelDut string
	switch su.GetExposeType() {
	case ufspb.SchedulingUnit_DEFAULT:
		dutLabels = []string{"label-board", "label-model", "dut_name"}
		conjunctionLabels = []string{"label-device-stable"}
	case ufspb.SchedulingUnit_DEFAULT_PLUS_PRIMARY:
		dutLabels = []string{"label-board", "label-model", "dut_name"}
		detailedLabelDut = su.GetPrimaryDut()
	case ufspb.SchedulingUnit_STRICTLY_PRIMARY_ONLY:
		// For the strict primary dut mode, we will only expose primary dut labels
		// scheduling unit.
		dutLabels = []string{"dut_name"}
		detailedLabelDut = su.GetPrimaryDut()
	}

	// Join DUT labels. SU labels is the union of all of its DUTs' labels.
	joinDutLabelsToSU(dutLabels, dutsDims, suDims)

	// conjunctionLabels defines labels we want present in the SU only if all
	// its devices has the given label. The SU will only inherit values that
	// are common among all devices under the SU.
	for _, label := range conjunctionLabels {
		values := labelIntersection(label, dutsDims)
		if len(values) > 0 {
			suDims[label] = values
		}
	}

	// Add label from detailedLabelDut to suDims.
	// detailedLabelDut labels that are not already in suDims are added.
	if detailedLabelDut != "" {
		var detailDim swarming.Dimensions
		for _, dim := range dutsDims {
			if dim["dut_name"][0] == detailedLabelDut {
				detailDim = dim
				break
			}
		}
		for key, val := range detailDim {
			if _, ok := suDims[key]; !ok {
				suDims[key] = val
			}
		}
	}

	// Add peripheral dims from all duts.
	for dim, value := range collectPeripheralDimensions(dutsDims) {
		suDims[dim] = value
	}
	return suDims
}

// GetSchedulingUnitBotState converts UFS SchedulingUnit data to bot state
// and returns it as a map.
func GetSchedulingUnitBotState(su *ufspb.SchedulingUnit) map[string][]string {
	return map[string][]string{
		"scheduling_unit_version_index": {su.GetUpdateTime().AsTime().Format(ufsUtil.TimestampBasedVersionKeyFormat)},
	}
}

// CheckIfLSEBelongsToSU checks if the DUT/Labstation belongs to a SchedulingUnit.
//
// User is not allowed to udpate a DUT/Labstation which belongs to a SU.
// The DUT/Labstation needs to be removed from the SU and then updated.
func CheckIfLSEBelongsToSU(ctx context.Context, ic ufsAPI.FleetClient, lseName string) error {
	req := &ufsAPI.ListSchedulingUnitsRequest{
		Filter:   fmt.Sprintf("duts=%s", lseName),
		KeysOnly: true,
	}
	res, err := ic.ListSchedulingUnits(ctx, req)
	if err != nil {
		return err
	}
	if len(res.GetSchedulingUnits()) > 0 {
		return fmt.Errorf("DUT/Labstation is associated with SchedulingUnit. Run `shivas update schedulingunit -name %s -removeduts %s` to remove association before updating the DUT/Labstation", ufsUtil.RemovePrefix(res.GetSchedulingUnits()[0].GetName()), lseName)
	}
	return nil
}

// collectPeripheralDimensions collects all the dimensions specific to
// peripherals from all the individual dut dimensions. Values are combined as
// needed to reflect state of the whole unit.
//
// The dut converter functions are used to create the dims the same way it's
// done for duts once the values are finalized. Then, only the related
// peripheral labels are returned.
func collectPeripheralDimensions(dutsDims []swarming.Dimensions) swarming.Dimensions {
	// Initialize defaults.
	peripheralWifiState := inventory.PeripheralState_NOT_APPLICABLE
	peripheralBtpeerState := inventory.PeripheralState_NOT_APPLICABLE
	workingBtpeers := int32(0)
	var wifiRouterModels []string

	// Set values based on dims from all duts.
	commonWifiRouterFeatures := make(map[inventory.Peripherals_WifiRouterFeature]bool)
	for _, dutDims := range dutsDims {
		// Unmarshall peripheral dimensions.
		dLabels := swarming.Revert(dutDims)
		dPeripherals := dLabels.GetPeripherals()
		if dPeripherals == nil {
			continue
		}

		// Simple collection labels, adding values across all duts.
		workingBtpeers += dPeripherals.GetWorkingBluetoothBtpeer()
		if len(dPeripherals.GetWifiRouterModels()) > 0 {
			wifiRouterModels = append(wifiRouterModels, dPeripherals.GetWifiRouterModels()...)
		}

		// States should be WORKING only if all duts with applicable states are WORKING.
		peripheralWifiState = combinePeripheralState(peripheralWifiState, dPeripherals.GetPeripheralWifiState())
		peripheralBtpeerState = combinePeripheralState(peripheralBtpeerState, dPeripherals.GetPeripheralBtpeerState())

		// Router features should only be included if they are in every non-empty set.
		if len(dPeripherals.GetWifiRouterFeatures()) > 0 {
			dutWifiRouterFeatures := make(map[inventory.Peripherals_WifiRouterFeature]bool)
			for _, feature := range dPeripherals.GetWifiRouterFeatures() {
				dutWifiRouterFeatures[feature] = true
			}
			if len(commonWifiRouterFeatures) == 0 {
				// First set of features so include them all.
				commonWifiRouterFeatures = dutWifiRouterFeatures
			} else {
				// Mark common features as uncommon if they are not in dut's features.
				for feature, isCommon := range commonWifiRouterFeatures {
					if isCommon {
						commonWifiRouterFeatures[feature] = dutWifiRouterFeatures[feature]
					}
				}
			}
		}
	}
	// Add only common router features to unit's router features.
	var wifiRouterFeatures []inventory.Peripherals_WifiRouterFeature
	for feature, isCommon := range commonWifiRouterFeatures {
		if isCommon {
			wifiRouterFeatures = append(wifiRouterFeatures, feature)
		}
	}
	// Sort features to keep order consistent and make it easier to read.
	if len(wifiRouterFeatures) > 0 {
		sortWifiRouterFeaturesByName(wifiRouterFeatures)
	}

	// Create dimensions map based on aggregate schedule labels data.
	pLabels := &inventory.SchedulableLabels{}
	pLabels.Peripherals = &inventory.Peripherals{}
	pLabels.Peripherals.PeripheralWifiState = &peripheralWifiState
	pLabels.Peripherals.PeripheralBtpeerState = &peripheralBtpeerState
	pLabels.Peripherals.WorkingBluetoothBtpeer = &workingBtpeers
	pLabels.Peripherals.WifiRouterFeatures = wifiRouterFeatures
	pLabels.Peripherals.WifiRouterModels = wifiRouterModels
	pDims := swarming.Convert(pLabels)

	// Filter for only the dims we are interested in for peripherals.
	pDimsFiltered := make(map[string][]string)
	peripheralDims := []string{
		"label-peripheral_wifi_state",
		"label-peripheral_btpeer_state",
		"label-working_bluetooth_btpeer",
		"label-wifi_router_features",
		"label-wifi_router_models",
	}
	for _, dim := range peripheralDims {
		if value, ok := pDims[dim]; ok {
			pDimsFiltered[dim] = value
		}
	}

	return pDimsFiltered
}

// combinePeripheralState will return a state which returns an updated suState
// that reflects the overall combined state of the unit with dutState included.
func combinePeripheralState(suState, dutState inventory.PeripheralState) inventory.PeripheralState {
	if suState == inventory.PeripheralState_BROKEN {
		// Unit state already broken by another peripheral, the additional dutState
		// will have no effect.
		return suState
	}
	switch dutState {
	case inventory.PeripheralState_UNKNOWN,
		inventory.PeripheralState_NOT_APPLICABLE:
		// No relevant dutState, so no change.
		return suState
	case inventory.PeripheralState_WORKING:
		return inventory.PeripheralState_WORKING
	default:
		// All states other than WORKING are treated as BROKEN for suState.
		return inventory.PeripheralState_BROKEN
	}
}

// sortWifiRouterFeaturesByName sorts the list of features first by known names
// and then unknown names (would be integers). Has the same effect as the
// sorting method used when setting the dut-level dimensions.
func sortWifiRouterFeaturesByName(features []inventory.Peripherals_WifiRouterFeature) {
	sort.SliceStable(features, func(i, j int) bool {
		aName := features[i].String()
		bName := features[j].String()
		// Determine if these are known enum names or just int strings.
		aKnown := false
		bKnown := false
		aInt, err := strconv.Atoi(aName)
		if err != nil {
			aKnown = true
		}
		bInt, err := strconv.Atoi(bName)
		if err != nil {
			bKnown = true
		}
		// Compare known names by string and unknown names by their int value,
		// with known names coming before all unknown names.
		if aKnown && bKnown {
			return aName < bName
		}
		if !aKnown && !bKnown {
			return aInt < bInt
		}
		return aKnown && !bKnown
	})
}
