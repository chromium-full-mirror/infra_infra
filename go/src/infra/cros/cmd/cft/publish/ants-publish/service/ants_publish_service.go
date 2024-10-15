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
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"

	androidlib "infra/cros/cmd/common_lib/android_api"
	ab_qa_atp "infra/cros/cmd/common_lib/ants-qa/androidbuildinternal/v3_qa_atp"
)

const (
	artifactsDir = "/tmp/artifacts"
)

type AntsPublishService struct {
	metadata *metadata.PublishAntsMetadata
	results  []*api.CrosTestResponse_GivenTestResult
	service  *androidlib.Service
}

func defaultResults() []*api.CrosTestResponse_GivenTestResult {
	r := []*api.CrosTestResponse_GivenTestResult{
		{
			ParentTest: "CtsAccelerationTestCases",
			ChildTestCaseResults: []*api.TestCaseResult{
				{
					TestCaseId:  &api.TestCase_Id{Value: "android.acceleration.cts.HardwareAccelerationTest#testIsHardwareAccelerated"},
					Verdict:     &api.TestCaseResult_Pass_{},
					TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Tradefed_{}},
					Duration:    &durationpb.Duration{Seconds: 0},
					StartTime:   &timestamppb.Timestamp{Seconds: 12456},
				},
				{
					TestCaseId:  &api.TestCase_Id{Value: "android.acceleration.cts.HardwareAccelerationTest#testNotAttachedView"},
					Verdict:     &api.TestCaseResult_Pass_{},
					TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Tradefed_{}},
					Duration:    &durationpb.Duration{Seconds: 0},
					StartTime:   &timestamppb.Timestamp{Seconds: 12556},
				},
				{
					TestCaseId:  &api.TestCase_Id{Value: "android.acceleration.cts.SoftwareAccelerationTest#testIsHardwareAccelerated"},
					Verdict:     &api.TestCaseResult_Pass_{},
					TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Tradefed_{}},
					Duration:    &durationpb.Duration{Seconds: 0},
					StartTime:   &timestamppb.Timestamp{Seconds: 12556},
				},
				{
					TestCaseId:  &api.TestCase_Id{Value: "android.acceleration.cts.SoftwareAccelerationTest#testNotAttachedView"},
					Verdict:     &api.TestCaseResult_Pass_{},
					TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Tradefed_{}},
					Duration:    &durationpb.Duration{Seconds: 0},
					StartTime:   &timestamppb.Timestamp{Seconds: 12556},
				},
				{
					TestCaseId:  &api.TestCase_Id{Value: "android.acceleration.cts.WindowFlagHardwareAccelerationTest#testIsHardwareAccelerated"},
					Verdict:     &api.TestCaseResult_Pass_{},
					TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Tradefed_{}},
					Duration:    &durationpb.Duration{Seconds: 0},
					StartTime:   &timestamppb.Timestamp{Seconds: 12556},
				},
				{
					TestCaseId:  &api.TestCase_Id{Value: "android.acceleration.cts.WindowFlagHardwareAccelerationTest#testNotAttachedView"},
					Verdict:     &api.TestCaseResult_Pass_{},
					TestHarness: &api.TestHarness{TestHarnessType: &api.TestHarness_Tradefed_{}},
					Duration:    &durationpb.Duration{Seconds: 0},
					StartTime:   &timestamppb.Timestamp{Seconds: 12556},
				},
			},
		},
	}

	return r
}

// NewAntsPublishService creates a new publish service to interact with Ants.
func NewAntsPublishService(ctx context.Context, req *api.PublishRequest) (*AntsPublishService, error) {
	m, err := unpackMetadata(req)
	if err = validateAntsPublishRequest(req); err != nil {
		log.Print(req)
		//return nil, err
	}

	s, err := androidlib.NewAndroidBuildService(ctx, androidlib.CONTAINER_SATLAB)
	if err != nil {
		return nil, err
	}

	r := req.GetTestResponse().GetGivenTestResults()
	if len(r) == 0 {
		r = defaultResults()
		log.Printf("Using default results for testing: %v", r)
	}

	return &AntsPublishService{
		metadata: m,
		results:  r,
		service:  s,
	}, nil
}

