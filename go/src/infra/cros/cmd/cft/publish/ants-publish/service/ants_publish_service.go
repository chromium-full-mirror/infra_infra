// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service provides the API handlers for ants publish.
package service

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	common_utils "go.chromium.org/chromiumos/test/publish/cmd/common-utils"

	androidlib "infra/cros/cmd/common_lib/android_api"
	ants "infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
)

const (
	artifactsDir = "/tmp/artifacts"
)

type AntsPublishService struct {
	metadata *metadata.PublishAntsMetadata
	results  []*api.TestCaseResult
	service  *androidlib.Service
}

// NewAntsPublishService creates a new publish service to interact with Ants.
func NewAntsPublishService(ctx context.Context, req *api.PublishRequest) (*AntsPublishService, error) {
	m, err := unpackMetadata(req)
	if err = validateAntsPublishRequest(req); err != nil {
		if req == nil || m == nil || m.GetAntsInvocationId() == "" {
			return createTestService(ctx)
		}
		return nil, err
	}

	s, err := androidlib.NewAndroidBuildService(ctx, androidlib.CONTAINER_SATLAB)
	if err != nil {
		return nil, err
	}

	return &AntsPublishService{
		metadata: m,
		results:  req.GetTestResponse().GetTestCaseResults(),
		service:  s,
	}, nil
}

func createTestService(ctx context.Context) (*AntsPublishService, error) {
	s, err := androidlib.NewAndroidBuildService(ctx, androidlib.CONTAINER_SATLAB)
	if err != nil {
		return nil, err
	}

	aps := &AntsPublishService{
		service: s,
		results: []*api.TestCaseResult{
			{
				TestCaseId: &api.TestCase_Id{
					Value: "tradefed.dts.CtsBluetoothTestCases#android.bluetooth.cts.AdvertiseDataTest#serviceUuid",
				},
				Verdict:  &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
				Duration: &durationpb.Duration{Seconds: 0},
			},
			{
				TestCaseId: &api.TestCase_Id{
					Value: "tradefed.dts.CtsBluetoothTestCases#android.bluetooth.cts.AdvertiseDataTest#emptyManufacturerData",
				},
				Verdict: &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}},
				Errors: []*api.TestCaseResult_Error{
					{Message: "error message"},
				},
				Duration: &durationpb.Duration{Seconds: 5},
			},
			{
				TestCaseId: &api.TestCase_Id{
					Value: "tradefed.dts.CtsBluetoothTestCases#android.bluetooth.cts.SampleDataTest#emptyManufacturerData",
				},
				Verdict:  &api.TestCaseResult_Abort_{Abort: &api.TestCaseResult_Abort{}},
				Duration: &durationpb.Duration{Seconds: 1},
			},
			{
				TestCaseId: &api.TestCase_Id{
					Value: "tradefed.dts.CtsBluetoothTestCases#android.bluetooth.dts.SampleDTSTest#emptyManufacturerData1",
				},
				Verdict:  &api.TestCaseResult_Skip_{Skip: &api.TestCaseResult_Skip{}},
				Duration: &durationpb.Duration{Seconds: 1},
			},
		},
	}

	inv, err := aps.createInvocation()
	if err != nil {
		return nil, err
	}

	wu := &ants.WorkUnit{
		Name:         "ParentWorkUnit1",
		Type:         "TF_MODULE",
		InvocationId: inv.InvocationId,
	}

	pwu, err := aps.service.WorkUnitService.Insert(wu)
	if err != nil {
		return nil, err
	}

	log.Printf("created invocation: %s and wu: %s", inv.InvocationId, pwu.Id)
	aps.metadata = &metadata.PublishAntsMetadata{
		AntsInvocationId: inv.InvocationId,
		ParentWorkUnitId: pwu.Id,
		AccountId:        "1",
	}

	return aps, nil
}

// createInvocation is used to create a default invocation
// This is used for testing only. Invocation Id should be received from ATP/CTP
func (aps *AntsPublishService) createInvocation() (*ants.Invocation, error) {
	log.Println("creating invocation")

	build := &ants.BuildDescriptor{
		Branch:      "git_main-al-dev",
		BuildTarget: "brya-trunk_staging-userdebug",
		BuildId:     "12425286",
	}
	inv := &ants.Invocation{
		PrimaryBuild: build,
		Properties: []*ants.Property{
			{Name: "account_id", Value: "1"},
		},
	}

	return aps.service.InvocationService.Insert(inv)
}

func (aps *AntsPublishService) insertModuleWorkUnit(name string, wuType string, parent string) (*ants.WorkUnit, error) {
	wu := &ants.WorkUnit{
		Name:         name,
		Type:         wuType,
		ParentId:     parent,
		InvocationId: aps.metadata.GetAntsInvocationId(),
	}

	return aps.service.WorkUnitService.Insert(wu)
}

