// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"testing"

	gax "github.com/googleapis/gax-go/v2"
	moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type MockMoblabClient struct {
	mu                          sync.Mutex
	StageBuildCalls             []StageBuildCall
	CheckBuildStageStatusCalls  []CheckBuildStageStatusCall
	StageBuildReturn            []*StageBuildReturn
	CheckBuildStageStatusReturn []*CheckBuildStageStatusReturn
	BuildFailures               map[string]error
}

type StageBuildCall struct {
	Ctx  context.Context
	Req  *moblabpb.StageBuildRequest
	Opts []gax.CallOption
}

type CheckBuildStageStatusCall struct {
	Ctx  context.Context
	Req  *moblabpb.CheckBuildStageStatusRequest
	Opts []gax.CallOption
}

type StageBuildReturn struct {
	Response *moblabpb.StageBuildResponse
	Error    error
}

type CheckBuildStageStatusReturn struct {
	Response *moblabpb.CheckBuildStageStatusResponse
	Error    error
}

func (m *MockMoblabClient) StageBuild(ctx context.Context, req *moblabpb.StageBuildRequest, opts ...gax.CallOption) (*moblabpb.StageBuildResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StageBuildCalls = append(m.StageBuildCalls, StageBuildCall{Ctx: ctx, Req: req, Opts: opts})
	if err, exists := m.BuildFailures[req.Name]; exists {
		return nil, err
	}
	i := min(len(m.StageBuildCalls), len(m.StageBuildReturn)) - 1

	return m.StageBuildReturn[i].Response, m.StageBuildReturn[i].Error
}

func (m *MockMoblabClient) CheckBuildStageStatus(ctx context.Context, req *moblabpb.CheckBuildStageStatusRequest, opts ...gax.CallOption) (*moblabpb.CheckBuildStageStatusResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CheckBuildStageStatusCalls = append(m.CheckBuildStageStatusCalls, CheckBuildStageStatusCall{Ctx: ctx, Req: req, Opts: opts})
	i := min(len(m.CheckBuildStageStatusCalls), len(m.CheckBuildStageStatusReturn)) - 1
	return m.CheckBuildStageStatusReturn[i].Response, m.CheckBuildStageStatusReturn[i].Error
}

func GenerateLegacySW(build string, bucket string) *api.LegacySW {
	return &api.LegacySW{
		KeyValues: []*api.KeyValue{
			{
				Key:   "chromeos_build",
				Value: build,
			},
			{
				Key:   "chromeos_build_gcs_bucket",
				Value: bucket,
			},
		},
	}
}

func GenerateSchedulingUnit(build string, bucket string) *api.SchedulingUnit {
	schedulingUnit := &api.SchedulingUnit{
		PrimaryTarget: &api.Target{
			SwReq: GenerateLegacySW(build, bucket),
		},
	}
	return schedulingUnit
}

func GenerateTestPlan() *api.InternalTestplan {
	testPlan := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				SchedulingUnits: []*api.SchedulingUnit{
					GenerateSchedulingUnit("skyrim-release/R130-16033.49.0", "chromeos-distributed-fleet-s4p"),
					GenerateSchedulingUnit("zork-release/130-16033.49.0", "chromeos-distributed-fleet-s4p"),
					GenerateSchedulingUnit("strongbad-release/R130-16033.49.0", "chromeos-image-archive"),
				},
			},
		},
	}
	return testPlan
}

func TestExtractBuildsToStage(t *testing.T) {
	expectedBuildsToStage := map[string]string{
		"skyrim-release/R130-16033.49.0": "chromeos-distributed-fleet-s4p",
		"zork-release/130-16033.49.0":    "chromeos-distributed-fleet-s4p",
	}
	testPlan := GenerateTestPlan()
	buildsToStage, cft := extractBuildsToStage(testPlan)
	if !cft {
		t.Errorf("Expected cft to be true, got false")
	}
	if !reflect.DeepEqual(buildsToStage, expectedBuildsToStage) {
		t.Errorf("Expected %v, got %v", expectedBuildsToStage, buildsToStage)
	}
}

