// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package model

import (
	"context"
	"testing"
	"time"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"

	"infra/appengine/arquebus/app/util"
)

func TestUpdateAssigners(t *testing.T) {
	t.Parallel()

	ftt.Run("UpdateAssigners", t, func(t *ftt.Test) {
		c := util.CreateTestContext()
		rev1, rev2, rev3 := "abc", "def", "ghi"

		// Ensure empty now.
		assigners, err := GetAllAssigners(c)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, assigners, should.BeNil)

		t.Run("Creates a new Assigner", func(t *ftt.Test) {
			assigners := updateAndGetAllAssigners(
				c, t, rev1, createConfig(t, "test-a"))
			assert.Loosely(t, len(assigners), should.Equal(1))
			assert.Loosely(t, assigners[0].ID, should.Equal("test-a"))
		})

		t.Run("Updates an existing Assigner", func(t *ftt.Test) {
			assigners := updateAndGetAllAssigners(c, t, rev1, createConfig(t, "test-a"))
			assert.Loosely(t, len(assigners), should.Equal(1))
			assert.Loosely(t, assigners[0].ID, should.Equal("test-a"))

			// increase the interval just to check the Assigner
			// has been updated or not.
			cfg := createConfig(t, "test-a")
			original := cfg.Interval.Seconds
			cfg.Interval.Seconds = original + 1
			changed := time.Duration(original+1) * time.Second

			t.Run("With a new revision", func(t *ftt.Test) {
				assigners := updateAndGetAllAssigners(c, t, rev2, cfg)
				assert.Loosely(t, len(assigners), should.Equal(1))
				assert.Loosely(t, assigners[0].Interval, should.Equal(changed))
			})

			t.Run("With the same new revision", func(t *ftt.Test) {
				assigners := updateAndGetAllAssigners(c, t, rev1, cfg)
				assert.Loosely(t, len(assigners), should.Equal(1))
				du := time.Duration(original) * time.Second
				assert.Loosely(t, assigners[0].Interval, should.Equal(du))
			})
		})

		t.Run("Marks as removed if config removed", func(t *ftt.Test) {
			// create
			id := "test-a"
			cfg := createConfig(t, id)
			assigners := updateAndGetAllAssigners(c, t, rev1, cfg)
			assert.Loosely(t, len(assigners), should.Equal(1))
			assert.Loosely(t, assigners[0].ID, should.Equal(id))

			// remove
			assigners = updateAndGetAllAssigners(c, t, rev2)
			assert.Loosely(t, assigners, should.BeNil)

			// put it back
			assigners = updateAndGetAllAssigners(c, t, rev3, cfg)
			assert.Loosely(t, assigners[0].ID, should.Equal(id))
		})
	})
}

