// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package interfaces

import (
	"context"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/luciexe/build"
)

// SchedulerType represents scheduler type
type SchedulerType string

// SchedulerInterface defines the contract a scheduler will have to satisfy.
type SchedulerInterface interface {
	// GetSchedulerType returns the scheduler type
	GetSchedulerType() SchedulerType

	// Setup sets up the scheduler
	Setup(string) error

	// ScheduleRequest schedules requests, returning a BBID and a Device Manager
	// lease ID (if any).
	ScheduleRequest(context.Context, *buildbucketpb.ScheduleBuildRequest, *build.Step) (*buildbucketpb.Build, string, error)

	// GetStatus fetches and returns the current build status of the provided
	// BBID.
	GetStatus(requestID int64) (*buildbucketpb.Build, error)

	// GetResults returns the test results from the child TestRunner.
	GetResult(requestID int64) error

	// CancelTask terminates a running BuildBucket build with the passed in
	// BBID.
	CancelTask(requestID int64) error
}
