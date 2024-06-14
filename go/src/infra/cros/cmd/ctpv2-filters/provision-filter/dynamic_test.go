// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main_test

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	storage_path "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	. "go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/common"
	. "go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/helpers"

	. "infra/cros/cmd/ctpv2-filters/provision-filter"
)

const (
	LegacyPrimary  = "drallion"
	MultiPrimary   = "dedede"
	MultiCompanion = "zork"
	MockRNum       = "R123.0.0"
)

func TestDynamic(t *testing.T) {
	Convey("Legacy", t, func() {
		req := getMockInternalTestPlan(getMockTargetRequirements(3), nil)

		err := GenerateDynamicInfo(req)
		So(err, ShouldBeNil)

		dynamicUpdates := req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates()
		So(dynamicUpdates, ShouldHaveLength, 1)
		validateProvisionRequest(dynamicUpdates[0], NewPrimaryDeviceIdentifier().Id, InstallPath.AsPlaceholder())

		targetReqs := req.GetSuiteInfo().GetSuiteMetadata().GetTargetRequirements()
		So(targetReqs, ShouldHaveLength, 3)
		for _, targetReq := range targetReqs {
			hwDef := targetReq.GetHwRequirements().GetHwDefinition()
			So(hwDef, ShouldHaveLength, 1)
			lookup := hwDef[0].DynamicUpdateLookupTable
			validateLookupTable(lookup, LegacyPrimary, 0)
		}
	})

	Convey("Single dut (non-legacy)", t, func() {
		req := getMockInternalTestPlan(nil, getMockSchedulingUnits(3, 0))

		err := GenerateDynamicInfo(req)
		So(err, ShouldBeNil)

		dynamicUpdates := req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates()
		So(dynamicUpdates, ShouldHaveLength, 1)
		validateProvisionRequest(dynamicUpdates[0], NewPrimaryDeviceIdentifier().Id, InstallPath.AsPlaceholder())

		units := req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits()
		So(units, ShouldHaveLength, 3)
		for _, unit := range units {
			lookup := unit.DynamicUpdateLookupTable
			validateLookupTable(lookup, MultiPrimary, 0)

			So(unit.CompanionTargets, ShouldHaveLength, 0)
		}
	})

	Convey("Multi dut", t, func() {
		req := getMockInternalTestPlan(nil, getMockSchedulingUnits(3, 2))

		err := GenerateDynamicInfo(req)
		So(err, ShouldBeNil)

		dynamicUpdates := req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates()
		So(dynamicUpdates, ShouldHaveLength, 3)
		validateProvisionRequest(dynamicUpdates[0], NewPrimaryDeviceIdentifier().Id, InstallPath.AsPlaceholder())
		validateProvisionRequest(dynamicUpdates[1], NewCompanionDeviceIdentifier(Board.WithIndex(1).AsPlaceholder()).Id, InstallPath.WithIndex(1).AsPlaceholder())
		validateProvisionRequest(dynamicUpdates[2], NewCompanionDeviceIdentifier(Board.WithIndex(2).AsPlaceholder()).Id, InstallPath.WithIndex(2).AsPlaceholder())

		units := req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits()
		So(units, ShouldHaveLength, 3)
		for _, unit := range units {
			lookup := unit.DynamicUpdateLookupTable
			validateLookupTable(lookup, MultiPrimary, 0)

			So(unit.CompanionTargets, ShouldHaveLength, 2)
			for i := range unit.CompanionTargets {
				board := fmt.Sprintf("%s_%d", MultiCompanion, i)
				validateLookupTable(lookup, board, i+1)
			}
		}
	})
}

func validateLookupTable(lookup map[string]string, board string, idx int) {
	So(lookup[Board.WithIndex(idx).AsKey()], ShouldEqual, board)
	So(lookup[InstallPath.WithIndex(idx).AsKey()], ShouldEqual, board+MockRNum)
}

