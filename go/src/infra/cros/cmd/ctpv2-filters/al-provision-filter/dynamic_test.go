// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

// --- Test cases for getTestType ---

func TestGetTestType_OSTestType(t *testing.T) {
	req := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{
					Args: []*api.Arg{
						{
							Flag:  "test-type",
							Value: string(common.OSTestType),
						},
					},
				},
			},
		},
	}
	if got := getTestType(req); got != common.OSTestType {
		t.Errorf("getTestType() = %v, want %v", got, common.OSTestType)
	}
}

func TestGetTestType_KernelTestType(t *testing.T) {
	req := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{
					Args: []*api.Arg{
						{
							Flag:  "test-type",
							Value: string(common.KernelTestType),
						},
					},
				},
			},
		},
	}
	if got := getTestType(req); got != common.KernelTestType {
		t.Errorf("getTestType() = %v, want %v", got, common.KernelTestType)
	}
}

func TestGetTestType_DefaultToOS(t *testing.T) {
	reqNoTestTypeArg := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{
					Args: []*api.Arg{
						{
							Flag:  "some-other-flag",
							Value: "some-value",
						},
					},
				},
			},
		},
	}
	if got := getTestType(reqNoTestTypeArg); got != common.OSTestType {
		t.Errorf("getTestType() with no test-type = %v, want %v", got, common.OSTestType)
	}

	reqUnknownTestTypeValue := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{
					Args: []*api.Arg{
						{
							Flag:  "test-type",
							Value: "XYZ",
						},
					},
				},
			},
		},
	}
	if got := getTestType(reqUnknownTestTypeValue); got != common.OSTestType {
		t.Errorf("getTestType() with an unknown test-type value = %v, want %v", got, common.OSTestType)
	}

	reqNoArgs := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{},
			},
		},
	}
	if got := getTestType(reqNoArgs); got != common.OSTestType {
		t.Errorf("getTestType() with no args = %v, want %v", got, common.OSTestType)
	}
}

// --- Tests for getAllSchedulingUnits ---

func TestGetAllSchedulingUnits_NotNil(t *testing.T) {
	su1 := &api.SchedulingUnit{}
	su2 := &api.SchedulingUnit{}
	su3 := &api.SchedulingUnit{}
	su4 := &api.SchedulingUnit{}
	su5 := &api.SchedulingUnit{}
	metadata := &api.SuiteMetadata{
		SchedulingUnits: []*api.SchedulingUnit{su1, su2},
		SchedulingUnitOptions: []*api.SchedulingUnitOptions{
			{
				SchedulingUnits: []*api.SchedulingUnit{su3, su4},
			},
			{
				SchedulingUnits: []*api.SchedulingUnit{su5},
			},
		},
	}
	expected := []*api.SchedulingUnit{su1, su2, su3, su4, su5}
	if got := getAllSchedulingUnits(metadata); !reflect.DeepEqual(got, expected) {
		t.Errorf("getAllSchedulingUnits(%+v) = %+v, want %+v", metadata, got, expected)
	}
}

func TestGetAllSchedulingUnits_SchedulingUnitsAreNil(t *testing.T) {
	su := &api.SchedulingUnit{}
	metadata := &api.SuiteMetadata{
		// SchedulingUnits here is unset, and therefore nil.
		SchedulingUnitOptions: []*api.SchedulingUnitOptions{
			// This SchedulingUnitOptions's SchedulingUnits is unset, and therefore nil.
			{},
			{
				SchedulingUnits: []*api.SchedulingUnit{su},
			},
		},
	}
	expected := []*api.SchedulingUnit{su}
	if got := getAllSchedulingUnits(metadata); !reflect.DeepEqual(got, expected) {
		t.Errorf("getAllSchedulingUnits(%+v) = %+v, want %+v", metadata, got, expected)
	}
}

