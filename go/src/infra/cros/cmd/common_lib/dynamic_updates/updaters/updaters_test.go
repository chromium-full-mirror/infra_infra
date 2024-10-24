// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updaters_test

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	. "infra/cros/cmd/common_lib/dynamic_updates/updaters"
)

func TestInsertHelpers(t *testing.T) {
	ftt.Run("InsertLeft", t, func(t *ftt.Test) {
		taskList := []*api.CrosTestRunnerDynamicRequest_Task{
			getGenericTaskWithId("1"),
			getGenericTaskWithId("3"),
			getGenericTaskWithId("5"),
			getGenericTaskWithId("7"),
		}

		InsertTaskLeft(&taskList, getGenericTaskWithId("0"), 0)
		InsertTaskLeft(&taskList, getGenericTaskWithId("2"), 2)
		InsertTaskLeft(&taskList, getGenericTaskWithId("4"), 4)
		InsertTaskLeft(&taskList, getGenericTaskWithId("6"), 6)

		assert.Loosely(t, taskList, should.HaveLength(8))
		taskHasId(t, taskList[0], "0")
		taskHasId(t, taskList[1], "1")
		taskHasId(t, taskList[2], "2")
		taskHasId(t, taskList[3], "3")
		taskHasId(t, taskList[4], "4")
		taskHasId(t, taskList[5], "5")
		taskHasId(t, taskList[6], "6")
		taskHasId(t, taskList[7], "7")
	})

	ftt.Run("InsertRight", t, func(t *ftt.Test) {
		taskList := []*api.CrosTestRunnerDynamicRequest_Task{
			getGenericTaskWithId("0"),
			getGenericTaskWithId("2"),
			getGenericTaskWithId("4"),
			getGenericTaskWithId("6"),
		}

		InsertTaskRight(&taskList, getGenericTaskWithId("1"), 0)
		InsertTaskRight(&taskList, getGenericTaskWithId("3"), 2)
		InsertTaskRight(&taskList, getGenericTaskWithId("5"), 4)
		InsertTaskRight(&taskList, getGenericTaskWithId("7"), 6)

		assert.Loosely(t, taskList, should.HaveLength(8))
		taskHasId(t, taskList[0], "0")
		taskHasId(t, taskList[1], "1")
		taskHasId(t, taskList[2], "2")
		taskHasId(t, taskList[3], "3")
		taskHasId(t, taskList[4], "4")
		taskHasId(t, taskList[5], "5")
		taskHasId(t, taskList[6], "6")
		taskHasId(t, taskList[7], "7")
	})

	ftt.Run("InsertLeftEmpty", t, func(t *ftt.Test) {
		taskList := []*api.CrosTestRunnerDynamicRequest_Task{}

		InsertTaskLeft(&taskList, getGenericTaskWithId("0"), 0)

		assert.Loosely(t, taskList, should.HaveLength(1))
		taskHasId(t, taskList[0], "0")
	})

	ftt.Run("InsertRightEmpty", t, func(t *ftt.Test) {
		taskList := []*api.CrosTestRunnerDynamicRequest_Task{}

		InsertTaskRight(&taskList, getGenericTaskWithId("0"), 0)

		assert.Loosely(t, taskList, should.HaveLength(1))
		taskHasId(t, taskList[0], "0")
	})
}

func getGenericTaskWithId(id string) *api.CrosTestRunnerDynamicRequest_Task {
	return &api.CrosTestRunnerDynamicRequest_Task{
		Task: &api.CrosTestRunnerDynamicRequest_Task_Generic{
			Generic: &api.GenericTask{
				DynamicIdentifier: id,
			},
		},
	}
}

func taskHasId(t testing.TB, task *api.CrosTestRunnerDynamicRequest_Task, id string) {
	t.Helper()
	assert.Loosely(t, task.GetGeneric().DynamicIdentifier, should.Equal(id), truth.LineContext())
}
