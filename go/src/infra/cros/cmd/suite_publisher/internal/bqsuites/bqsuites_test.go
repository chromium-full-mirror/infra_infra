// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package bqsuites

import (
	"errors"
	"testing"

	"cloud.google.com/go/bigquery"
	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cros/cmd/suite_publisher/internal/suite"
	"go.chromium.org/infra/cros/cmd/suite_publisher/test"
)

func TestSavePublishInfo(t *testing.T) {
	type wantInfo struct {
		values   map[string]bigquery.Value
		dedupeID string
		err      error
	}

	for _, tc := range []struct {
		publishInfo *PublishInfo
		want        wantInfo
	}{
		{
			publishInfo: &PublishInfo{
				Suite: suite.NewSuite(test.ExampleSuite()),
				Build: BuildInfo{
					BuildTarget:   "example_build_target",
					CrosVersion:   "15755.0.0",
					CrosMilestone: "123",
				},
			},
			want: wantInfo{
				values: map[string]bigquery.Value{
					"build_target":   "example_build_target",
					"cros_version":   "15755.0.0",
					"cros_milestone": "123",
					"id":             "example_suite",
					"bug_component":  "b:123456",
					"criteria":       "This is an example suite",
					"owners":         []string{"example@chromium.org", "example2@chromium.org"},
					"test_ids":       []string{"example_test_0", "example_test_1", "example_test_2"},
					"suites":         []string{},
					"suite_sets":     []string{},
				},
				dedupeID: "example_suite.example_build_target.15755.0.0",
				err:      nil,
			},
		},
		{
			publishInfo: &PublishInfo{
				Suite: suite.NewSuiteSet(test.ExampleSuiteSet()),
				Build: BuildInfo{
					BuildTarget:   "example_build_target_2",
					CrosVersion:   "15754.0.0",
					CrosMilestone: "122",
				},
			},
			want: wantInfo{
				values: map[string]bigquery.Value{
					"build_target":   "example_build_target_2",
					"cros_version":   "15754.0.0",
					"cros_milestone": "122",
					"id":             "example_suite_set",
					"bug_component":  "b:123456",
					"criteria":       "This is an example suite set",
					"owners":         []string{"example@chromium.org", "example2@chromium.org"},
					"test_ids":       []string{},
					"suites":         []string{"example_suite"},
					"suite_sets":     []string{"example_suite_set_b"},
				},
				dedupeID: "example_suite_set.example_build_target_2.15754.0.0",
				err:      nil,
			},
		},
		{
			publishInfo: &PublishInfo{
				Suite: suite.NewSuiteSet(test.ExampleSuiteSet()),
				Build: BuildInfo{
					BuildTarget:   "example_build_target_2",
					CrosVersion:   "15754.0.0",
					CrosMilestone: "122",
				},
			},
			want: wantInfo{
				values: map[string]bigquery.Value{
					"build_target":   "example_build_target_2",
					"cros_version":   "15754.0.0",
					"cros_milestone": "122",
					"id":             "example_suite_set",
					"bug_component":  "b:123456",
					"criteria":       "This is an example suite set",
					"owners":         []string{"example@chromium.org", "example2@chromium.org"},
					"test_ids":       []string{},
					"suites":         []string{"example_suite"},
					"suite_sets":     []string{"example_suite_set_b"},
				},
				dedupeID: "example_suite_set.example_build_target_2.15754.0.0",
				err:      nil,
			},
		},
	} {
		t.Run(tc.publishInfo.Suite.ID(), func(t *testing.T) {
			gotValues, gotDedupeID, err := tc.publishInfo.Save()
			if !errors.Is(err, tc.want.err) {
				t.Errorf("Save() got error: %v, want: %v", err, tc.want.err)
			}
			if gotDedupeID != tc.want.dedupeID {
				t.Errorf("Save() got dedupeID: %q, want: %q", gotDedupeID, tc.want.dedupeID)
			}
			if diff := cmp.Diff(gotValues, tc.want.values); diff != "" {
				t.Errorf("Save() got values mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestSaveSuiteClosure(t *testing.T) {
	type wantInfo struct {
		values   map[string]bigquery.Value
		dedupeID string
		err      error
	}

	for _, tc := range []struct {
		closure ClosurePublishInfo
		want    wantInfo
	}{
		{
			closure: ClosurePublishInfo{
				Closure: &suite.SuiteClosure{
					ID:    "test_1",
					Child: "test_2",
					Depth: 1,
					Path:  "test_1 > test_2",
				},
				Build: BuildInfo{
					BuildTarget:   "example_build_target",
					CrosVersion:   "15755.0.0",
					CrosMilestone: "123",
				},
			},
			want: wantInfo{
				values: map[string]bigquery.Value{
					"build_target":   "example_build_target",
					"cros_version":   "15755.0.0",
					"cros_milestone": "123",
					"id":             "test_1",
					"child":          "test_2",
					"depth":          1,
					"path":           "test_1 > test_2",
				},
				dedupeID: "",
				err:      nil,
			},
		},
	} {
		t.Run(tc.closure.Closure.ID, func(t *testing.T) {
			gotValues, gotDedupeID, err := tc.closure.Save()
			if !errors.Is(err, tc.want.err) {
				t.Errorf("Save() got error: %v, want: %v", err, tc.want.err)
			}
			if gotDedupeID != tc.want.dedupeID {
				t.Errorf("Save() got dedupeID: %q, want: %q", gotDedupeID, tc.want.dedupeID)
			}
			if diff := cmp.Diff(gotValues, tc.want.values); diff != "" {
				t.Errorf("Save() got values mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
