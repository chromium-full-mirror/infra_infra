// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package model

import (
	"testing"
	"time"

	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	"infra/appengine/arquebus/app/util"
)

func TestGetTasks(t *testing.T) {
	t.Parallel()
	now := testclock.TestTimeUTC

	ftt.Run("GetTasks", t, func(t *ftt.Test) {
		c := util.CreateTestContext()

		// create sample tasks.
		assigner := updateAndGetAllAssigners(c, t, "rev-1", createConfig(t, "a"))[0]
		times := []time.Time{now, now.Add(assigner.Interval)}
		createTasks(c, t, assigner, TaskStatus_Scheduled, times...)

		t.Run("working", func(t *ftt.Test) {
			tasks, err := GetTasks(c, assigner, int32(len(times)), false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(tasks), should.Equal(len(times)))
			for i := 0; i < len(times); i++ {
				assert.Loosely(t, tasks[i].ExpectedStart.Unix(), should.Equal(
					times[len(times)-i-1].Unix()))
			}
		})

		t.Run("with NoopSuccess", func(t *ftt.Test) {
			tasks, err := GetTasks(c, assigner, int32(len(times)), false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(tasks), should.Equal(len(times)))

			tasks[0].WasNoopSuccess = true
			tasks[0].Status = TaskStatus_Succeeded
			assert.Loosely(t, datastore.Put(c, tasks), should.BeNil)

			tasks, err = GetTasks(c, assigner, int32(len(times)), false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(tasks), should.Equal(len(times)-1))

			tasks, err = GetTasks(c, assigner, int32(len(times)), true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(tasks), should.Equal(len(times)))
		})
	})
}
