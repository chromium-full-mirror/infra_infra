// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import (
	"fmt"
)

// Validate validates inputs of ScheduleTaskRequest.
func (r *ScheduleTaskRequest) Validate() error {
	if r.GetBuildbucketRequest().GetBuilder().GetBuilder() == "" {
		return fmt.Errorf("invalid argument: no builder specified in BB request")
	}
	if r.GetDeviceName() == "" {
		return fmt.Errorf("invalid format: no device name")
	}
	return nil
}

// Validate validates inputs of CancelTasksRequest.
func (r *CancelTasksRequest) Validate() error {
	if len(r.GetTaskIds()) == 0 {
		return fmt.Errorf("invalid argument: no task IDs")
	}
	return nil
}