func TestEnsureScheduledTasks(t *testing.T) {
	t.Parallel()
	var err error

	ftt.Run("EnsureScheduledTasks", t, func(t *ftt.Test) {
		c := util.CreateTestContext()
		cl := testclock.New(time.Unix(testclock.TestTimeUTC.Unix(), 0).UTC())
		c = clock.Set(c, cl)

		assigner := updateAndGetAllAssigners(c, t, "rev1", createConfig(t, "a"))[0]
		assert.Loosely(t, assigner.IsDrained, should.Equal(false))

		// helpers to make the body of unit tests smaller.
		ensureScheduledTasks := func(c context.Context) (tasks []*Task) {
			err = ds.RunInTransaction(c, func(c context.Context) error {
				tasks, err = EnsureScheduledTasks(c, assigner.ID)
				return err
			}, &ds.TransactionOptions{})
			assert.Loosely(t, err, should.BeNil)
			return tasks
		}
		getTasks := func(c context.Context, n int32) []*Task {
			tasks, err := GetTasks(c, assigner, n, false)
			assert.Loosely(t, err, should.BeNil)
			return tasks
		}

		t.Run("if assigner_interval == scheduler_interval", func(t *ftt.Test) {
			assert.Loosely(t, assigner.Interval, should.Equal(scheduleAssignerCronInterval))
			tasks := ensureScheduledTasks(c)
			// Then, it should just create 1 task.
			assert.Loosely(t, len(tasks), should.Equal(1))

			t.Run("with completed tasks only", func(t *ftt.Test) {
				// mark the task as completed.
				tasks[0].Status = TaskStatus_Succeeded
				assert.Loosely(t, ds.Put(c, tasks[0]), should.BeNil)

				// advance the current timestamp by the interval and run the
				// scheduler logic.
				cl.Add(assigner.Interval)
				ensureScheduledTasks(c)

				// getTasks() returns Tasks in the order of desc ExpectedStart.
				tasks = getTasks(c, 100)
				assert.Loosely(t, len(tasks), should.Equal(2))
				newTask, existingTask := tasks[0], tasks[1]

				assert.Loosely(t, newTask.Status, should.Equal(TaskStatus_Scheduled))
				assert.Loosely(t, newTask.ExpectedStart, should.Match(cl.Now().Add(assigner.Interval)))
				assert.Loosely(t, existingTask.Status, should.Equal(TaskStatus_Succeeded))
			})

			t.Run("with a scheduled task", func(t *ftt.Test) {
				cl.Add(time.Second)
				// There shouldn't be any new Tasks created.
				assert.Loosely(t, ensureScheduledTasks(c), should.BeNil)
				tasks = getTasks(c, 100)
				assert.Loosely(t, len(tasks), should.Equal(1))
			})

			t.Run("with a stale, scheduled task", func(t *ftt.Test) {
				// Advance the time.
				cl.Add(assigner.Interval)
				// Now, the existing task should be considered stale.
				// A new task should be created by the scheduler.
				assert.Loosely(t, len(ensureScheduledTasks(c)), should.Equal(1))

				// getTasks() returns Tasks in the order of desc ExpectedStart.
				tasks = getTasks(c, 100)
				newTask, existingTask := tasks[0], tasks[1]

				// Verify the existing Task is still marked as Scheduled.
				assert.Loosely(t, existingTask.Status, should.Equal(TaskStatus_Scheduled))
				assert.Loosely(t, newTask.Status, should.Equal(TaskStatus_Scheduled))
				assert.That(t, newTask.ExpectedStart, should.Match(cl.Now().Add(assigner.Interval)))
			})
		})

		t.Run("if assigner_interval > scheduler_interval", func(t *ftt.Test) {
			// This is the case where the next scheduler run comes before
			// latestSchedule + Assigner.Interval. In theory, it's not
			// necessary to schedule a Task immediately because there will be
			// another chance for the scheduler to create a new Task
			// for the next ETA. However, EnsureScheduledTasks() ensures
			// that there is at least one Task always, and, therefore, it
			// should create a new Task regardless.
			assigner.Interval = scheduleAssignerCronInterval * 2
			now := cl.Now().UTC()
			assert.Loosely(t, ds.Put(c, assigner), should.BeNil)

			// This should create a new one.
			tasks := ensureScheduledTasks(c)
			assert.Loosely(t, len(tasks), should.Equal(1))
			newTask := tasks[0]

			assert.That(t,
				newTask.ExpectedStart, should.Match(
					now.Add(scheduleAssignerCronInterval*2),
				))
		})

		t.Run("if assigner_interval < scheduler_interval", func(t *ftt.Test) {
			// EnsureScheduledTasks() should create an enough number of Task(s)
			// to cover all the period until the next scheduler run.
			assigner.Interval = scheduleAssignerCronInterval / 4
			now := cl.Now().UTC()
			assert.Loosely(t, ds.Put(c, assigner), should.BeNil)
			tasks := ensureScheduledTasks(c)
			assert.Loosely(t, len(tasks), should.Equal(4))
			for i := 0; i < 4; i++ {
				// each should assigner.Interval further away from the previous
				// schedule.
				start := now.Add(assigner.Interval * time.Duration(i+1))
				assert.That(t, tasks[i].ExpectedStart, should.Match(start))
			}
		})

		t.Run("with drained Assigner", func(t *ftt.Test) {
			// Drain it.
			assigner.IsDrained = true
			assert.Loosely(t, ds.Put(c, assigner), should.BeNil)
			assert.Loosely(t, ensureScheduledTasks(c), should.BeNil)
		})
	})
}
