// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

// GenerateDynamicInfo creates dynamic updates for provision
// requests, and adds their relevant information to each
// scheduling unit's dynamic lookup table.
func GenerateDynamicInfo(ctx context.Context, commonParams *common.CommonFilterParams, req *api.InternalTestplan, specs *FirmwareSpecs, log *log.Logger, matcher gcsMatcher) error {
	// Fix cros-provision settings to avoid flashing the firmware twice
	if specs.Ro != "" || specs.Rw != "" || specs.ECRO != "" || specs.ECRW != "" {
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
		if err := generateProvisionRequests(ctx, commonParams, req, specs, log); err != nil {
			return fmt.Errorf("generateProvisionRequests failed: %w", err)
		}
	}

	if specs.TestArgReplacements != nil {
		suiteMetadata := req.GetSuiteInfo().GetSuiteMetadata()

		// TODO (oldProto-azrahman): remove when schedulingOptions is fully rolled in.
		if len(suiteMetadata.GetSchedulingUnits()) > 0 {
			if len(suiteMetadata.GetSchedulingUnits()) != 1 {
				return fmt.Errorf("test arg replacement only allowed for a single suite_metadata.scheduling_units, got %d", len(suiteMetadata.GetSchedulingUnits()))
			}
			target := suiteMetadata.GetSchedulingUnits()[0]
			if err := addFwPathsToTestArgs(
				ctx,
				req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata(),
				target.GetPrimaryTarget().GetSwarmingDef(),
				specs,
				log,
				matcher); err != nil {
				return fmt.Errorf("addFwPathsToTestArgs failed: %w", err)
			}
		} else {
			if len(suiteMetadata.GetSchedulingUnitOptions()) != 1 {
				return fmt.Errorf("test arg replacement only allowed for a single scheduling_unit_options, got %d", len(suiteMetadata.GetSchedulingUnitOptions()))
			}
			schedOptions := suiteMetadata.GetSchedulingUnitOptions()[0]
			if len(schedOptions.GetSchedulingUnits()) != 1 {
				return fmt.Errorf("test arg replacement only allowed for a single scheduling_unit_options.scheduling_units, got %d", len(schedOptions.GetSchedulingUnits()))
			}
			target := schedOptions.GetSchedulingUnits()[0]
			if err := addFwPathsToTestArgs(
				ctx,
				req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata(),
				target.GetPrimaryTarget().GetSwarmingDef(),
				specs,
				log,
				matcher); err != nil {
				return fmt.Errorf("addFwPathsToTestArgs failed: %w", err)
			}
		}
	}

	// Add provision/DUT related information to dynamic
	// lookup table for resolving placeholders
	// found within generated Dynamic Updates.
	if err := generateDynamicUpdateLookupTables(ctx, req, specs, log, matcher); err != nil {
		return fmt.Errorf("generateDynamicUpdateLookupTables failed: %w", err)
	}
	return nil
}

// generateProvisionRequests loops through each swarming definition and
// builds out the firmware provision request.
func generateProvisionRequests(ctx context.Context, commonParams *common.CommonFilterParams, req *api.InternalTestplan, specs *FirmwareSpecs, log *log.Logger) error {
	dynamicHelper := NewDynamicFirmwareProvisionHelper(specs)

	// Add container request
	return dynamicHelper.GenerateProvisionRequest(ctx, commonParams, req, log)
}

