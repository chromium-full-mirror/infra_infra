// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scheduling

import (
	"fmt"

	"infra/libs/fleet/scheduling/api"
	"infra/libs/fleet/scheduling/internal/scheduke"
)

// NewTaskSchedulingAPI serves as the entry point to the task scheduling
// library by returning an api.TaskSchedulingAPI for the given provider.
func NewTaskSchedulingAPI(pid api.ProviderId) (api.TaskSchedulingAPI, error) {
	switch pid {
	case api.ProviderId_PROVIDER_ID_SCHEDUKE:
		return scheduke.New()
	default:
		return nil, fmt.Errorf("provider %v is not implemented", pid)
	}
}