func (aps *AntsPublishService) insertModuleWorkUnit(name string, wuType string, parent string) (*ab_qa_atp.WorkUnit, error) {
	wu := &ab_qa_atp.WorkUnit{
		Name:         name,
		Type:         wuType,
		ParentId:     parent,
		InvocationId: aps.metadata.GetAntsInvocationId(),
	}

	return aps.service.WorkUnitService.Insert(wu)
}

func (aps *AntsPublishService) resultEntries(module *ab_qa_atp.WorkUnit, token int64, results []*api.TestCaseResult) ([]*ab_qa_atp.BatchInsertEntry, int64, error) {
	tcWorkunits := make(map[string]string)
	var entries []*ab_qa_atp.BatchInsertEntry

	for _, result := range results {
		names := strings.Split(result.GetTestCaseId().GetValue(), "#")
		parentwu := module
		var tr *ab_qa_atp.TestResult
		var err error
		// If testcase exists, use that as the parent module instead
		if len(names) == 2 {
			// Create work unit if it does not exist.
			if tcWorkunits[names[0]] == "" {
				parentwu, err = aps.insertModuleWorkUnit(names[0], "TF_TEST_RUN", module.Id)
				if err != nil {
					log.Printf("unable to create test run workunit for %s due to %q", names[0], err)
					return nil, token, err
				}
				tcWorkunits[names[0]] = parentwu.Id
			}

			startTime := result.GetStartTime().AsTime().Unix()
			tr = &ab_qa_atp.TestResult{
				InvocationId: aps.metadata.GetAntsInvocationId(),
				WorkUnitId:   parentwu.Id,
				TestIdentifier: &ab_qa_atp.TestIdentifier{
					Module:    module.Name,
					TestClass: names[0],
					Method:    names[1],
				},
				TestStatus: antsTestStatus(result),
				Timing: &ab_qa_atp.Timing{
					CreationTimestamp: startTime,
					CompleteTimestamp: startTime + result.GetDuration().GetSeconds(),
				},
				AggregationDetail: &ab_qa_atp.AggregationDetail{
					AggregationLevel: "method",
				},
			}
		} else if len(names) == 1 {
			tr = &ab_qa_atp.TestResult{
				InvocationId: aps.metadata.GetAntsInvocationId(),
				WorkUnitId:   parentwu.Id,
				TestIdentifier: &ab_qa_atp.TestIdentifier{
					TestClass: module.Name,
					Method:    names[0],
				},
				TestStatus: antsTestStatus(result),
			}
		} else {
			return nil, token, fmt.Errorf("unexpected testcaseid: %s", result.GetTestCaseId().GetValue())
		}

		entries = append(entries, &ab_qa_atp.BatchInsertEntry{TestResult: tr, Token: token})
		token = token + 1
	}

	return entries, token, nil
}

// UploadToAnts uploads test results to Ants.
func (aps *AntsPublishService) UploadToAnts(ctx context.Context) error {
	log.Printf("Uploading to AnTS: %+v", aps.results)

	var entries []*ab_qa_atp.BatchInsertEntry
	token := int64(0)
	for _, result := range aps.results {
		log.Printf("looking at result: %+v", result)

		// Add a module workunit
		mwu, err := aps.insertModuleWorkUnit(result.GetParentTest(), "TF_MODULE", aps.metadata.GetParentWorkUnitId())
		if err != nil {
			return err
		}

		var childEntries []*ab_qa_atp.BatchInsertEntry
		childEntries, token, err = aps.resultEntries(mwu, token, result.GetChildTestCaseResults())
		if err != nil {
			return err
		}
		entries = append(entries, childEntries...)
	}

	request := &ab_qa_atp.TestResultBatchInsertRequest{
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

func (aps *AntsPublishService) uploadArtifact(path string) (*ab_qa_atp.BuildArtifactMetadata, error) {
	filename := filepath.Base(path)

	artifactMetadata := &ab_qa_atp.BuildArtifactMetadata{
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

func antsTestStatus(result *api.TestCaseResult) string {
	switch result.Verdict.(type) {
	case *api.TestCaseResult_Pass_:
		if len(result.Errors) > 0 {
			return "assumptionFailure"
		}
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
	if len(req.GetTestResponse().GetTestCaseResults()) == 0 && len(req.GetTestResponse().GetGivenTestResults()) == 0 {
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
