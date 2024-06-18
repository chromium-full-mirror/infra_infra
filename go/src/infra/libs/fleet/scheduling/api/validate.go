// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import (
	"errors"
	"fmt"
)

// Validate validates inputs of ScheduleTaskRequest.
func (r *ScheduleTaskRequest) Validate() error {
	if r.GetConfig() == nil {
		return errors.New("invalid argument: no config found")
	}
	schedukeBackend := r.GetConfig().GetSchedukeBackend()
	if schedukeBackend == nil {
		return fmt.Errorf("invalid argument: bad backend: want scheduke backend, got %v", r.GetConfig())
	}
	return nil
}

// Validate validates inputs of CancelTasksRequest.
func (r *CancelTasksRequest) Validate() error {
	if r.GetConfig() == nil {
		return errors.New("invalid argument: no config found")
	}
	schedukeBackend := r.GetConfig().GetSchedukeBackend()
	if schedukeBackend == nil {
		return fmt.Errorf("invalid argument: bad backend: want scheduke backend, got %v", r.GetConfig())
	}
	return nil
}
