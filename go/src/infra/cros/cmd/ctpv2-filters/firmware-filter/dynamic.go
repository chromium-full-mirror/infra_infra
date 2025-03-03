// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
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
		err := updateDynamicLookupTableForSchedUnit(target, specs, log)
		if err != nil {
			return fmt.Errorf("metadata sched units failed update dynamic to %s: %w", target, err)
		}
	}

	for _, schedOptions := range suiteMetadata.GetSchedulingUnitOptions() {
		for _, target := range schedOptions.GetSchedulingUnits() {
			err := updateDynamicLookupTableForSchedUnit(target, specs, log)
			if err != nil {
				return fmt.Errorf("options sched units failed update dynamic to %s: %w", target, err)
			}
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
			err := addFwProvisionValuesToLookup(lookup, hwDef, dynamicHelper, specs, log)
			if err != nil {
				return fmt.Errorf("failed adding values to %s: %w", lookup, err)
			}
		}
	}
	return nil
}

func updateDynamicLookupTableForSchedUnit(schedUnit *api.SchedulingUnit, specs *FirmwareSpecs, log *log.Logger) error {
	dynamicHelper := NewDynamicFirmwareProvisionHelper(specs)
	if schedUnit.DynamicUpdateLookupTable == nil {
		schedUnit.DynamicUpdateLookupTable = map[string]string{}
	}
	lookup := schedUnit.DynamicUpdateLookupTable

	// Do primary
	primarySwarming := schedUnit.PrimaryTarget.GetSwarmingDef()
	err := addFwProvisionValuesToLookup(lookup, primarySwarming, dynamicHelper, specs, log)
	if err != nil {
		return fmt.Errorf("primary failed adding values to %s: %w", lookup, err)
	}

	// Do companions
	for _, companion := range schedUnit.GetCompanionTargets() {
		swarmingDef := companion.GetSwarmingDef()
		err = addFwProvisionValuesToLookup(lookup, swarmingDef, dynamicHelper, specs, log)
		if err != nil {
			return fmt.Errorf("companion failed adding values to %s: %w", lookup, err)
		}
	}
	return nil
}

func resolveSpec(spec string, specs *FirmwareSpecs, swarmingDef *api.SwarmingDefinition, fallbackToOSSource bool) (string, error) {
	if fallbackToOSSource {
		spec = fmt.Sprintf("%s,%s", spec, OSSource)
	}
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
	for _, spec := range strings.Split(spec, ",") {
		if spec == LatestFirmwareBranch {
			build, ok := specs.FirmwareBuilds[board]
			if ok {
				url, err := url.JoinPath(build.ArtifactLink, build.FirmwareByBoard)
				if err != nil {
					return "", err
				}
				return url, nil
			}
			log.Printf("No branch build for board %q", board)
		} else if spec == OSSource {
			if len(swarmingDef.GetProvisionInfo()) > 0 {
				osPath := swarmingDef.GetProvisionInfo()[0].GetInstallRequest()
				url, err := url.JoinPath(osPath.GetImagePath().GetPath(), "/firmware_from_source.tar.bz2")
				if err != nil {
					return "", err
				}
				return url, nil
			}
			return "", fmt.Errorf("no install request")
		} else if strings.HasPrefix(spec, ECMilestonePrefix) {
			milestoneOffset, err := strconv.Atoi(spec[len(ECMilestonePrefix):])
			if err != nil {
				return "", fmt.Errorf("invalid firmware-filter spec %q: %w", spec, err)
			}
			targetMilestone := specs.LatestMilestone - milestoneOffset
			milestoneToBuild, ok := specs.ECMilestoneBuilds[board]
			if !ok {
				log.Printf("No milestone builds for board %q", board)
				continue
			}
			build, ok := milestoneToBuild[targetMilestone]
			if !ok {
				log.Printf("No milestone R%d builds for board %q", targetMilestone, board)
				continue
			}
			url, err := url.JoinPath(build.ArtifactLink, build.FirmwareByBoard)
			if err != nil {
				return "", err
			}
			return url, nil
		} else if strings.HasPrefix(spec, "gs://") {
			return spec, nil
		} else if spec == "" {
			return "", nil
		} else {
			return "", fmt.Errorf("invalid firmware-filter spec %q", spec)
		}
	}
	return "", fmt.Errorf("no builds found for spec %q, board %q", spec, board)
}

// addFwProvisionValuesToLookup provides the actual values that will be
// stored within the lookup table for firmware provisioning.
func addFwProvisionValuesToLookup(
	lookup map[string]string,
	swarmingDef *api.SwarmingDefinition,
	dynamicHelper *DynamicFirmwareProvisionHelper,
	specs *FirmwareSpecs,
	_ *log.Logger) error {

	switch swarmingDef.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		lookupValues := &FirmwareProvisionLookupValues{}
		var err error

		lookupValues.Ro, err = resolveSpec(specs.Ro, specs, swarmingDef, specs.FallbackToCros)
		if err != nil {
			return err
		}
		lookupValues.Rw, err = resolveSpec(specs.Rw, specs, swarmingDef, specs.FallbackToCros)
		if err != nil {
			return err
		}
		lookupValues.ECRO, err = resolveSpec(specs.ECRO, specs, swarmingDef, specs.FallbackToCros)
		if err != nil {
			return err
		}
		lookupValues.ECRW, err = resolveSpec(specs.ECRW, specs, swarmingDef, specs.FallbackToCros)
		if err != nil {
			return err
		}

		dynamicHelper.ApplyFirmwareProvisionToLookup(lookup, lookupValues)
	}
	return nil
}
