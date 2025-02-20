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

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

// GenerateDynamicInfo creates dynamic updates for provision
// requests, and adds their relevant information to each
// scheduling unit's dynamic lookup table.
func GenerateDynamicInfo(req *api.InternalTestplan, specs *FirmwareSpecs, log *log.Logger) error {
	// Fix cros-provision settings to avoid flashing the firmware twice
	for _, du := range req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates() {
		provision := du.GetUpdateAction().GetInsert().GetTask().GetProvision()
		if provision.GetInstallRequest().GetMetadata().MessageIs((*api.CrOSProvisionMetadata)(nil)) {
			provision.DynamicDeps = append(provision.DynamicDeps, &api.DynamicDep{
				Key:   common.CrosProvisionMetadataUpdateFirmware,
				Value: "BOOL=false",
			})
		}
	}

	// Create Dynamic Updates.
	if err := generateProvisionRequests(req, specs, log); err != nil {
		return err
	}

	// Add provision/DUT related information to dynamic
	// lookup table for resolving placeholders
	// found within generated Dynamic Updates.
	return generateDynamicUpdateLookupTables(req, specs, log)
}

// generateProvisionRequests loops through each swarming definition and
// builds out the firmware provision request.
func generateProvisionRequests(req *api.InternalTestplan, specs *FirmwareSpecs, log *log.Logger) error {
	dynamicHelper := NewDynamicFirmwareProvisionHelper(specs)

	// Add container request
	return dynamicHelper.GenerateProvisionRequest(req, log)
}

// generateDynamicUpdateLookupTables populates the lookup table for the primary
// and companion devices.
func generateDynamicUpdateLookupTables(req *api.InternalTestplan, specs *FirmwareSpecs, log *log.Logger) error {
	suiteMetadata := req.GetSuiteInfo().GetSuiteMetadata()

	// TODO (oldProto-azrahman): remove when schedulingOptions is fully rolled in.
	for _, target := range suiteMetadata.GetSchedulingUnits() {
		updateDynamicLookupTableForSchedUnit(target, specs, log)
	}

	for _, schedOptions := range suiteMetadata.GetSchedulingUnitOptions() {
		for _, target := range schedOptions.GetSchedulingUnits() {
			updateDynamicLookupTableForSchedUnit(target, specs, log)
		}
	}

	// TODO (oldProto-azrahman): remove when schedulingOptions is fully rolled in.
	//nolint:staticcheck
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

func updateDynamicLookupTableForSchedUnit(schedUnit *api.SchedulingUnit, specs *FirmwareSpecs, log *log.Logger) {
	dynamicHelper := NewDynamicFirmwareProvisionHelper(specs)
	if schedUnit.DynamicUpdateLookupTable == nil {
		schedUnit.DynamicUpdateLookupTable = map[string]string{}
	}
	lookup := schedUnit.DynamicUpdateLookupTable

	// Do primary
	primarySwarming := schedUnit.PrimaryTarget.GetSwarmingDef()
	addFwProvisionValuesToLookup(lookup, primarySwarming, dynamicHelper, specs, log)

	// Do companions
	for _, companion := range schedUnit.GetCompanionTargets() {
		swarmingDef := companion.GetSwarmingDef()
		addFwProvisionValuesToLookup(lookup, swarmingDef, dynamicHelper, specs, log)
	}
}

func resolveSpec(spec string, specs *FirmwareSpecs, swarmingDef *api.SwarmingDefinition, fallbackToOSSource bool) string {
	if spec == LatestFirmwareBranch {
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
		if ok {
			url, err := url.JoinPath(build.ArtifactLink, build.FirmwareByBoard)
			if err != nil {
				panic(err)
			}
			return url
		}
		if !fallbackToOSSource {
			return "gs://invalid_path/no branch build for board " + board
		}
		spec = OSSource
	}
	if spec == OSSource {
		if len(swarmingDef.GetProvisionInfo()) > 0 {
			osPath := swarmingDef.GetProvisionInfo()[0].GetInstallRequest()
			url, err := url.JoinPath(osPath.GetImagePath().GetPath(), "/firmware_from_source.tar.bz2")
			if err != nil {
				panic(err)
			}
			return url
		}
		return "gs://invalid_path/no install request"
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
	_ *log.Logger) {

	switch swarmingDef.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		lookupValues := &FirmwareProvisionLookupValues{}

		lookupValues.Ro = resolveSpec(specs.Ro, specs, swarmingDef, specs.FallbackToCros)
		lookupValues.Rw = resolveSpec(specs.Rw, specs, swarmingDef, specs.FallbackToCros)

		dynamicHelper.ApplyFirmwareProvisionToLookup(lookup, lookupValues)
	}
}