func TestGetAllSchedulingUnits_SchedulingUnitOptionsIsNil(t *testing.T) {
	su := &api.SchedulingUnit{}
	metadata := &api.SuiteMetadata{
		SchedulingUnits: []*api.SchedulingUnit{su},
		// SchedulingUnitOptions is unset, and therefore nil.
	}
	expected := []*api.SchedulingUnit{su}
	if got := getAllSchedulingUnits(metadata); !reflect.DeepEqual(got, expected) {
		t.Errorf("getAllSchedulingUnits(%+v) = %+v, want %+v", metadata, got, expected)
	}
}

// --- Tests for extractBuildInfoFromInstallPath ---

func TestExtractBuildInfoFromInstallPath_Success(t *testing.T) {
	tests := []struct {
		name                string
		installPath         string
		expectedBuildId     string
		expectedBuildTarget string
		expectedBoard       string
	}{
		{
			name:                "Standard path with prefix",
			installPath:         common.AndroidBuildPrefix + "12345/board-target/board-ota-12345.zip",
			expectedBuildId:     "12345",
			expectedBuildTarget: "board-target",
			expectedBoard:       "board",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildId, buildTarget, board := extractBuildInfoFromInstallPath(tt.installPath)
			if buildId != tt.expectedBuildId {
				t.Errorf("extractBuildInfoFromInstallPath() buildId = %q, want %q", buildId, tt.expectedBuildId)
			}
			if buildTarget != tt.expectedBuildTarget {
				t.Errorf("extractBuildInfoFromInstallPath() buildTarget = %q, want %q", buildTarget, tt.expectedBuildTarget)
			}
			if board != tt.expectedBoard {
				t.Errorf("extractBuildInfoFromInstallPath() board = %q, want %q", board, tt.expectedBoard)
			}
		})
	}
}

func TestExtractBuildInfoFromInstallPath_Failure(t *testing.T) {
	tests := []struct {
		name        string
		installPath string
	}{
		{
			name:        "Too few segments after prefix",
			installPath: common.AndroidBuildPrefix + "12345",
		},
		{
			name:        "Only prefix",
			installPath: common.AndroidBuildPrefix,
		},
		{
			name:        "Empty string",
			installPath: "",
		},
		{
			name:        "No prefix, too few segments",
			installPath: "12345",
		},
		{
			name:        "Too many segments",
			installPath: common.AndroidBuildPrefix + "67890/another-target/subdir/another-ota-67890.img",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expect empty strings and a log message (log check omitted)
			buildId, buildTarget, board := extractBuildInfoFromInstallPath(tt.installPath)
			if buildId != "" {
				t.Errorf("extractBuildInfoFromInstallPath() buildId = %q, want \"\"", buildId)
			}
			if buildTarget != "" {
				t.Errorf("extractBuildInfoFromInstallPath() buildTarget = %q, want \"\"", buildTarget)
			}
			if board != "" {
				t.Errorf("extractBuildInfoFromInstallPath() board = %q, want \"\"", board)
			}
		})
	}
}

// --- Tests for applyBuildInfoToTarget ---