// generateDynamicUpdateLookupTables populates the lookup table for the primary
// and companion devices.
func generateDynamicUpdateLookupTables(ctx context.Context, req *api.InternalTestplan, specs *FirmwareSpecs, log *log.Logger, matcher gcsMatcher) error {
	suiteMetadata := req.GetSuiteInfo().GetSuiteMetadata()

	// TODO (oldProto-azrahman): remove when schedulingOptions is fully rolled in.
	for _, target := range suiteMetadata.GetSchedulingUnits() {
		err := updateDynamicLookupTableForSchedUnit(ctx, target, specs, log, matcher)
		if err != nil {
			return fmt.Errorf("metadata sched units failed update dynamic to %s: %w", target, err)
		}
	}

	for _, schedOptions := range suiteMetadata.GetSchedulingUnitOptions() {
		for _, target := range schedOptions.GetSchedulingUnits() {
			err := updateDynamicLookupTableForSchedUnit(ctx, target, specs, log, matcher)
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
			err := addFwProvisionValuesToLookup(ctx, lookup, hwDef, dynamicHelper, specs, log, matcher)
			if err != nil {
				return fmt.Errorf("failed adding values to %s: %w", lookup, err)
			}
		}
	}
	return nil
}

func updateDynamicLookupTableForSchedUnit(ctx context.Context, schedUnit *api.SchedulingUnit, specs *FirmwareSpecs, log *log.Logger, matcher gcsMatcher) error {
	dynamicHelper := NewDynamicFirmwareProvisionHelper(specs)
	if schedUnit.DynamicUpdateLookupTable == nil {
		schedUnit.DynamicUpdateLookupTable = map[string]string{}
	}
	lookup := schedUnit.DynamicUpdateLookupTable

	// Do primary
	primarySwarming := schedUnit.PrimaryTarget.GetSwarmingDef()
	err := addFwProvisionValuesToLookup(ctx, lookup, primarySwarming, dynamicHelper, specs, log, matcher)
	if err != nil {
		return fmt.Errorf("primary failed adding values to %s: %w", lookup, err)
	}

	// Do companions
	for _, companion := range schedUnit.GetCompanionTargets() {
		swarmingDef := companion.GetSwarmingDef()
		err = addFwProvisionValuesToLookup(ctx, lookup, swarmingDef, dynamicHelper, specs, log, matcher)
		if err != nil {
			return fmt.Errorf("companion failed adding values to %s: %w", lookup, err)
		}
	}
	return nil
}

var versionNumbnerRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// firmwareArtifactRe parses the url into groups: bucket, prefix, release, suffix
var firmwareArtifactRe = regexp.MustCompile(`^gs://(chromeos-image-archive)/(firmware-[^/]*-\d+\.)[^/]*/(R\d+-)?(?:[\d\.]+)(?:-[^/]*)?/(.*)$`)

// gcsMatcher is a function that searches google cloud storage for a specific version artifact. It is injected for testing purposes.
// bucket is the storage bucket, i.e. chromeos-image-archive
// branchPrefix is the branch up through the major version and dot, i.e. firmware-brya-14505.
// release is optional, but if provided should be a milestone with trailing hyphen like R100-
// version is the desired version, i.e. 14505.102.0
// suffix is the path to the file, i.e. brya/firmware_from_source.tar.bz2
// board is the name of the board, i.e. zork
type gcsMatcher func(ctx context.Context, bucket, branchPrefix, release, version, suffix, board string) (string, error)

func firmwareBranchUrl(ctx context.Context, specs *FirmwareSpecs, board string) (string, error) {
	build, ok := specs.FirmwareBuilds[board]
	if ok {
		url, err := url.JoinPath(build.ArtifactLink, build.FirmwareByBoard)
		if err != nil {
			return "", err
		}
		return url, nil
	}
	log.Printf("No branch build for board %q", board)
	return "", nil
}

func osBranchUrl(ctx context.Context, specs *FirmwareSpecs, swarmingDef *api.SwarmingDefinition) (string, error) {
	if len(swarmingDef.GetProvisionInfo()) > 0 {
		osPath := swarmingDef.GetProvisionInfo()[0].GetInstallRequest()
		url, err := url.JoinPath(osPath.GetImagePath().GetPath(), "/firmware_from_source.tar.bz2")
		if err != nil {
			return "", err
		}
		return url, nil
	}
	return "", fmt.Errorf("no install request")
}

