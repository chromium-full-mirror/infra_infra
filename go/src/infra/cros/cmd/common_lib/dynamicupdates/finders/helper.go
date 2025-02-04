// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package finders defines dynamic finders.
package finders

import (
	"fmt"
	"reflect"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// FindFirstTaskOfType is a helper that finds the
// first task that matches the given type.
func FindFirstTaskOfType(req *api.CrosTestRunnerDynamicRequest, t reflect.Type) (int, error) {
	taskMap := MapTasksByType(req)
	if tasks, ok := taskMap[t]; ok && len(tasks) > 0 {
		return tasks[0].Index, nil
	}
	return -1, fmt.Errorf("no %s task found", t.Kind())
}

// FindLastTaskOfType is a helper that finds the
// last task that matches the given type.
func FindLastTaskOfType(req *api.CrosTestRunnerDynamicRequest, t reflect.Type) (int, error) {
	taskMap := MapTasksByType(req)
	if tasks, ok := taskMap[t]; ok && len(tasks) > 0 {
		return tasks[len(tasks)-1].Index, nil
	}
	return -1, fmt.Errorf("no %s task found", t.Kind())
}

type TaskIndexPair struct {
	Task  *api.CrosTestRunnerDynamicRequest_Task
	Index int
}

// MapTasksByType is a helper that reorganizes the trv2 ordered tasks
// into a map format for easier indexing by type.
func MapTasksByType(req *api.CrosTestRunnerDynamicRequest) map[reflect.Type][]*TaskIndexPair {
	m := map[reflect.Type][]*TaskIndexPair{}

	for i, task := range req.GetOrderedTasks() {
		taskType := reflect.TypeOf(task.GetTask())
		if _, ok := m[taskType]; !ok {
			m[taskType] = []*TaskIndexPair{}
		}
		m[taskType] = append(m[taskType], &TaskIndexPair{
			Task:  task,
			Index: i,
		})
	}

	return m
}
