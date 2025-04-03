// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/publish/cmd/publishserver/storage"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

const (
	binName         = "use_flag_filter"
	fileName        = "tast_use_flags.txt"
	useFlagsDirBase = "/tmp/use_flag_filter/tast_use_flags_"
)

// mapToString is helper function, converts a map to string format for pretty print in log.
func mapToString(m map[string]map[string]bool) string {
	result := ""
	for k, v := range m {
		result += fmt.Sprintf("%s: {\n", k)
		for k2, v2 := range v {
			result += fmt.Sprintf("\t%s: %t,\n", k2, v2)
		}
		result += "},\n"
	}
	return result
}

// downloadUseFlags downloads all use flags from a given gsUrl.
func downloadUseFlags(ctx context.Context, gsURL string, gsClient *storage.GSClient, buildTarget string, log *log.Logger, useFlagsDir string) (map[string]bool, error) {

	destFilePath := filepath.Join(useFlagsDir, buildTarget)

	if err := gsClient.DownloadFile(ctx, gsURL, destFilePath); err != nil {
		log.Printf("File not present at gsUrl : %s", gsURL)
		return nil, nil
	}
	log.Printf("Downloaded file :%s", buildTarget)
	content, err := os.ReadFile(destFilePath)
	if err != nil {
		return nil, err
	}

	useFlagList := strings.Split(string(content), "\n")

	useFlagSet := make(map[string]bool)
	for _, item := range useFlagList {
		item = strings.TrimSpace(item)
		// skip comments and empty spaces from us flag file
		if strings.HasPrefix(item, "#") || item == "" {
			continue
		}
		useFlagSet[item] = true
	}

	return useFlagSet, nil
}

// createDirectory generates a unique directory for each request.
// This is important to avoid any race condition as multiple requests are processed by CTP on one machine.
func createDirectory(log *log.Logger) (string, error) {

	useFlagsDir := useFlagsDirBase + uuid.New().String()
	log.Printf("Creating dir for storing use flags ...")
	if err := os.MkdirAll(useFlagsDir, 0755); err != nil {
		return "", err
	}
	log.Printf("UseFlagsDir set to :%s", useFlagsDir)
	return useFlagsDir, nil
}

// generateUseFlagDict creates a dictionary for buildTarget and respective use flag list.
func generateUseFlagDict(ctx context.Context, req *api.InternalTestplan, log *log.Logger) (map[string]map[string]bool, error) {

	buildTargetGSMap, err := createBuildTargetGSMap(req)
	if err != nil {
		return nil, fmt.Errorf("Error generating board-gcsPath map: %w", err)
	}

	gsClient, err := storage.NewGSClient(ctx, "")
	if err != nil {
		return nil, err
	}

	useFlagsDir, err := createDirectory(log)
	if err != nil {
		return nil, fmt.Errorf("Error setting up parent directory for downloading files from gs: %w", err)
	}

	// useFlagDict will look like below, after collecting use flags for all buildTarget in suite info.
	// map[
	// 	"nissa-variant": map[
	// 		"amd64" : true,
	// 		"vulkan" : true,
	// 		"wpa3_sae" : true
	// 	],
	// 	"octopus-variant": map[
	// 		"arc" : true,
	// 		"arc-camera3" : true,
	// 		"arcvm" : true,
	// 		"cheets_user_64" : true
	// 	]
	// ]
	useFlagDict := make(map[string]map[string]bool)

	for buildTarget, gcsPath := range buildTargetGSMap {
		gcsPath = gcsPath + "/" + fileName
		useFlagSet, err := downloadUseFlags(ctx, gcsPath, gsClient, buildTarget, log, useFlagsDir)
		if err != nil {
			return nil, err
		}
		// Should not add to dict if use flags was not found
		if useFlagSet == nil {
			continue
		}

		useFlagDict[buildTarget] = useFlagSet
	}

	defer gsClient.Close()
	log.Printf("useFlagDict: %s", mapToString(useFlagDict))

	return useFlagDict, nil
}

// createBuildTargetGSMap creates a map with board+variant value as key and gspath for image location as value
func createBuildTargetGSMap(req *api.InternalTestplan) (map[string]string, error) {

	buildTargetGSMap := make(map[string]string)
	for _, testCases := range req.GetTestCases() {
		for _, schedulingOptions := range testCases.GetSchedulingUnitOptions() {
			for _, schedulingUnit := range schedulingOptions.GetSchedulingUnits() {
				// retrieve build target values "board+variant" for each schduling unit for a test
				buildTargetGSKey, err := retrieveBuildTargetKey(schedulingUnit.GetPrimaryTarget().GetSwarmingDef())
				if err != nil {
					return nil, fmt.Errorf("Error retrieving build target key from hwDef: %w", err)
				}

				// Check if buildTargetGSKey is not empty and "installPath" exists in the lookup table
				if buildTargetGSKey != "" {
					if installPath, exists := schedulingUnit.GetDynamicUpdateLookupTable()["installPath"]; exists {
						buildTargetGSMap[buildTargetGSKey] = installPath
					} else {
						log.Printf("No gcs install path defined for scheduling unit : %s", buildTargetGSKey)
					}
				}
			}
		}
	}
	log.Printf("BoardGsURLMap: %s", buildTargetGSMap)
	return buildTargetGSMap, nil
}

// retrieveBuildTargetKey retrieves ChromeOS buildTarget+variant from the hwDef
func retrieveBuildTargetKey(req *api.SwarmingDefinition) (string, error) {

	// only chromeos type DUTs, rest should be skipped.
	if req.GetDutInfo().GetChromeos() != nil {
		board := req.GetDutInfo().GetChromeos().GetDutModel().GetBuildTarget()
		retrieveBuildTargetKey := board
		if req.GetVariant() != "" {
			retrieveBuildTargetKey += "-" + req.GetVariant()
		}
		return retrieveBuildTargetKey, nil
	}

	return "", nil
}

