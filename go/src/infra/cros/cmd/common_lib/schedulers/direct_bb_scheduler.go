// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package schedulers contains implementations of the scheduler interface.
package schedulers

import (
	"context"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
)

// DirectBBScheduler defines scheduler that schedules request(s) directly
// through buildbucket.
type DirectBBScheduler struct {
	*AbstractScheduler

	BBClient *buildbucketpb.BuildsClient
}

func NewDirectBBScheduler() *DirectBBScheduler {
	absSched := NewAbstractScheduler(DirectBBSchedulerType)
	return &DirectBBScheduler{AbstractScheduler: absSched}
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
