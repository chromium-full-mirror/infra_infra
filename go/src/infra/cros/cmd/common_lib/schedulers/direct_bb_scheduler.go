// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package schedulers contains implementations of the scheduler interface.
package schedulers

import (
	"context"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// DirectBBScheduler defines scheduler that schedules request(s) directly
// through buildbucket.
type DirectBBScheduler struct {
	schedulerType interfaces.SchedulerType

	BBClient *buildbucketpb.BuildsClient
}

func NewDirectBBScheduler() interfaces.SchedulerInterface {
	return &DirectBBScheduler{schedulerType: DirectBBSchedulerType}
}

func (sc *DirectBBScheduler) GetSchedulerType() interfaces.SchedulerType {
	return sc.schedulerType
}

func (sc *DirectBBScheduler) Setup(_ string) error {
	ctx := context.Background()
	if sc.BBClient == nil {
		client, err := common.NewBBClient(ctx)
		if err != nil {
			return err
		}
		sc.BBClient = &client
	}
	return nil
}

func (sc *DirectBBScheduler) ScheduleRequest(ctx context.Context, req *buildbucketpb.ScheduleBuildRequest, _ *build.Step) (*buildbucketpb.Build, string, error) {
	scheduledBuild, err := (*sc.BBClient).ScheduleBuild(ctx, req)
	if err != nil {
		return nil, "", err
	}
	return scheduledBuild, "", nil
}

func (sc *DirectBBScheduler) GetStatus(requestID int64) (*buildbucketpb.Build, error) {
	// no-op
	return nil, nil
}

func (sc *DirectBBScheduler) GetResult(requestID int64) error {
	// no-op
	return nil
}

func (sc *DirectBBScheduler) CancelTask(requestID int64) error {
	// no-op
	return nil
}
