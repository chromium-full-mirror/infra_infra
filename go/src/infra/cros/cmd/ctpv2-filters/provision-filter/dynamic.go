// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates"
	dynamic_common "go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/common"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/generators"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/helpers"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
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
	// TODO (oldProto-azrahman): remove when SchedulingUnitsOptions are fully rolled in.
	if len(suiteMetadata.GetSchedulingUnits()) > 0 {
		// 0 index as we only need to count the length of one companions list.
		generateProvisionRequestForSchedUnit(suiteMetadata.GetSchedulingUnits()[0], req, provisionHelper)
	}

	if len(suiteMetadata.GetSchedulingUnitOptions()) > 0 && len(suiteMetadata.GetSchedulingUnitOptions()[0].GetSchedulingUnits()) > 0 {
		// 0 index as we only need to count the length of one companions list.
		generateProvisionRequestForSchedUnit(suiteMetadata.GetSchedulingUnitOptions()[0].GetSchedulingUnits()[0], req, provisionHelper)
	}

	return
}

// generateProvisionRequestForSchedUnit generates provision request for provided scheduling unit
func generateProvisionRequestForSchedUnit(schedulingUnit *api.SchedulingUnit, req *api.InternalTestplan, provisionHelper *helpers.DynamicProvisionHelper) {
	// Create primary request.
	swarmingDef := schedulingUnit.GetPrimaryTarget().GetSwarmingDef()
	provisionHelper.GenerateProvisionRequest(req, swarmingDef, true)

	// Create companion requests.
	for _, companion := range schedulingUnit.GetCompanionTargets() {
		swarmingDef := companion.GetSwarmingDef()
		provisionHelper.GenerateProvisionRequest(req, swarmingDef, false)
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
			lookupHelper := helpers.NewDynamicProvisionHelper()
			if hwDef.DynamicUpdateLookupTable == nil {
				hwDef.DynamicUpdateLookupTable = map[string]string{}
			}
			lookup := hwDef.DynamicUpdateLookupTable
			addProvisionValuesToLookup(lookup, hwDef, lookupHelper)
		}
	}
}

func generateLookupTableForSchedUnits(schedUnits []*api.SchedulingUnit) {
	for _, target := range schedUnits {
		lookupHelper := helpers.NewDynamicProvisionHelper()
		if target.DynamicUpdateLookupTable == nil {
			target.DynamicUpdateLookupTable = map[string]string{}
		}
		lookup := target.DynamicUpdateLookupTable

		// Do primary
		primarySwarming := target.PrimaryTarget.GetSwarmingDef()
		primaryBoard := extractDutModel(primarySwarming).GetBuildTarget()
		addProvisionValuesToLookup(lookup, primarySwarming, lookupHelper)
		if common.IsSupportedVMBoard(primaryBoard) {
			generateVMDynamicProvisionRequest(target)
			modifyTestRequestForVM(target)
			continue
		}

		// Do Companion
		for _, companion := range target.GetCompanionTargets() {
			swarmingDef := companion.GetSwarmingDef()
			addProvisionValuesToLookup(lookup, swarmingDef, lookupHelper)
		}
	}
}

// generateVMDynamicProvisionRequest
func generateVMDynamicProvisionRequest(sUnit *api.SchedulingUnit) {
	if sUnit.SecondaryDynamicUpdates == nil {
		sUnit.SecondaryDynamicUpdates = []*api.UserDefinedDynamicUpdate{}
	}

	generator := generators.NewInsertGenerator()
	generator.AddInsertion(&api.CrosTestRunnerDynamicRequest_Task{
		OrderedContainerRequests: []*api.ContainerRequest{
			helpers.NewCacheServerContainer().Build(),
			helpers.NewCrosDutContainer(dynamic_common.NewPrimaryDeviceIdentifier()).Build(),
		},
	}, dynamic_common.ReplaceTaskWrapper(dynamic_common.FindFirst(api.FocalTaskFinder_PROVISION)))

	dynamic_updates.AppendUserDefinedDynamicUpdates(&sUnit.SecondaryDynamicUpdates, generator.Generate)
}

func extractDutModel(swarmingDef *api.SwarmingDefinition) *dut_api.DutModel {
	switch dutType := swarmingDef.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		return dutType.Chromeos.GetDutModel()
	case *dut_api.Dut_Android_:
		return dutType.Android.GetDutModel()
	default:
		return nil
	}
}

// modifyTestRequestForVM modifies the cros-test task by adding
// extra dynamic dependencies that are needed for VM runs.
func modifyTestRequestForVM(sUnit *api.SchedulingUnit) {
	if sUnit.SecondaryDynamicUpdates == nil {
		sUnit.SecondaryDynamicUpdates = []*api.UserDefinedDynamicUpdate{}
	}

	generator := generators.NewModifyGenerator(dynamic_common.FindByDynamicIdentifier(common.CrosTest))
	_ = generator.AddModification(
		&api.DynamicDep{
			Key:   common.TestRequestPrimary + ".dut.cacheServer.address",
			Value: common.CacheServer,
		},
		map[string]string{
			common.TestDynamicDeps: "",
		},
	)
	_ = generator.AddModification(
		&api.DynamicDep{
			// Update the cacheServer's address string to the
			// external host ip address.
			Key:   common.TestRequestPrimary + ".dut.cacheServer.address.address",
			Value: common.HostIP,
		},
		map[string]string{
			common.TestDynamicDeps: "",
		},
	)

	_ = dynamic_updates.AppendUserDefinedDynamicUpdates(&sUnit.SecondaryDynamicUpdates, generator.Generate)
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
