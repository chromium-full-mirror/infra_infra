// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package finders

import (
	"fmt"
	"reflect"

	"go.chromium.org/chromiumos/config/go/test/api"
)

var taskTypeMap = map[api.FocalTaskFinder_TaskType]reflect.Type{
	api.FocalTaskFinder_PROVISION: reflect.TypeOf((*api.CrosTestRunnerDynamicRequest_Task_Provision)(nil)),
	api.FocalTaskFinder_PRETEST:   reflect.TypeOf((*api.CrosTestRunnerDynamicRequest_Task_PreTest)(nil)),
	api.FocalTaskFinder_TEST:      reflect.TypeOf((*api.CrosTestRunnerDynamicRequest_Task_Test)(nil)),
	api.FocalTaskFinder_POSTTEST:  reflect.TypeOf((*api.CrosTestRunnerDynamicRequest_Task_PostTest)(nil)),
	api.FocalTaskFinder_PUBLISH:   reflect.TypeOf((*api.CrosTestRunnerDynamicRequest_Task_Publish)(nil)),
}

// GetFocalTaskFinder maps each finder to a handler.
func GetFocalTaskFinder(req *api.CrosTestRunnerDynamicRequest, focalTaskFinder *api.FocalTaskFinder) (int, error) {
	switch finder := focalTaskFinder.Finder.(type) {
	case *api.FocalTaskFinder_First_:
		return findFirstOrLastTask(req, finder.First.GetTaskType(), true)
	case *api.FocalTaskFinder_Last_:
		return findFirstOrLastTask(req, finder.Last.GetTaskType(), false)
	case *api.FocalTaskFinder_Beginning_:
		return 0, nil
	case *api.FocalTaskFinder_End_:
		return len(req.GetOrderedTasks()), nil
	case *api.FocalTaskFinder_ByDynamicIdentifier_:
		return findByDyanmicIdentifier(req, finder.ByDynamicIdentifier.GetDynamicIdentifier())
	default:
		return -1, fmt.Errorf("unhandled focal task finder %s", reflect.TypeOf(finder))
	}
}

// findFirstOrLastTask checks for the task type and finds either the first or last.
func findFirstOrLastTask(req *api.CrosTestRunnerDynamicRequest, taskType api.FocalTaskFinder_TaskType, first bool) (int, error) {
	taskReflectType, ok := taskTypeMap[taskType]
	if !ok {
		return -1, fmt.Errorf("unhandled task type: %s", taskType)
	}

	if first {
		return FindFirstTaskOfType(req, taskReflectType)
	} else {
		return FindLastTaskOfType(req, taskReflectType)
	}
}

// findByDynamicIdentifier searches through each task for the dynamic identifier provided.
func findByDyanmicIdentifier(req *api.CrosTestRunnerDynamicRequest, dynamicID string) (int, error) {
	// Switch statement necessary due to value being placed
	// inside of objects under a OneOf.
	for i, task := range req.GetOrderedTasks() {
		switch dynamicID {
		case task.GetProvision().GetDynamicIdentifier():
			return i, nil
		case task.GetPreTest().GetDynamicIdentifier():
			return i, nil
		case task.GetTest().GetDynamicIdentifier():
			return i, nil
		case task.GetPostTest().GetDynamicIdentifier():
			return i, nil
		case task.GetPublish().GetDynamicIdentifier():
			return i, nil
		case task.GetGeneric().GetDynamicIdentifier():
			return i, nil
		}
	}
	return -1, fmt.Errorf("failed to find task with dynamic identifier: %s", dynamicID)
}