func TestParseChromeOSBuild(t *testing.T) {
	type parseResult struct {
		board   string
		version string
		err     bool
	}
	testCases := map[string]parseResult{
		"skyrim-release/R130-16033.49.0": {
			board:   "skyrim",
			version: "16033.49.0",
			err:     false,
		},
		"zork-release/130-16033.49.0": {
			board:   "zork",
			version: "16033.49.0",
			err:     false,
		},
		"zork-dev/130-16033.49.0": {
			board:   "",
			version: "",
			err:     true,
		},
		"zork-release/GGG130-16033.49.0": {
			board:   "",
			version: "",
			err:     true,
		},
	}

	for build, expected := range testCases {
		t.Run(build, func(t *testing.T) {
			board, version, err := parseChromeOSBuild(build)
			if err != nil && !expected.err {
				t.Errorf("Unexpected error: %v", err)
			}
			if err == nil && expected.err {
				t.Errorf("Expected error, but got none")
			}
			if board != expected.board || version != expected.version {
				t.Errorf("Expected board: %s, version: %s, got board: %s, version: %s", expected.board, expected.version, board, version)
			}
		})
	}
}

func TestStageChromeOSBuild(t *testing.T) {
	build := "skyrim-release/R130-16033.49.0"
	bucket := "chromeos-distributed-fleet-s4p"
	expectedName := "buildTargets/skyrim/models/foo/builds/16033.49.0/artifacts/chromeos-distributed-fleet-s4p"
	expectedFilter := "type=release"
	cloud_build_id := "12345"
	expectedCheckStatusFilter := fmt.Sprintf("cloud_build_id=%s", cloud_build_id)

	ctx := context.Background()
	mockMoblabClient := &MockMoblabClient{
		StageBuildReturn: []*StageBuildReturn{{
			Response: &moblabpb.StageBuildResponse{
				CloudBuild: &moblabpb.CloudBuild{
					Id:     "12345",
					Status: moblabpb.CloudBuild_QUEUED,
				},
			},
		},
		},
		CheckBuildStageStatusReturn: []*CheckBuildStageStatusReturn{
			{
				Response: &moblabpb.CheckBuildStageStatusResponse{
					IsBuildStaged: true,
					StagedBuildArtifact: &moblabpb.BuildArtifact{
						Path: "path/to/artifact",
					},
					CloudBuild: &moblabpb.CloudBuild{
						Id:     "12345",
						Status: moblabpb.CloudBuild_QUEUED,
					},
				},
			},
			{
				Response: &moblabpb.CheckBuildStageStatusResponse{
					IsBuildStaged: true,
					StagedBuildArtifact: &moblabpb.BuildArtifact{
						Path: "path/to/artifact",
					},
					CloudBuild: &moblabpb.CloudBuild{
						Id:     "12345",
						Status: moblabpb.CloudBuild_SUCCEEDED,
					},
				},
			},
		},
	}

	log := log.Default()
	err := stageChromeOSBuild(ctx, mockMoblabClient, build, bucket, false, log)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(mockMoblabClient.StageBuildCalls) == 1 {
		actualRequest := mockMoblabClient.StageBuildCalls[0].Req
		if actualRequest.Name != expectedName || actualRequest.Filter != expectedFilter {
			t.Errorf("Expected Name: %s, Filter: %s, got Name: %s, Filter: %s", expectedName, expectedFilter, actualRequest.Name, actualRequest.Filter)
		}
	} else {
		t.Errorf("Expected 1 call to StageBuild, got %d", len(mockMoblabClient.StageBuildCalls))
	}

	if len(mockMoblabClient.CheckBuildStageStatusCalls) == 2 {
		actualRequest := mockMoblabClient.CheckBuildStageStatusCalls[0].Req
		if actualRequest.Name != expectedName {
			t.Errorf("Expected Name: %s, got Name: %s", expectedName, actualRequest.Name)
		}
		if actualRequest.Filter != expectedCheckStatusFilter {
			t.Errorf("Expected Filter: %s, got Filter: %s", expectedCheckStatusFilter, actualRequest.Filter)
		}
	} else {
		t.Errorf("Expected 2 call to CheckBuildStageStatus, got %d", len(mockMoblabClient.CheckBuildStageStatusCalls))
	}
}

func TestStageChromeOSBuild_StageBuildError(t *testing.T) {
	build := "skyrim-release/R130-16033.49.0"
	bucket := "chromeos-distributed-fleet-s4p"

	ctx := context.Background()
	mockMoblabClient := &MockMoblabClient{
		StageBuildReturn: []*StageBuildReturn{{
			Error: fmt.Errorf("simulated StageBuild error"),
		}},
	}

	log := log.Default()
	err := stageChromeOSBuild(ctx, mockMoblabClient, build, bucket, false, log)
	if err == nil {
		t.Errorf("Expected an error from stageChromeOSBuild, but got none")
	}

	if len(mockMoblabClient.StageBuildCalls) != 1 {
		t.Errorf("Expected 1 call to StageBuild, got %d", len(mockMoblabClient.StageBuildCalls))
	}

	if len(mockMoblabClient.CheckBuildStageStatusCalls) != 0 {
		t.Errorf("Expected 0 calls to CheckBuildStageStatus, got %d",
			len(mockMoblabClient.CheckBuildStageStatusCalls))
	}
}

