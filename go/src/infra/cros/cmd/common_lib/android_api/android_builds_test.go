// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"reflect"
	"testing"
)

func TestUpdateTestCases_extractBuildNumber(t *testing.T) {
	jsonResponse := `{
		"nextPageToken": "MCwxNzI2MTI1MzQzMTI3LCJicnlhLXRydW5rX3N0YWdpbmctdXNlcmRlYnVnIiwxLCIwMDAwMDAwMDAwMDAxMjM1ODIyNSIsImJyeWEtdHJ1bmtfc3RhZ2luZy11c2VyZGVidWci",
		"builds": [
			{
				"buildId": "12358225",
				"target": {}
			}
		]
	}`
	buildNumber, err := extractBuildNumber(jsonResponse)
	if err != nil {
		t.Errorf("extractBuildNumber() error = %v", err)
	}
	if buildNumber != 12358225 {
		t.Errorf("extractBuildNumber() = %v, want %v", buildNumber, "12358225")
	}
}

func TestExtractBuildInfo(t *testing.T) {
	tests := []struct {
		name    string
		resp    string
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid response",
			resp:    `{"builds": [{"buildId": "123", "branch": "test-branch"}]}`,
			want:    map[string]interface{}{"buildId": "123", "branch": "test-branch"},
			wantErr: false,
		},
		{
			name:    "empty builds array",
			resp:    `{"builds": []}`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty JSON",
			resp:    ``,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			resp:    `{"builds": [{"buildId": "123"`,
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractBuildInfo(tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractBuildInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractBuildInfo() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractBuildNumber(t *testing.T) {
	tests := []struct {
		name    string
		resp    string
		want    int
		wantErr bool
	}{
		{
			name:    "valid response",
			resp:    `{"builds": [{"buildId": "123"}]}`,
			want:    123,
			wantErr: false,
		},
		{
			name:    "buildId as string",
			resp:    `{"builds": [{"buildId": "456"}]}`,
			want:    456,
			wantErr: false,
		},
		{
			name:    "empty builds array",
			resp:    `{"builds": []}`,
			want:    0,
			wantErr: true,
		},
		{
			name:    "missing buildId field",
			resp:    `{"builds": [{}]}`,
			want:    0,
			wantErr: true,
		},
		{
			name:    "buildId is not a string",
			resp:    `{"builds": [{"buildId": 1011}]}`,
			want:    0,
			wantErr: true,
		},
		{
			name:    "buildId is empty string",
			resp:    `{"builds": [{"buildId": ""}]}`,
			want:    0,
			wantErr: true,
		},
		{
			name:    "buildId is not a number",
			resp:    `{"builds": [{"buildId": "abc"}]}`,
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			resp:    `{"builds": [{"buildId": "123"`,
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractBuildNumber(tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractBuildNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractBuildNumber() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractBuildBranch(t *testing.T) {
	tests := []struct {
		name    string
		resp    string
		want    string
		wantErr bool
	}{
		{
			name:    "valid response",
			resp:    `{"builds": [{"buildId": "123", "branch": "test-branch"}]}`,
			want:    "test-branch",
			wantErr: false,
		},
		{
			name:    "empty builds array",
			resp:    `{"builds": []}`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "missing branch field",
			resp:    `{"builds": [{"buildId": "456"}]}`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "branch is not a string",
			resp:    `{"builds": [{"buildId": "789", "branch": 123}]}`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "branch is empty string",
			resp:    `{"builds": [{"buildId": "101", "branch": ""}]}`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			resp:    `{"builds": [{"buildId": "123", "branch": "test"`,
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractBuildBranch(tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractBuildBranch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractBuildBranch() got = %v, want %v", got, tt.want)
			}
		})
	}
}
