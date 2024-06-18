// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package scheduke contains the implementation to use Scheduke as a task
// scheduler.
package scheduke

import (
	"context"
	"errors"

	"infra/libs/fleet/scheduling/api"
)

// schedukeAPI implements api.TaskSchedulingAPI.
//
// The struct itself doesn't need to be public.
type schedukeAPI struct{}

// New constructs a new api.TaskSchedulingAPI with Scheduke Service backend.
func New() (api.TaskSchedulingAPI, error) {
	return &schedukeAPI{}, nil
}

// ScheduleTask takes a ScheduleTaskRequest and returns a Task.
//
// ScheduleTask parses the ScheduleTaskRequest into a BuildBucket request and
// sends it to Scheduke for task scheduling.
func (g *schedukeAPI) ScheduleTask(ctx context.Context, req *api.ScheduleTaskRequest) (*api.Task, error) {
	return nil, errors.New("not implemented")
}

// CancelTasks takes a CancelTasksRequest and returns error if encountered.
func (g *schedukeAPI) CancelTasks(ctx context.Context, req *api.CancelTasksRequest) error {
	return errors.New("not implemented")
}
