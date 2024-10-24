// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package backend

import (
	"testing"
	"time"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	"infra/appengine/arquebus/app/backend/model"
	monorail "infra/monorailv2/api/api_proto"
)

func TestBackend(t *testing.T) {
	t.Parallel()
	assignerID := "test-assigner"

	ftt.Run("scheduleAssignerTaskHandler", t, func(t *ftt.Test) {
		c := createTestContextWithTQ()

		// create a sample assigner with tasks.
		createAssigner(c, t, assignerID)
		tasks := triggerScheduleTaskHandler(c, t, assignerID)
		assert.Loosely(t, tasks, should.NotBeNil)

		t.Run("works", func(t *ftt.Test) {
			for _, task := range tasks {
				assert.Loosely(t, task.Status, should.Equal(model.TaskStatus_Scheduled))
			}
		})

		t.Run("doesn't schedule new tasks for a drained assigner.", func(t *ftt.Test) {
			// TODO(crbug/967519): implement me.
		})
	})

	ftt.Run("runAssignerTaskHandler", t, func(t *ftt.Test) {
		c := createTestContextWithTQ()
		assigner := createAssigner(c, t, assignerID)
		tasks := triggerScheduleTaskHandler(c, t, assignerID)
		assert.Loosely(t, tasks, should.NotBeNil)

		t.Run("works", func(t *ftt.Test) {
			mockGetAndListIssues(
				c, &monorail.Issue{ProjectName: "test", LocalId: 123},
			)

			for _, task := range tasks {
				assert.Loosely(t, task.Status, should.Equal(model.TaskStatus_Scheduled))
				task = triggerRunTaskHandler(c, t, assignerID, task.ID)

				assert.Loosely(t, task.Status, should.Equal(model.TaskStatus_Succeeded))
				assert.Loosely(t, task.Started.IsZero(), should.BeFalse)
				assert.Loosely(t, task.Ended.IsZero(), should.BeFalse)
			}
		})

		t.Run("cancelling stale tasks.", func(t *ftt.Test) {
			// make one stale schedule
			task := tasks[0]
			task.ExpectedStart = task.ExpectedStart.Add(-10 * time.Hour)
			assert.Loosely(t, task.ExpectedStart.Before(clock.Now(c).UTC()), should.BeTrue)
			assert.Loosely(t, task.Status, should.Equal(model.TaskStatus_Scheduled))
			assert.Loosely(t, datastore.Put(c, task), should.BeNil)

			// It should be marked as cancelled after runTaskHandler().
			processedTask := triggerRunTaskHandler(c, t, assignerID, task.ID)
			assert.Loosely(t, processedTask.Status, should.Equal(model.TaskStatus_Cancelled))
			assert.Loosely(t, processedTask.Started.IsZero(), should.BeFalse)
			assert.Loosely(t, processedTask.Ended.IsZero(), should.BeFalse)
		})

		t.Run("task status is kept as original, if not scheduled.", func(t *ftt.Test) {
			// make one with an invalid status. TaskStatus_Scheduled is the
			// the only status valid for runTaskHandler()
			task := tasks[0]
			task.Status = model.TaskStatus_Failed
			task.Started = time.Date(2000, 1, 1, 2, 3, 4, 0, time.UTC)
			task.Ended = task.Started.AddDate(0, 1, 2)
			assert.Loosely(t, datastore.Put(c, task), should.BeNil)

			// The task should stay the same after runTaskHandler().
			processedTask := triggerRunTaskHandler(c, t, assignerID, task.ID)
			assert.Loosely(t, processedTask.Status, should.Equal(task.Status))
			assert.Loosely(t, processedTask.Started, should.Match(task.Started))
			assert.Loosely(t, processedTask.Ended, should.Match(task.Ended))
		})

		t.Run("skips assigners with stale format", func(t *ftt.Test) {
			assigner.FormatVersion = 0
			datastore.Put(c, assigner)

			for _, task := range tasks {
				task = triggerRunTaskHandler(c, t, assignerID, task.ID)
				assert.Loosely(t, task.Status, should.Equal(model.TaskStatus_Cancelled))
			}
		})

		t.Run("cancelling tasks, if the assigner has been drained.", func(t *ftt.Test) {
			// TODO(crbug/967519): implement me.
		})
	})

	ftt.Run("RemoveNoopTasks", t, func(t *ftt.Test) {
		c := createTestContextWithTQ()
		assigner := createAssigner(c, t, assignerID)
		addTasks := func(n int, noop bool) error {
			tks := make([]*model.Task, n)
			for i := 0; i < n; i++ {
				tks[i] = &model.Task{
					AssignerKey:    model.GenAssignerKey(c, assigner),
					Status:         model.TaskStatus_Succeeded,
					WasNoopSuccess: noop,
				}
			}
			return datastore.Put(c, tks)
		}
		getTasks := func(n int32) []*model.Task {
			tks, err := model.GetTasks(c, assigner, n, true)
			assert.Loosely(t, err, should.BeNil)
			return tks
		}

		t.Run("With noop tasks", func(t *ftt.Test) {
			// nTask == 0
			assert.Loosely(t, RemoveNoopTasks(c, assigner, 5), should.BeNil)
			assert.Loosely(t, len(getTasks(5)), should.BeZero)

			// nTask < nDel
			addTasks(3, true)
			assert.Loosely(t, RemoveNoopTasks(c, assigner, 5), should.BeNil)
			assert.Loosely(t, len(getTasks(5)), should.BeZero)

			// nTask > nDel
			addTasks(7, true)
			assert.Loosely(t, RemoveNoopTasks(c, assigner, 5), should.BeNil)
			assert.Loosely(t, getTasks(5), should.HaveLength(2))
		})

		t.Run("w/o noop tasks", func(t *ftt.Test) {
			addTasks(7, false)
			assert.Loosely(t, RemoveNoopTasks(c, assigner, 5), should.BeNil)
			assert.Loosely(t, getTasks(7), should.HaveLength(7))
		})
	})
}
