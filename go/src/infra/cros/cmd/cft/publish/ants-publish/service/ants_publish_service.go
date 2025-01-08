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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	androidlib "infra/cros/cmd/common_lib/android_api"
	atp "infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
	"infra/cros/cmd/common_lib/common"
)

const (
	artifactsDir      = "/tmp/artifacts/"
	ancestorsPropName = "ancestor_buildbucket_ids"
	luciInvPropName   = "luci_invocation_id"
	defaultChunkSize  = 1000
	internalAccountID = 1
)

type AntsPublishService struct {
	metadata *metadata.PublishAntsMetadata
	results  []*api.CrosTestResponse_GivenTestResult
	service  *androidlib.Service
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

	s, err := androidService(ctx, m.GetAtpEnvironment())
	if err != nil {
		return nil, err
	}

	return &AntsPublishService{
		metadata: m,
		results:  req.GetTestResponse().GetGivenTestResults(),
		service:  s,
	}, nil
}

func androidService(ctx context.Context, env metadata.PublishAntsMetadata_ATPEnvironment) (*androidlib.Service, error) {
	log.Printf("Getting android service for env: %s", env.String())
	switch env {
	case metadata.PublishAntsMetadata_ENV_STAGING:
		log.Printf("Getting android service for staging env.")
		return androidlib.NewAndroidBuildService(ctx, androidlib.SERVICEACCOUNT, common.Staging)
	default:
		return androidlib.NewAndroidBuildService(ctx, androidlib.SERVICEACCOUNT, common.Prod)
	}
}

func (aps *AntsPublishService) insertModuleWorkUnit(name string, wuType string, parent string) (*atp.WorkUnit, error) {
	start := time.Now()
	defer timeTrack(start, fmt.Sprintf("insert workunit with name: %s type: %s", name, wuType))

	dutProps, _, err := aps.testProperties()
	if err != nil {
		return nil, err
	}

	wu := &atp.WorkUnit{
		Name:         name,
		Type:         wuType,
		ParentId:     parent,
		InvocationId: aps.metadata.GetAntsInvocationId(),
		Properties:   dutProps,
	}

	return aps.service.WorkUnitService.Insert(wu)
}

