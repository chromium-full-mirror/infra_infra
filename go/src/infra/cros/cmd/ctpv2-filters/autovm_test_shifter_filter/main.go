// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	goconfig "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

const (
	binName         = "autovm_test_shifter_filter"
	installPath     = "installPath"
	hwAgnostic      = "hw_agnostic"
	release         = "release/"
	gsBucketPath    = "gs://chromeos-image-archive/"
	buildReportJSON = "build_report.json"
	success         = "SUCCESS"
)

var vmBoards = []string{"betty", "reven-vmtest", "amd64-generic"}

// isTestVMCompatible returns true if the test can be safely executed on a VM.
func isTestVMCompatible(testCase *api.CTPTestCase) bool {
	compatibilityCriteria := []func(*api.CTPTestCase) bool{
		isHwAgnostic,
		// Add more criteria functions here in the future
	}

	for _, criterion := range compatibilityCriteria {
		if criterion(testCase) {
			return true
		}
	}
	return false
}

// isHwAgnostic checks if the test case has the hwAgnostic tag.
func isHwAgnostic(testCase *api.CTPTestCase) bool {
	if testCase == nil {
		return false
	}
	if testCase.GetMetadata() == nil {
		return false
	}
	if testCase.GetMetadata().GetTestCaseInfo() == nil {
		return false
	}
	if testCase.GetMetadata().GetTestCaseInfo().GetHwAgnostic() == nil {
		return false
	}
	return testCase.GetMetadata().GetTestCaseInfo().GetHwAgnostic().GetValue()
}

func generateVMSchedulingUnit(board string, version string) *api.SchedulingUnit {
	return &api.SchedulingUnit{
		PrimaryTarget: &api.Target{
			SwarmingDef: &api.SwarmingDefinition{
				DutInfo: &testapi.Dut{
					DutType: &testapi.Dut_Chromeos{
						Chromeos: &testapi.Dut_ChromeOS{
							DutModel: &testapi.DutModel{BuildTarget: board},
						},
					},
				},
				ProvisionInfo: []*api.ProvisionInfo{
					{
						InstallRequest: &api.InstallRequest{
							ImagePath: &goconfig.StoragePath{
								HostType: goconfig.StoragePath_GS,
								Path:     "gs://chromeos-image-archive/" + board + "-" + release + version,
							},
						},
					},
				},
			}},
		DynamicUpdateLookupTable: map[string]string{
			"board":       board,
			"installPath": "gs://chromeos-image-archive/" + board + "-" + release + version,
		},
	}
}

// updateSchedulingUnitOption updates the list of Scheduling units. Returns nil if no VMlab Scheduling unit found.
func updateSchedulingUnitOption(schedulingUnitOption *api.SchedulingUnitOptions, board string, version string) *api.SchedulingUnitOptions {
	var filteredSchedulingUnits []*api.SchedulingUnit
	for _, schedulingUnit := range schedulingUnitOption.GetSchedulingUnits() {
		// if VM board in scheduling unit then do not remove. This will trigger VMlab flow
		if checkBoardInVMBoardsList(getBoard(schedulingUnit)) {
			filteredSchedulingUnits = append(filteredSchedulingUnits, schedulingUnit)
		}
	}
	if len(filteredSchedulingUnits) == 0 {
		return nil
	}
	schedulingUnitOption.SchedulingUnits = filteredSchedulingUnits
	return schedulingUnitOption
}

// updateSchedulingUnitOptions updates the schedulingUnitOptions for each test case in internal test plan request.
func updateSchedulingUnitOptions(req *api.InternalTestplan, board string, version string, log *log.Logger) error {

	for _, testCase := range req.GetTestCases() {
		if isTestVMCompatible(testCase) {
			log.Printf("Updating Scheduling units for test : %s", testCase.GetName())
			var updatedSchedulingUnitOptions []*api.SchedulingUnitOptions
			for _, schedulingUnitOption := range testCase.GetSchedulingUnitOptions() {
				updatedSchedulingUnitOption := updateSchedulingUnitOption(schedulingUnitOption, board, version)
				if updatedSchedulingUnitOption == nil {
					continue
				}
				updatedSchedulingUnitOptions = append(updatedSchedulingUnitOptions, updatedSchedulingUnitOption)
			}
			// if no schedulign unit found from intended scheduling units, then create a VM schedulign unit with available board/version
			if len(updatedSchedulingUnitOptions) == 0 {
				log.Printf("Generating VM Scheduling unit for test : %s", testCase.GetName())
				testCase.SchedulingUnitOptions = []*api.SchedulingUnitOptions{
					{
						SchedulingUnits: []*api.SchedulingUnit{
							generateVMSchedulingUnit(board, version),
						},
					},
				}
			} else {
				testCase.SchedulingUnitOptions = updatedSchedulingUnitOptions
			}

		} else {
			log.Printf("Skippping as test : %s, isn't VM compatible", testCase.GetName())
		}
	}

	return nil
}

type AutoVMTestShifterFilter struct {
	servertemplate.Filter
}

func (*AutoVMTestShifterFilter) Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {

	available, board, version := isAnyVMImageAvailable(context.Background(), req, log)

	if available {
		// iterates each test case and updates the scheduling units based on if test is hw agnostic.
		err := updateSchedulingUnitOptions(req, board, version, log)
		if err != nil {
			return nil, fmt.Errorf("error during AutoVM test shifter filter execution: %w", err)
		}
	} else {
		log.Printf("Skipping AutoVM test shifter filter execution. VM Image is not available.")
	}

	return req, nil
}

func NewFilter() servertemplate.Filter {
	return &AutoVMTestShifterFilter{}
}

func main() {
	err := servertemplate.Server(NewFilter, binName)
	if err != nil {
		os.Exit(2)
	}

	os.Exit(0)
}
