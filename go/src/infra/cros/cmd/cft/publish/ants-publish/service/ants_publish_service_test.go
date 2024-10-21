// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service provides the API handlers for ants publish.
package service

import (
	"slices"
	"testing"

	"google.golang.org/protobuf/types/known/anypb"

	storage_path "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
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
