// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"log"
	"os"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
)

func TestIsInternal(t *testing.T) {
	testCases := []struct {
		name      string
		accountID string
		want      bool
	}{
		{
			name:      "internal",
			accountID: "1",
			want:      true,
		},
		{
			name:      "external",
			accountID: "2",
			want:      false,
		},
		{
			name:      "missing",
			accountID: "",
			want:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isInternal(tc.accountID)
			if got != tc.want {
				t.Errorf("Unexpected. want %v got %v", tc.want, got)
			}
		})
	}
}

func TestGeneratePublishTask(t *testing.T) {
	testCases := []struct {
		name string
		du   []*api.UserDefinedDynamicUpdate
	}{
		{
			name: "existing",
			du:   []*api.UserDefinedDynamicUpdate{{UpdateAction: &api.UpdateAction{Action: &api.UpdateAction_Insert_{}}}},
		},
		{
			name: "missing",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &api.InternalTestplan{
				SuiteInfo: &api.SuiteInfo{
					SuiteMetadata: &api.SuiteMetadata{
						DynamicUpdates: tc.du,
					},
				},
			}
			m := &metadata.PublishAntsMetadata{}
			log := log.New(os.Stdout, "test", 1)
			err := GeneratePublishTask(req, m, "path", log)
			if err != nil {
				t.Errorf("Unexpected error: %q", err)
			}

			du := req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates()
			if len(du) != len(tc.du)+1 {
				t.Errorf("Unexpected dynamic updates length. got %d want %d", len(du), len(tc.du)+1)
			}
		})
	}
}
