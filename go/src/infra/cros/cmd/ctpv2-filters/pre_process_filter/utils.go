// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"errors"
	"log"
	"regexp"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/pre_process_filter/interfaces"
)

// BoardTestInfo holds list of tests and milestone for a given board.
type BoardTestInfo struct {
	tests     []string
	milestone string
	variant   string
}

// testMDToSet returns a set of test case names froma Test suite if type TestCaseMetadata.
func testMDToSet(op *api.TestSuite_TestCasesMetadata) map[string]struct{} {
	tcList := make(map[string]struct{})
	mdl := op.TestCasesMetadata.Values
	for _, tc := range mdl {
		value := tc.TestCase.Id.Value
		tcList[value] = struct{}{}
	}

	return tcList
}

// inverseRemoveBoardTestMap inverts the original map of boards to tests
// into a map of tests to boards.
func inverseRemoveBoardTestMap(removeBoardTestMap map[string][]string) map[string]map[string]struct{} {
	// Create the inverse map: tests as keys, boards as values
	removeTestToBoardsMap := make(map[string]map[string]struct{})

	// Iterate over the original map
	for board, tests := range removeBoardTestMap {
		for _, test := range tests {
			// If the test is not in the inverse map, initialize its set
			if _, exists := removeTestToBoardsMap[test]; !exists {
				removeTestToBoardsMap[test] = make(map[string]struct{})
			}
			// Add the board to the test's set in the new map
			removeTestToBoardsMap[test][board] = struct{}{}
		}
	}

	return removeTestToBoardsMap
}

// fetchPolicy fetches default policy.
func fetchPolicy() *api.FilterFlakyRequest_PassRatePolicy {
	return &api.FilterFlakyRequest_PassRatePolicy{
		PassRatePolicy: &api.PassRatePolicy{
			PassRate:           99,
			MinRuns:            5,
			NumOfMilestones:    1,
			MaxFilteredPercent: 50,
		},
	}
}

// makeSuite returns a TestSuite object.
func makeSuite(tests []string) *api.TestSuite {
	testInfos := []*api.TestCase{}
	for _, md := range tests {
		testInfos = append(testInfos, &api.TestCase{
			Id: &api.TestCase_Id{Value: md},
		})
	}
	return &api.TestSuite{
		Spec: &api.TestSuite_TestCases{
			TestCases: &api.TestCaseList{TestCases: testInfos},
		},
	}

}

// testCasesToSet converts a list ofTestCase to a set of test case names.
func testCasesToSet(testcases []*api.TestCase) map[string]struct{} {
	tcList := make(map[string]struct{})
	for _, tc := range testcases {
		value := tc.Id.Value
		tcList[value] = struct{}{}
	}
	return tcList
}

// dutModelFromDut returns the dut model for a DUT.
func dutModelFromDut(dut *labapi.Dut) *labapi.DutModel {
	if dut == nil {
		return nil
	}

	switch hw := dut.GetDutType().(type) {
	case *labapi.Dut_Chromeos:
		return hw.Chromeos.GetDutModel()
	case *labapi.Dut_Android_:
		return hw.Android.GetDutModel()
	case *labapi.Dut_Devboard_:
		return hw.Devboard.GetDutModel()
	}
	return nil
}

// getBoard returns board value for a scheduling unit.
func getBoard(unit *api.SchedulingUnit) string {
	return dutModelFromDut(unit.GetPrimaryTarget().GetSwarmingDef().GetDutInfo()).GetBuildTarget()
}

// getBoardWVariant returns board value for a scheduling unit w/ its variant (if applicable).
func getBoardWVariant(unit *api.SchedulingUnit) string {
	board := dutModelFromDut(unit.GetPrimaryTarget().GetSwarmingDef().GetDutInfo()).GetBuildTarget()
	variant := unit.GetPrimaryTarget().GetSwarmingDef().GetVariant()
	if variant != "" {
		return board + "-" + variant
	}
	return board

}

