// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package schedulers

import "infra/cros/cmd/common_lib/interfaces"

// All supported scheduler types.
const (
	// Unsupported scheduler type (For testing purposes only).
	UnsupportedSchedulerType interfaces.SchedulerType = "UnsupportedScheduler"
	// Direct bb scheduler schedules requests directly through buildbucket.
	DirectBBSchedulerType interfaces.SchedulerType = "DirectBBScheduler"
	// DryRunSchedulerType is a stubbed scheduler for dry run mode/debugging. It
	// will print out the request without scheduling anywhere.
	DryRunSchedulerType interfaces.SchedulerType = "DryRunScheduler"
	// Scheduke scheduler schedules requests through Scheduke.
	SchedukeSchedulerType interfaces.SchedulerType = "SchedukeScheduler"
)
