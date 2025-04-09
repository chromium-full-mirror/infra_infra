// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
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