func resolveSpec(ctx context.Context, spec string, specs *FirmwareSpecs, swarmingDef *api.SwarmingDefinition, log *log.Logger, matcher gcsMatcher) (string, error) {
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
	if board == "nissa" {
		// Trulo is still an active program, so this will get out of date.
		// https://chromeos.google.com/partner/dlm/device/list?q=referenceDesign:trulo
		switch dutModel.GetModelName() {
		case "kaladin", "kelsier", "pujjocento", "pujjolo", "pujjoquince", "pujjoteenlo", "uldrenite", "uldrenite360":
			board = "trulo"
		}
	}
	for _, spec := range strings.Split(spec, ",") {
		if spec == LatestFirmwareBranch {
			url, err := firmwareBranchUrl(ctx, specs, board)
			if url != "" || err != nil {
				return url, err
			}
		} else if spec == OSSource {
			return osBranchUrl(ctx, specs, swarmingDef)
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
		} else if versionNumbnerRe.MatchString(spec) {
			// Try the cache
			if specs.VersionCache == nil {
				specs.VersionCache = make(map[string]string)
			}
			url, ok := specs.VersionCache[spec]
			if ok {
				return url, nil
			}
			// Try firmware branch (including stabilization branches)
			url, err := firmwareBranchUrl(ctx, specs, board)
			if err != nil {
				return "", err
			}
			if url != "" {
				groups := firmwareArtifactRe.FindStringSubmatch(url)
				if groups != nil {
					url, err = matcher(ctx /* bucket */, groups[1] /* prefix */, groups[2] /* release */, groups[3], spec /* suffix */, groups[4], board)
					if err != nil {
						return "", err
					}
					specs.VersionCache[spec] = url
					return url, nil
				}
			}
			return "", fmt.Errorf("failed to find path for %q (no fw branch for %s)", spec, board)
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
	ctx context.Context,
	lookup map[string]string,
	swarmingDef *api.SwarmingDefinition,
	dynamicHelper *DynamicFirmwareProvisionHelper,
	specs *FirmwareSpecs,
	log *log.Logger,
	matcher gcsMatcher) error {

	switch swarmingDef.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		lookupValues := &FirmwareProvisionLookupValues{}
		var err error

		lookupValues.Ro, err = resolveSpec(ctx, specs.Ro, specs, swarmingDef, log, matcher)
		if err != nil {
			return err
		}
		lookupValues.Rw, err = resolveSpec(ctx, specs.Rw, specs, swarmingDef, log, matcher)
		if err != nil {
			return err
		}
		lookupValues.ECRO, err = resolveSpec(ctx, specs.ECRO, specs, swarmingDef, log, matcher)
		if err != nil {
			return err
		}
		lookupValues.ECRW, err = resolveSpec(ctx, specs.ECRW, specs, swarmingDef, log, matcher)
		if err != nil {
			return err
		}

		dynamicHelper.ApplyFirmwareProvisionToLookup(lookup, lookupValues)
	}
	return nil
}

// addFwPathsToTestArgs updates test args.
func addFwPathsToTestArgs(
	ctx context.Context,
	executionMetadata *api.ExecutionMetadata,
	swarmingDef *api.SwarmingDefinition,
	specs *FirmwareSpecs,
	log *log.Logger,
	matcher gcsMatcher) error {

	for key, val := range specs.TestArgReplacements {
		val, err := resolveSpec(ctx, val, specs, swarmingDef, log, matcher)
		if err != nil {
			return fmt.Errorf("failed to resolve test arg %s=%s: %w", key, val, err)
		}
		for i := 0; i < len(executionMetadata.Args); i++ {
			if executionMetadata.Args[i].GetFlag() == key {
				executionMetadata.Args = append(executionMetadata.Args[:i], executionMetadata.Args[i+1:]...)
				i--
			}
		}
		executionMetadata.Args = append(executionMetadata.Args, &api.Arg{Flag: key, Value: val})
	}
	return nil
}
