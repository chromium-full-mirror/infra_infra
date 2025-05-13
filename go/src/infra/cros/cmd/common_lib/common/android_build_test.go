// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"strings"
	"testing"
)

func TestGetABOTAPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		buildId     string
		buildTarget string
		board       string
		want        string
	}{
		{
			name:        "basic valid input",
			buildId:     "12345678",
			buildTarget: "brya-trunk_staging-userdebug",
			board:       "brya",
			want:        fmt.Sprintf("%s12345678/brya-trunk_staging-userdebug/brya-ota-12345678.zip", AndroidBuildPrefix),
		},
		{
			name:        "different board and target",
			buildId:     "98765",
			buildTarget: "dedede-some_branch-user",
			board:       "dedede",
			want:        fmt.Sprintf("%s98765/dedede-some_branch-user/dedede-ota-98765.zip", AndroidBuildPrefix),
		},
		{
			name:        "empty inputs", // Although unlikely in practice, test edge case
			buildId:     "",
			buildTarget: "",
			board:       "",
			want:        fmt.Sprintf("%s//-ota-.zip", AndroidBuildPrefix),
		},
		{
			name:        "inputs with spaces", // Test if spaces are handled (they are just inserted)
			buildId:     "1 1",
			buildTarget: "target with space",
			board:       "board space",
			want:        fmt.Sprintf("%s1 1/target with space/board space-ota-1 1.zip", AndroidBuildPrefix),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GetABOTAPath(tt.buildId, tt.buildTarget, tt.board)
			if got != tt.want {
				t.Errorf("GetABOTAPath(%q, %q, %q) = %q; want %q", tt.buildId, tt.buildTarget, tt.board, got, tt.want)
			}
		})
	}
}

func TestParseAndroidPath(t *testing.T) {
	tests := []struct {
		name             string
		path             string
		wantBuildID      string
		wantBuildTarget  string
		wantArtifactName string
		wantErr          bool
		wantErrMsg       string
	}{
		{
			name:             "valid path",
			path:             "android-build/build_explorer/artifacts_list/12345/aosp_arm64-userdebug/image.zip",
			wantBuildID:      "12345",
			wantBuildTarget:  "aosp_arm64-userdebug",
			wantArtifactName: "image.zip",
			wantErr:          false,
		},
		{
			name:             "valid path with artifact having slashes",
			path:             "android-build/build_explorer/artifacts_list/abc/def/some/nested/artifact.bin",
			wantBuildID:      "abc",
			wantBuildTarget:  "def",
			wantArtifactName: "some/nested/artifact.bin",
			wantErr:          false,
		},
		{
			name:       "invalid prefix",
			path:       "wrong-prefix/123/target/artifact.zip",
			wantErr:    true,
			wantErrMsg: "does not have expected prefix",
		},
		{
			name:       "path too short",
			path:       "android-build/build_explorer/artifacts_list/12345/targetonly",
			wantErr:    true,
			wantErrMsg: "path format invalid after prefix",
		},
		{
			name:       "path just prefix",
			path:       "android-build/build_explorer/artifacts_list/",
			wantErr:    true,
			wantErrMsg: "path format invalid after prefix",
		},
		{
			name:       "empty path",
			path:       "",
			wantErr:    true,
			wantErrMsg: "does not have expected prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBuildID, gotBuildTarget, gotArtifactName, err := ParseAndroidPath(tt.path)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseAndroidPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("ParseAndroidPath() error = %q, want error containing %q", err.Error(), tt.wantErrMsg)
			}

			if !tt.wantErr {
				if gotBuildID != tt.wantBuildID {
					t.Errorf("ParseAndroidPath() gotBuildID = %v, want %v", gotBuildID, tt.wantBuildID)
				}
				if gotBuildTarget != tt.wantBuildTarget {
					t.Errorf("ParseAndroidPath() gotBuildTarget = %v, want %v", gotBuildTarget, tt.wantBuildTarget)
				}
				if gotArtifactName != tt.wantArtifactName {
					t.Errorf("ParseAndroidPath() gotArtifactName = %v, want %v", gotArtifactName, tt.wantArtifactName)
				}
			}
		})
	}
}
