// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"log"
	"os"
	"strings"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

const (
	address       = "localhost"
	binName       = "legacy_hw_filter"
	TrueStringVal = "True"
)

// hwTargetsFromReq returns the list of hwTargets from the suiteMetadata definition.
func hwTargetsFromReq(req *api.InternalTestplan) (hwTargets []*api.SwarmingDefinition) {
	for _, target := range req.GetSuiteInfo().GetSuiteMetadata().GetTargetRequirements() {
		// Skip optional; as they are only there for informational purposes.
		if target.GetHwRequirements().GetState() == api.HWRequirements_OPTIONAL {
			continue
		}
		hwTargets = append(hwTargets, target.GetHwRequirements().GetHwDefinition()...)
	}
	return hwTargets

}

// tcDeps returns a list of tc deps as a string.
func tcDeps(tc *api.CTPTestCase) (deps []string) {
	for _, dep := range tc.GetMetadata().GetTestCase().GetDependencies() {
		deps = append(deps, dep.GetValue())
	}
	return deps
}

// populatePrimaryTargetInSchedUnit populates primary target info to provided scheduling unit.
func populatePrimaryTargetInSchedUnit(schedUnit *api.SchedulingUnit, swarmingLabels []string) {
	if len(schedUnit.GetPrimaryTarget().GetSwarmingDef().GetSwarmingLabels()) > 0 {
		schedUnit.PrimaryTarget.SwarmingDef.SwarmingLabels = append(schedUnit.PrimaryTarget.SwarmingDef.SwarmingLabels, swarmingLabels...)
	} else {
		schedUnit.PrimaryTarget.SwarmingDef.SwarmingLabels = swarmingLabels
	}

	if schedUnit.GetPrimaryTarget().GetSwarmingDef().GetProvisionInfo() != nil {
		schedUnit.PrimaryTarget.SwarmingDef.ProvisionInfo = []*api.ProvisionInfo{}
	}
	if schedUnit.GetPrimaryTarget().GetSwReq() != nil {
		schedUnit.PrimaryTarget.SwReq = &api.LegacySW{}
	}
}

// generateTcHwReqs will return a list that represents all HW the test
// is to be scheduled on. Each testcase will need its own unique list;
// which will later sharded/organized based on HW/SW requirements.
func generateTcHwReqs(hwTargets []*api.SchedulingUnitOptions, tcDeps []string) (tcHwReqs []*api.SchedulingUnitOptions) {
	for _, target := range hwTargets {
		// We need to clone the target as different tests can set different hw requirements.
		// If we just set the field directly then it would change for all tests every time we change
		// the field.
		optionTarget := proto.Clone(target).(*api.SchedulingUnitOptions)
		swarmingLabels := tcDepsToSwarmingLabels(tcDeps)
		schedUnits := []*api.SchedulingUnit{}

		// If labels are already added by the request, respect by appending the TC given labels.
		// Otherwise, just the TC given labels.
		for _, t := range optionTarget.GetSchedulingUnits() {
			populatePrimaryTargetInSchedUnit(t, swarmingLabels)

			schedUnits = append(schedUnits, t)
		}

		targ := &api.SchedulingUnitOptions{
			// For legacy solver, each SchedulingUnitOption is a list of 1
			// This is because for a test dut matching will follow this logic:
			// SchedulingUnitOptions = [[a], [b]]; must run the test on a AND b
			// SchedulingUnitOptions = [[a, b], [c]]; must run on A OR B, AND C
			// Legacy always lists of 1 item. Others, eg 3D; will give OR options.
			SchedulingUnits: schedUnits,
			State:           api.SchedulingUnitOptions_ONEOF,
		}
		tcHwReqs = append(tcHwReqs, targ)
	}

	return tcHwReqs
}

