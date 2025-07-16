// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGetTesthausURL(t *testing.T) {
	t.Parallel()

	ftt.Run("Test get Testhaus URL", t, func(t *ftt.Test) {
		tests := []struct {
			invocationName      string
			gcsURL              string
			wantTesthausPostfix string
		}{
			{
				invocationName:      "invocations/inv-123",
				gcsURL:              "gs://test_bucket/test_prefix",
				wantTesthausPostfix: "invocations/inv-123",
			},
			{
				invocationName:      "invocations/inv-123",
				gcsURL:              "",
				wantTesthausPostfix: "invocations/inv-123",
			},
			{
				invocationName:      "",
				gcsURL:              "gs://test_bucket/test_prefix",
				wantTesthausPostfix: "test_bucket/test_prefix",
			},
			{
				invocationName:      "",
				gcsURL:              "gs://",
				wantTesthausPostfix: "",
			},
			{
				invocationName:      "",
				gcsURL:              "",
				wantTesthausPostfix: "",
			},
		}
		for _, tc := range tests {

			t.Run(fmt.Sprintf("When the invocation is: %q and gcsURL is: %q should get Testhaus URL postfix: %q", tc.invocationName, tc.gcsURL, tc.wantTesthausPostfix), func(t *ftt.Test) {
				wantTesthausURL := fmt.Sprintf("%s%s", TesthausURLPrefix, tc.wantTesthausPostfix)
				gotTesthausURL := GetTesthausURL(tc.invocationName, tc.gcsURL)
				assert.Loosely(t, gotTesthausURL, should.Equal(wantTesthausURL))
			})
		}
	})
}

func TestHasExecutionMetadataKey(t *testing.T) {
	t.Parallel()
	ftt.Run("Test HasExecutionMetadataKey", t, func(t *ftt.Test) {
		testCases := []struct {
			name       string
			testSuites []*api.TestSuite
			key        string
			want       bool
		}{
			{
				name:       "nil test suites",
				testSuites: nil,
				key:        "some-key",
				want:       false,
			},
			{
				name:       "empty test suites",
				testSuites: []*api.TestSuite{},
				key:        "some-key",
				want:       false,
			},
			{
				name: "key exists",
				testSuites: []*api.TestSuite{
					{
						ExecutionMetadata: &api.ExecutionMetadata{
							Args: []*api.Arg{
								{Flag: "some-key", Value: "some-value"},
							},
						},
					},
				},
				key:  "some-key",
				want: true,
			},
			{
				name: "key does not exist",
				testSuites: []*api.TestSuite{
					{
						ExecutionMetadata: &api.ExecutionMetadata{
							Args: []*api.Arg{
								{Flag: "other-key", Value: "other-value"},
							},
						},
					},
				},
				key:  "some-key",
				want: false,
			},
			{
				name: "no args",
				testSuites: []*api.TestSuite{
					{
						ExecutionMetadata: &api.ExecutionMetadata{
							Args: []*api.Arg{},
						},
					},
				},
				key:  "some-key",
				want: false,
			},
			{
				name: "nil args",
				testSuites: []*api.TestSuite{
					{
						ExecutionMetadata: &api.ExecutionMetadata{
							Args: nil,
						},
					},
				},
				key:  "some-key",
				want: false,
			},
			{
				name: "nil execution metadata",
				testSuites: []*api.TestSuite{
					{
						ExecutionMetadata: nil,
					},
				},
				key:  "some-key",
				want: false,
			},
			{
				name: "multiple suites, key in first",
				testSuites: []*api.TestSuite{
					{ExecutionMetadata: &api.ExecutionMetadata{Args: []*api.Arg{{Flag: "some-key"}}}},
					{ExecutionMetadata: &api.ExecutionMetadata{Args: []*api.Arg{{Flag: "other-key"}}}},
				},
				key:  "some-key",
				want: true,
			},
			{
				name: "multiple suites, key not in first",
				testSuites: []*api.TestSuite{
					{ExecutionMetadata: &api.ExecutionMetadata{Args: []*api.Arg{{Flag: "other-key"}}}},
					{ExecutionMetadata: &api.ExecutionMetadata{Args: []*api.Arg{{Flag: "some-key"}}}},
				},
				key:  "some-key",
				want: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *ftt.Test) {
				got := HasExecutionMetadataKey(tc.testSuites, tc.key)
				assert.Loosely(t, got, should.Equal(tc.want))
			})
		}
	})
}

