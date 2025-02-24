// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"
	tq "go.chromium.org/luci/gae/service/taskqueue"

	admin "go.chromium.org/infra/tricium/api/admin/v1"
	tricium "go.chromium.org/infra/tricium/api/v1"
	"go.chromium.org/infra/tricium/appengine/common"
	"go.chromium.org/infra/tricium/appengine/common/config"
	"go.chromium.org/infra/tricium/appengine/common/triciumtest"
)

const (
	hello   = "Hello"
	pylint  = "Pylint"
	project = "playground-gerrit-tricium"
)

func TestLaunchRequest(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		cp := &mockConfigProvider{
			Projects: map[string]*tricium.ProjectConfig{
				project: {
					Selections: []*tricium.Selection{
						{
							Function: hello,
							Platform: tricium.Platform_UBUNTU,
						},
						{
							Function: pylint,
							Platform: tricium.Platform_UBUNTU,
						},
					},
				},
			},
			ServiceConfig: &tricium.ServiceConfig{
				BuildbucketServerHost: "cr-buildbucket-dev",
				Platforms: []*tricium.Platform_Details{
					{
						Name:       tricium.Platform_UBUNTU,
						HasRuntime: true,
					},
				},
				DataDetails: []*tricium.Data_TypeDetails{
					{
						Type:               tricium.Data_GIT_FILE_DETAILS,
						IsPlatformSpecific: false,
					},
					{
						Type:               tricium.Data_RESULTS,
						IsPlatformSpecific: true,
					},
				},
				Functions: []*tricium.Function{
					{
						Type:     tricium.Function_ANALYZER,
						Name:     hello,
						Needs:    tricium.Data_GIT_FILE_DETAILS,
						Provides: tricium.Data_RESULTS,
						Impls: []*tricium.Impl{
							{
								ProvidesForPlatform: tricium.Platform_UBUNTU,
								RuntimePlatform:     tricium.Platform_UBUNTU,
								Impl: &tricium.Impl_Recipe{
									Recipe: &tricium.Recipe{
										Project: "chromium",
										Bucket:  "try",
										Builder: "hello-analysis",
									},
								},
							},
						},
					},
					{
						Type:     tricium.Function_ANALYZER,
						Name:     pylint,
						Needs:    tricium.Data_GIT_FILE_DETAILS,
						Provides: tricium.Data_RESULTS,
						Impls: []*tricium.Impl{
							{
								ProvidesForPlatform: tricium.Platform_UBUNTU,
								RuntimePlatform:     tricium.Platform_UBUNTU,
								Impl: &tricium.Impl_Recipe{
									Recipe: &tricium.Recipe{
										Project: "chromium",
										Bucket:  "try",
										Builder: "pylint-analysis",
									},
								},
							},
						},
					},
				},
			},
		}

		runID := int64(123456789)
		t.Run("Launch request", func(t *ftt.Test) {
			err := launch(ctx, &admin.LaunchRequest{
				RunId:   runID,
				Project: project,
				GitRef:  "ref/test",
				Files: []*tricium.Data_File{
					{Path: "README.md"},
					{Path: "README2.md"},
				},
				CommitMessage: "CL summary\n\nBug: 123\n",
			}, cp, common.MockPubSub)
			assert.Loosely(t, err, should.BeNil)

			t.Run("Enqueues track request", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.TrackerQueue]), should.Equal(1))
			})

			t.Run("Stores workflow config", func(t *ftt.Test) {
				wf := &config.Workflow{ID: runID}
				assert.Loosely(t, ds.Get(ctx, wf), should.BeNil)
			})

			t.Run("Enqueues driver requests", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.DriverQueue]), should.Equal(2))
			})

			// Check guard: one more launch request with the same run ID results in no added tasks.
			err = launch(ctx, &admin.LaunchRequest{
				RunId:   runID,
				Project: project,
				GitRef:  "ref/test",
				Files: []*tricium.Data_File{
					{Path: "README.md"},
					{Path: "README2.md"},
				},
				CommitMessage: "CL summary\n\nBug: 123\n",
			}, config.MockProvider, common.MockPubSub)
			assert.Loosely(t, err, should.BeNil)

			t.Run("Succeeding launch request for the same run enqueues no track request", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.TrackerQueue]), should.Equal(1))
			})

		})
	})
}