func (aps *AntsPublishService) resultEntries(ctx context.Context, module *atp.WorkUnit, token int64, results []*api.TestCaseResult, buildInfo *atp.BuildDescriptor) ([]*atp.BatchInsertEntry, int64, error) {
	tcWorkunits := make(map[string]string)
	dutProps, testIdentifierProps, err := aps.testProperties()
	if err != nil {
		log.Printf("Cannot find dut properties due to: %q", err)
	}

	var entries []*atp.BatchInsertEntry
	for _, result := range results {
		names := strings.SplitN(result.GetTestCaseId().GetValue(), "#", 2)
		var parentWUID string
		var testID *atp.TestIdentifier
		if len(names) == 2 {
			// Add test class name as parent ID for now. This will be later
			// replaced by the actual id once wu is created.
			tcWorkunits[names[0]] = ""
			parentWUID = names[0]
			testID = &atp.TestIdentifier{
				Module:           module.Name,
				ModuleParameters: testIdentifierProps,
				TestClass:        names[0],
				Method:           names[1],
			}
		} else if len(names) == 1 {
			testID = &atp.TestIdentifier{
				Module:           module.Name,
				ModuleParameters: testIdentifierProps,
				TestClass:        module.Name,
				Method:           names[0],
			}
			parentWUID = module.Id
		} else {
			return nil, token, fmt.Errorf("unexpected testcaseid: %s", result.GetTestCaseId().GetValue())
		}

		tr := aps.antsResult(result, dutProps, parentWUID, testID, buildInfo)
		entries = append(entries, &atp.BatchInsertEntry{TestResult: tr, Token: token})
		token = token + 1
	}

	log.Printf("Create %d parent test class workunits in parallel", len(tcWorkunits))
	var mu sync.Mutex
	g, _ := errgroup.WithContext(ctx)
	for wuName := range tcWorkunits {
		g.Go(func() error {
			parentwu, err := aps.insertModuleWorkUnit(wuName, "TF_TEST_RUN", module.Id)
			if err != nil {
				log.Printf("unable to create test run workunit for %s due to %q", wuName, err)
				return err
			}

			mu.Lock()
			tcWorkunits[wuName] = parentwu.Id
			mu.Unlock()
			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return nil, token, err
	}

	log.Print("Replace parent names with workunit ids.")
	for _, entry := range entries {
		if v, ok := tcWorkunits[entry.TestResult.WorkUnitId]; ok {
			entry.TestResult.WorkUnitId = v
		}
	}

	return entries, token, err
}

func (aps *AntsPublishService) antsResult(result *api.TestCaseResult, props []*atp.Property, parentWUID string, testID *atp.TestIdentifier, buildInfo *atp.BuildDescriptor) *atp.TestResult {
	var skipReason *atp.SkippedReason
	if result.Reason != "" {
		skipReason = &atp.SkippedReason{ReasonMessage: result.Reason}
	}

	var debugInfo *atp.DebugInfo
	if len(result.GetErrors()) > 0 {
		debugInfo = &atp.DebugInfo{
			// Use the first error as it is the primary error and
			// ants only allows one error.
			ErrorMessage: result.GetErrors()[0].GetMessage(),
		}
	}

	startTime := result.GetStartTime().AsTime().UnixMilli()
	duration := result.GetDuration().AsDuration().Milliseconds()
	return &atp.TestResult{
		InvocationId:     aps.metadata.GetAntsInvocationId(),
		WorkUnitId:       parentWUID,
		PrimaryBuildInfo: buildInfo,
		TestIdentifier:   testID,
		TestStatus:       antsTestStatus(result),
		Timing: &atp.Timing{
			CreationTimestamp: startTime,
			CompleteTimestamp: startTime + duration,
		},
		Properties:    props,
		SkippedReason: skipReason,
		DebugInfo:     debugInfo,
	}
}

func (aps *AntsPublishService) uploadResults(ctx context.Context, entries []*atp.BatchInsertEntry, chunkSize int) error {
	start := time.Now()
	defer timeTrack(start, "upload test results")

	var chunks [][]*atp.BatchInsertEntry
	for i := 0; i < len(entries); i += chunkSize {
		end := i + chunkSize
		if end > len(entries) {
			end = len(entries)
		}
		chunks = append(chunks, entries[i:end])
	}

	g, ctx := errgroup.WithContext(ctx)
	for i, chunk := range chunks {
		request := &atp.TestResultBatchInsertRequest{
			TestResults:     chunk,
			InsertBatchSize: int64(len(chunk)),
		}
		g.Go(func() error {
			log.Printf("worker %d start", i)
			if _, err := aps.service.TestResultService.BatchInsert(ctx, aps.metadata.AntsInvocationId, request); err != nil {
				return err
			}

			log.Printf("worker %d done", i)
			return nil
		})
	}

	return g.Wait()
}

func (aps *AntsPublishService) updateParentWorkUnitProperties() error {
	start := time.Now()
	defer timeTrack(start, "Parent WU props update")
	pwu, err := aps.service.WorkUnitService.Get(aps.metadata.ParentWorkUnitId)
	if err != nil {
		return err
	}

	_, props, err := aps.testProperties()
	if err != nil {
		return err
	}

	pwu.Properties = append(pwu.Properties, props...)
	_, err = aps.service.WorkUnitService.Update(pwu.Id, pwu)
	return err
}

// UploadToAnts uploads test results to Ants.
func (aps *AntsPublishService) UploadToAnts(ctx context.Context) error {
	log.Printf("Uploading to AnTS: %+v", aps.results)

	if !isInternalAccount(aps.metadata.GetAccountId()) {
		return nil
	}

	if len(aps.results) == 0 {
		log.Println("no given test results to upload. Skipping results upload")
		return nil
	}

	log.Printf("Update parent workunit properties.")
	if err := aps.updateParentWorkUnitProperties(); err != nil {
		return err
	}

	// Track time taken to upload results
	start := time.Now()
	defer timeTrack(start, "Overall result upload")

	var buildInfo *atp.BuildDescriptor
	inv, err := aps.service.InvocationService.Get(aps.metadata.AntsInvocationId)
	if err != nil {
		log.Printf("Could not get invocation id. skipping adding build info to results due to: %q. ", err)
	} else {
		buildInfo = inv.PrimaryBuild
	}

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
		childEntries, token, err = aps.resultEntries(ctx, mwu, token, result.GetChildTestCaseResults(), buildInfo)
		if err != nil {
			return err
		}
		entries = append(entries, childEntries...)
	}

	return aps.uploadResults(ctx, entries, defaultChunkSize)
}

func (aps *AntsPublishService) testProperties() ([]*atp.Property, []*atp.Property, error) {
	dutInfo := aps.metadata.GetPrimaryExecutionInfo().GetDutInfo()
	var model *labapi.DutModel
	var sku string
	switch dutInfo.GetDut().GetDutType().(type) {
	case *labapi.Dut_Android_:
		model = dutInfo.GetDut().GetAndroid().GetDutModel()
	case *labapi.Dut_Chromeos:
		model = dutInfo.GetDut().GetChromeos().GetDutModel()
		sku = dutInfo.GetDut().GetChromeos().GetSku()
	default:
		return nil, nil, fmt.Errorf("unsupported dut type")
	}

	testIdentifierProps := []*atp.Property{
		{Name: "board", Value: model.GetBuildTarget()},
		{Name: "model", Value: model.GetModelName()},
		{Name: "sku", Value: sku},
	}

	props := []*atp.Property{
		{Name: luciInvPropName, Value: aps.metadata.GetLuciInvocationId()},
	}

	for k, v := range dutInfo.GetTags() {
		props = append(props, &atp.Property{Name: k, Value: v})
	}

	for k, v := range aps.metadata.GetSchedulingMetadata().GetSchedulingArgs() {
		props = append(props, &atp.Property{Name: k, Value: v})
	}

	props = append(props, testIdentifierProps...)
	return props, testIdentifierProps, nil
}

func (aps *AntsPublishService) UploadArtifacts(ctx context.Context) error {
	if !isInternalAccount(aps.metadata.GetAccountId()) {
		return nil
	}

	if _, err := os.Stat(artifactsDir); err != nil {
		log.Printf("%s does not exists. Skipping artifacts upload.", artifactsDir)
		return nil
	}

	log.Printf("Uploading artifacts from: %s", artifactsDir)

	// Track time taken to upload artifacts
	start := time.Now()
	defer timeTrack(start, "Overall artifacts upload")

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
	// Mime type is of the form `text/plain; charset utf-8`
	// Just use the content type from this.
	contentType := strings.Split(mime.TypeByExtension(filepath.Ext(path)), ";")[0]

	return &atp.BuildArtifactMetadata{
		Name:         strings.TrimPrefix(path, artifactsDir),
		InvocationId: aps.metadata.AntsInvocationId,
		WorkUnitId:   aps.metadata.ParentWorkUnitId,
		ContentType:  contentType,
		ArtifactType: artifactType(path),
	}
}

// artifactType gets the artifact type for the given file
func artifactType(path string) string {
	filename := filepath.Base(path)
	if strings.Contains(filename, "device_logcat") {
		return "logcat"
	} else if strings.Contains(filename, "adb_log") {
		return "adb log"
	} else if strings.Contains(filename, "host_log") {
		return "host log"
	} else if strings.Contains(filename, "perfetto") {
		return "perfetto"
	} else if strings.Contains(filename, "xml") {
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
	m, err := unpackMetadata(req)
	if err != nil {
		return errors.Wrap(err, "could not unpack metadata")
	}

	if m.GetAntsInvocationId() == "" {
		return fmt.Errorf("ants invocation id is required")
	} else if m.GetParentWorkUnitId() == "" {
		return fmt.Errorf("parent workunit id is required")
	}

	return nil
}

func timeTrack(start time.Time, msg string) {
	elapsed := time.Since(start)
	log.Printf("%s took: %s", msg, elapsed)
}

func isInternalAccount(accountID string) bool {
	// Sometimes, we do not have accountId for internal users.
	if accountID == "" {
		log.Printf("accountID is empty")
		return true
	}

	id, err := strconv.Atoi(accountID)
	if err != nil {
		log.Printf("Cannot convert accountID %s to int", accountID)
		return false
	}

	if id < 1 {
		log.Printf("Account ID %s not supported", accountID)
		return false
	}

	return id == internalAccountID
}

// unpackMetadata unpacks the Any metadata field into PublishGcsMetadata
func unpackMetadata(req *api.PublishRequest) (*metadata.PublishAntsMetadata, error) {
	var m metadata.PublishAntsMetadata
	if err := req.Metadata.UnmarshalTo(&m); err != nil {
		return &m, fmt.Errorf("improperly formatted input proto metadata: %w", err)
	}
	return &m, nil
}
