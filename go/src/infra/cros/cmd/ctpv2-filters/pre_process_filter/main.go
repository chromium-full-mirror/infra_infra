// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the pre-process-filter for finding tests based on tags.
package main

import (
	"fmt"
	"log"
	"os"

	"google.golang.org/api/option"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/pre_process_filter/interfaces"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/pre_process_filter/policies"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/pre_process_filter/structs"
)

const (
	binName = "flaky_filter"
	saFile  = "/creds/service_accounts/service-account-chromeos.json"
)

// getStabilityData gets the flaky tests data BQ FlakeCacheTest.
func getStabilityData(req *api.FilterFlakyRequest, tcList map[string]struct{}, log *log.Logger, clientOpts ...option.ClientOption) (map[string]structs.SignalFormat, error) {
	var data map[string]structs.SignalFormat

	// TODO: this datatype will need to evolve from a board string to something more complex.
	var variant string
	switch variantOp := req.Variant.(type) {
	case *api.FilterFlakyRequest_Board:
		variant = variantOp.Board
	}
	if variant == "" {
		log.Println("No Board Provided")
		return nil, fmt.Errorf("no board provided, cannot filter")
	}
	// Currently 2 types of policies will be supported. This can be expanded if newer types are added.
	switch op := req.Policy.(type) {
	case *api.FilterFlakyRequest_PassRatePolicy:
		return policies.StabilityFromPolicy(op, variant, req.Milestone, tcList, log, clientOpts...)

	}

	return data, nil
}

// updateSchedulingUnitOption updates the list of Scheduling units.
func updateSchedulingUnitOption(schedulingUnitOption *api.SchedulingUnitOptions, removeTestToBoardsMap map[string]struct{}, log *log.Logger) (*api.SchedulingUnitOptions, error) {
	var filteredSchedulingUnits []*api.SchedulingUnit
	for _, schedulingUnit := range schedulingUnitOption.GetSchedulingUnits() {
		board := getBoard(schedulingUnit)

		if _, ok := removeTestToBoardsMap[getBoard(schedulingUnit)]; ok {
			log.Printf("Board : %s removed", board)
		} else {
			filteredSchedulingUnits = append(filteredSchedulingUnits, schedulingUnit)

		}

	}
	if len(filteredSchedulingUnits) == 0 {
		return nil, nil
	}
	schedulingUnitOption.SchedulingUnits = filteredSchedulingUnits
	return schedulingUnitOption, nil
}

// filterSchedulingUnitOptionsBasedOnUseFlag returns a new updated schedulingUnitOptions object.
func filterSchedulingUnitOptionsBasedOnUseFlag(schedulingUnitOptions []*api.SchedulingUnitOptions, removeTestToBoardsMap map[string]struct{}, log *log.Logger) ([]*api.SchedulingUnitOptions, error) {

	var filteredSchedulingUnitOptions []*api.SchedulingUnitOptions
	for _, schedulingUnitOption := range schedulingUnitOptions {
		// scheduling units will be removed if no buildDeps in test metadata is present in scheduling unit use flag set in useFlagDict
		updatedSchedulingUnitOption, err := updateSchedulingUnitOption(schedulingUnitOption, removeTestToBoardsMap, log)
		if err != nil {
			log.Printf("Error while updating scheduling unit option: %s", err)
		}
		if updatedSchedulingUnitOption == nil {
			continue
		}
		filteredSchedulingUnitOptions = append(filteredSchedulingUnitOptions, updatedSchedulingUnitOption)

	}
	return filteredSchedulingUnitOptions, nil
}

// updateTestCases updates the schedulingUnitOptions for each test case in internal test plan request.
func updateTestCases(req *api.InternalTestplan, removeBoardTestMap map[string][]string, log *log.Logger) error {

	// this is to simplyfy removal logic as Test is at top level in internal test plan
	removeTestToBoardsMap := inverseRemoveBoardTestMap(removeBoardTestMap)

	log.Printf("-----Map-----: %s\n", removeBoardTestMap)
	for _, testCase := range req.GetTestCases() {

		log.Printf("Filtering scheduling units for test : %s", testCase.GetName())
		filteredSchedulingUnitOptions, err := filterSchedulingUnitOptionsBasedOnUseFlag(testCase.GetSchedulingUnitOptions(), removeTestToBoardsMap[testCase.GetName()], log)
		if err != nil {
			return fmt.Errorf("error while filtering scheduling unit options: %w", err)
		}
		testCase.SchedulingUnitOptions = filteredSchedulingUnitOptions

	}
	// remove testcase for which schedulingUnitOptions is empty
	var newTestCases []*api.CTPTestCase
	for _, testCase := range req.GetTestCases() {
		if len(testCase.GetSchedulingUnitOptions()) > 0 {
			newTestCases = append(newTestCases, testCase)
		}
	}
	req.TestCases = newTestCases

	return nil
}

