// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
// Package service provides the API handlers for ants publish.
package service

import (
	"context"
	"slices"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/googleapi"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"

	storage_path "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	androidlib "infra/cros/cmd/common_lib/android_api"
	mock_androidapi "infra/cros/cmd/common_lib/android_api/mocks"
	atp "infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
)

func TestAntsStatus(t *testing.T) {
	testCases := []struct {
		name   string
		result *api.TestCaseResult
		want   string
	}{
		{
			name:   "pass",
			result: &api.TestCaseResult{Verdict: &api.TestCaseResult_Pass_{}},
			want:   "pass",
		},
		{
			name:   "fail",
			result: &api.TestCaseResult{Verdict: &api.TestCaseResult_Fail_{}},
			want:   "fail",
		},
		{
			name: "assumptionFailure",
			result: &api.TestCaseResult{
				Verdict: &api.TestCaseResult_Pass_{},
				Errors: []*api.TestCaseResult_Error{
					{Message: "err"},
				},
			},
			want: "assumptionFailure",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := antsTestStatus(tc.result)
			t.Log(tc.result)
			t.Log(tc.result.GetVerdict())
			if got != tc.want {
				t.Errorf("TestAntsStatus: got %v want %v", got, tc.want)
			}
		})
	}
}
func TestValidateAntsPublishRequest(t *testing.T) {
	defaultResult := &api.TestCaseResult{TestCaseId: &api.TestCase_Id{Value: "test"}}
	testCases := []struct {
		name    string
		request *metadata.PublishAntsMetadata
		gtr     *api.CrosTestResponse_GivenTestResult
		wantErr bool
	}{
		{
			name: "missingInvocation",
			request: &metadata.PublishAntsMetadata{
				ParentWorkUnitId: "WU1",
				AccountId:        "1",
			},
			wantErr: true,
		},
		{
			name: "missingParentWU",
			request: &metadata.PublishAntsMetadata{
				AntsInvocationId: "I1234",
				AccountId:        "1",
			},
			wantErr: true,
		},
		{
			name: "missingAccountID",
			request: &metadata.PublishAntsMetadata{
				ParentWorkUnitId: "WU1",
				AntsInvocationId: "I1234",
			},
			wantErr: true,
		},
		{
			name: "givenResult",
			request: &metadata.PublishAntsMetadata{
				ParentWorkUnitId: "WU1",
				AntsInvocationId: "I1234",
				AccountId:        "1",
			},
			gtr: &api.CrosTestResponse_GivenTestResult{
				ParentTest:           "parent",
				ChildTestCaseResults: []*api.TestCaseResult{defaultResult},
			},
		},
		{
			name: "missingResults",
			request: &metadata.PublishAntsMetadata{
				ParentWorkUnitId: "WU1",
				AntsInvocationId: "I1234",
				AccountId:        "1",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metadata := &anypb.Any{}
			err := metadata.MarshalFrom(tc.request)
			if err != nil {
				t.Error(err)
			}
			req := &api.PublishRequest{
				ArtifactDirPath: &storage_path.StoragePath{Path: "gs://test", HostType: storage_path.StoragePath_LOCAL},
				Metadata:        metadata,
				TestResponse: &api.CrosTestResponse{
					GivenTestResults: []*api.CrosTestResponse_GivenTestResult{tc.gtr},
				},
			}
			gotErr := validateAntsPublishRequest(req)
			if (tc.wantErr && gotErr == nil) || (gotErr != nil && !tc.wantErr) {
				t.Errorf("Unexpected error. want: %v, got %v", tc.wantErr, gotErr)
			}
		})
	}
}

