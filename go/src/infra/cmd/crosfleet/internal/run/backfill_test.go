// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/structpb"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	"go.chromium.org/infra/cmd/crosfleet/internal/buildbucket"
	"go.chromium.org/infra/cmd/crosfleet/internal/common"
	crosbb "go.chromium.org/infra/cros/lib/buildbucket"
)

func TestRemoveBackfills(t *testing.T) {
	t.Parallel()
	backfillBuild := &buildbucketpb.Build{
		Tags: []*buildbucketpb.StringPair{
			{Key: "crosfleet-tool", Value: "backfill"}}}
	nonBackfillBuild := &buildbucketpb.Build{
		Tags: []*buildbucketpb.StringPair{
			{Key: "crosfleet-tool", Value: "suite"}}}

	wantFilteredBuilds := []*buildbucketpb.Build{nonBackfillBuild}
	gotFilteredBuilds := removeBackfills([]*buildbucketpb.Build{
		backfillBuild, nonBackfillBuild})
	if diff := cmp.Diff(wantFilteredBuilds, gotFilteredBuilds, common.CmpOpts); diff != "" {
		t.Errorf("unexpected diff (%s)", diff)
	}
}

var testBackfillTagsData = []struct {
	build     *buildbucketpb.Build
	wantTags  map[string]string
	qsAccount string
}{
	{
		build: &buildbucketpb.Build{
			Id: 1,
			Tags: []*buildbucketpb.StringPair{
				{Key: "foo", Value: "bar"},
				{Key: "baz", Value: "lol"},
				{Key: "quota_account", Value: "original_account"},
			}},
		wantTags: map[string]string{
			"foo":            "bar",
			"baz":            "lol",
			"crosfleet-tool": "backfill",
			"backfill":       "1",
			"quota_account":  "new_account",
			"user_agent":     "crosfleet",
		},
		qsAccount: "new_account",
	},
	{
		build: &buildbucketpb.Build{
			Id: 2,
			Tags: []*buildbucketpb.StringPair{
				{Key: "bar", Value: "foo"},
				{Key: "lol", Value: "baz"},
				{Key: "backfill", Value: "3"},
				{Key: "crosfleet-tool", Value: "suite"},
				{Key: "quota_account", Value: "original_account"},
			}},
		wantTags: map[string]string{
			"bar":            "foo",
			"lol":            "baz",
			"crosfleet-tool": "backfill",
			"backfill":       "2",
			"quota_account":  "original_account",
			"user_agent":     "crosfleet",
		},
	},
}

func TestBackfillTags(t *testing.T) {
	t.Parallel()
	for _, tt := range testBackfillTagsData {
		tt := tt
		t.Run(fmt.Sprintf("(%s)", tt.wantTags), func(t *testing.T) {
			t.Parallel()
			r := backfillRun{
				qsAccount: tt.qsAccount,
			}
			gotTags := r.backfillTags(tt.build)
			if diff := cmp.Diff(tt.wantTags, gotTags); diff != "" {
				t.Errorf("unexpected diff (%s)", diff)
			}
		})
	}
}

