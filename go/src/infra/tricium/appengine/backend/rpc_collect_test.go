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
	tq "go.chromium.org/luci/gae/service/taskqueue"

	admin "go.chromium.org/infra/tricium/api/admin/v1"
	tricium "go.chromium.org/infra/tricium/api/v1"
	"go.chromium.org/infra/tricium/appengine/common"
	"go.chromium.org/infra/tricium/appengine/common/triciumtest"
)

// Mock task server API that returns a canned task result.
type mockTaskServer struct {
	State common.ResultState
}

func (mockTaskServer) Trigger(c context.Context, params *common.TriggerParameters) (*common.TriggerResult, error) {
	return &common.TriggerResult{}, nil
}
func (m mockTaskServer) Collect(c context.Context, params *common.CollectParameters) (*common.CollectResult, error) {
	return &common.CollectResult{
		State: m.State,
	}, nil
}

func TestCollectRequest(t *testing.T) {
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

		t.Run("Driver collect request for worker without successors", func(t *ftt.Test) {
			err := collect(ctx, &admin.CollectRequest{
				RunId:  runID,
				Worker: "Hello",
			}, workflowProvider, common.MockTaskServerAPI)
			assert.Loosely(t, err, should.BeNil)

			t.Run("Enqueues track request", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.TrackerQueue]), should.Equal(1))
			})

			t.Run("Enqueues no driver request", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.DriverQueue]), should.BeZero)
			})
		})
	})
}

func TestValidateCollectRequest(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		t.Run("A request with run ID and worker name is valid", func(t *ftt.Test) {
			assert.Loosely(t, validateCollectRequest(&admin.CollectRequest{
				RunId:  int64(1234),
				Worker: "Hello",
			}), should.BeNil)
		})

		t.Run("A request missing either run ID or worker name is not valid", func(t *ftt.Test) {
			assert.Loosely(t, validateCollectRequest(&admin.CollectRequest{
				Worker: "Hello",
			}), should.NotBeNil)
			assert.Loosely(t, validateCollectRequest(&admin.CollectRequest{
				RunId: int64(1234),
			}), should.NotBeNil)
		})
	})
}