// generateTcHwReqs will return a list that represents all HW the test
// is to be scheduled on. Each testcase will need its own unique list;
// which will later sharded/organized based on HW/SW requirements.
func generateTcHwReqsLegacy(hwTargets []*api.SchedulingUnit, tcDeps []string) (tcHwReqs []*api.SchedulingUnitOptions) {

	for _, target := range hwTargets {
		// We need to clone the target as different tests can set different hw requirements.
		// If we just set the field directly then it would change for all tests every time we change
		// the field.
		t := proto.Clone(target).(*api.SchedulingUnit)
		swarmingLabels := tcDepsToSwarmingLabels(tcDeps)

		// If labels are already added by the request, respect by appending the TC given labels.
		// Otherwise, just the TC given labels.
		populatePrimaryTargetInSchedUnit(t, swarmingLabels)

		targ := &api.SchedulingUnitOptions{
			// For legacy solver, each SchedulingUnitOption is a list of 1
			// This is because for a test dut matching will follow this logic:
			// SchedulingUnitOptions = [[a], [b]]; must run the test on a AND b
			// SchedulingUnitOptions = [[a, b], [c]]; must run on A OR B, AND C
			// Legacy always lists of 1 item. Others, eg 3D; will give OR options.
			SchedulingUnits: []*api.SchedulingUnit{t},
			State:           api.SchedulingUnitOptions_REQUIRED,
		}
		tcHwReqs = append(tcHwReqs, targ)
	}

	return tcHwReqs
}

func tcDepsToSwarmingLabels(tcDeps []string) []string {
	swarmingLabels := []string{}
	for _, dep := range tcDeps {
		label := dep
		if !strings.Contains(label, ":") {
			label = label + ":" + TrueStringVal
		}

		swarmingLabels = append(swarmingLabels, label)
	}

	return swarmingLabels
}

func executor(req *api.InternalTestplan, log *log.Logger, commonParams *servertemplate.CommonFilterParams) (*api.InternalTestplan, error) {
	// Step 1. Get all the HWTargets from the suite metadata
	// these might contain provision info.
	hwTargets := req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnitOptions()
	hwTargetsLegacy := req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits()
	hwTargetsLegacy2 := hwTargetsFromReq(req)

	// Step 2.  Iterate through the tests, adding the HW to each test
	// Include the deps from the test metadata into the HW.
	for _, tc := range req.GetTestCases() {
		tcDeps := tcDeps(tc)
		if len(hwTargets) != 0 {
			log.Println("Hitting new scheduleUnitOptions flow!")
			tc.SchedulingUnitOptions = generateTcHwReqs(hwTargets, tcDeps)
		} else if len(hwTargetsLegacy) != 0 {
			tc.SchedulingUnitOptions = generateTcHwReqsLegacy(hwTargetsLegacy, tcDeps)
		} else {
			tc.HwRequirements = generateTcHwReqsLegacy2(hwTargetsLegacy2, tcDeps)

		}
	}

	return req, nil
}

// generateTcHwReqsLegacy2 is the same as generateTcHwReqs; but using the legacy API
// to be deprecated soon.
func generateTcHwReqsLegacy2(hwTargets []*api.SwarmingDefinition, tcDeps []string) (tcHwReqs []*api.HWRequirements) {
	for _, target := range hwTargets {
		t := proto.Clone(target).(*api.SwarmingDefinition)
		swarmingLabels := tcDepsToSwarmingLabels(tcDeps)
		if len(t.SwarmingLabels) > 0 {
			t.SwarmingLabels = append(t.SwarmingLabels, swarmingLabels...)
		} else {
			t.SwarmingLabels = swarmingLabels
		}
		targ := &api.HWRequirements{
			HwDefinition: []*api.SwarmingDefinition{t},
			State:        api.HWRequirements_REQUIRED,
		}
		tcHwReqs = append(tcHwReqs, targ)
	}

	return tcHwReqs
}

func main() {
	err := servertemplate.Server(executor, binName)
	if err != nil {
		os.Exit(2)
	}

	os.Exit(0)
}
