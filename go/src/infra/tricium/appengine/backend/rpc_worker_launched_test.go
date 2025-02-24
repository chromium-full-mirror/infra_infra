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

func TestWorkerLaunchedRequest(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()
		helloUbuntu := "Hello_Ubuntu"

		workflowProvider := &mockWorkflowProvider{
			Workflow: &admin.Workflow{
				Workers: []*admin.Worker{
					{
						Name:     helloUbuntu,
						Needs:    tricium.Data_GIT_FILE_DETAILS,
						Provides: tricium.Data_RESULTS,
					},
				},
			},
		}

		// Add pending workflow run entity.
		request := &track.AnalyzeRequest{}
		assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
		requestKey := ds.KeyForObj(ctx, request)
		workflowRun := &track.WorkflowRun{ID: 1, Parent: requestKey}
		assert.Loosely(t, ds.Put(ctx, workflowRun), should.BeNil)
		workflowRunKey := ds.KeyForObj(ctx, workflowRun)
		assert.Loosely(t, ds.Put(ctx, &track.WorkflowRunResult{
			ID:     1,
			Parent: workflowRunKey,
			State:  tricium.State_PENDING,
		}), should.BeNil)

		// Mark workflow as launched and add tracking entities for workers.
		err := workflowLaunched(ctx, &admin.WorkflowLaunchedRequest{
			RunId: request.ID,
		}, workflowProvider)
		assert.Loosely(t, err, should.BeNil)

		// Mark worker as launched.
		err = workerLaunched(ctx, &admin.WorkerLaunchedRequest{
			RunId:  request.ID,
			Worker: helloUbuntu,
		})
		assert.Loosely(t, err, should.BeNil)

		t.Run("Marks worker as launched", func(t *ftt.Test) {
			functionName, _, err := track.ExtractFunctionPlatform(helloUbuntu)
			assert.Loosely(t, err, should.BeNil)
			functionRunKey := ds.NewKey(ctx, "FunctionRun", functionName, 0, workflowRunKey)
			workerKey := ds.NewKey(ctx, "WorkerRun", helloUbuntu, 0, functionRunKey)
			wr := &track.WorkerRunResult{ID: 1, Parent: workerKey}
			err = ds.Get(ctx, wr)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, wr.State, should.Equal(tricium.State_RUNNING))
			fr := &track.FunctionRunResult{ID: 1, Parent: functionRunKey}
			err = ds.Get(ctx, fr)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, fr.State, should.Equal(tricium.State_RUNNING))
		})

		t.Run("Validates request", func(t *ftt.Test) {
			// Validate run ID.
			s := &trackerServer{}
			_, err = s.WorkerLaunched(ctx, &admin.WorkerLaunchedRequest{})
			assert.Loosely(t, err.Error(), should.Equal("rpc error: code = InvalidArgument desc = missing run ID"))

			// Validate worker.
			_, err = s.WorkerLaunched(ctx, &admin.WorkerLaunchedRequest{
				RunId: request.ID,
			})
			assert.Loosely(t, err.Error(), should.Equal("rpc error: code = InvalidArgument desc = missing worker"))

			// Validate buildbucket.
			_, err = s.WorkerLaunched(ctx, &admin.WorkerLaunchedRequest{
				RunId:              request.ID,
				Worker:             helloUbuntu,
				BuildbucketBuildId: 12,
			})
			assert.Loosely(t, err, should.BeNil)
		})
	})
}
