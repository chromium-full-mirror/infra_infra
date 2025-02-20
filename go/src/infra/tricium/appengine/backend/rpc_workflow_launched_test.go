// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"

	admin "go.chromium.org/infra/tricium/api/admin/v1"
	tricium "go.chromium.org/infra/tricium/api/v1"
	"go.chromium.org/infra/tricium/appengine/common/track"
	"go.chromium.org/infra/tricium/appengine/common/triciumtest"
)

func TestWorkflowLaunchedRequest(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		analyzerUbuntu := "Analyzer_Ubuntu"

		workflowProvider := &mockWorkflowProvider{
			Workflow: &admin.Workflow{
				Workers: []*admin.Worker{
					{
						Name:     analyzerUbuntu,
						Needs:    tricium.Data_GIT_FILE_DETAILS,
						Provides: tricium.Data_RESULTS,
					},
				},
			},
		}

		t.Run("Trivial workflow with no analyzers is marked done immediately", func(t *ftt.Test) {
			request := &track.AnalyzeRequest{}
			assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
			requestKey := ds.KeyForObj(ctx, request)
			run := &track.WorkflowRun{ID: 1, Parent: requestKey}
			assert.Loosely(t, ds.Put(ctx, run), should.BeNil)
			runKey := ds.KeyForObj(ctx, run)
			runResult := &track.WorkflowRunResult{
				ID:     1,
				Parent: runKey,
				State:  tricium.State_PENDING,
			}
			assert.Loosely(t, ds.Put(ctx, runResult), should.BeNil)

			// Use a mock workflow with no workers (and thus no functions).
			emptyWorkflowProvider := &mockWorkflowProvider{
				Workflow: &admin.Workflow{Workers: nil},
			}

			// Call workflowLaunched.
			err := workflowLaunched(ctx, &admin.WorkflowLaunchedRequest{
				RunId: request.ID,
			}, emptyWorkflowProvider)
			assert.Loosely(t, err, should.BeNil)

			t.Run("Marks workflow run as (trivially) done", func(t *ftt.Test) {
				assert.Loosely(t, ds.Get(ctx, runResult), should.BeNil)
				assert.Loosely(t, runResult.State, should.Equal(tricium.State_SUCCESS))
			})

			t.Run("There are no functions in this fun", func(t *ftt.Test) {
				assert.Loosely(t, ds.Get(ctx, run), should.BeNil)
				assert.Loosely(t, len(run.Functions), should.BeZero)
			})
		})

		t.Run("Workflow request", func(t *ftt.Test) {
			// Add pending workflow run entity.
			request := &track.AnalyzeRequest{}
			assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
			requestKey := ds.KeyForObj(ctx, request)
			run := &track.WorkflowRun{ID: 1, Parent: requestKey}
			assert.Loosely(t, ds.Put(ctx, run), should.BeNil)
			runKey := ds.KeyForObj(ctx, run)
			runResult := &track.WorkflowRunResult{
				ID:     1,
				Parent: runKey,
				State:  tricium.State_PENDING,
			}
			assert.Loosely(t, ds.Put(ctx, runResult), should.BeNil)

			// Mark workflow as launched.
			err := workflowLaunched(ctx, &admin.WorkflowLaunchedRequest{
				RunId: request.ID,
			}, workflowProvider)
			assert.Loosely(t, err, should.BeNil)

			name, _, err := track.ExtractFunctionPlatform(analyzerUbuntu)

			t.Run("Marks workflow run as launched", func(t *ftt.Test) {
				assert.Loosely(t, ds.Get(ctx, runResult), should.BeNil)
				assert.Loosely(t, runResult.State, should.Equal(tricium.State_RUNNING))
			})

			t.Run("Adds list of functions to WorkflowRun", func(t *ftt.Test) {
				assert.Loosely(t, ds.Get(ctx, run), should.BeNil)
				assert.Loosely(t, len(run.Functions), should.Equal(1))
			})

			t.Run("Worker and function is marked pending", func(t *ftt.Test) {
				assert.Loosely(t, err, should.BeNil)
				functionRunKey := ds.NewKey(ctx, "FunctionRun", name, 0, runKey)
				workerKey := ds.NewKey(ctx, "WorkerRun", analyzerUbuntu, 0, functionRunKey)
				wr := &track.WorkerRunResult{ID: 1, Parent: workerKey}
				assert.Loosely(t, ds.Get(ctx, wr), should.BeNil)
				assert.Loosely(t, wr.State, should.Equal(tricium.State_PENDING))
				fr := &track.FunctionRunResult{ID: 1, Parent: functionRunKey}
				assert.Loosely(t, ds.Get(ctx, fr), should.BeNil)
				assert.Loosely(t, fr.State, should.Equal(tricium.State_PENDING))
			})
		})
	})
}
