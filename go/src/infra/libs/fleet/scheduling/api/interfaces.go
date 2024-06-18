// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import "context"

// TaskSchedulingAPI is the Device management API that all providers implement.
type TaskSchedulingAPI interface {
	// ScheduleTask schedules a task.
	ScheduleTask(context.Context, *ScheduleTaskRequest) (*Task, error)
	// CancelTasks cancels specified tasks.
	CancelTasks(context.Context, *CancelTasksRequest) error
}