func TestGetProductName(t *testing.T) {
	t.Parallel()

	ftt.Run("Test get product name", t, func(t *ftt.Test) {

		testCases := []struct {
			name     string
			dutModel *labapi.DutModel
			botDims  []*buildbucketpb.StringPair
			build    string
			isALRun  bool
			wantName string
		}{
			{
				name: "Variant exists with product in DUT model",
				dutModel: &labapi.DutModel{
					BuildTarget: "brya",
					ModelName:   "gimble",
				},
				botDims:  nil,
				build:    "brya-arc-t/R133-16104.0.0",
				isALRun:  false,
				wantName: "brya.gimble-arc-t",
			},
			{
				name:     "Variant exists with product in bot dims",
				dutModel: &labapi.DutModel{},
				botDims: []*buildbucketpb.StringPair{
					{Key: "label-board", Value: "brya"},
					{Key: "label-model", Value: "gimble"},
				},
				build:    "brya-kernelnext/R133-16104.0.0",
				isALRun:  false,
				wantName: "brya.gimble-kernelnext",
			},
			{
				name: "No variant",
				dutModel: &labapi.DutModel{
					BuildTarget: "brya",
					ModelName:   "gimble",
				},
				botDims:  nil,
				build:    "brya/R133-16104.0.0",
				isALRun:  false,
				wantName: "brya.gimble",
			},
			{
				name: "Variant with release build",
				dutModel: &labapi.DutModel{
					BuildTarget: "brya",
					ModelName:   "gimble",
				},
				botDims:  nil,
				build:    "brya-kernelnext-release/R133-16104.0.0",
				isALRun:  false,
				wantName: "brya.gimble-kernelnext",
			},
			{
				name:     "Missing board name",
				dutModel: &labapi.DutModel{ModelName: "gimble"},
				botDims:  nil,
				build:    "brya/R133-16104.0.0",
				isALRun:  false,
				wantName: "gimble",
			},
			{
				name:     "Missing model name",
				dutModel: &labapi.DutModel{BuildTarget: "brya"},
				botDims:  nil,
				build:    "brya/R133-16104.0.0",
				isALRun:  false,
				wantName: "brya",
			},
			{
				name:     "Empty build",
				dutModel: &labapi.DutModel{BuildTarget: "brya", ModelName: "gimble"},
				botDims:  nil,
				build:    "",
				isALRun:  false,
				wantName: "brya.gimble",
			},
			{
				name:     "Board name exists only in bot dimensions",
				dutModel: &labapi.DutModel{ModelName: "gimble"},
				botDims: []*buildbucketpb.StringPair{
					{Key: "label-board", Value: "brya"},
				},
				build:    "brya/build7",
				isALRun:  false,
				wantName: "brya.gimble",
			},
			{
				name:     "Model name exists only in bot dimensions",
				dutModel: &labapi.DutModel{BuildTarget: "brya"},
				botDims: []*buildbucketpb.StringPair{
					{Key: "label-model", Value: "gimble"},
				},
				build:    "brya/build8",
				isALRun:  false,
				wantName: "brya.gimble",
			},
			{
				name: "AL run with variant in build string",
				dutModel: &labapi.DutModel{
					BuildTarget: "brya",
					ModelName:   "gimble",
				},
				botDims:  nil,
				build:    "brya-arc-t/R133-16104.0.0",
				isALRun:  true,
				wantName: "brya.gimble",
			},
			{
				name:     "nil dutModel",
				dutModel: nil,
				botDims: []*buildbucketpb.StringPair{
					{Key: "label-board", Value: "brya"},
					{Key: "label-model", Value: "gimble"},
				},
				build:    "brya-kernelnext/R133-16104.0.0",
				isALRun:  false,
				wantName: "brya.gimble-kernelnext",
			},
			{
				name:     "no board or model",
				dutModel: &labapi.DutModel{},
				botDims:  nil,
				build:    "brya-kernelnext/R133-16104.0.0",
				isALRun:  false,
				wantName: "",
			},
			{
				name:     "nil dutModel and botDims",
				dutModel: nil,
				botDims:  nil,
				build:    "brya-kernelnext/R133-16104.0.0",
				isALRun:  false,
				wantName: "",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *ftt.Test) {
				got := GetProductName(tc.dutModel, tc.botDims, tc.build, tc.isALRun)
				assert.Loosely(t, got, should.Equal(tc.wantName))
			})
		}
	})
}

func TestSanitizeGCSPrefix(t *testing.T) {
	t.Parallel()

	ftt.Run("Test SanitizeGCSPrefix", t, func(t *ftt.Test) {
		testCases := []struct {
			name       string
			prefix     string
			wantPrefix string
		}{
			{
				name:       "with trailing '/' removes the char",
				prefix:     "gs://test-results-bucket/",
				wantPrefix: "gs://test-results-bucket",
			},
			{
				name:       "without trailing '/' returns unchanged",
				prefix:     "gs://test-results-bucket",
				wantPrefix: "gs://test-results-bucket",
			},
			{
				name:       "with two trailing '/' removes one char",
				prefix:     "gs://test-results-bucket//",
				wantPrefix: "gs://test-results-bucket/",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *ftt.Test) {
				got := SanitizeGCSPrefix(tc.prefix)
				assert.Loosely(t, got, should.Equal(tc.wantPrefix))
			})
		}
	})
}