func validateProvisionRequest(update *api.UserDefinedDynamicUpdate, expectedDeviceId, expectedInstallPath string) {
	deviceId := DeviceIdentifierFromString(expectedDeviceId)

	insert := update.GetUpdateAction().GetInsert()
	So(insert, ShouldNotBeNil)
	So(insert.GetTask(), ShouldNotBeNil)

	provisionTask := insert.GetTask().GetProvision()
	So(provisionTask, ShouldNotBeNil)
	So(provisionTask.Target, ShouldEqual, expectedDeviceId)

	// Check DynamicDeps.
	deps := provisionTask.GetDynamicDeps()
	So(deps, ShouldNotBeNil)
	validateDependencyKeyValue(deps, "startupRequest.dut", deviceId.GetDevice("dut"))
	validateDependencyKeyValue(deps, "startupRequest.dutServer", deviceId.GetCrosDutServer())

	// Check InstallRequest.
	installRequest := provisionTask.GetInstallRequest()
	So(installRequest, ShouldNotBeNil)

	imagePath := installRequest.GetImagePath()
	So(imagePath, ShouldNotBeNil)
	So(imagePath.Path, ShouldEqual, expectedInstallPath)

	// Check containers.
	containers := insert.GetTask().GetOrderedContainerRequests()
	So(containers, ShouldHaveLength, 2)
}

func validateDependencyKeyValue(deps []*api.DynamicDep, key, expectedValue string) {
	found := false
	for _, dep := range deps {
		if dep.Key == key {
			found = true
			So(dep.Value, ShouldEqual, expectedValue)
			break
		}
	}

	So(found, ShouldBeTrue)
}

func getMockInternalTestPlan(targetRequirements []*api.TargetRequirements, schedulingUnits []*api.SchedulingUnit) *api.InternalTestplan {
	return &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				TargetRequirements: targetRequirements,
				SchedulingUnits:    schedulingUnits,
				DynamicUpdates:     []*api.UserDefinedDynamicUpdate{},
			},
		},
	}
}

func getMockSchedulingUnits(count, companionCount int) []*api.SchedulingUnit {
	units := []*api.SchedulingUnit{}

	for i := 0; i < count; i++ {
		units = append(units, getMockSchedulingUnit(companionCount))
	}

	return units
}

func getMockSchedulingUnit(companionCount int) *api.SchedulingUnit {
	unit := &api.SchedulingUnit{
		DynamicUpdateLookupTable: map[string]string{},
		PrimaryTarget: &api.Target{
			SwarmingDef: getMockSwarmingDefinition(MultiPrimary),
		},
		CompanionTargets: []*api.Target{},
	}

	for i := 0; i < companionCount; i++ {
		board := fmt.Sprintf("%s_%d", MultiCompanion, i)
		unit.CompanionTargets = append(unit.CompanionTargets, &api.Target{
			SwarmingDef: getMockSwarmingDefinition(board),
		})
	}

	return unit
}

func getMockTargetRequirements(count int) []*api.TargetRequirements {
	requirements := []*api.TargetRequirements{}

	for i := 0; i < count; i++ {
		requirements = append(requirements, &api.TargetRequirements{
			HwRequirements: &api.HWRequirements{
				HwDefinition: []*api.SwarmingDefinition{
					getMockSwarmingDefinition(LegacyPrimary),
				},
			},
		})
	}

	return requirements
}

func getMockSwarmingDefinition(board string) *api.SwarmingDefinition {
	return &api.SwarmingDefinition{
		DynamicUpdateLookupTable: map[string]string{},
		ProvisionInfo:            getMockProvisionInfo(board),
		DutInfo: &dut_api.Dut{
			DutType: &dut_api.Dut_Chromeos{
				Chromeos: &dut_api.Dut_ChromeOS{
					DutModel: &dut_api.DutModel{
						BuildTarget: board,
					},
				},
			},
		},
	}
}

func getMockProvisionInfo(board string) []*api.ProvisionInfo {
	return []*api.ProvisionInfo{
		{
			InstallRequest: &api.InstallRequest{
				ImagePath: &storage_path.StoragePath{
					HostType: storage_path.StoragePath_GS,
					Path:     board + MockRNum,
				},
			},
		},
	}
}