// UploadToAnts uploads test results to Ants.
func (aps *AntsPublishService) UploadToAnts(ctx context.Context) error {
	log.Printf("Uploading to AnTS: %+v", aps.results)

	modules := make(map[string]*ants.WorkUnit)
	testCases := make(map[string]*ants.WorkUnit)
	var entries []*ants.BatchInsertEntry

	for i, result := range aps.results {
		log.Printf("looking at result: %s", result.GetTestCaseId().Value)

		// TODO(srinivashegde): Add support for mobly
		moduleName, tcName, testName, err := tradefedNames(result.GetTestCaseId().Value)
		if err != nil {
			return err
		}

		if _, ok := modules[moduleName]; !ok {
			mwu, err := aps.insertModuleWorkUnit(moduleName, "TF_MODULE", aps.metadata.GetParentWorkUnitId())
			if err != nil {
				return err
			}
			modules[moduleName] = mwu
		}

		if _, ok := testCases[tcName]; !ok {
			tcwu, err := aps.insertModuleWorkUnit(tcName, "TF_TESTCASE", modules[moduleName].Id)
			if err != nil {
				return err
			}
			testCases[tcName] = tcwu
		}

		tr := &ants.BatchInsertEntry{
			Token: int64(i),
			TestResult: &ants.TestResult{
				InvocationId: aps.metadata.GetAntsInvocationId(),
				WorkUnitId:   testCases[tcName].Id,
				TestIdentifier: &ants.TestIdentifier{
					Module:    moduleName,
					TestClass: tcName,
					Method:    testName,
				},
				TestStatus: antsTestStatus(result),
			},
		}
		entries = append(entries, tr)
	}

	request := &ants.TestResultBatchInsertRequest{
		TestResults:     entries,
		InsertBatchSize: int64(len(entries)),
	}

	result, err := aps.service.TestResultService.BatchInsert(ctx, aps.metadata.AntsInvocationId, request)
	if err != nil {
		return err
	}
	log.Printf("BatchInsert test results response: %d", result.ServerResponse.HTTPStatusCode)
	// Return nil error response to indicate success.
	return nil
}

func (aps *AntsPublishService) UploadArtifacts(ctx context.Context) error {
	log.Printf("Uploading artifacts from: %s", artifactsDir)

	return filepath.Walk(artifactsDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error walking artifactsDir: %q", err)
			return err
		}

		// We only need to look at files inside cros-test dir
		if info.IsDir() || !strings.HasPrefix(path, "cros-test") {
			return nil
		}

		artifactMetadata, err := aps.uploadArtifact(path)
		if err != nil {
			log.Printf("Cannot open file: %s due to error: %q. Skipping upload", path, err)
		}

		log.Printf("Uploaded artifact for: %s", artifactMetadata.Name)
		return nil
	})
}

func (aps *AntsPublishService) uploadArtifact(path string) (*ants.BuildArtifactMetadata, error) {
	filename := filepath.Base(path)

	artifactMetadata := &ants.BuildArtifactMetadata{
		Name:         filename,
		InvocationId: aps.metadata.AntsInvocationId,
		WorkUnitId:   aps.metadata.ParentWorkUnitId,
		ContentType:  mime.TypeByExtension(filename),
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return aps.service.TestArtifactsService.Update(filename, f, artifactMetadata)
}

// TestCaseID is of the form:
// `tradefed.<xts_type>.<module_name>#<class_name>#<test_name>`
func tradefedNames(testcaseID string) (string, string, string, error) {
	if !strings.HasPrefix(testcaseID, "tradefed.") {
		return "", "", "", fmt.Errorf("cannot get testnames. got: %s", testcaseID)
	}

	names := strings.Split(testcaseID, "#")
	modules := strings.Split(names[0], ".")
	if len(names) != 3 || len(modules) != 3 {
		return "", "", "", fmt.Errorf("unexpected format for testcaseID: %s", testcaseID)
	}

	return modules[2], names[1], names[2], nil
}

func antsTestStatus(result *api.TestCaseResult) string {
	switch result.Verdict.(type) {
	case *api.TestCaseResult_Pass_:
		return "pass"
	case *api.TestCaseResult_Fail_:
		return "fail"
	case *api.TestCaseResult_Abort_, *api.TestCaseResult_Crash_:
		return "testError"
	case *api.TestCaseResult_NotRun_:
		return "ignored"
	case *api.TestCaseResult_Skip_:
		return "testSkipped"
	default:
		return "unspecified"
	}
}

func validateAntsPublishRequest(req *api.PublishRequest) error {
	if err := common_utils.ValidateGenericPublishRequest(req); err != nil {
		return err
	}

	if len(req.GetTestResponse().GetTestCaseResults()) == 0 {
		return fmt.Errorf("no test responses found")
	}

	m, err := unpackMetadata(req)
	if err != nil {
		return err
	}

	if m.GetAntsInvocationId() == "" {
		return fmt.Errorf("ants invocation id is required")
	} else if m.GetParentWorkUnitId() == "" {
		return fmt.Errorf("parent workunit id is required")
	} else if m.GetAccountId() == "" {
		return fmt.Errorf("partner account id is required")
	}

	return nil
}

// unpackMetadata unpacks the Any metadata field into PublishGcsMetadata
func unpackMetadata(req *api.PublishRequest) (*metadata.PublishAntsMetadata, error) {
	var m metadata.PublishAntsMetadata
	if err := req.Metadata.UnmarshalTo(&m); err != nil {
		return &m, fmt.Errorf("improperly formatted input proto metadata: %w", err)
	}
	return &m, nil
}
