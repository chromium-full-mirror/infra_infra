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

	"github.com/pkg/errors"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	androidlib "infra/cros/cmd/common_lib/android_api"
	atp "infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
	"infra/cros/cmd/common_lib/common"
)

const (
	artifactsDir      = "/tmp/artifacts/"
	aggregationStatus = "method"
)

type AntsPublishService struct {
	metadata   *metadata.PublishAntsMetadata
	results    []*api.CrosTestResponse_GivenTestResult
	service    *androidlib.Service
	invocation *atp.Invocation
}

// NewAntsPublishService creates a new publish service to interact with Ants.
func NewAntsPublishService(ctx context.Context, req *api.PublishRequest) (*AntsPublishService, error) {
	if err := validateAntsPublishRequest(req); err != nil {
		return nil, err
	}

	m, err := unpackMetadata(req)
	if err != nil {
		return nil, err
	}

	s, err := androidlib.NewAndroidBuildService(ctx, androidlib.CONTAINER_SATLAB, common.Prod)
	if err != nil {
		return nil, err
	}

	inv, err := s.InvocationService.Get(m.AntsInvocationId)
	if err != nil {
		log.Printf("Cannot get primary invocation for: %s", m.AntsInvocationId)
	}

	return &AntsPublishService{
		metadata:   m,
		results:    req.GetTestResponse().GetGivenTestResults(),
		service:    s,
		invocation: inv,
	}, nil
}

func (aps *AntsPublishService) insertModuleWorkUnit(name string, wuType string, parent string) (*atp.WorkUnit, error) {
	wu := &atp.WorkUnit{
		Name:         name,
		Type:         wuType,
		ParentId:     parent,
		InvocationId: aps.metadata.GetAntsInvocationId(),
	}

	return aps.service.WorkUnitService.Insert(wu)
}

func (aps *AntsPublishService) resultEntries(module *atp.WorkUnit, token int64, results []*api.TestCaseResult) ([]*atp.BatchInsertEntry, int64, error) {
	tcWorkunits := make(map[string]string)
	var entries []*atp.BatchInsertEntry

	for _, result := range results {
		names := strings.Split(result.GetTestCaseId().GetValue(), "#")
		parentwu := module
		var testID *atp.TestIdentifier
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

			testID = &atp.TestIdentifier{
				Module:    module.Name,
				TestClass: names[0],
				Method:    names[1],
			}
		} else if len(names) == 1 {
			testID = &atp.TestIdentifier{
				Module:    module.Name,
				TestClass: module.Name,
				Method:    names[0],
			}
		} else {
			return nil, token, fmt.Errorf("unexpected testcaseid: %s", result.GetTestCaseId().GetValue())
		}

		startTime := result.GetStartTime().AsTime().Unix()
		tr := &atp.TestResult{
			InvocationId:   aps.metadata.GetAntsInvocationId(),
			WorkUnitId:     parentwu.Id,
			TestIdentifier: testID,
			TestStatus:     antsTestStatus(result),
			Timing: &atp.Timing{
				CreationTimestamp: startTime,
				CompleteTimestamp: startTime + result.GetDuration().GetSeconds(),
			},
			AggregationDetail: &atp.AggregationDetail{
				AggregationLevel: aggregationStatus,
			},
		}

		if aps.invocation != nil && aps.invocation.PrimaryBuild != nil {
			tr.PrimaryBuildInfo = aps.invocation.PrimaryBuild
		}

		entries = append(entries, &atp.BatchInsertEntry{TestResult: tr, Token: token})
		token = token + 1
	}

	return entries, token, nil
}

// UploadToAnts uploads test results to Ants.
func (aps *AntsPublishService) UploadToAnts(ctx context.Context) error {
	log.Printf("Uploading to AnTS: %+v", aps.results)

	var entries []*atp.BatchInsertEntry
	token := int64(0)
	for _, result := range aps.results {
		log.Printf("looking at result: %+v", result)

		// Add a module workunit
		mwu, err := aps.insertModuleWorkUnit(result.GetParentTest(), "TF_MODULE", aps.metadata.GetParentWorkUnitId())
		if err != nil {
			return err
		}

		var childEntries []*atp.BatchInsertEntry
		childEntries, token, err = aps.resultEntries(mwu, token, result.GetChildTestCaseResults())
		if err != nil {
			return err
		}
		entries = append(entries, childEntries...)
	}

	request := &atp.TestResultBatchInsertRequest{
		TestResults:     entries,
		InsertBatchSize: int64(len(entries)),
	}

	result, err := aps.service.TestResultService.BatchInsert(ctx, aps.metadata.AntsInvocationId, request)
	if err != nil {
		return err
	}
	log.Printf("BatchInsert test results response: %d", result.ServerResponse.HTTPStatusCode)

	return aps.uploadInvocationProperties()
}

