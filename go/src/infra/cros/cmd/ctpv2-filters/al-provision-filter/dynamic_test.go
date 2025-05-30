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

	"github.com/golang/mock/gomock"

	"go.chromium.org/chromiumos/config/go/test/api"

	androidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api"
	mockandroidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api/mocks"
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
			installPath:         common.GetABOTAPath("12345", "board-target", "board"),
			expectedBuildId:     "12345",
			expectedBuildTarget: "board-target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildId, buildTarget := extractBuildInfoFromInstallPath(tt.installPath)
			if buildId != tt.expectedBuildId {
				t.Errorf("extractBuildInfoFromInstallPath() buildId = %q, want %q", buildId, tt.expectedBuildId)
			}
			if buildTarget != tt.expectedBuildTarget {
				t.Errorf("extractBuildInfoFromInstallPath() buildTarget = %q, want %q", buildTarget, tt.expectedBuildTarget)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expect empty strings and a log message (log check omitted)
			buildId, buildTarget := extractBuildInfoFromInstallPath(tt.installPath)
			if buildId != "" {
				t.Errorf("extractBuildInfoFromInstallPath() buildId = %q, want \"\"", buildId)
			}
			if buildTarget != "" {
				t.Errorf("extractBuildInfoFromInstallPath() buildTarget = %q, want \"\"", buildTarget)
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
		buildBranch       string
		expectedKeyValues []*api.KeyValue
	}{
		{
			name:             "Nil initial KeyValues",
			initialKeyValues: nil,
			buildId:          "12345",
			buildTarget:      "board-target",
			buildBranch:      "brya-branch",
			expectedKeyValues: []*api.KeyValue{
				{Key: "al_build_id", Value: "12345"},
				{Key: "al_build_target", Value: "board-target"},
				{Key: "al_build_branch", Value: "brya-branch"},
			},
		},
		{
			name:             "Empty initial KeyValues",
			initialKeyValues: []*api.KeyValue{},
			buildId:          "67890",
			buildTarget:      "another-target",
			buildBranch:      "another-branch",
			expectedKeyValues: []*api.KeyValue{
				{Key: "al_build_id", Value: "67890"},
				{Key: "al_build_target", Value: "another-target"},
				{Key: "al_build_branch", Value: "another-branch"},
			},
		},
		{
			name: "Existing KeyValues",
			initialKeyValues: []*api.KeyValue{
				{Key: "existing_key", Value: "existing_value"},
			},
			buildId:     "99999",
			buildTarget: "final-target",
			buildBranch: "final-branch",
			expectedKeyValues: []*api.KeyValue{
				{Key: "existing_key", Value: "existing_value"},
				{Key: "al_build_id", Value: "99999"},
				{Key: "al_build_target", Value: "final-target"},
				{Key: "al_build_branch", Value: "final-branch"},
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

			applyBuildInfoToTarget(tt.buildId, tt.buildTarget, tt.buildBranch, target)

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

func TestUpdateSchedulingUnit_LatestGreenBuild(t *testing.T) {
	originalGcsPath := "some-other-prefix/board/build/path.zip" // Does not start with android-build
	board := "test-board"
	latestBuild := 98765
	expectedBuildId := strconv.Itoa(latestBuild)
	expectedBuildTarget := board + "-trunk_staging-userdebug"
	expectedBusytownBranch := "git_main-al-dev"
	expectedCalculatedInstallPath := common.GetABOTAPath(expectedBuildId, expectedBuildTarget, board)

	su := newTestSchedulingUnit(originalGcsPath, nil)
	su.DynamicUpdateLookupTable["board"] = board

	// Mock the latest build lookup
	updater := &ALProvisionRequestUpdater{
		LatestBuildsByBoard: map[string]int{
			board: latestBuild,
		},
	}
	logger := log.Default() // Use default logger
	commonParams := &common.CommonFilterParams{}

	updateSchedulingUnit(su, &api.InternalTestplan{}, updater, logger, commonParams)

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
		{Key: "al_build_branch", Value: expectedBusytownBranch},
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

// TODO: b/406307693 - Remove this once we get QA builds for git_main-throttled.
func TestUpdateSchedulingUnit_LatestGreenBuildProd(t *testing.T) {
	originalGcsPath := "some-other-prefix/board/build/path.zip" // Does not start with android-build
	board := "test-board"
	latestBuild := 98765
	expectedBuildId := strconv.Itoa(latestBuild)
	expectedBuildTarget := board + "-trunk_staging-userdebug"
	expectedBusytownBranch := "git_main-throttled"
	expectedCalculatedInstallPath := common.GetABOTAPath(expectedBuildId, expectedBuildTarget, board)

	su := newTestSchedulingUnit(originalGcsPath, nil)
	su.DynamicUpdateLookupTable["board"] = board

	// Mock the latest build lookup
	updater := &ALProvisionRequestUpdater{
		LatestBuildsByBoard: map[string]int{
			board: latestBuild,
		},
	}
	logger := log.Default() // Use default logger
	commonParams := &common.CommonFilterParams{Environment: common.LabelProd}

	updateSchedulingUnit(su, &api.InternalTestplan{}, updater, logger, commonParams)

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
		{Key: "al_build_branch", Value: expectedBusytownBranch},
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

// --- Misc tests ---

func TestFixInstallPathForKernelTest_Success(t *testing.T) {
	originalInstallPath := "android-build/build_explorer/artifacts_list/123456789/brya_device_x86_64/brya-ota-123456789.zip"
	expected := "android-build/build_explorer/artifacts_list/987654321/brya-trunk_staging-userdebug/brya-ota-987654321.zip"
	board := "brya"
	req := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{
					Args: []*api.Arg{
						{Flag: "build_id", Value: "987654321"},
						{Flag: "build_target", Value: "brya-trunk_staging-userdebug"},
					},
				},
			},
		},
	}
	if got, err := fixInstallPathForKernelTest(originalInstallPath, req, board); err != nil {
		t.Fatalf("fixInstallPathForKernelTest(%q, %q) raised error %q", originalInstallPath, req, err)
	} else if got != expected {
		t.Fatalf("fixInstallPathForKernelTest(%q, %q) = %q, want %q", originalInstallPath, req, got, expected)
	}
}

// TestUpdateSchedulingUnit_AndroidBuildPath tests the updateSchedulingUnit function
// specifically for the flow where the GCS path starts with "android-build".
func TestUpdateSchedulingUnit_ExplicitAndroidBuildPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := log.Default() // Use default logger
	commonParams := &common.CommonFilterParams{}

	mockBuilds := mockandroidapi.NewMockAndroidBuildClient(ctrl)
	// Override the global factory function to return the mock
	androidapi.AndroidBuildFactory = func() androidapi.AndroidBuildClient {
		return mockBuilds
	}
	defer func() {
		// Reset to the original factory after the test
		androidapi.AndroidBuildFactory = func() androidapi.AndroidBuildClient {
			return &androidapi.DefaultAndroidBuildClient{} // Or whatever the original was
		}
	}()

	tests := []struct {
		name           string
		su             *api.SchedulingUnit
		req            *api.InternalTestplan
		wantErr        bool
		wantSU         *api.SchedulingUnit
		mockBuildsFunc func(*mockandroidapi.MockAndroidBuildClient)
	}{
		{
			name: "successful update with android build path",
			su: &api.SchedulingUnit{
				DynamicUpdateLookupTable: map[string]string{"board": "brya"},
				PrimaryTarget: &api.Target{
					SwReq: &api.LegacySW{
						GcsPath:   "android-build/build_explorer/artifacts_list/123/brya_device_x86_64/brya-ota-123456789.zip",
						KeyValues: []*api.KeyValue{},
					},
				},
			},
			req: &api.InternalTestplan{},
			wantSU: &api.SchedulingUnit{
				DynamicUpdateLookupTable: map[string]string{"board": "brya", "buildNumber": "123", "installPath": "android-build/build_explorer/artifacts_list/123/brya_device_x86_64/brya-ota-123456789.zip"},
				PrimaryTarget: &api.Target{
					SwReq: &api.LegacySW{
						GcsPath: "android-build/build_explorer/artifacts_list/123/brya_device_x86_64/brya-ota-123456789.zip",
						KeyValues: []*api.KeyValue{
							{Key: "al_build_id", Value: "123"},
							{Key: "al_build_target", Value: "brya_device_x86_64"},
							{Key: "al_build_branch", Value: DefaultBranch},
						},
					},
				},
			},
			mockBuildsFunc: func(mockBuilds *mockandroidapi.MockAndroidBuildClient) {
				mockBuilds.EXPECT().GetBranchFromBuildID(gomock.Any(), gomock.Any()).Return(DefaultBranch, nil).Times(1)
			},
		},
		{
			name: "failed to get branch from build id",
			su: &api.SchedulingUnit{
				DynamicUpdateLookupTable: map[string]string{"board": "brya"},
				PrimaryTarget: &api.Target{
					SwReq: &api.LegacySW{
						GcsPath:   "android-build/build_explorer/artifacts_list/456/brya_device_x86_64/brya-ota-123456789.zip",
						KeyValues: []*api.KeyValue{},
					},
				},
			},
			req: &api.InternalTestplan{},
			wantSU: &api.SchedulingUnit{
				DynamicUpdateLookupTable: map[string]string{"board": "brya", "buildNumber": "456", "installPath": "android-build/build_explorer/artifacts_list/456/brya_device_x86_64/brya-ota-123456789.zip"},
				PrimaryTarget: &api.Target{
					SwReq: &api.LegacySW{
						GcsPath: "android-build/build_explorer/artifacts_list/456/brya_device_x86_64/brya-ota-123456789.zip",
						KeyValues: []*api.KeyValue{
							{Key: "al_build_id", Value: "456"},
							{Key: "al_build_target", Value: "brya_device_x86_64"},
							{Key: "al_build_branch", Value: ""}, // Branch will be empty due to failure
						},
					},
				},
			},
			mockBuildsFunc: func(mockBuilds *mockandroidapi.MockAndroidBuildClient) {
				mockBuilds.EXPECT().GetBranchFromBuildID(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("failed to get branch")).Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Mock the latest build lookup
			tt.mockBuildsFunc(mockBuilds)
			updater := &ALProvisionRequestUpdater{}
			err := updateSchedulingUnit(tt.su, tt.req, updater, logger, commonParams)
			if (err != nil) != tt.wantErr {
				t.Errorf("updateSchedulingUnit() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if tt.wantSU.DynamicUpdateLookupTable["buildNumber"] != tt.su.DynamicUpdateLookupTable["buildNumber"] {
					t.Errorf("updateSchedulingUnit() buildNumber got = %v, want %v", tt.su.DynamicUpdateLookupTable["buildNumber"], tt.wantSU.DynamicUpdateLookupTable["buildNumber"])
				}
				if tt.wantSU.DynamicUpdateLookupTable["installPath"] != tt.su.DynamicUpdateLookupTable["installPath"] {
					t.Errorf("updateSchedulingUnit() installPath got = %v, want %v", tt.su.DynamicUpdateLookupTable["installPath"], tt.wantSU.DynamicUpdateLookupTable["installPath"])
				}
				gotGcsPath := tt.su.GetPrimaryTarget().GetSwReq().GetGcsPath()
				wantGcsPath := tt.wantSU.PrimaryTarget.GetSwReq().GetGcsPath()
				if wantGcsPath != gotGcsPath {
					t.Errorf("updateSchedulingUnit() GcsPath got = %v, want %v", gotGcsPath, wantGcsPath)
				}
				gotKeyValues := tt.su.GetPrimaryTarget().GetSwReq().GetKeyValues()
				wantKeyValues := tt.wantSU.PrimaryTarget.GetSwReq().GetKeyValues()
				if len(wantKeyValues) != len(gotKeyValues) {
					t.Errorf("updateSchedulingUnit() KeyValues length got = %v, want %v", len(gotKeyValues), len(wantKeyValues))
				} else {
					for i, wantKV := range wantKeyValues {
						if gotKeyValues[i].GetKey() != wantKV.GetKey() || gotKeyValues[i].GetValue() != wantKV.GetValue() {
							t.Errorf("updateSchedulingUnit() KeyValues[%d] got = %v, want %v", i, gotKeyValues[i], wantKV)
						}
					}
				}
			}
		})
	}
}