func TestStageChromeOSBuild_CheckBuildStageStatusError(t *testing.T) {
	build := "skyrim-release/R130-16033.49.0"
	bucket := "chromeos-distributed-fleet-s4p"

	ctx := context.Background()
	mockMoblabClient := &MockMoblabClient{
		StageBuildReturn: []*StageBuildReturn{{
			Response: &moblabpb.StageBuildResponse{},
		}},
		CheckBuildStageStatusReturn: []*CheckBuildStageStatusReturn{{Error: fmt.Errorf("simulated CheckBuildStageStatus error")}},
	}

	log := log.Default()
	err := stageChromeOSBuild(ctx, mockMoblabClient, build, bucket, false, log)
	if err == nil {
		t.Errorf("Expected an error from stageChromeOSBuild, but got none")
	}

	if len(mockMoblabClient.StageBuildCalls) != 1 {
		t.Errorf("Expected 1 call to StageBuild, got %d", len(mockMoblabClient.StageBuildCalls))
	}

	if len(mockMoblabClient.CheckBuildStageStatusCalls) == 0 {
		t.Errorf("Expected at least 1 call to CheckBuildStageStatus, got %d",
			len(mockMoblabClient.CheckBuildStageStatusCalls))
	}
}

func TestStageBuilds_Success(t *testing.T) {
	ctx := context.Background()
	logger := log.Default()

	mockMoblabClient := &MockMoblabClient{
		StageBuildReturn: []*StageBuildReturn{
			{
				Response: &moblabpb.StageBuildResponse{},
			}},
		CheckBuildStageStatusReturn: []*CheckBuildStageStatusReturn{
			{
				Response: &moblabpb.CheckBuildStageStatusResponse{
					IsBuildStaged: true,
					StagedBuildArtifact: &moblabpb.BuildArtifact{
						Path: "path/to/artifact",
					},
				},
			},
		},
	}

	buildsToStage := map[string]string{
		"build1-release/R1234.56.7": "bucket1",
		"build2-release/R1234.56.8": "bucket2",
		"build3-release/R1234.56.9": "bucket3",
	}

	err := stageBuilds(ctx, mockMoblabClient, buildsToStage, false, logger)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	expectedCalls := len(buildsToStage)
	if len(mockMoblabClient.StageBuildCalls) != expectedCalls {
		t.Errorf("Expected %d calls to StageBuild, got %d", expectedCalls, len(mockMoblabClient.StageBuildCalls))
	}
}

func TestStageBuilds_PartialFailure(t *testing.T) {
	ctx := context.Background()
	logger := log.Default()

	mockMoblabClient := &MockMoblabClient{
		StageBuildReturn: []*StageBuildReturn{{
			Response: &moblabpb.StageBuildResponse{},
		}},
		CheckBuildStageStatusReturn: []*CheckBuildStageStatusReturn{
			{
				Response: &moblabpb.CheckBuildStageStatusResponse{
					IsBuildStaged: true,
					StagedBuildArtifact: &moblabpb.BuildArtifact{
						Path: "path/to/artifact",
					},
				},
			}},
		BuildFailures: map[string]error{
			"buildTargets/build2/models/foo/builds/1234.56.8/artifacts/bucket2": fmt.Errorf("simulated failure"),
		},
	}

	buildsToStage := map[string]string{
		"build1-release/R1234.56.7": "bucket1",
		"build2-release/R1234.56.8": "bucket2", // This build will fail.
		"build3-release/R1234.56.9": "bucket3",
	}

	err := stageBuilds(ctx, mockMoblabClient, buildsToStage, false, logger)
	if err == nil {
		t.Errorf("Expected an error, but got none")
	} else {
		expectedErrorMessage := "Failed staging 1 out of 3 builds"
		if !strings.Contains(err.Error(), expectedErrorMessage) {
			t.Errorf("Expected error message to contain '%s', but got: %v", expectedErrorMessage, err)
		}
	}

	expectedCalls := len(buildsToStage)
	if len(mockMoblabClient.StageBuildCalls) != expectedCalls {
		t.Errorf("Expected %d calls to StageBuild, got %d", expectedCalls, len(mockMoblabClient.StageBuildCalls))
	}
}
