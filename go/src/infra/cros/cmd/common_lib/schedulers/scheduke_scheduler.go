// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package schedulers

import (
	"context"
	"fmt"
	"time"

	schedukepb "go.chromium.org/chromiumos/config/go/test/scheduling"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
)

const schedukePollingWait = 30 * time.Second

// SchedukeScheduler defines a scheduler that schedules request(s) through
// Scheduke.
type SchedukeScheduler struct {
	*AbstractScheduler

	schedukeClient *common.SchedukeClient
}

func NewSchedukeScheduler() *SchedukeScheduler {
	absSched := NewAbstractScheduler(SchedukeSchedulerType)
	return &SchedukeScheduler{AbstractScheduler: absSched}
}

func (s *SchedukeScheduler) Setup(pool string) error {
	ctx := context.Background()
	if s.schedukeClient == nil {
		c, err := common.NewSchedukeClient(ctx, pool, false)
		if err != nil {
			return err
		}
		s.schedukeClient = c
	}
	return nil
}

func (s *SchedukeScheduler) ScheduleRequest(ctx context.Context, req *buildbucketpb.ScheduleBuildRequest, step *build.Step) (*buildbucketpb.Build, string, error) {
	schedukeReq, err := s.schedukeClient.TestRunnerBBReqToSchedukeReq(req)
	if err != nil {
		return nil, "", err
	}

	logging.Infof(ctx, "Sending Request to Scheduke: %s", schedukeReq)
	createTaskResponse, err := s.schedukeClient.ScheduleExecution(schedukeReq)
	if err != nil {
		return nil, "", err
	}
	logging.Infof(ctx, "Got reply from Scheduke: %s", createTaskResponse)

	// Scheduke supports batch task creation, but we send individually for now.
	taskID, ok := createTaskResponse.Ids[common.SchedukeTaskRequestKey]
	if !ok {
		return nil, "", fmt.Errorf("no task ID returned from Scheduke for request %v", schedukeReq)
	}
	step.SetSummaryMarkdown(fmt.Sprintf("task %d scheduled in Scheduke (no BB link yet)", taskID))
	taskIDsList := []int64{taskID}
	for {
		if ctx.Err() != nil {
			return nil, "", nil
		}

		taskStateResponse, err := s.schedukeClient.ReadTaskStates(taskIDsList, nil, nil)
		if err != nil {
			return nil, "", err
		}
		states := taskStateResponse.GetTasks()
		if len(states) != 1 || states[0].GetTaskStateId() != taskID {
			return nil, "", fmt.Errorf("polling Scheduke for state of task %d returned the wrong information: %v", taskID, taskStateResponse)
		}
		taskWithState := states[0]
		switch s := taskWithState.GetState(); s {
		case schedukepb.TaskState_LAUNCHED:
			// Step status will be updated by the caller (CTPv2).
			return &buildbucketpb.Build{Id: taskWithState.GetBbid()}, taskWithState.GetLeaseId(), nil
		case schedukepb.TaskState_EXPIRED:
			step.SetSummaryMarkdown(fmt.Sprintf("task %d expired in Scheduke", taskID))
			return nil, "", fmt.Errorf("scheduke task %d expired without launching", taskID)
		}

		time.Sleep(schedukePollingWait)
	}
}
