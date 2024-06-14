// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/helpers"
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
	provisionHelper := helpers.NewDynamicProvisionHelper()

	suiteMetadata := req.GetSuiteInfo().GetSuiteMetadata()
	// TODO (oldProto-azrahman): remove old proto stuffs when schedulingUnits are fully rolled in.
	// Create legacy primary request.
	if len(suiteMetadata.GetTargetRequirements()) > 0 {
		hwDef := suiteMetadata.GetTargetRequirements()[0].GetHwRequirements().GetHwDefinition()
		if len(hwDef) > 0 {
			swarmingDef := hwDef[0]
			provisionHelper.GenerateProvisionRequest(req, swarmingDef, true)
		}
	}

	// Create companion requests for chromeos devices.
	if len(suiteMetadata.GetSchedulingUnits()) > 0 {
		// 0 index as we only need to count the length of one companions list.
		schedulingUnit := suiteMetadata.GetSchedulingUnits()[0]

		// Create primary request.
		swarmingDef := schedulingUnit.GetPrimaryTarget().GetSwarmingDef()
		provisionHelper.GenerateProvisionRequest(req, swarmingDef, true)

		// Create companion requests.
		for _, companion := range schedulingUnit.GetCompanionTargets() {
			swarmingDef := companion.GetSwarmingDef()
			provisionHelper.GenerateProvisionRequest(req, swarmingDef, false)
		}
	}

	return
}

// generateDynamicUpdateLookupTables adds provision related info to
// each HwDefinition's dynamic lookup table.
func generateDynamicUpdateLookupTables(req *api.InternalTestplan) {
	for _, target := range req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits() {
		lookupHelper := helpers.NewDynamicProvisionHelper()
		if target.DynamicUpdateLookupTable == nil {
			target.DynamicUpdateLookupTable = map[string]string{}
		}
		lookup := target.DynamicUpdateLookupTable

		// Do primary
		primarySwarming := target.PrimaryTarget.GetSwarmingDef()
		addProvisionValuesToLookup(lookup, primarySwarming, lookupHelper)

		// Do Companion
		for _, companion := range target.GetCompanionTargets() {
			swarmingDef := companion.GetSwarmingDef()
			addProvisionValuesToLookup(lookup, swarmingDef, lookupHelper)
		}
	}

	// TODO (oldProto-azrahman): remove old proto stuffs when schedulingUnits are fully rolled in.
	// Support legacy.
	for _, targetReq := range req.GetSuiteInfo().GetSuiteMetadata().GetTargetRequirements() {
		for _, hwDef := range targetReq.GetHwRequirements().GetHwDefinition() {
			lookupHelper := helpers.NewDynamicProvisionHelper()
			if hwDef.DynamicUpdateLookupTable == nil {
				hwDef.DynamicUpdateLookupTable = map[string]string{}
			}
			lookup := hwDef.DynamicUpdateLookupTable
			addProvisionValuesToLookup(lookup, hwDef, lookupHelper)
		}
	}
}

// addProvisionValuesToLookup switches on the DUT type to add in the
// appropriate lookup values to the lookup table.
func addProvisionValuesToLookup(lookup map[string]string, swarmingDef *api.SwarmingDefinition, lookupHelper *helpers.DynamicProvisionHelper) {
	switch dutType := swarmingDef.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		lookupValues := &helpers.CrosProvisionLookupValues{}
		lookupValues.Board = dutType.Chromeos.GetDutModel().GetBuildTarget()
		if len(swarmingDef.GetProvisionInfo()) > 0 {
			lookupValues.InstallPath = swarmingDef.GetProvisionInfo()[len(swarmingDef.GetProvisionInfo())-1].GetInstallRequest().GetImagePath().GetPath()
		}
		lookupHelper.ApplyCrosProvisionToLookup(lookup, lookupValues)

	case *dut_api.Dut_Android_:
		lookupValues := &helpers.AndroidProvisionLookupValues{}
		lookupValues.Board = dutType.Android.GetDutModel().GetBuildTarget()
		lookupHelper.ApplyAndroidProvisionToLookup(lookup, lookupValues)
	}
}