// getMilestone returns the image milestone for a scheduling unit.
func getMilestone(installPath string) (string, error) {
	if installPath == "" {
		return "", errors.New("empty Installpath")
	}
	// Define a regex pattern to match "R" followed by digits and a hyphen
	re := regexp.MustCompile(`R(\d+)-`)

	match := re.FindStringSubmatch(installPath)
	if len(match) < 2 {
		return "", errors.New("milestone not found in installPath")
	}

	milestone := match[1]
	return milestone, nil
}

// createBoardTestMap creates a map of board and BoardTestInfo.
func createBoardTestMap(req *api.InternalTestplan, log *log.Logger) (map[string]*BoardTestInfo, error) {
	boardTestMap := make(map[string]*BoardTestInfo)
	for _, tc := range req.TestCases {
		for _, units := range tc.GetSchedulingUnitOptions() {
			for _, unit := range units.GetSchedulingUnits() {
				board := getBoard(unit)
				variant := getBoardWVariant(unit)
				milestone, err := getMilestone(unit.GetDynamicUpdateLookupTable()["installPath"])
				if err != nil {
					log.Printf("skipping as scheduling unit doesn't have install path")
					continue
				}
				if _, ok := boardTestMap[board]; !ok {
					// If the board does not exist, create a new BoardTestInfo and add it to the map
					boardTestMap[board] = &BoardTestInfo{
						tests:     []string{tc.Name},
						milestone: milestone,
						variant:   variant,
					}
				} else {
					boardTestMap[board].tests = append(boardTestMap[board].tests, tc.Name)
				}
			}
		}
	}
	return boardTestMap, nil
}

func flakeFilteringLogAndResults(rspn *api.FilterFlakyResponse, board string, req *api.FilterFlakyRequest, filter *Filter, log *log.Logger) {
	log.Printf("***********Flake filtering for board %s start***********\n", board)
	if len(rspn.RemovedTests) > 0 {

		log.Printf("The following tests are set to be removed from this scheduled attempt for all models under board: %s\n", board)
		for _, test := range rspn.RemovedTests {
			log.Printf(test + "\n")
		}
	} else {
		log.Printf("No tests to be removed from this scheduled attempt for any models under board: %s\n", board)
	}
	log.Printf("***********Flake filtering for board %s end***********\n", board)

	err := interfaces.WriteResults(rspn.RemovedTests, req, filter.data, log)
	if err != nil {

		log.Println("!!!")
		log.Println(err)
	}
}

func prettyLogInternalTestplanDetail(req *api.InternalTestplan, log *log.Logger) {
	log.Printf("************Internal Test plan Detail Start************")
	for _, testCase := range req.GetTestCases() {
		// map for storing all models for all boards on which test is scheduled
		boardModelMap := make(map[string][]string)
		totalSchedulingUnits := 0
		for _, suo := range testCase.GetSchedulingUnitOptions() {
			for _, unit := range suo.GetSchedulingUnits() {
				totalSchedulingUnits = totalSchedulingUnits + 1
				dut := dutModelFromDut(unit.GetPrimaryTarget().GetSwarmingDef().GetDutInfo())
				board := dut.GetBuildTarget()
				model := dut.GetModelName()
				boardModelMap[board] = append(boardModelMap[board], model)
			}
		}
		log.Printf("Test %s scheduled on total %d scheduling units\n", testCase.GetName(), totalSchedulingUnits)
		for board, modelList := range boardModelMap {
			for _, model := range modelList {
				log.Printf("Test %s scheduled on board %s for model %s\n", testCase.GetName(), board, model)
			}
		}
	}
	log.Printf("************Internal Test plan Detail End************")
}

func isCqFlow(req *api.InternalTestplan, log *log.Logger) bool {
	for _, tc := range req.GetTestCases() {
		for _, schedulingUnitOptions := range tc.GetSchedulingUnitOptions() {
			for _, schedulingUnit := range schedulingUnitOptions.GetSchedulingUnits() {
				installPath, exists := schedulingUnit.GetDynamicUpdateLookupTable()["installPath"]
				if !exists {
					log.Printf("Scheduling unit doesn't have install path")
					return false
				}
				if strings.Contains(installPath, "-cq/") {
					return true
				}
			}
		}
	}
	return false
}
