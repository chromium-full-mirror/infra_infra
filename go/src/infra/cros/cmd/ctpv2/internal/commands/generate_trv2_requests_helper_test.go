// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	. "go.chromium.org/infra/cros/cmd/ctpv2/internal/commands"
)

func TestSchedulingMatch(t *testing.T) {
	metadataMap := fakeSchedulingUnitsMetadataMap()

	// Primary and companions with models.
	primary := FakeHwTarget("dedede", "model", "one")
	companions := []*HwTarget{
		FakeHwTarget("pixel6", "pixel6pro", ""),
		FakeHwTarget("pixel6", "pixel6pro", ""),
	}
	schedUnit := FindSchedulingUnit(primary, companions, metadataMap)
	if schedUnit == nil {
		t.Fatalf("Expected scheduling unit match, found none.")
	}
	if schedUnit.GetCompanionTargets()[0].GetSwarmingDef().GetDutInfo().GetChromeos().GetDutModel().GetModelName() != "pixel6pro" {
		t.Fatalf("Expected pixel6pro, got %s", schedUnit.GetCompanionTargets()[0].GetSwarmingDef().GetDutInfo().GetChromeos().GetDutModel().GetModelName())
	}
	if schedUnit.GetCompanionTargets()[1].GetSwarmingDef().GetDutInfo().GetChromeos().GetDutModel().GetModelName() != "" {
		t.Fatalf("Expected pixel6pro, got %s", schedUnit.GetCompanionTargets()[0].GetSwarmingDef().GetDutInfo().GetChromeos().GetDutModel().GetModelName())
	}

	// Primary and companions without models.
	primary = FakeHwTarget("dedede", "", "one")
	companions = []*HwTarget{
		FakeHwTarget("pixel6", "", ""),
	}
	schedUnit = FindSchedulingUnit(primary, companions, metadataMap)
	if schedUnit == nil {
		t.Fatalf("Expected scheduling unit match, found none.")
	}

	// Primary without variant.
	primary = FakeHwTarget("dedede", "", "")
	companions = []*HwTarget{
		FakeHwTarget("pixel6", "", ""),
	}
	schedUnit = FindSchedulingUnit(primary, companions, metadataMap)
	if schedUnit == nil {
		t.Fatalf("Expected scheduling unit match, found none.")
	}

	// Primary match, but companions do not.
	primary = FakeHwTarget("dedede", "", "one")
	companions = []*HwTarget{
		FakeHwTarget("pixel6", "pixel6pro", ""),
		FakeHwTarget("pixel6", "pixel6pro", ""),
		FakeHwTarget("pixel6", "", ""),
	}
	schedUnit = FindSchedulingUnit(primary, companions, metadataMap)
	if schedUnit != nil {
		t.Fatalf("Expected no scheduling unit match, found one.")
	}

	// Primary has no match.
	primary = FakeHwTarget("dedede", "", "two")
	companions = []*HwTarget{
		FakeHwTarget("pixel6", "pixel6pro", ""),
	}
	schedUnit = FindSchedulingUnit(primary, companions, metadataMap)
	if schedUnit != nil {
		t.Fatalf("Expected no scheduling unit match, found one.")
	}
}

func fakeSchedulingUnitsMetadataMap() map[string][]*api.SchedulingUnit {
	return map[string][]*api.SchedulingUnit{
		"dedede-one": {
			{
				PrimaryTarget: fakeAPITarget("dedede", "model", "one"),
				CompanionTargets: []*api.Target{
					fakeAPITarget("pixel6", "pixel6pro", ""),
					fakeAPITarget("pixel6", "", ""),
				},
			},
			{
				PrimaryTarget: fakeAPITarget("dedede", "model", "one"),
				CompanionTargets: []*api.Target{
					fakeAPITarget("pixel7", "pixel7pro", ""),
				},
			},
		},
		"dedede-": {
			{
				PrimaryTarget: fakeAPITarget("dedede", "model", ""),
				CompanionTargets: []*api.Target{
					fakeAPITarget("pixel6", "pixel6pro", ""),
				},
			},
			{
				PrimaryTarget: fakeAPITarget("dedede", "model", ""),
				CompanionTargets: []*api.Target{
					fakeAPITarget("pixel7", "pixel7pro", ""),
				},
			},
		},
	}
}

func fakeAPITarget(board, model, variant string) *api.Target {
	return &api.Target{
		SwarmingDef: &api.SwarmingDefinition{
			DutInfo: &labapi.Dut{
				DutType: &labapi.Dut_Chromeos{
					Chromeos: &labapi.Dut_ChromeOS{
						DutModel: &labapi.DutModel{
							ModelName:   model,
							BuildTarget: board,
						},
					},
				},
			},
			Variant: variant,
		},
	}
}

func TestGetBranchFromTestArg(t *testing.T) {
	testCases := []struct {
		name           string
		testArgs       []*api.Arg
		expectedBranch string
	}{
		{
			name: "valid branch in test args",
			testArgs: []*api.Arg{
				{Flag: "pool", Value: "chromeos-testing"},
				{Flag: "branch", Value: "release-R120-15662.B"},
			},
			expectedBranch: "release-R120-15662.B",
		},
		{
			name: "empty branch in test args",
			testArgs: []*api.Arg{
				{Flag: "branch", Value: ""},
			},
			expectedBranch: "",
		},
		{
			name:           "no branch in test args",
			testArgs:       []*api.Arg{},
			expectedBranch: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			branch := GetBranchFromTestArg(tc.testArgs)
			if branch != tc.expectedBranch {
				t.Errorf("getBranchFromTestArg(%v) returned unexpected branch: got %v, want %v", tc.testArgs, branch, tc.expectedBranch)
			}
		})
	}
}
