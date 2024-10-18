// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"
	tq "go.chromium.org/luci/gae/service/taskqueue"

	admin "infra/tricium/api/admin/v1"
	tricium "infra/tricium/api/v1"
	"infra/tricium/appengine/common"
	"infra/tricium/appengine/common/track"
	"infra/tricium/appengine/common/triciumtest"
)

// triggerTestWorkflowProvider mocks common.WorkflowProvider.
type mockWorkflowProvider struct {
	Workflow *admin.Workflow
}

func (p mockWorkflowProvider) GetWorkflow(c context.Context, runID int64) (*admin.Workflow, error) {
	return p.Workflow, nil
}

func TestTriggerRequest(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()
		runID := int64(123456789)

		workflowProvider := &mockWorkflowProvider{
			Workflow: &admin.Workflow{
				Workers: []*admin.Worker{
					{
						Name:     "Hello",
						Needs:    tricium.Data_GIT_FILE_DETAILS,
						Provides: tricium.Data_RESULTS,
						Impl:     &admin.Worker_Recipe{},
					},
				},
			},
		}

		t.Run("Driver trigger request", func(t *ftt.Test) {
			err := trigger(ctx, &admin.TriggerRequest{
				RunId:  runID,
				Worker: "Hello",
			}, workflowProvider, common.MockTaskServerAPI)
			assert.Loosely(t, err, should.BeNil)
			t.Run("Enqueues track request", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.TrackerQueue]), should.Equal(1))
			})
		})
	})
}

func TestHelperFunctions(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		assert.Loosely(t, ds.Put(ctx, &track.AnalyzeRequest{
			ID:            123,
			Project:       "my-luci-config-project-id",
			GitURL:        "http://my-gerrit.com/my-project",
			GitRef:        "refs/changes/97/597/2",
			Files:         []tricium.Data_File{{Path: "README.md"}},
			GerritHost:    "http://my-gerrit-review.com/my-project",
			GerritProject: "my-project",
			GerritChange:  "my-project~master~I8473b95934b5732ac55d26311a706c9c2bde9940",
			CommitMessage: "My CL summary\n\nBug: 123\n",
		}), should.BeNil)

		assert.Loosely(t, ds.Put(ctx, &track.AnalyzeRequest{
			ID:      321,
			Project: "another-luci-config-project-id",
			GitURL:  "http://my-nongerrit.com/repo-url",
			GitRef:  "refs/foo",
			Files:   []tricium.Data_File{{Path: "README.md"}},
		}), should.BeNil)

		t.Run("Tags include Gerrit details for Gerrit requests", func(t *ftt.Test) {
			patch := fetchPatchDetails(ctx, 123)
			assert.Loosely(t, patch.GitilesHost, should.Equal("http://my-gerrit.com/my-project"))
			assert.Loosely(t, patch.GitilesProject, should.Equal("my-luci-config-project-id"))
			assert.Loosely(t, patch.GerritHost, should.Equal("http://my-gerrit-review.com/my-project"))
			assert.Loosely(t, patch.GerritProject, should.Equal("my-project"))
			assert.Loosely(t, patch.GerritChange, should.Equal("my-project~master~I8473b95934b5732ac55d26311a706c9c2bde9940"))
			assert.Loosely(t, patch.GerritCl, should.Equal("597"))
			assert.Loosely(t, patch.GerritPatch, should.Equal("2"))
			assert.Loosely(t, getTags(ctx, "Spacey_UBUNTU", 123, patch), should.Match([]string{
				"function:Spacey",
				"platform:UBUNTU",
				"run_id:123",
				"tricium:1",
				"gerrit_project:my-project",
				"gerrit_change:my-project~master~I8473b95934b5732ac55d26311a706c9c2bde9940",
				"gerrit_cl_number:597",
				"gerrit_patch_set:2",
				"buildset:patch/gerrit/http://my-gerrit-review.com/my-project/597/2",
			}))
		})

		t.Run("Tags omit Gerrit details for non-Gerrit requests", func(t *ftt.Test) {
			patch := fetchPatchDetails(ctx, 321)
			expected := common.PatchDetails{
				GitilesHost:    "http://my-nongerrit.com/repo-url",
				GitilesProject: "another-luci-config-project-id",
			}
			assert.Loosely(t, patch, should.Match(expected))
			assert.Loosely(t, getTags(ctx, "Pylint_UBUNTU", 321, patch), should.Match([]string{
				"function:Pylint",
				"platform:UBUNTU",
				"run_id:321",
				"tricium:1",
			}))
		})

		t.Run("Tags omit Gerrit details if run not found", func(t *ftt.Test) {
			patch := fetchPatchDetails(ctx, 789)
			assert.Loosely(t, patch, should.Match(common.PatchDetails{}))
			assert.Loosely(t, getTags(ctx, "Spacey_UBUNTU", 789, patch), should.Match([]string{
				"function:Spacey",
				"platform:UBUNTU",
				"run_id:789",
				"tricium:1",
			}))
		})

		t.Run("Tags are nil for invalid worker names", func(t *ftt.Test) {
			assert.Loosely(t, getTags(ctx, "invalidworker", 1, common.PatchDetails{}), should.BeNil)
		})
	})
}