// End to end tests.
func TestBackfill_ByTags(t *testing.T) {
	r := backfillRun{
		buildTags: map[string]string{
			"suite":       "bvt-installer",
			"build":       "asurada-release/R112-15357.0.0",
			"label-board": "asurada",
		},
		allowDupes:         false,
		skipConfirmation:   true,
		releaseRetryUrgent: true,
	}
	ctx := context.Background()

	directSchedProps, err := structpb.NewStruct(map[string]interface{}{
		"requests": map[string]interface{}{
			"default": map[string]interface{}{
				"params": map[string]interface{}{
					"scheduling": map[string]interface{}{
						"qsAccount": "release_direct_sched",
					},
					"softwareDependencies": []interface{}{
						map[string]interface{}{
							"chromeosBuildGcsBucket": "chromeos-image-archive",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Error(err)
	}

	p0Props, err := structpb.NewStruct(directSchedProps.AsMap())
	if err != nil {
		t.Error(err)
	}
	if err := crosbb.SetProperty(p0Props,
		"requests.default.params.scheduling.qsAccount",
		"release_p0"); err != nil {
		t.Error(err)
	}

	bb := &buildbucket.FakeClient{
		ExpectedGetAllBuildsWithTags: []*buildbucket.ExpectedGetWithTagsCall{
			{
				Tags: map[string]string{
					"build":       "asurada-release/R112-15357.0.0",
					"suite":       "bvt-installer",
					"label-board": "asurada",
				},
				Response: []*buildbucketpb.Build{
					{
						Id: 1000,
						Builder: &buildbucketpb.BuilderID{
							Builder: "cros_test_platform",
						},
						Status: buildbucketpb.Status_FAILURE,
						Tags: []*buildbucketpb.StringPair{
							{
								Key:   "build",
								Value: "asurada-release/R112-15357.0.0",
							},
							{
								Key:   "suite",
								Value: "bvt-installer",
							},
							{
								Key:   "label-board",
								Value: "asurada",
							},
						},
						Input: &buildbucketpb.Build_Input{
							Properties: directSchedProps,
						},
					}, {
						Id: 2000,
						Builder: &buildbucketpb.BuilderID{
							Builder: "cros_test_platform",
						},
						Status: buildbucketpb.Status_SUCCESS,
						Tags: []*buildbucketpb.StringPair{
							{
								Key:   "build",
								Value: "asurada-release/R112-15357.0.0",
							},
							{
								Key:   "suite",
								Value: "bvt-installer",
							},
							{
								Key:   "label-board",
								Value: "asurada",
							},
							{
								Key:   "arbitrary-tag",
								Value: "foo",
							},
						},
						Input: &buildbucketpb.Build_Input{
							Properties: directSchedProps,
						},
					}, {
						Builder: &buildbucketpb.BuilderID{
							Builder: "cros_test_platform",
						},
						Status: buildbucketpb.Status_FAILURE,
						Tags: []*buildbucketpb.StringPair{
							{
								Key:   "backfill",
								Value: "0",
							},
							{
								Key:   "crosfleet-tool",
								Value: "backfill",
							},
							{
								Key:   "build",
								Value: "asurada-release/R112-15357.0.0",
							},
							{
								Key:   "suite",
								Value: "bvt-installer",
							},
							{
								Key:   "label-board",
								Value: "asurada",
							},
						},
						Input: &buildbucketpb.Build_Input{
							Properties: directSchedProps,
						},
					},
				},
			},
		},
		ExpectedAnyIncompleteBuildsWithTags: []*buildbucket.ExpectedGetWithTagsCall{
			{
				Tags: map[string]string{
					"backfill":      "1000",
					"quota_account": releaseP0QSaccount,
				},
				Response: []*buildbucketpb.Build{
					{
						Builder: &buildbucketpb.BuilderID{
							Builder: "cros_test_platform",
						},
						Status: buildbucketpb.Status_SCHEDULED,
						Tags: []*buildbucketpb.StringPair{
							{
								Key:   "build",
								Value: "asurada-release/R112-15357.0.0",
							},
							{
								Key:   "suite",
								Value: "bvt-installer",
							},
							{
								Key:   "label-board",
								Value: "asurada",
							},
							{
								Key:   "backfill",
								Value: "1000",
							},
						},
						Input: &buildbucketpb.Build_Input{
							Properties: p0Props,
						},
					},
				},
			},
			{
				Tags: map[string]string{
					"backfill":      "2000",
					"quota_account": releaseP0QSaccount,
				},
				// No existing backfills, should backfill this build.
				Response: nil,
			},
		},
		ExpectedScheduleBuild: []*buildbucket.ExpectedScheduleCall{
			{
				Tags: map[string]string{
					// Tags are copied over from the original build.
					"arbitrary-tag":  "foo",
					"backfill":       "2000",
					"build":          "asurada-release/R112-15357.0.0",
					"crosfleet-tool": "backfill",
					"label-board":    "asurada",
					"suite":          "bvt-installer",
					"user_agent":     "crosfleet",
					"quota_account":  releaseP0QSaccount,
				},
				Props: p0Props.AsMap(),
				Response: &buildbucketpb.Build{
					Id: 1,
				},
			},
		},
	}
	if err := r.innerRun(nil, nil, ctx, bb); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBackfill_ByTags_AllowDupes(t *testing.T) {
	r := backfillRun{
		buildTags: map[string]string{
			"suite":       "bvt-installer",
			"build":       "asurada-release/R112-15357.0.0",
			"label-board": "asurada",
		},
		releaseRetryUrgent: true,
		allowDupes:         true,
	}
	ctx := context.Background()

	inputProps, err := structpb.NewStruct(map[string]interface{}{
		"requests": map[string]interface{}{
			"default": map[string]interface{}{
				"params": map[string]interface{}{
					"scheduling": map[string]interface{}{
						"qsAccount": "release_direct_sched",
					},
					"softwareDependencies": []interface{}{
						map[string]interface{}{
							"chromeosBuildGcsBucket": "chromeos-image-archive",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Error(err)
	}

	expectedProps, err := structpb.NewStruct(inputProps.AsMap())
	if err != nil {
		t.Error(err)
	}
	if err := crosbb.SetProperty(expectedProps,
		"requests.default.params.scheduling.qsAccount",
		"release_p0"); err != nil {
		t.Error(err)
	}

	bb := &buildbucket.FakeClient{
		ExpectedGetAllBuildsWithTags: []*buildbucket.ExpectedGetWithTagsCall{
			{
				Tags: map[string]string{
					"build":       "asurada-release/R112-15357.0.0",
					"suite":       "bvt-installer",
					"label-board": "asurada",
				},
				Response: []*buildbucketpb.Build{
					{
						Builder: &buildbucketpb.BuilderID{
							Builder: "cros_test_platform",
						},
						Status: buildbucketpb.Status_SCHEDULED,
						Tags: []*buildbucketpb.StringPair{
							{
								Key:   "build",
								Value: "asurada-release/R112-15357.0.0",
							},
							{
								Key:   "suite",
								Value: "bvt-installer",
							},
							{
								Key:   "label-board",
								Value: "asurada",
							},
							{
								Key:   "user_agent",
								Value: "recipe",
							},
						},
						Input: &buildbucketpb.Build_Input{
							Properties: inputProps,
						},
					},
				},
			},
		},
		ExpectedScheduleBuild: []*buildbucket.ExpectedScheduleCall{
			{
				Tags: map[string]string{
					"crosfleet-tool": "backfill",
					"backfill":       "0",
					"build":          "asurada-release/R112-15357.0.0",
					"suite":          "bvt-installer",
					"label-board":    "asurada",
					"user_agent":     "crosfleet",
					"quota_account":  "release_p0",
				},
				Props: expectedProps.AsMap(),
				Response: &buildbucketpb.Build{
					Id: 1,
				},
			},
		},
	}
	if err := r.innerRun(nil, nil, ctx, bb); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBackfill_ByTags_DryRun(t *testing.T) {
	r := backfillRun{
		buildTags: map[string]string{
			"suite":       "bvt-installer",
			"build":       "asurada-release/R112-15357.0.0",
			"label-board": "asurada",
		},
		// Simplify the test, allow dupes.
		allowDupes: true,
		dryrun:     true,
	}
	ctx := context.Background()

	inputProps, err := structpb.NewStruct(map[string]interface{}{
		"request": "foo",
	})
	if err != nil {
		t.Error(err)
	}

	bb := &buildbucket.FakeClient{
		ExpectedGetAllBuildsWithTags: []*buildbucket.ExpectedGetWithTagsCall{
			{
				Tags: map[string]string{
					"build":       "asurada-release/R112-15357.0.0",
					"suite":       "bvt-installer",
					"label-board": "asurada",
				},
				Response: []*buildbucketpb.Build{
					{
						Builder: &buildbucketpb.BuilderID{
							Builder: "cros_test_platform",
						},
						Status: buildbucketpb.Status_SCHEDULED,
						Tags: []*buildbucketpb.StringPair{
							{
								Key:   "build",
								Value: "asurada-release/R112-15357.0.0",
							},
							{
								Key:   "suite",
								Value: "bvt-installer",
							},
							{
								Key:   "label-board",
								Value: "asurada",
							},
						},
						Input: &buildbucketpb.Build_Input{
							Properties: inputProps,
						},
					},
				},
			},
		},
	}
	if err := r.innerRun(nil, nil, ctx, bb); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