func TestArtifactMetadata(t *testing.T) {
	testCases := []struct {
		name      string
		path      string
		wantTypes []string
	}{
		{
			name:      "log",
			path:      "provision/foo/log.txt",
			wantTypes: []string{"text/plain"},
		},
		{
			name:      "xml",
			path:      "test/tf/result.xml",
			wantTypes: []string{"application/xml", "text/xml"},
		},
	}
	aps := &AntsPublishService{
		metadata: &metadata.PublishAntsMetadata{AntsInvocationId: "I123", ParentWorkUnitId: "WU1"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := aps.artifactMetadata(tc.path)
			if got.Name != tc.path {
				t.Errorf("Unexpected name. want %s got %s", tc.path, got.Name)
			}
			if !slices.Contains(tc.wantTypes, got.ContentType) {
				t.Errorf("Unexpected content type. want %s got %s", tc.wantTypes, got.ContentType)
			}
		})
	}
}
func TestArtifactType(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		wantType string
	}{
		{
			name:     "logcat",
			path:     "device_logcat_setup_satlab-0wgatfqi22088039.txt",
			wantType: "logcat",
		},
		{
			name:     "adblog",
			path:     "host_adb_log-0wgatfqi22088039.txt",
			wantType: "adb log",
		},
		{
			name:     "hostlog",
			path:     "end_host_log-0wgatfqi22088039.txt",
			wantType: "host log",
		},
		{
			name:     "perfetto",
			path:     "invocation-tract_perfetto-trace.gz",
			wantType: "perfetto",
		},
		{
			name:     "xml",
			path:     "tf_result.xml.gz",
			wantType: "xml",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotType := artifactType(tc.path)
			if gotType != tc.wantType {
				t.Errorf("Unexpected artifact type. want %s got %s", tc.wantType, gotType)
			}
		})
	}
}
func TestWorkUnitProperties(t *testing.T) {
	testCases := []struct {
		name      string
		dut       *labapi.Dut
		luciInvID string
		wantProps []*atp.Property
	}{
		{
			name: "crosDut",
			dut: &labapi.Dut{
				DutType: &labapi.Dut_Chromeos{
					Chromeos: &labapi.Dut_ChromeOS{
						DutModel: &labapi.DutModel{
							BuildTarget: "brya",
							ModelName:   "mithrax",
						},
					},
				},
			},
			wantProps: []*atp.Property{
				{Name: "board", Value: "brya"},
				{Name: "model", Value: "mithrax"},
			},
		},
		{
			name: "AndroidDut",
			dut: &labapi.Dut{
				DutType: &labapi.Dut_Android_{
					Android: &labapi.Dut_Android{
						DutModel: &labapi.DutModel{
							BuildTarget: "brya",
							ModelName:   "mithrax",
						},
					},
				},
			},
			wantProps: []*atp.Property{
				{Name: "board", Value: "brya"},
				{Name: "model", Value: "mithrax"},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aps := &AntsPublishService{
				metadata: &metadata.PublishAntsMetadata{
					AntsInvocationId: "I123",
					ParentWorkUnitId: "WU1",
					LuciInvocationId: tc.luciInvID,
					PrimaryExecutionInfo: &artifact.ExecutionInfo{
						DutInfo: &artifact.DutInfo{Dut: tc.dut},
					},
				},
			}
			got, err := aps.workunitProperties()
			if err != nil {
				t.Errorf("error calling invocation properties: %q", err)
			}
			if diff := cmp.Diff(tc.wantProps, got, protocmp.Transform()); diff != "" {
				t.Errorf("Unexpected diff: diff: %s", diff)
			}
		})
	}
}

