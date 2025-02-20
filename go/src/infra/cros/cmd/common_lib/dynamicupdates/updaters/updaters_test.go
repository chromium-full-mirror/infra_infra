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

	. "go.chromium.org/infra/cros/cmd/common_lib/dynamicupdates/updaters"
)

func TestInsertHelpers(t *testing.T) {
	ftt.Run("InsertLeft", t, func(t *ftt.Test) {
		taskList := []*api.CrosTestRunnerDynamicRequest_Task{
			getGenericTaskWithID("1"),
			getGenericTaskWithID("3"),
			getGenericTaskWithID("5"),
			getGenericTaskWithID("7"),
		}

		InsertTaskLeft(&taskList, getGenericTaskWithID("0"), 0)
		InsertTaskLeft(&taskList, getGenericTaskWithID("2"), 2)
		InsertTaskLeft(&taskList, getGenericTaskWithID("4"), 4)
		InsertTaskLeft(&taskList, getGenericTaskWithID("6"), 6)

		assert.Loosely(t, taskList, should.HaveLength(8))
		taskHasID(t, taskList[0], "0")
		taskHasID(t, taskList[1], "1")
		taskHasID(t, taskList[2], "2")
		taskHasID(t, taskList[3], "3")
		taskHasID(t, taskList[4], "4")
		taskHasID(t, taskList[5], "5")
		taskHasID(t, taskList[6], "6")
		taskHasID(t, taskList[7], "7")
	})

	ftt.Run("InsertRight", t, func(t *ftt.Test) {
		taskList := []*api.CrosTestRunnerDynamicRequest_Task{
			getGenericTaskWithID("0"),
			getGenericTaskWithID("2"),
			getGenericTaskWithID("4"),
			getGenericTaskWithID("6"),
		}

		InsertTaskRight(&taskList, getGenericTaskWithID("1"), 0)
		InsertTaskRight(&taskList, getGenericTaskWithID("3"), 2)
		InsertTaskRight(&taskList, getGenericTaskWithID("5"), 4)
		InsertTaskRight(&taskList, getGenericTaskWithID("7"), 6)

		assert.Loosely(t, taskList, should.HaveLength(8))
		taskHasID(t, taskList[0], "0")
		taskHasID(t, taskList[1], "1")
		taskHasID(t, taskList[2], "2")
		taskHasID(t, taskList[3], "3")
		taskHasID(t, taskList[4], "4")
		taskHasID(t, taskList[5], "5")
		taskHasID(t, taskList[6], "6")
		taskHasID(t, taskList[7], "7")
	})

	ftt.Run("InsertLeftEmpty", t, func(t *ftt.Test) {
		taskList := []*api.CrosTestRunnerDynamicRequest_Task{}

		InsertTaskLeft(&taskList, getGenericTaskWithID("0"), 0)

		assert.Loosely(t, taskList, should.HaveLength(1))
		taskHasID(t, taskList[0], "0")
	})

	ftt.Run("InsertRightEmpty", t, func(t *ftt.Test) {
		taskList := []*api.CrosTestRunnerDynamicRequest_Task{}

		InsertTaskRight(&taskList, getGenericTaskWithID("0"), 0)

		assert.Loosely(t, taskList, should.HaveLength(1))
		taskHasID(t, taskList[0], "0")
	})
}

func getGenericTaskWithID(id string) *api.CrosTestRunnerDynamicRequest_Task {
	return &api.CrosTestRunnerDynamicRequest_Task{
		Task: &api.CrosTestRunnerDynamicRequest_Task_Generic{
			Generic: &api.GenericTask{
				DynamicIdentifier: id,
			},
		},
	}
}

func taskHasID(t testing.TB, task *api.CrosTestRunnerDynamicRequest_Task, id string) {
	t.Helper()
	assert.Loosely(t, task.GetGeneric().DynamicIdentifier, should.Equal(id), truth.LineContext())
}
