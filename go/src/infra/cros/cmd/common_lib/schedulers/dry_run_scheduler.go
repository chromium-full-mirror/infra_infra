// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package schedulers

import (
	"context"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// DryRunScheduler defines a scheduler that doesn't schedule request(s)
// anywhere. It is a dummy scheduler that prints out request(s) without
// scheduling them.
type DryRunScheduler struct {
	schedulerType interfaces.SchedulerType
}

func NewDryRunScheduler() interfaces.SchedulerInterface {
	return &DryRunScheduler{schedulerType: DryRunSchedulerType}
}

func (ds *DryRunScheduler) GetSchedulerType() interfaces.SchedulerType {
	return ds.schedulerType
}

func (ds *DryRunScheduler) Setup(_ string) error {
	// no-op
	return nil
}

func (ds *DryRunScheduler) ScheduleRequest(_ context.Context, _ *buildbucketpb.ScheduleBuildRequest, _ *build.Step) (*buildbucketpb.Build, string, error) {
	// no-op
	return nil, "", nil
}

func (ds *DryRunScheduler) GetStatus(requestID int64) (*buildbucketpb.Build, error) {
	// no-op
	return nil, nil
}

func (ds *DryRunScheduler) GetResult(requestID int64) error {
	// no-op
	return nil
}

func (ds *DryRunScheduler) CancelTask(requestID int64) error {
	// no-op
	return nil
}
