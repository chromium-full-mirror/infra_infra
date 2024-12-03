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

	log := log.New(os.Stdout, "test", 1)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var args []*api.Arg
			if tc.alRun {
				args = []*api.Arg{{Flag: alRunKey, Value: "true"}}
			}
			req := &api.InternalTestplan{
				SuiteInfo: &api.SuiteInfo{
					SuiteMetadata: &api.SuiteMetadata{
						DynamicUpdates: tc.du,
						ExecutionMetadata: &api.ExecutionMetadata{
							Args: args,
						},
					},
				},
			}
			m := &metadata.PublishAntsMetadata{}
			err := GeneratePublishTask(req, m, "path", log)
			if err != nil {
				t.Errorf("Unexpected error: %q", err)
			}

			du := req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates()
			if len(du) != tc.wantDu {
				t.Errorf("Unexpected dynamic updates length. got %d want %d", len(du), tc.wantDu)
			}

			gotArgs := req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().GetArgs()
			if tc.alRun {
				if len(gotArgs) != 2 {
					t.Errorf("Unexpected execution metadata args len: got(%d), want(2)", len(gotArgs))
				}
				wantArg := &api.Arg{Flag: skipTFUploadFlag, Value: "true"}
				if diff := cmp.Diff(gotArgs[1], wantArg, protocmp.Transform()); diff != "" {
					t.Errorf("Unexpected diff: %s", diff)
				}
			} else {
				if len(gotArgs) != 0 {
					t.Errorf("Unexpected execution metadata args len: got(%d), want(0)", len(gotArgs))
				}
			}
		})
	}
}

func TestSkipAntsPublish(t *testing.T) {
	testCases := []struct {
		name     string
		metadata *metadata.PublishAntsMetadata
		wantSkip bool
	}{
		{
			name:     "missingAccountID",
			metadata: &metadata.PublishAntsMetadata{},
			wantSkip: false,
		},
		{
			name: "externalPartner",
			metadata: &metadata.PublishAntsMetadata{
				AccountId: "2",
			},
			wantSkip: true,
		},
		{
			name: "success",
			metadata: &metadata.PublishAntsMetadata{
				AccountId: "1",
			},
		},
	}

	log := log.New(os.Stdout, "test", 1)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotSkip := skipAntsPublish(tc.metadata, log)

			if gotSkip != tc.wantSkip {
				t.Errorf("Unexpected error: got %v want %v", gotSkip, tc.wantSkip)
			}
		})
	}
}