// flakyTestPerBoard evaluates lists of flaky tests to be removed for a given board.
func flakyTestPerBoard(req *api.FilterFlakyRequest, board string, log *log.Logger, clientOpts ...option.ClientOption) (*api.FilterFlakyResponse, error) {
	filter := Filter{req: req}
	var err error
	var filteredSuites []*api.TestSuite
	for _, testSuite := range req.TestSuites {
		// The input request, TestSuites, can be either TestCases OR TestCaseMetadata. Support both options.
		var filteredSuite *api.TestSuite
		switch op := testSuite.Spec.(type) {
		case *api.TestSuite_TestCases:

			// Generate a set of tests, these will be used when searching for signal on the tests.
			testCases := op.TestCases.TestCases
			tcS := testCasesToSet(testCases)
			filter.data, err = getStabilityData(filter.req, tcS, log, clientOpts...)
			if err != nil {
				log.Printf("err during stability fetching, will apply rules as possible %s,", err)
			}
			filteredSuite = filter.filterCases(testCases, testSuite.Name, log)
		case *api.TestSuite_TestCasesMetadata:
			// Generate a set of tests, these will be used when searching for signal on the tests.
			metadataList := op.TestCasesMetadata.Values
			filter.data, err = getStabilityData(filter.req, testMDToSet(op), log, clientOpts...)
			if err != nil {
				log.Printf("err during stability fetching, will apply rules as possible %s,", err)
			}
			filteredSuite = filter.filterMetadata(metadataList, testSuite.Name, log)
		}
		filteredSuites = append(filteredSuites, filteredSuite)

	}

	rspn := &api.FilterFlakyResponse{
		TestSuites:   filteredSuites,
		RemovedTests: filter.removed,
	}

	// log filtering results and write results
	flakeFilteringLogAndResults(rspn, board, req, &filter, log, clientOpts...)

	err = interfaces.WriteResults(rspn.RemovedTests, req, filter.data, log, clientOpts...)
	if err != nil {
		log.Println("error observed during results bq insertion: ", err)
	}

	log.Println("Removing the following: ", rspn.RemovedTests)
	return rspn, nil
}

func generateFlakyTestMap(boardTestMap map[string]*BoardTestInfo, log *log.Logger, clientOpts ...option.ClientOption) map[string][]string {
	// policy driving flaky test identification
	policy := fetchPolicy()

	// removeBoardTestMap will hold all tests to be removed for a given board
	removeBoardTestMap := make(map[string][]string)

	// for each board, evaluate lists of flaky test to be removed.
	for board, boardTestInfo := range boardTestMap {
		flakeReq := &api.FilterFlakyRequest{
			Policy:     policy,
			TestSuites: []*api.TestSuite{makeSuite(boardTestInfo.tests)},
			Variant: &api.FilterFlakyRequest_Board{
				Board: boardTestInfo.variant,
			},
			DefaultEnabled: true,
			Milestone:      boardTestInfo.milestone,
		}
		resp, _ := flakyTestPerBoard(flakeReq, board, log, clientOpts...)
		removeBoardTestMap[board] = resp.RemovedTests
	}
	return removeBoardTestMap
}

// innerMain evaluates flaky tests to be removed for all boards in the request and then updates the request by removing flaky tests.
func innerMain(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) *api.InternalTestplan {
	// If flow is other than CQ, then skip and return the original req
	if !isCqFlow(req, log) {
		return req
	}

	// log internal test plan before flaky filtering
	prettyLogInternalTestplanDetail(req, log)

	// create board-test map from the internal test plan
	boardTestMap, err := createBoardTestMap(req, log)
	if err != nil {
		log.Printf("Error while creating board-test map %v\n", err)
	}

	tokenSource := commonParams.AuthHelper.GetTokenSource([]string{saFile}, common.BigqueryScope)

	// removeBoardTestMap will hold all tests to be removed for a given board
	removeBoardTestMap := generateFlakyTestMap(boardTestMap, log, option.WithTokenSource(tokenSource))
	//update/remove flaky tests from each board under internal test plan request
	err = updateTestCases(req, removeBoardTestMap, log)
	if err != nil {
		log.Printf("Error while updating internalTestPlan request %v\n", err)
	}
	// log internal test plan after flaky filtering
	prettyLogInternalTestplanDetail(req, log)
	return req
}

type PreProcessFilter struct {
	servertemplate.FilterBase
}

func NewPreProcessFilter() servertemplate.Filter {
	return &PreProcessFilter{}
}

func (*PreProcessFilter) Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	return innerMain(req, log, commonParams), nil
}

func main() {
	err := servertemplate.Server(NewPreProcessFilter, binName)
	if err != nil {
		os.Exit(2)
	}

	os.Exit(0)
}
