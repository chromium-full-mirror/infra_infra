// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updaters

import "go.chromium.org/chromiumos/config/go/test/api"

// InsertTaskLeft places the task to the left of the index.
func InsertTaskLeft(
	taskList *[]*api.CrosTestRunnerDynamicRequest_Task,
	task *api.CrosTestRunnerDynamicRequest_Task,
	at int) {

	if at == 0 {
		*taskList = append([]*api.CrosTestRunnerDynamicRequest_Task{task}, *taskList...)
		return
	}
	*taskList = append((*taskList)[:at+1], (*taskList)[at:]...)
	(*taskList)[at] = task
}

// InsertTaskRight places the task to the right of the index.
func InsertTaskRight(
	taskList *[]*api.CrosTestRunnerDynamicRequest_Task,
	task *api.CrosTestRunnerDynamicRequest_Task,
	at int) {

	if len(*taskList) == at {
		*taskList = append(*taskList, task)
		return
	}
	*taskList = append((*taskList)[:at+1], (*taskList)[at:]...)
	(*taskList)[at+1] = task
}
