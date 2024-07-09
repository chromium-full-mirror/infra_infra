// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"log"
	"net/url"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"
)

// GenerateDynamicInfo creates dynamic updates for provision
// requests, and adds their relevant information to each
// scheduling unit's dynamic lookup table.
func GenerateDynamicInfo(req *api.InternalTestplan, specs *FirmwareSpecs, log *log.Logger) error {
	// Create Dynamic Updates.
	if err := generateProvisionRequests(req, specs); err != nil {
		return err
	}

	// Add provision/DUT related information to dynamic
	// lookup table for resolving placeholders
	// found within generated Dynamic Updates.
	return generateDynamicUpdateLookupTables(req, specs, log)
}

// generateProvisionRequests loops through each swarming definition and
// builds out the firmware provision request.
func generateProvisionRequests(req *api.InternalTestplan, specs *FirmwareSpecs) error {
	dynamicHelper := NewDynamicFirmwareProvisionHelper(specs)

	// Legacy primary
	suiteMetadata := req.GetSuiteInfo().GetSuiteMetadata()
	if len(suiteMetadata.GetTargetRequirements()) > 0 {
		hwDef := suiteMetadata.GetTargetRequirements()[0].GetHwRequirements().GetHwDefinition()
		if len(hwDef) > 0 {
			swarmingDef := hwDef[0]
			dynamicHelper.GenerateProvisionRequest(req, swarmingDef)
		}
	}

	if len(suiteMetadata.GetSchedulingUnits()) > 0 {
		schedulingUnit := suiteMetadata.GetSchedulingUnits()[0]
		swarmingDef := schedulingUnit.GetPrimaryTarget().GetSwarmingDef()
		dynamicHelper.GenerateProvisionRequest(req, swarmingDef)

		for _, companion := range schedulingUnit.GetCompanionTargets() {
			swarmingDef := companion.GetSwarmingDef()
			dynamicHelper.GenerateProvisionRequest(req, swarmingDef)
		}
	}

	return nil
}

// generateDynamicUpdateLookupTables populates the lookup table for the primary
// and companion devices.
func generateDynamicUpdateLookupTables(req *api.InternalTestplan, specs *FirmwareSpecs, log *log.Logger) error {
	suiteMetadata := req.GetSuiteInfo().GetSuiteMetadata()

	for _, target := range suiteMetadata.GetSchedulingUnits() {
		dynamicHelper := NewDynamicFirmwareProvisionHelper(specs)
		if target.DynamicUpdateLookupTable == nil {
			target.DynamicUpdateLookupTable = map[string]string{}
		}
		lookup := target.DynamicUpdateLookupTable

		// Do primary
		primarySwarming := target.PrimaryTarget.GetSwarmingDef()
		addFwProvisionValuesToLookup(lookup, primarySwarming, dynamicHelper, specs, log)

		// Do companions
		for _, companion := range target.GetCompanionTargets() {
			swarmingDef := companion.GetSwarmingDef()
			addFwProvisionValuesToLookup(lookup, swarmingDef, dynamicHelper, specs, log)
		}
	}

	// Support legacy.
	for _, targetReq := range suiteMetadata.GetTargetRequirements() {
		for _, hwDef := range targetReq.GetHwRequirements().GetHwDefinition() {
			dynamicHelper := NewDynamicFirmwareProvisionHelper(specs)
			if hwDef.DynamicUpdateLookupTable == nil {
				hwDef.DynamicUpdateLookupTable = map[string]string{}
			}
			lookup := hwDef.DynamicUpdateLookupTable
			addFwProvisionValuesToLookup(lookup, hwDef, dynamicHelper, specs, log)
		}
	}
	return nil
}

func resolveSpec(spec string, specs *FirmwareSpecs, swarmingDef *api.SwarmingDefinition) string {
	if spec == LATEST_FIRMWARE_BRANCH {
		dutModel := swarmingDef.GetDutInfo().GetChromeos().GetDutModel()
		board := dutModel.GetBuildTarget()
		// Remove suffix
		board = strings.TrimSuffix(board, "-kernelnext")
		// Special case icarus models
		if board == "jacuzzi" {
			switch dutModel.GetModelName() {
			case "cozmo", "pico", "pico6":
				board = "icarus"
			}
		}
		build, ok := specs.FirmwareBuilds[board]
		if !ok {
			return "gs://invalid_path/no branch build for board " + board
		}
		url, err := url.JoinPath(build.ArtifactLink, build.FirmwareByBoard)
		if err != nil {
			panic(err)
		}
		return url
	}
	if strings.HasPrefix(spec, "gs://") {
		return spec
	}
	if spec == "" {
		return ""
	}
	return "gs://invalid_path/invalid firmware-filter spec " + spec
}

// addFwProvisionValuesToLookup provides the actual values that will be
// stored within the lookup table for firmware provisioning.
func addFwProvisionValuesToLookup(
	lookup map[string]string,
	swarmingDef *api.SwarmingDefinition,
	dynamicHelper *DynamicFirmwareProvisionHelper,
	specs *FirmwareSpecs,
	log *log.Logger) {

	switch swarmingDef.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		lookupValues := &FirmwareProvisionLookupValues{}

		lookupValues.Ro = resolveSpec(specs.Ro, specs, swarmingDef)
		lookupValues.Rw = resolveSpec(specs.Rw, specs, swarmingDef)

		dynamicHelper.ApplyFirmwareProvisionToLookup(lookup, lookupValues)
	}
}
