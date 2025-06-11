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

func TestGeneratePublishTask(t *testing.T) {
	testCases := []struct {
		name       string
		du         []*api.UserDefinedDynamicUpdate
		alRun      bool
		partnerRun bool
		tfFlag     bool
		wantDu     int
		wantArg    *api.Arg
	}{
		{
			name: "existingDU",
			du: []*api.UserDefinedDynamicUpdate{
				{UpdateAction: &api.UpdateAction{Action: &api.UpdateAction_Insert_{}}},
			},
			alRun:   true,
			tfFlag:  false,
			wantDu:  2,
			wantArg: &api.Arg{Flag: skipTFUploadFlag, Value: "true"},
		},
		{
			name:    "missingDU",
			alRun:   true,
			tfFlag:  false,
			wantDu:  1,
			wantArg: &api.Arg{Flag: skipTFUploadFlag, Value: "true"},
		},
		{
			name: "nonAL",
			// wantDu defaults to 0, which is correct.
			// wantArg is nil, which is correct as nothing is added.
		},
		{
			name:       "partnerTfEnabled",
			alRun:      true,
			partnerRun: true,
			tfFlag:     true,
			wantDu:     1,
			wantArg:    &api.Arg{Flag: skipTFUploadFlag, Value: "true"},
		},
		{
			name:       "partnerTfDisabled",
			alRun:      true,
			partnerRun: true,
			tfFlag:     false,
			wantDu:     1,
			wantArg:    &api.Arg{Flag: skipTFUploadFlag, Value: "true"},
		},
		{
			name:    "tfEnabled",
			alRun:   true,
			tfFlag:  true,
			wantDu:  1,
			wantArg: &api.Arg{Flag: skipTFUploadFlag, Value: "false"},
		},
		{
			name:    "tfDisabled",
			alRun:   true,
			tfFlag:  false,
			wantDu:  1,
			wantArg: &api.Arg{Flag: skipTFUploadFlag, Value: "true"},
		},
	}

	log := log.New(os.Stdout, "test", 1)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var initialArgs []*api.Arg
			var wantLen int
			if tc.alRun {
				initialArgs = append(initialArgs, &api.Arg{Flag: alRunKey, Value: "true"})
				wantLen = 2 // Starts with 1, will end with 2
				if tc.partnerRun {
					initialArgs = append(initialArgs, &api.Arg{Flag: partnerRunKey, Value: "true"})
					wantLen = 3 // Starts with 2, will end with 3
				}
			}

			var execMeta *api.ExecutionMetadata
			if tc.alRun {
				execMeta = &api.ExecutionMetadata{Args: initialArgs}
			}

			req := &api.InternalTestplan{
				SuiteInfo: &api.SuiteInfo{
					SuiteMetadata: &api.SuiteMetadata{
						DynamicUpdates:    tc.du,
						ExecutionMetadata: execMeta,
					},
				},
			}
			m := &metadata.PublishAntsMetadata{IsTfPluginEnabled: tc.tfFlag}

			if tc.alRun {
				if err := GeneratePublishTask(req, m, "path", log); err != nil {
					t.Fatalf("Unexpected error: %q", err)
				}
			}

			du := req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates()
			if len(du) != tc.wantDu {
				t.Errorf("Unexpected dynamic updates length: got %d, want %d", len(du), tc.wantDu)
			}

			var gotArgs []*api.Arg
			if execMeta != nil {
				gotArgs = req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().GetArgs()
			}

			if !tc.alRun {
				if len(gotArgs) != 0 {
					t.Errorf("Expected no args for non-AL run, but got %d", len(gotArgs))
				}
				return
			}

			if len(gotArgs) != wantLen {
				t.Fatalf("Unexpected execution metadata args len: got(%d), want(%d)", len(gotArgs), wantLen)
			}

			lastArg := gotArgs[len(gotArgs)-1]
			if diff := cmp.Diff(tc.wantArg, lastArg, protocmp.Transform()); diff != "" {
				t.Errorf("Unexpected diff in the last added arg (-want +got):\n%s", diff)
			}
		})
	}
}