func TestResultEntries(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()
	mockWU := mock_androidapi.NewMockWorkUnitService(mockCtl)
	parentWU := &atp.WorkUnit{Id: "WU1", Name: "Parent WU"}
	returnWUID := "WUTR123"

	dutProps := []*atp.Property{
		{Name: "board", Value: "brya"},
		{Name: "model", Value: "vell"},
	}
	executionInfo := &artifact.ExecutionInfo{
		DutInfo: &artifact.DutInfo{
			Dut: &labapi.Dut{
				DutType: &labapi.Dut_Chromeos{
					Chromeos: &labapi.Dut_ChromeOS{
						DutModel: &labapi.DutModel{BuildTarget: "brya", ModelName: "vell"},
					},
				},
			},
		},
	}
	aps := &AntsPublishService{
		service: &androidlib.Service{
			WorkUnitService: mockWU,
		},
		metadata: &metadata.PublishAntsMetadata{
			PrimaryExecutionInfo: executionInfo,
		},
	}
	buildInfo := &atp.BuildDescriptor{Branch: "git-main_cl_dev"}
	testCases := []struct {
		name        string
		wu          *atp.WorkUnit
		expectWU    *atp.WorkUnit
		results     []*api.TestCaseResult
		wantResults []*atp.TestResult
	}{
		{
			name: "crash",
			results: []*api.TestCaseResult{
				{
					TestCaseId: &api.TestCase_Id{Value: "tradefed.cts.CtsWrapWrapNoDebugTestCases"},
					Verdict:    &api.TestCaseResult_Crash_{},
				},
			},
			wantResults: []*atp.TestResult{
				{
					TestIdentifier: &atp.TestIdentifier{
						Module:           parentWU.Name,
						ModuleParameters: dutProps,
						TestClass:        parentWU.Name,
						Method:           "tradefed.cts.CtsWrapWrapNoDebugTestCases",
					},
					TestStatus:       "testError",
					Properties:       dutProps,
					WorkUnitId:       parentWU.Id,
					Timing:           &atp.Timing{},
					PrimaryBuildInfo: buildInfo,
				},
			},
		},
		{
			name: "skip",
			results: []*api.TestCaseResult{
				{
					TestCaseId: &api.TestCase_Id{Value: "tradefed.cts.CtsWrapWrapNoDebugTestCases"},
					Verdict:    &api.TestCaseResult_Skip_{},
					Reason:     "skip reason",
				},
			},
			wantResults: []*atp.TestResult{
				{
					TestIdentifier: &atp.TestIdentifier{
						Module:           parentWU.Name,
						ModuleParameters: dutProps,
						TestClass:        parentWU.Name,
						Method:           "tradefed.cts.CtsWrapWrapNoDebugTestCases",
					},
					TestStatus:       "testSkipped",
					Properties:       dutProps,
					WorkUnitId:       parentWU.Id,
					Timing:           &atp.Timing{},
					PrimaryBuildInfo: buildInfo,
					SkippedReason:    &atp.SkippedReason{ReasonMessage: "skip reason"},
				},
			},
		},
		{
			name: "Mobly_Pass",
			results: []*api.TestCaseResult{
				{
					TestCaseId: &api.TestCase_Id{Value: "testmethod"},
					Verdict:    &api.TestCaseResult_Pass_{},
				},
			},
			wantResults: []*atp.TestResult{
				{
					TestIdentifier: &atp.TestIdentifier{
						Module:           parentWU.Name,
						ModuleParameters: dutProps,
						TestClass:        parentWU.Name,
						Method:           "testmethod",
					},
					TestStatus:       "pass",
					Properties:       dutProps,
					WorkUnitId:       parentWU.Id,
					Timing:           &atp.Timing{},
					PrimaryBuildInfo: buildInfo,
				},
			},
		},
		{
			name: "TF_Pass",
			results: []*api.TestCaseResult{
				{
					TestCaseId: &api.TestCase_Id{Value: "testcase#testname1"},
					Verdict:    &api.TestCaseResult_Pass_{},
				},
				{
					TestCaseId: &api.TestCase_Id{Value: "testcase#testname2"},
					Verdict:    &api.TestCaseResult_Fail_{},
					Errors:     []*api.TestCaseResult_Error{{Message: "error"}},
				},
			},
			expectWU: &atp.WorkUnit{
				Name:       "testcase",
				ParentId:   parentWU.Id,
				Type:       "TF_TEST_RUN",
				Properties: dutProps,
			},
			wantResults: []*atp.TestResult{
				{
					TestIdentifier: &atp.TestIdentifier{
						Module:           parentWU.Name,
						ModuleParameters: dutProps,
						TestClass:        "testcase",
						Method:           "testname1",
					},
					TestStatus:       "pass",
					Properties:       dutProps,
					WorkUnitId:       returnWUID,
					Timing:           &atp.Timing{},
					PrimaryBuildInfo: buildInfo,
				},
				{
					TestIdentifier: &atp.TestIdentifier{
						Module:           parentWU.Name,
						ModuleParameters: dutProps,
						TestClass:        "testcase",
						Method:           "testname2",
					},
					TestStatus:       "fail",
					Properties:       dutProps,
					WorkUnitId:       returnWUID,
					Timing:           &atp.Timing{},
					PrimaryBuildInfo: buildInfo,
					DebugInfo:        &atp.DebugInfo{ErrorMessage: "error"},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectWU != nil {
				returnWU := &atp.WorkUnit{
					Name:       tc.expectWU.Name,
					Id:         returnWUID,
					ParentId:   tc.expectWU.ParentId,
					Properties: tc.expectWU.Properties,
				}
				mockWU.EXPECT().Insert(tc.expectWU).Return(returnWU, nil)
			}
			gotEntries, gotToken, err := aps.resultEntries(parentWU, 0, tc.results, buildInfo)
			if err != nil {
				t.Errorf("Unexpected error: %q", err)
			}
			if gotToken != int64(len(tc.results)) {
				t.Errorf("Unexpected token: got %d, want %d", gotToken, len(tc.results))
			}

			for i, entry := range gotEntries {
				if diff := cmp.Diff(entry.TestResult, tc.wantResults[i], protocmp.Transform()); diff != "" {
					t.Errorf("%s", diff)
				}
			}
		})
	}
}

func TestUploadResults(t *testing.T) {
	ctx := context.Background()
	invID := "I123"
	okResp := &atp.TestResultBatchInsertResponse{ServerResponse: googleapi.ServerResponse{HTTPStatusCode: 200}}

	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockTRService := mock_androidapi.NewMockTestResultService(mockCtl)
	aps := &AntsPublishService{
		metadata: &metadata.PublishAntsMetadata{AntsInvocationId: invID},
		service:  &androidlib.Service{TestResultService: mockTRService},
	}
	testCases := []struct {
		name         string
		entries      []*atp.BatchInsertEntry
		chunkSize    int
		expectations func()
	}{
		{
			name: "evenChunks",
			entries: []*atp.BatchInsertEntry{
				{TestResult: &atp.TestResult{AttemptNumber: 1}},
				{TestResult: &atp.TestResult{AttemptNumber: 2}},
			},
			chunkSize: 1,
			expectations: func() {
				mockTRService.EXPECT().BatchInsert(ctx, invID, gomock.Any()).Return(okResp, nil).Times(2)
			},
		},
		{
			name: "oddChunks",
			entries: []*atp.BatchInsertEntry{
				{TestResult: &atp.TestResult{AttemptNumber: 1}},
				{TestResult: &atp.TestResult{AttemptNumber: 2}},
				{TestResult: &atp.TestResult{AttemptNumber: 3}},
			},
			chunkSize: 2,
			expectations: func() {
				mockTRService.EXPECT().BatchInsert(ctx, invID, gomock.Any()).Return(okResp, nil).Times(2)
			},
		},
		{
			name: "noChunks",
			entries: []*atp.BatchInsertEntry{
				{TestResult: &atp.TestResult{AttemptNumber: 1}},
				{TestResult: &atp.TestResult{AttemptNumber: 2}},
				{TestResult: &atp.TestResult{AttemptNumber: 3}},
			},
			chunkSize: 5,
			expectations: func() {
				mockTRService.EXPECT().BatchInsert(ctx, invID, gomock.Any()).Return(okResp, nil).Times(1)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectations != nil {
				tc.expectations()
			}

			err := aps.uploadResults(ctx, tc.entries, tc.chunkSize)
			if err != nil {
				t.Errorf("Unexpected error for uploadResults(): %q", err)
			}
		})
	}
}
