// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/commonbuilders"
)

// GenerateDynamicInfo creates dynamic updates for provision
// requests, and adds their relevant information to each
// scheduling unit's dynamic lookup table.
func GenerateDynamicInfo(req *api.InternalTestplan) error {
	// Create Dynamic Updates.
	if err := generateProvisionRequests(req); err != nil {
		return err
	}

	// Add provision/DUT related information to dynamic
	// lookup table for resolving placeholders
	// found within generated Dynamic Updates.
	generateDynamicUpdateLookupTables(req)

	return nil
}

// generateProvisionRequests creates the primary and companion cros-provision
// requests and sets the relevant placeholders.
func generateProvisionRequests(req *api.InternalTestplan) (err error) {
	provisionHelper := NewDynamicAshChromeProvisionHelper()
	suiteMetadata := req.GetSuiteInfo().GetSuiteMetadata()
	if len(suiteMetadata.GetSchedulingUnitOptions()) > 0 && len(suiteMetadata.GetSchedulingUnitOptions()[0].GetSchedulingUnits()) > 0 {
		generateProvisionRequestForSchedUnit(suiteMetadata.GetSchedulingUnitOptions()[0].GetSchedulingUnits()[0], req, provisionHelper)

	}

	// TODO(oldProto-azrahman): remove when schedulingOptions are fully rolled in.
	if len(suiteMetadata.GetSchedulingUnits()) > 0 {
		generateProvisionRequestForSchedUnit(suiteMetadata.GetSchedulingUnits()[0], req, provisionHelper)
	}

	// TODO (oldProto-azrahman): remove old proto stuffs when schedulingUnits are fully rolled in.
	if len(suiteMetadata.GetTargetRequirements()) > 0 {
		hwDef := suiteMetadata.GetTargetRequirements()[0].GetHwRequirements().GetHwDefinition()
		swReq := suiteMetadata.GetTargetRequirements()[0].GetSwRequirement()
		if len(hwDef) > 0 {
			provisionHelper.GenerateProvisionRequest(req, hwDef[0], swReq, true)
		}
	}

	return
}

// generateProvisionRequestForSchedUnit generates provision request for provided scheduling unit
func generateProvisionRequestForSchedUnit(schedulingUnit *api.SchedulingUnit, req *api.InternalTestplan, provisionHelper *DynamicAshChromeProvisionHelper) {
	// Create primary request.
	swarmingDef := schedulingUnit.GetPrimaryTarget().GetSwarmingDef()
	swReq := schedulingUnit.GetPrimaryTarget().GetSwReq()
	provisionHelper.GenerateProvisionRequest(req, swarmingDef, swReq, true)

	// Create companion requests.
	for _, companion := range schedulingUnit.GetCompanionTargets() {
		swarmingDef := companion.GetSwarmingDef()
		swReq := companion.GetSwReq()
		provisionHelper.GenerateProvisionRequest(req, swarmingDef, swReq, false)
	}
}

// generateDynamicUpdateLookupTables adds provision related info to
// each HwDefinition's dynamic lookup table.
func generateDynamicUpdateLookupTables(req *api.InternalTestplan) {
	for _, schedOption := range req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnitOptions() {
		generateLookupTableForSchedUnits(schedOption.GetSchedulingUnits())
	}

	// TODO (oldProto-azrahman): remove when schedulingUnitOptions are fully rolled in.
	generateLookupTableForSchedUnits(req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits())

	// TODO (oldProto-azrahman): remove old proto stuffs when schedulingUnits are fully rolled in.
	// Support legacy.
	for _, targetReq := range req.GetSuiteInfo().GetSuiteMetadata().GetTargetRequirements() {
		for _, hwDef := range targetReq.GetHwRequirements().GetHwDefinition() {
			lookupHelper := NewDynamicAshChromeProvisionHelper()
			if hwDef.DynamicUpdateLookupTable == nil {
				hwDef.DynamicUpdateLookupTable = map[string]string{}
			}
			lookup := hwDef.DynamicUpdateLookupTable
			addProvisionValuesToLookup(lookup, hwDef, targetReq.GetSwRequirement(), lookupHelper)
		}
	}
}

func generateLookupTableForSchedUnits(schedUnits []*api.SchedulingUnit) {
	for _, target := range schedUnits {
		lookupHelper := NewDynamicAshChromeProvisionHelper()
		if target.DynamicUpdateLookupTable == nil {
			target.DynamicUpdateLookupTable = map[string]string{}
		}
		lookup := target.DynamicUpdateLookupTable

		// Do primary
		primarySwarming := target.PrimaryTarget.GetSwarmingDef()
		primarySw := target.PrimaryTarget.GetSwReq()
		addProvisionValuesToLookup(lookup, primarySwarming, primarySw, lookupHelper)

		// Do Companion
		for _, companion := range target.GetCompanionTargets() {
			swarmingDef := companion.GetSwarmingDef()
			swReq := companion.GetSwReq()
			addProvisionValuesToLookup(lookup, swarmingDef, swReq, lookupHelper)

		}
	}
}

// addProvisionValuesToLookup switches on the DUT type to add in the
// appropriate lookup values to the lookup table.
func addProvisionValuesToLookup(lookup map[string]string, swarmingDef *api.SwarmingDefinition, swReq *api.LegacySW, lookupHelper *DynamicAshChromeProvisionHelper) {
	lookupValues := &AshChromeProvisionLookupValues{}
	for _, kvs := range swReq.GetKeyValues() {
		if kvs.Key == commonbuilders.AshChromeGcsPath {
			lookupValues.AshChromeGcsPath = kvs.Value
		}
		if kvs.Key == commonbuilders.AshChromeBuildOutputDir {
			lookupValues.AshChromeBuildOutputDir = kvs.Value
		}
	}
	lookupHelper.ApplyAshChromeProvisionToLookup(lookup, lookupValues)
}