// checkIfTestHwDepQualify checks if includeFlags and excludeFlags from a test satisfy a given schduling unit use flag set.
func checkIfTestHwDepQualify(includeFlags []string, excludeFlags []string, useFlagSet map[string]bool) bool {
	// Check if all items in includeFlags are present in useFlagSet
	for _, flag := range includeFlags {
		if !useFlagSet[flag] {
			return false
		}
	}
	// Check if any items in excludeFlags are present in useFlagSet
	for _, flag := range excludeFlags {
		if useFlagSet[flag] {
			return false
		}
	}
	return true
}

// checkIfAnyTestHwDepsQualify converts a test case build deps to includeFlags and excludeFlags for,at and then checks if it qualifies.
func checkIfAnyTestHwDepsQualify(useFlagSet map[string]bool, testHwDeps []*api.TestCase_BuildDeps, log *log.Logger) bool {
	for _, buildDep := range testHwDeps {
		includeFlags, excludeFlags := createIncludeAndExcludeUseFlagList(buildDep, log)
		if checkIfTestHwDepQualify(includeFlags, excludeFlags, useFlagSet) {
			return true
		}
	}
	return false
}

// createIncludeAndExcludeUseFlagList converts a test case build deps to list of string (includeFlags and excludeFlags).
func createIncludeAndExcludeUseFlagList(buildDep *api.TestCase_BuildDeps, log *log.Logger) ([]string, []string) {
	var includeFlag []string
	var excludeFlag []string

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Error parsing use flag expression from test metadata %s", buildDep.GetValue())
			includeFlag = nil
			excludeFlag = nil
		}
	}()

	condition := strings.Split(buildDep.GetValue(), ",")
	for _, item := range condition {
		if len(item) > 0 && item[0] == '!' {
			excludeFlag = append(excludeFlag, item[1:])
		} else {
			includeFlag = append(includeFlag, item)
		}
	}
	return includeFlag, excludeFlag
}

// updateSchedulingUnitOption updates the list of Scheduling units.
func updateSchedulingUnitOption(schedulingUnitOption *api.SchedulingUnitOptions, useFlagDict map[string]map[string]bool, testHwDeps []*api.TestCase_BuildDeps, log *log.Logger) (*api.SchedulingUnitOptions, error) {
	var filteredSchedulingUnits []*api.SchedulingUnit
	for _, schedulingUnit := range schedulingUnitOption.GetSchedulingUnits() {
		buildTargetKey, err := retrieveBuildTargetKey(schedulingUnit.GetPrimaryTarget().GetSwarmingDef())
		if err != nil {
			return nil, err
		}

		if buildTargetKey != "" && useFlagDict[buildTargetKey] != nil {
			// check if any test buildDeps is present in the set.
			useFlagSet := useFlagDict[buildTargetKey]
			qualify := checkIfAnyTestHwDepsQualify(useFlagSet, testHwDeps, log)
			if qualify {
				filteredSchedulingUnits = append(filteredSchedulingUnits, schedulingUnit)
			} else {
				log.Printf("DUT with build target : %s removed", buildTargetKey)
			}
		} else {
			log.Printf("DUT skipped check, due to missing use flag list or DUT is not of ChromeOS type")
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
func filterSchedulingUnitOptionsBasedOnUseFlag(schedulingUnitOptions []*api.SchedulingUnitOptions, useFlagDict map[string]map[string]bool, testHwDeps []*api.TestCase_BuildDeps, log *log.Logger) ([]*api.SchedulingUnitOptions, error) {

	var filteredSchedulingUnitOptions []*api.SchedulingUnitOptions
	for _, schedulingUnitOption := range schedulingUnitOptions {
		// scheduling units will be removed if no buildDeps in test metadata is present in scheduling unit use flag set in useFlagDict
		updatedSchedulingUnitOption, err := updateSchedulingUnitOption(schedulingUnitOption, useFlagDict, testHwDeps, log)
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
func updateTestCases(req *api.InternalTestplan, useFlagDict map[string]map[string]bool, log *log.Logger) error {

	for _, testCase := range req.GetTestCases() {
		if len(testCase.GetMetadata().GetTestCase().GetBuildDependencies()) > 0 {
			log.Printf("Pruning scheduling units for test : %s", testCase.GetName())
			filteredSchedulingUnitOptions, err := filterSchedulingUnitOptionsBasedOnUseFlag(testCase.GetSchedulingUnitOptions(), useFlagDict, testCase.GetMetadata().GetTestCase().GetBuildDependencies(), log)
			if err != nil {
				return fmt.Errorf("Error while pruning scheduling unit options: %w", err)
			}
			testCase.SchedulingUnitOptions = filteredSchedulingUnitOptions
		} else {
			log.Printf("Skippping as test : %s, doesn't have any build deps", testCase.GetName())
		}
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

func executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	ctx := context.Background()

	// parses the request and generates the use flag set for each board+variant
	useFlagDict, err := generateUseFlagDict(ctx, req, log)
	if err != nil {
		return nil, fmt.Errorf("Error generating board-use flags map: %w", err)
	}

	// iterates each test case and removes the scheduling unit as per build dependencies and use flag set of the scheduling unit
	err = updateTestCases(req, useFlagDict, log)
	if err != nil {
		return nil, fmt.Errorf("Error during use flag based test cases pruning: %w", err)
	}

	return req, nil
}

func main() {
	err := servertemplate.Server(executor, binName)
	if err != nil {
		os.Exit(2)
	}

	os.Exit(0)
}
