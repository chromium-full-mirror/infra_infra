// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
// Package service provides the API handlers for ants publish.
package service

import (
	androidlib "infra/cros/cmd/common_lib/android_api"
	mock_androidapi "infra/cros/cmd/common_lib/android_api/mocks"
	atp "infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
	"slices"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	storage_path "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"
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
		wantName  string
		wantTypes []string
	}{
		{
			name:      "log",
			path:      "provision/foo/log.txt",
			wantName:  "provision_foo_log.txt",
			wantTypes: []string{"text/plain"},
		},
		{
			name:      "xml",
			path:      "test/tf/result.xml",
			wantName:  "result.xml",
			wantTypes: []string{"application/xml", "text/xml"},
		},
	}
	aps := &AntsPublishService{
		metadata: &metadata.PublishAntsMetadata{AntsInvocationId: "I123", ParentWorkUnitId: "WU1"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := aps.artifactMetadata(tc.path)
			if got.Name != tc.wantName {
				t.Errorf("Unexpected name. want %s got %s", tc.wantName, got.Name)
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
func TestUploadInvocationProperties(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()
	mockInv := mock_androidapi.NewMockInvocationService(mockCtl)
	testCases := []struct {
		name    string
		exeInfo *artifact.ExecutionInfo
		wantInv *atp.Invocation
	}{
		{
			name: "satlab",
			exeInfo: &artifact.ExecutionInfo{
				EnvInfo: &artifact.ExecutionInfo_SatlabInfo{
					SatlabInfo: &artifact.SatlabInfo{
						BuildbucketInfo: &artifact.BuildbucketInfo{
							AncestorIds: []int64{int64(123)},
						},
					},
				},
			},
			wantInv: &atp.Invocation{
				InvocationId: "I987654321",
				Properties: []*atp.Property{
					{Name: ancestorsPropName, Value: "123"},
				},
			},
		},
		{
			name: "moreAncestorsSatlab",
			exeInfo: &artifact.ExecutionInfo{
				EnvInfo: &artifact.ExecutionInfo_SatlabInfo{
					SatlabInfo: &artifact.SatlabInfo{
						BuildbucketInfo: &artifact.BuildbucketInfo{
							AncestorIds: []int64{int64(123), int64(456)},
						},
					},
				},
			},
			wantInv: &atp.Invocation{
				InvocationId: "I987654321",
				Properties: []*atp.Property{
					{Name: ancestorsPropName, Value: "123,456"},
				},
			},
		},
		{
			name: "skylab",
			exeInfo: &artifact.ExecutionInfo{
				EnvInfo: &artifact.ExecutionInfo_SkylabInfo{
					SkylabInfo: &artifact.SkylabInfo{
						BuildbucketInfo: &artifact.BuildbucketInfo{
							AncestorIds: []int64{int64(123)},
						},
					},
				},
			},
			wantInv: &atp.Invocation{
				InvocationId: "I987654321",
				Properties: []*atp.Property{
					{Name: ancestorsPropName, Value: "123"},
				},
			},
		},
		{
			name: "missing",
			exeInfo: &artifact.ExecutionInfo{
				EnvInfo: &artifact.ExecutionInfo_SatlabInfo{
					SatlabInfo: &artifact.SatlabInfo{
						BuildbucketInfo: &artifact.BuildbucketInfo{
							AncestorIds: []int64{},
						},
					},
				},
			},
			wantInv: &atp.Invocation{
				InvocationId: "I987654321",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inv := &atp.Invocation{
				InvocationId: "I987654321",
			}
			aps := &AntsPublishService{
				metadata: &metadata.PublishAntsMetadata{
					PrimaryExecutionInfo: tc.exeInfo,
				},
				service:    &androidlib.Service{InvocationService: mockInv},
				invocation: inv,
			}
			// Verify that we get correct args for update call
			mockInv.EXPECT().Update(inv.InvocationId, tc.wantInv).Return(tc.wantInv, nil)
			err := aps.uploadInvocationProperties()
			if err != nil {
				t.Errorf("Error uploading invocation props: %q", err)
			}
		})
	}
}
func TestResultEntries(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()
	mockWU := mock_androidapi.NewMockWorkUnitService(mockCtl)
	parentwuID := "WU1"
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
	testCases := []struct {
		name       string
		wuName     string
		expectWU   *atp.WorkUnit
		result     *api.TestCaseResult
		wantResult *atp.TestResult
	}{
		{
			name:   "crash",
			wuName: "tradefed.cts.CtsWrapWrapNoDebugTestCases",
			result: &api.TestCaseResult{
				TestCaseId: &api.TestCase_Id{Value: "tradefed.cts.CtsWrapWrapNoDebugTestCases"},
				Verdict:    &api.TestCaseResult_Crash_{},
			},
			wantResult: &atp.TestResult{
				TestIdentifier: &atp.TestIdentifier{
					Module:           "tradefed.cts.CtsWrapWrapNoDebugTestCases",
					ModuleParameters: dutProps,
					TestClass:        "tradefed.cts.CtsWrapWrapNoDebugTestCases",
					Method:           "tradefed.cts.CtsWrapWrapNoDebugTestCases",
				},
				TestStatus: "testError",
				Properties: []*atp.Property{
					{Name: "board", Value: "brya"},
					{Name: "model", Value: "vell"},
				},
			},
		},
		{
			name:   "Mobly_Pass",
			wuName: "mobly.CtsWrapWrapNoDebugTestCases",
			result: &api.TestCaseResult{
				TestCaseId: &api.TestCase_Id{Value: "testmethod"},
				Verdict:    &api.TestCaseResult_Pass_{},
			},
			wantResult: &atp.TestResult{
				TestIdentifier: &atp.TestIdentifier{
					Module:           "mobly.CtsWrapWrapNoDebugTestCases",
					ModuleParameters: dutProps,
					TestClass:        "mobly.CtsWrapWrapNoDebugTestCases",
					Method:           "testmethod",
				},
				TestStatus: "pass",
				Properties: dutProps,
			},
		},
		{
			name:   "TF_Pass",
			wuName: "tradefed.cts.CtsWrapWrapNoDebugTestCases",
			result: &api.TestCaseResult{
				TestCaseId: &api.TestCase_Id{Value: "testcase#testname"},
				Verdict:    &api.TestCaseResult_Pass_{},
			},
			expectWU: &atp.WorkUnit{
				Name:       "testcase",
				ParentId:   parentwuID,
				Type:       "TF_TEST_RUN",
				Properties: dutProps,
			},
			wantResult: &atp.TestResult{
				TestIdentifier: &atp.TestIdentifier{
					Module:           "tradefed.cts.CtsWrapWrapNoDebugTestCases",
					ModuleParameters: dutProps,
					TestClass:        "testcase",
					Method:           "testname",
				},
				TestStatus: "pass",
				Properties: dutProps,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wu := &atp.WorkUnit{Id: parentwuID, Name: tc.wuName}
			results := []*api.TestCaseResult{tc.result}
			if tc.expectWU != nil {
				mockWU.EXPECT().Insert(tc.expectWU).Return(wu, nil)
			}
			gotEntries, gotToken, err := aps.resultEntries(wu, 0, results)
			if err != nil {
				t.Errorf("Unexpected error: %q", err)
			}
			if gotToken != int64(len(results)) {
				t.Errorf("Unexpected token: got %d, want %d", gotToken, len(results))
			}
			tc.wantResult.WorkUnitId = parentwuID
			tc.wantResult.Timing = &atp.Timing{}
			if diff := cmp.Diff(gotEntries[0].TestResult, tc.wantResult, protocmp.Transform()); diff != "" {
				t.Errorf("%s", diff)
			}
		})
	}
}
