// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clients

import (
	"sort"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/appengine/crosskylabadmin/internal/tq"
)

func TestSuccessfulPushDuts(t *testing.T) {
	ftt.Run("success", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContext()
		tqt := tq.GetTestable(ctx)
		qn := "repair-bots"
		tqt.CreateQueue(qn)
		hosts := []string{"host1", "host2"}
		swarmingPool := "pool-a"
		err := PushRepairDUTs(ctx, hosts, "needs_repair", swarmingPool)
		assert.Loosely(t, err, should.BeNil)
		tasks := tqt.GetScheduledTasks()
		ta, ok := tasks[qn]
		assert.Loosely(t, ok, should.BeTrue)
		var taskPaths, taskParams []string
		for _, v := range ta {
			taskPaths = append(taskPaths, v.Path)
			taskParams = append(taskParams, string(v.Payload))
		}
		sort.Strings(taskPaths)
		sort.Strings(taskParams)
		expectedPaths := []string{"/internal/task/cros_repair/host1", "/internal/task/cros_repair/host2"}
		expectedParams := []string{"botID=host1&expectedState=needs_repair&swarmingPool=pool-a", "botID=host2&expectedState=needs_repair&swarmingPool=pool-a"}
		assert.Loosely(t, taskPaths, should.Match(expectedPaths))
		assert.Loosely(t, taskParams, should.Match(expectedParams))
	})
}

func TestSuccessfulPushLabstations(t *testing.T) {
	ftt.Run("success", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContext()
		tqt := tq.GetTestable(ctx)
		qn := "repair-labstations"
		tqt.CreateQueue(qn)
		hosts := []string{"host1", "host2"}
		err := PushRepairLabstations(ctx, hosts, "my-pool2")
		assert.Loosely(t, err, should.BeNil)
		tasks := tqt.GetScheduledTasks()
		ta, ok := tasks[qn]
		assert.Loosely(t, ok, should.BeTrue)
		var taskPaths, taskParams []string
		for _, v := range ta {
			taskPaths = append(taskPaths, v.Path)
			taskParams = append(taskParams, string(v.Payload))
		}
		sort.Strings(taskPaths)
		sort.Strings(taskParams)
		expectedPaths := []string{"/internal/task/labstation_repair/host1", "/internal/task/labstation_repair/host2"}
		expectedParams := []string{"botID=host1&swarmingPool=my-pool2", "botID=host2&swarmingPool=my-pool2"}
		assert.Loosely(t, taskPaths, should.Match(expectedPaths))
		assert.Loosely(t, taskParams, should.Match(expectedParams))
	})
}

func TestSuccessfulPushAuditTasks(t *testing.T) {
	ftt.Run("success", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContext()
		tqt := tq.GetTestable(ctx)
		qn := "audit-bots"
		tqt.CreateQueue(qn)
		hosts := []string{"host1", "host2"}
		actions := []string{"action1", "action2"}
		err := PushAuditDUTs(ctx, hosts, actions, "Storage", "my-pool")
		assert.Loosely(t, err, should.BeNil)
		tasks := tqt.GetScheduledTasks()
		ta, ok := tasks[qn]
		assert.Loosely(t, ok, should.BeTrue)
		var taskPaths, taskParams []string
		for _, v := range ta {
			taskPaths = append(taskPaths, v.Path)
			taskParams = append(taskParams, string(v.Payload))
		}
		sort.Strings(taskPaths)
		sort.Strings(taskParams)
		expectedPaths := []string{"/internal/task/audit/host1/action1-action2", "/internal/task/audit/host2/action1-action2"}
		expectedParams := []string{"actions=action1%2Caction2&botID=host1&swarmingPool=my-pool&taskname=Storage", "actions=action1%2Caction2&botID=host2&swarmingPool=my-pool&taskname=Storage"}
		assert.Loosely(t, taskPaths, should.Match(expectedPaths))
		assert.Loosely(t, taskParams, should.Match(expectedParams))
	})
}

func TestUnknownQueuePush(t *testing.T) {
	ftt.Run("no taskqueue", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContext()
		tqt := tq.GetTestable(ctx)
		tqt.CreateQueue("no-repair-bots")
		err := PushRepairDUTs(ctx, []string{"host1", "host2"}, "some_state", "some_builder")
		assert.Loosely(t, err, should.NotBeNil)
	})
}