func TestApplyBuildInfoToTarget(t *testing.T) {
	tests := []struct {
		name              string
		initialKeyValues  []*api.KeyValue
		buildId           string
		buildTarget       string
		expectedKeyValues []*api.KeyValue
	}{
		{
			name:             "Nil initial KeyValues",
			initialKeyValues: nil,
			buildId:          "12345",
			buildTarget:      "board-target",
			expectedKeyValues: []*api.KeyValue{
				{Key: "al_build_id", Value: "12345"},
				{Key: "al_build_target", Value: "board-target"},
			},
		},
		{
			name:             "Empty initial KeyValues",
			initialKeyValues: []*api.KeyValue{},
			buildId:          "67890",
			buildTarget:      "another-target",
			expectedKeyValues: []*api.KeyValue{
				{Key: "al_build_id", Value: "67890"},
				{Key: "al_build_target", Value: "another-target"},
			},
		},
		{
			name: "Existing KeyValues",
			initialKeyValues: []*api.KeyValue{
				{Key: "existing_key", Value: "existing_value"},
			},
			buildId:     "99999",
			buildTarget: "final-target",
			expectedKeyValues: []*api.KeyValue{
				{Key: "existing_key", Value: "existing_value"},
				{Key: "al_build_id", Value: "99999"},
				{Key: "al_build_target", Value: "final-target"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &api.Target{
				SwReq: &api.LegacySW{
					KeyValues: tt.initialKeyValues,
				},
			}
			// Create a copy if initialKeyValues is nil to avoid modifying the test case
			if target.SwReq.KeyValues == nil {
				target.SwReq.KeyValues = []*api.KeyValue{}
			}

			applyBuildInfoToTarget(tt.buildId, tt.buildTarget, target)

			// Handle nil vs empty slice for comparison
			actualKVs := target.SwReq.KeyValues
			expectedKVs := tt.expectedKeyValues
			if len(actualKVs) == 0 && len(expectedKVs) == 0 {
				// Consider empty and nil slices equal for this test purpose
				return
			}

			if !reflect.DeepEqual(actualKVs, expectedKVs) {
				t.Errorf("applyBuildInfoToTarget() KeyValues = %v, want %v", actualKVs, expectedKVs)
			}
		})
	}
}

// --- Tests for updateSchedulingUnit ---

// Helper to create a basic SchedulingUnit for tests
func newTestSchedulingUnit(gcsPath string, initialKVs []*api.KeyValue) *api.SchedulingUnit {
	if initialKVs == nil {
		initialKVs = []*api.KeyValue{} // Ensure non-nil slice
	}
	return &api.SchedulingUnit{
		PrimaryTarget: &api.Target{
			SwReq: &api.LegacySW{
				GcsPath:   gcsPath,
				KeyValues: initialKVs,
			},
		},
		DynamicUpdateLookupTable: make(map[string]string),
	}
}

func TestUpdateSchedulingUnit_ExplicitAndroidBuildPath(t *testing.T) {
	gcsPath := common.AndroidBuildPrefix + "54321/test-board-target/image.zip"
	expectedBuildId := "54321"
	expectedBuildTarget := "test-board-target"

	su := newTestSchedulingUnit(gcsPath, nil) // Start with empty KVs

	updater := &ALProvisionRequestUpdater{} // Not used in this path
	logger := log.Default()                 // Use default logger

	updateSchedulingUnit(su, updater, logger)

	// Check DynamicUpdateLookupTable
	if val, ok := su.DynamicUpdateLookupTable["buildNumber"]; !ok || val != expectedBuildId {
		t.Errorf("DynamicUpdateLookupTable['buildNumber'] = %q, want %q", val, expectedBuildId)
	}
	if val, ok := su.DynamicUpdateLookupTable["installPath"]; !ok || val != gcsPath {
		t.Errorf("DynamicUpdateLookupTable['installPath'] = %q, want %q", val, gcsPath)
	}

	// Check KeyValues were added by applyBuildInfoToTarget
	expectedKeyValues := []*api.KeyValue{
		{Key: "al_build_id", Value: expectedBuildId},
		{Key: "al_build_target", Value: expectedBuildTarget},
	}
	if !reflect.DeepEqual(su.PrimaryTarget.SwReq.KeyValues, expectedKeyValues) {
		t.Errorf("PrimaryTarget.SwReq.KeyValues = %v, want %v", su.PrimaryTarget.SwReq.KeyValues, expectedKeyValues)
	}

	// Check GcsPath was NOT modified in this case
	if su.PrimaryTarget.SwReq.GcsPath != gcsPath {
		t.Errorf("PrimaryTarget.SwReq.GcsPath was modified to %q, should remain %q", su.PrimaryTarget.SwReq.GcsPath, gcsPath)
	}
}

func TestUpdateSchedulingUnit_LatestGreenBuild(t *testing.T) {
	originalGcsPath := "some-other-prefix/board/build/path.zip" // Does not start with android-build
	board := "test-board"
	latestBuild := 98765
	expectedBuildId := strconv.Itoa(latestBuild)
	expectedBuildTarget := board + "-trunk_staging-userdebug"
	expectedCalculatedInstallPath := fmt.Sprintf(
		common.AndroidBuildPrefix+"%s/%s/%s-ota-%s.zip",
		expectedBuildId, expectedBuildTarget, board, expectedBuildId)

	su := newTestSchedulingUnit(originalGcsPath, nil)
	su.DynamicUpdateLookupTable["board"] = board

	// Mock the latest build lookup
	updater := &ALProvisionRequestUpdater{
		LatestBuildsByBoard: map[string]int{
			board: latestBuild,
		},
	}
	logger := log.Default() // Use default logger

	updateSchedulingUnit(su, updater, logger)

	// Check DynamicUpdateLookupTable
	if val, ok := su.DynamicUpdateLookupTable["buildNumber"]; !ok || val != expectedBuildId {
		t.Errorf("DynamicUpdateLookupTable['buildNumber'] = %q, want %q", val, expectedBuildId)
	}
	if val, ok := su.DynamicUpdateLookupTable["installPath"]; !ok || val != expectedCalculatedInstallPath {
		t.Errorf("DynamicUpdateLookupTable['installPath'] = %q, want %q", val, expectedCalculatedInstallPath)
	}

	// Check KeyValues were added by applyBuildInfoToTarget
	expectedKeyValues := []*api.KeyValue{
		{Key: "al_build_id", Value: expectedBuildId},
		{Key: "al_build_target", Value: expectedBuildTarget},
	}
	actualKeyValues := su.PrimaryTarget.SwReq.KeyValues
	// Handle nil vs empty slice for comparison
	if len(actualKeyValues) == 0 && len(expectedKeyValues) == 0 {
		// ok
	} else if !reflect.DeepEqual(actualKeyValues, expectedKeyValues) {
		t.Errorf("PrimaryTarget.SwReq.KeyValues = %v, want %v", actualKeyValues, expectedKeyValues)
	}

	// Check GcsPath WAS modified in this case to the calculated path
	if su.PrimaryTarget.SwReq.GcsPath != expectedCalculatedInstallPath {
		t.Errorf("PrimaryTarget.SwReq.GcsPath = %q, want calculated %q", su.PrimaryTarget.SwReq.GcsPath, expectedCalculatedInstallPath)
	}
}

// --- Tests for getOTAPath ---

func TestGetOTAPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		buildId     string
		buildTarget string
		board       string
		want        string
	}{
		{
			name:        "basic valid input",
			buildId:     "12345678",
			buildTarget: "brya-trunk_staging-userdebug",
			board:       "brya",
			want:        fmt.Sprintf("%s12345678/brya-trunk_staging-userdebug/brya-ota-12345678.zip", common.AndroidBuildPrefix),
		},
		{
			name:        "different board and target",
			buildId:     "98765",
			buildTarget: "dedede-some_branch-user",
			board:       "dedede",
			want:        fmt.Sprintf("%s98765/dedede-some_branch-user/dedede-ota-98765.zip", common.AndroidBuildPrefix),
		},
		{
			name:        "empty inputs", // Although unlikely in practice, test edge case
			buildId:     "",
			buildTarget: "",
			board:       "",
			want:        fmt.Sprintf("%s//-ota-.zip", common.AndroidBuildPrefix),
		},
		{
			name:        "inputs with spaces", // Test if spaces are handled (they are just inserted)
			buildId:     "1 1",
			buildTarget: "target with space",
			board:       "board space",
			want:        fmt.Sprintf("%s1 1/target with space/board space-ota-1 1.zip", common.AndroidBuildPrefix),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getOTAPath(tt.buildId, tt.buildTarget, tt.board)
			if got != tt.want {
				t.Errorf("getOTAPath(%q, %q, %q) = %q; want %q", tt.buildId, tt.buildTarget, tt.board, got, tt.want)
			}
		})
	}
}
