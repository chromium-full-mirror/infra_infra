// Copyright 2018 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package frontend_test

import (
	"testing"
	"time"

	"go.chromium.org/luci/appengine/gaetesting"
	. "go.chromium.org/luci/common/testing/truth/convey/facade"
	swarming "go.chromium.org/luci/swarming/proto/plugin"

	"go.chromium.org/infra/qscheduler/qslib/tutils"
	qscheduler "go.chromium.org/infra/qscheduler/service/api/qscheduler/v1"
	"go.chromium.org/infra/qscheduler/service/app/eventlog"
	"go.chromium.org/infra/qscheduler/service/app/frontend"
)

func TestAssignTasks(t *testing.T) {
	sch := frontend.NewBatchedServer()

	Convey("Given a batched qscheduler server", t, func(t *T) {

		Convey("in a testing context with a scheduler pool", t, func(t *T) {
			ctx := gaetesting.TestingContext()
			ctx = eventlog.Use(ctx, &eventlog.NullBQInserter{})

			poolID := "Pool1"
			admin := &frontend.QSchedulerAdminServerImpl{}
			view := &frontend.QSchedulerViewServerImpl{}
			_, err := admin.CreateSchedulerPool(ctx, &qscheduler.CreateSchedulerPoolRequest{
				PoolId: poolID,
			})
			So(t, err, ShouldBeNil)

			Convey("with an idle task that has been notified", t, func(t *T) {
				taskID := "Task1"
				req := swarming.NotifyTasksRequest{
					SchedulerId: poolID,
					Notifications: []*swarming.NotifyTasksItem{
						{
							Time: tutils.TimestampProto(time.Now()),
							Task: &swarming.TaskSpec{
								Id:    taskID,
								State: swarming.TaskSpec_PENDING,
								Slices: []*swarming.SliceSpec{
									{Dimensions: []string{"label1"}},
								},
								EnqueuedTime: tutils.TimestampProto(time.Now()),
							},
						},
					},
				}
				_, err := sch.NotifyTasks(ctx, &req)
				So(t, err, ShouldBeNil)

				resp, err := view.InspectPool(ctx, &qscheduler.InspectPoolRequest{PoolId: poolID})
				So(t, err, ShouldBeNil)
				So(t, resp.NumWaitingTasks, ShouldEqual(int32(1)))

				Convey("when AssignTasks is called with an idle bot", t, func(t *T) {
					botID := "Bot1"
					req := swarming.AssignTasksRequest{
						SchedulerId: poolID,
						Time:        tutils.TimestampProto(time.Now()),
						IdleBots: []*swarming.IdleBot{
							{BotId: botID, Dimensions: []string{"label1"}},
						},
					}
					resp, err := sch.AssignTasks(ctx, &req)
					Convey("then the task is assigned to the bot.", t, func(t *T) {
						So(t, err, ShouldBeNil)
						So(t, resp.Assignments, ShouldHaveLength(1))
						So(t, resp.Assignments[0].BotId, ShouldEqual(botID))
						So(t, resp.Assignments[0].TaskId, ShouldEqual(taskID))
					})
				})
			})
		})
	})
}
