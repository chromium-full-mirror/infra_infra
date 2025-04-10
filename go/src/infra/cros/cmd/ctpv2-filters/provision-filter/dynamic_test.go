// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main_test

import (
	"fmt"
	"testing"

	storage_path "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	. "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	. "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/helpers"
	. "go.chromium.org/infra/cros/cmd/ctpv2-filters/provision-filter"
)

const (
	LegacyPrimary  = "drallion"
	MultiPrimary   = "dedede"
	MultiCompanion = "zork"
	MockRNum       = "R123.0.0"
)

func TestDynamic(t *testing.T) {
	ftt.Run("Legacy", t, func(t *ftt.Test) {
		req := getMockInternalTestPlan(getMockTargetRequirements(3), nil)

		err := GenerateDynamicInfo(req)
		assert.Loosely(t, err, should.BeNil)

		dynamicUpdates := req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates()
		assert.Loosely(t, dynamicUpdates, should.HaveLength(1))
		validateProvisionRequest(t, dynamicUpdates[0], NewPrimaryDeviceIdentifier().Id, InstallPath.AsPlaceholder())

		targetReqs := req.GetSuiteInfo().GetSuiteMetadata().GetTargetRequirements()
		assert.Loosely(t, targetReqs, should.HaveLength(3))
		for _, targetReq := range targetReqs {
			hwDef := targetReq.GetHwRequirements().GetHwDefinition()
			assert.Loosely(t, hwDef, should.HaveLength(1))
			lookup := hwDef[0].DynamicUpdateLookupTable
			validateLookupTable(t, lookup, LegacyPrimary, 0)
		}
	})

	ftt.Run("Single dut (non-legacy)", t, func(t *ftt.Test) {
		req := getMockInternalTestPlan(nil, getMockSchedulingUnits(3, 0))

		err := GenerateDynamicInfo(req)
		assert.Loosely(t, err, should.BeNil)

		dynamicUpdates := req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates()
		assert.Loosely(t, dynamicUpdates, should.HaveLength(1))
		validateProvisionRequest(t, dynamicUpdates[0], NewPrimaryDeviceIdentifier().Id, InstallPath.AsPlaceholder())

		units := req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits()
		assert.Loosely(t, units, should.HaveLength(3))
		for _, unit := range units {
			lookup := unit.DynamicUpdateLookupTable
			validateLookupTable(t, lookup, MultiPrimary, 0)

			assert.Loosely(t, unit.CompanionTargets, should.HaveLength(0))
		}
	})

	ftt.Run("Multi dut", t, func(t *ftt.Test) {
		req := getMockInternalTestPlan(nil, getMockSchedulingUnits(3, 2))

		err := GenerateDynamicInfo(req)
		assert.Loosely(t, err, should.BeNil)

		dynamicUpdates := req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates()
		assert.Loosely(t, dynamicUpdates, should.HaveLength(3))
		validateProvisionRequest(t, dynamicUpdates[0], NewPrimaryDeviceIdentifier().Id, InstallPath.AsPlaceholder())
		validateProvisionRequest(t, dynamicUpdates[1], NewCompanionDeviceIdentifier(Board.WithIndex(1).AsPlaceholder()).Id, InstallPath.WithIndex(1).AsPlaceholder())
		validateProvisionRequest(t, dynamicUpdates[2], NewCompanionDeviceIdentifier(Board.WithIndex(2).AsPlaceholder()).Id, InstallPath.WithIndex(2).AsPlaceholder())

		units := req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits()
		assert.Loosely(t, units, should.HaveLength(3))
		for _, unit := range units {
			lookup := unit.DynamicUpdateLookupTable
			validateLookupTable(t, lookup, MultiPrimary, 0)

			assert.Loosely(t, unit.CompanionTargets, should.HaveLength(2))
			for i := range unit.CompanionTargets {
				board := fmt.Sprintf("%s_%d", MultiCompanion, i)
				validateLookupTable(t, lookup, board, i+1)
			}
		}
	})
}

func validateLookupTable(t testing.TB, lookup map[string]string, board string, idx int) {
	t.Helper()
	assert.Loosely(t, lookup[Board.WithIndex(idx).AsKey()], should.Equal(board), truth.LineContext())
	assert.Loosely(t, lookup[InstallPath.WithIndex(idx).AsKey()], should.Equal(board+MockRNum), truth.LineContext())
}

func validateProvisionRequest(t testing.TB, update *api.UserDefinedDynamicUpdate, expectedDeviceId, expectedInstallPath string) {
	t.Helper()

	deviceId := DeviceIdentifierFromString(expectedDeviceId)

	insert := update.GetUpdateAction().GetInsert()
	assert.Loosely(t, insert, should.NotBeNil, truth.LineContext())
	assert.Loosely(t, insert.GetTask(), should.NotBeNil, truth.LineContext())

	provisionTask := insert.GetTask().GetProvision()
	assert.Loosely(t, provisionTask, should.NotBeNil, truth.LineContext())
	assert.Loosely(t, provisionTask.Target, should.Equal(expectedDeviceId), truth.LineContext())

	// Check DynamicDeps.
	deps := provisionTask.GetDynamicDeps()
	assert.Loosely(t, deps, should.NotBeNil, truth.LineContext())
	validateDependencyKeyValue(t, deps, "startupRequest.dut", deviceId.GetDevice("dut"))
	validateDependencyKeyValue(t, deps, "startupRequest.dutServer", deviceId.GetCrosDutServer())

	// Check InstallRequest.
	installRequest := provisionTask.GetInstallRequest()
	assert.Loosely(t, installRequest, should.NotBeNil, truth.LineContext())

	imagePath := installRequest.GetImagePath()
	assert.Loosely(t, imagePath, should.NotBeNil, truth.LineContext())
	assert.Loosely(t, imagePath.Path, should.Equal(expectedInstallPath), truth.LineContext())

	// Check containers.
	containers := insert.GetTask().GetOrderedContainerRequests()
	assert.Loosely(t, containers, should.HaveLength(3), truth.LineContext())
}

func validateDependencyKeyValue(t testing.TB, deps []*api.DynamicDep, key, expectedValue string) {
	t.Helper()

	found := false
	for _, dep := range deps {
		if dep.Key == key {
			found = true
			assert.Loosely(t, dep.Value, should.Equal(expectedValue), truth.LineContext())
			break
		}
	}

	assert.Loosely(t, found, should.BeTrue, truth.LineContext())
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

	for range count {
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

	for i := range companionCount {
		board := fmt.Sprintf("%s_%d", MultiCompanion, i)
		unit.CompanionTargets = append(unit.CompanionTargets, &api.Target{
			SwarmingDef: getMockSwarmingDefinition(board),
		})
	}

	return unit
}

func getMockTargetRequirements(count int) []*api.TargetRequirements {
	requirements := []*api.TargetRequirements{}

	for range count {
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
