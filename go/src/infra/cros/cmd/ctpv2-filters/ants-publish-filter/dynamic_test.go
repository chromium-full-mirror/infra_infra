// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"log"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
)

func TestSkipTFUpload(t *testing.T) {
	testCases := []struct {
		name  string
		alRun bool
	}{
		{
			name:  "alRun",
			alRun: true,
		},
		{
			name: "non-AL",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &api.InternalTestplan{
				SuiteInfo: &api.SuiteInfo{
					SuiteMetadata: &api.SuiteMetadata{
						ExecutionMetadata: &api.ExecutionMetadata{
							Args: []*api.Arg{{Flag: "ants_invocation_id", Value: "test"}},
						},
					},
				},
			}
			log := log.New(os.Stdout, "test", 1)
			skipTFUpload(req, tc.alRun, log)

			wantArgs := 1
			if tc.alRun {
				wantArgs = 2
			}
			gotArgs := req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().GetArgs()
			if len(gotArgs) != wantArgs {
				t.Errorf("Unexpected number of args: got %d want %d", len(gotArgs), wantArgs)
			}

			if tc.alRun {
				wantArg := &api.Arg{Flag: skipTFUploadFlag, Value: "true"}
				if diff := cmp.Diff(gotArgs[1], wantArg, protocmp.Transform()); diff != "" {
					t.Errorf("Unexpected diff: %s", diff)
				}
			}
		})
	}
}

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

	log := log.New(os.Stdout, "test", 1)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isInternal(tc.accountID, log)
			if got != tc.want {
				t.Errorf("Unexpected. want %v got %v", tc.want, got)
			}
		})
	}
}

func TestGeneratePublishTask(t *testing.T) {
	testCases := []struct {
		name   string
		du     []*api.UserDefinedDynamicUpdate
		wantDu int
		alRun  bool
	}{
		{
			name: "existingDU",
			du: []*api.UserDefinedDynamicUpdate{
				{UpdateAction: &api.UpdateAction{Action: &api.UpdateAction_Insert_{}}},
			},
			wantDu: 2,
			alRun:  true,
		},
		{
			name:   "missingDU",
			wantDu: 1,
			alRun:  true,
		},
		{
			name:   "nonAL",
			wantDu: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var alRunArg *api.Arg
			if tc.alRun {
				alRunArg = &api.Arg{Flag: alRunKey, Value: "true"}
			}
			req := &api.InternalTestplan{
				SuiteInfo: &api.SuiteInfo{
					SuiteMetadata: &api.SuiteMetadata{
						DynamicUpdates: tc.du,
						ExecutionMetadata: &api.ExecutionMetadata{
							Args: []*api.Arg{alRunArg},
						},
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
			if len(du) != tc.wantDu {
				t.Errorf("Unexpected dynamic updates length. got %d want %d", len(du), tc.wantDu)
			}
		})
	}
}

func TestSkipAntsPublish(t *testing.T) {
	testCases := []struct {
		name     string
		metadata *metadata.PublishAntsMetadata
		alRun    bool
		wantSkip bool
	}{
		{
			name:     "missingAccountID",
			metadata: &metadata.PublishAntsMetadata{},
			alRun:    true,
			wantSkip: false,
		},
		{
			name: "externalPartner",
			metadata: &metadata.PublishAntsMetadata{
				AccountId: "2",
			},
			alRun:    true,
			wantSkip: true,
		},
		{
			name: "missingInv",
			metadata: &metadata.PublishAntsMetadata{
				AccountId: "1",
			},
			wantSkip: true,
		},
		{
			name: "success",
			metadata: &metadata.PublishAntsMetadata{
				AccountId: "1",
			},
			alRun: true,
		},
	}

	log := log.New(os.Stdout, "test", 1)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotSkip := skipAntsPublish(tc.metadata, tc.alRun, log)

			if gotSkip != tc.wantSkip {
				t.Errorf("Unexpected error: got %v want %v", gotSkip, tc.wantSkip)
			}

		})
	}
}
