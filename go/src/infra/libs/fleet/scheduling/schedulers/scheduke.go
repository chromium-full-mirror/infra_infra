// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package schedulers contains implementors of the TaskSchedulingAPI interface.
package schedulers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.chromium.org/luci/auth"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
	"infra/libs/fleet/scheduling/api"
)

const schedukeTaskSwarmingTagKey = "scheduke-admin-task"

// schedukeAPI implements api.TaskSchedulingAPI.
type schedukeAPI struct {
	schedukeClient *common.SchedukeClient
	gerritClient   *http.Client
	// The Swarming "label-pool" value of the DUT for which an admin task is being
	// scheduled. (Not a Swarming pool.)
	pool string
	// Whether this API is being used in a CLI (instead of other automation).
	usedByCLI bool
	authOpts  auth.Options
}

// NewSchedukeClientForCLI constructs a new Scheduke TaskSchedulingAPI for use
// in a CLI.
func NewSchedukeClientForCLI(ctx context.Context, pool string, authOpts auth.Options) (api.TaskSchedulingAPI, error) {
	gc, err := common.GerritClient(ctx, authOpts)
	if err != nil {
		return nil, errors.Annotate(err, "creating Scheduke client for CLI: initializing Gerrit client to read Device Manager allowlist").Err()
	}
	return &schedukeAPI{
		gerritClient: gc,
		pool:         pool,
		usedByCLI:    true,
		authOpts:     authOpts,
	}, nil
}

// NewSchedukeClientForAutomation constructs a new Scheduke TaskSchedulingAPI
// for use from automation.
func NewSchedukeClientForAutomation(ctx context.Context, pool string) (api.TaskSchedulingAPI, error) {
	gc, err := common.GerritClient(ctx, common.GerritAuthOptsOnBot)
	if err != nil {
		return nil, errors.Annotate(err, "creating Scheduke client for CLI: initializing Gerrit client to read Device Manager allowlist").Err()
	}
	return &schedukeAPI{
		gerritClient: gc,
		pool:         pool,
		usedByCLI:    false,
	}, nil
}

// ScheduleTask takes a ScheduleTaskRequest and returns a Task.
//
// ScheduleTask parses the ScheduleTaskRequest into a BuildBucket request and
// sends it to Scheduke for task scheduling.
func (s *schedukeAPI) ScheduleTask(ctx context.Context, req *api.ScheduleTaskRequest) (*api.Task, error) {
	// Initialize the Scheduke client if it has not already been initialized.
	if err := s.setupSchedukeClient(ctx); err != nil {
		return nil, errors.Annotate(err, "scheduling task via Scheduke").Err()
	}
	if err := req.Validate(); err != nil {
		return nil, errors.Annotate(err, "scheduling task via Scheduke: validating request").Err()
	}
	bbReq := req.GetBuildbucketRequest()
	builderName := bbReq.GetBuilder().GetBuilder()
	name := req.GetDeviceName()
	schedukeTagVal := fmt.Sprintf("%s-%d", name, time.Now().UnixMicro())
	bbReq.Tags = append(bbReq.GetTags(), &buildbucketpb.StringPair{
		Key:   schedukeTaskSwarmingTagKey,
		Value: schedukeTagVal,
	})
	schedukeReq, err := s.schedukeClient.AdminTaskReqToSchedukeReq(bbReq, name, s.pool)
	if err != nil {
		return nil, errors.Annotate(err, "scheduling task via Scheduke: generating Scheduke request for %s", builderName).Err()
	}
	resp, err := s.schedukeClient.ScheduleExecution(schedukeReq)
	if err != nil {
		return nil, errors.Annotate(err, "scheduling task via Scheduke: scheduling execution request on Scheduke").Err()
	}
	taskID, ok := resp.GetIds()[common.SchedukeTaskKey]
	if !ok {
		return nil, errors.Reason("scheduling task via Scheduke: response %v from Scheduke did not include an ID for the requested %s build", resp, builderName).Err()
	}
	return &api.Task{
		Id:  taskID,
		Url: fmt.Sprintf("https://chromeos-swarming.appspot.com/tasklist?f=%s:%s", schedukeTaskSwarmingTagKey, schedukeTagVal),
	}, nil
}

// CancelTasks takes a CancelTasksRequest and returns error if encountered.
func (s *schedukeAPI) CancelTasks(ctx context.Context, req *api.CancelTasksRequest) error {
	// Initialize the Scheduke client if it has not already been initialized.
	if err := s.setupSchedukeClient(ctx); err != nil {
		return errors.Annotate(err, "canceling Scheduke task").Err()
	}
	if err := req.Validate(); err != nil {
		return errors.Annotate(err, "canceling Scheduke task: validating request").Err()
	}
	err := s.schedukeClient.CancelTasks(req.GetTaskIds(), nil, nil)
	if err != nil {
		return errors.Annotate(err, "canceling Scheduke task: sending task cancellation request to Scheduke").Err()
	}
	return nil
}

// ShouldUseDM determines if the caller should use Device Manager or not.
func (s *schedukeAPI) ShouldUseDM() (bool, error) {
	useDM, err := common.ShouldUseDM(s.gerritClient, s.pool)
	if err != nil {
		return false, errors.Annotate(err, "should use DM: calling Scheduke").Err()
	}
	return useDM, nil
}

// setupSchedukeClient initializes the underlying Scheduke client for this API if
// it has not already been initialized.
func (s *schedukeAPI) setupSchedukeClient(ctx context.Context) error {
	// Return early if client has already been initialized.
	if s.schedukeClient != nil {
		return nil
	}

	var (
		c   *common.SchedukeClient
		err error
	)

	if s.usedByCLI {
		dev := s.pool == common.SchedukeDevPool
		c, err = common.NewSchedukeClientForEnv(ctx, dev, s.authOpts)
		if err != nil {
			return errors.Annotate(err, "initializing Scheduke client for CLI").Err()
		}
	} else {
		c, err = common.NewSchedukeClient(ctx, s.pool, false)
		if err != nil {
			return errors.Annotate(err, "initializing Scheduke client for automation").Err()
		}
	}

	s.schedukeClient = c
	return nil
}