func (aps *AntsPublishService) invocationProperties() ([]*atp.Property, error) {
	var props []*atp.Property
	dutInfo := aps.metadata.GetPrimaryExecutionInfo().GetDutInfo()

	var model *labapi.DutModel
	switch dutInfo.GetDut().GetDutType().(type) {
	case *labapi.Dut_Android_:
		model = dutInfo.GetDut().GetAndroid().GetDutModel()
	case *labapi.Dut_Chromeos:
		model = dutInfo.GetDut().GetChromeos().GetDutModel()
	default:
		return nil, fmt.Errorf("unsupported dut type")
	}

	props = append(props, &atp.Property{Name: "board", Value: model.GetBuildTarget()})
	props = append(props, &atp.Property{Name: "model", Value: model.GetModelName()})

	for k, v := range dutInfo.GetTags() {
		props = append(props, &atp.Property{Name: k, Value: v})
	}

	if aps.metadata.GetLuciInvocationId() != "" {
		props = append(props, &atp.Property{Name: "luci-invocation-id", Value: aps.metadata.GetLuciInvocationId()})
	}

	return props, nil
}

func (aps *AntsPublishService) uploadInvocationProperties() error {
	props, err := aps.invocationProperties()
	if err != nil {
		return err
	}

	aps.invocation.Properties = append(aps.invocation.Properties, props...)
	inv, err := aps.service.InvocationService.Update(aps.invocation.InvocationId, aps.invocation)
	if err != nil {
		return err
	}

	aps.invocation = inv
	return nil
}

func (aps *AntsPublishService) UploadArtifacts(ctx context.Context) error {
	log.Printf("Uploading artifacts from: %s", artifactsDir)

	return filepath.Walk(artifactsDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error walking artifactsDir: %q", err)
			return err
		}

		// Skip directories.
		if info.IsDir() {
			return nil
		}

		artifactMetadata, err := aps.uploadArtifact(path)
		if err != nil {
			log.Printf("Cannot open file: %s due to error: %q. Skipping upload", path, err)
		} else {
			log.Printf("Uploaded artifact for: %s", artifactMetadata.Name)
		}
		return nil
	})
}

func (aps *AntsPublishService) uploadArtifact(path string) (*atp.BuildArtifactMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	am := aps.artifactMetadata(path)
	return aps.service.TestArtifactsService.Update(am.Name, f, am)
}

func (aps *AntsPublishService) artifactMetadata(path string) *atp.BuildArtifactMetadata {
	filename := filepath.Base(path)
	if strings.HasSuffix(path, "log.txt") {
		// Multiple path names have the same log.txt file
		// Use the whole path for filename instead for log files.
		filename = strings.ReplaceAll(strings.TrimPrefix(path, artifactsDir), "/", "_")
	}

	// Mime type is of the form `text/plain; charset utf-8`
	// Just use the content type from this.
	contentType := strings.Split(mime.TypeByExtension(filepath.Ext(path)), ";")[0]

	return &atp.BuildArtifactMetadata{
		Name:         filename,
		InvocationId: aps.metadata.AntsInvocationId,
		WorkUnitId:   aps.metadata.ParentWorkUnitId,
		ContentType:  contentType,
		ArtifactType: artifactType(filename),
	}
}

// artifactType gets the artifact type for the given file
func artifactType(path string) string {
	if strings.Contains(path, "device_logcat") {
		return "logcat"
	} else if strings.Contains(path, "adb_log") {
		return "adb log"
	} else if strings.Contains(path, "host_log") {
		return "host log"
	} else if strings.Contains(path, "perfetto") {
		return "perfetto"
	} else if strings.Contains(path, "xml") {
		return "xml"
	}

	return ""
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
	if len(req.GetTestResponse().GetGivenTestResults()) == 0 {
		return fmt.Errorf("no given test results to upload")
	}

	m, err := unpackMetadata(req)
	if err != nil {
		return errors.Wrap(err, "could not unpack metadata")
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
