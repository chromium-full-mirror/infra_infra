// Copyright 2019 The LUCI Authors.
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

package state_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	. "go.chromium.org/luci/common/testing/truth/convey/facade"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/clock/testclock"
	swarming "go.chromium.org/luci/swarming/proto/plugin"

	"infra/qscheduler/qslib/tutils"
	"infra/qscheduler/service/app/eventlog"
	"infra/qscheduler/service/app/state"
	"infra/qscheduler/service/app/state/nodestore"
)

func TestBatcherCancellations(t *testing.T) {
	Convey("Given a testing context with a scheduler pool, and a batcher for that pool", t, func(t *T) {
		ctx := gaetesting.TestingContext()
		ctx, _ = testclock.UseTime(ctx, time.Now())
		ctx = eventlog.Use(ctx, &eventlog.NullBQInserter{})
		poolID := "pool 1"
		store := nodestore.For(poolID)
		store.Create(ctx, time.Now())

		batcher := state.NewBatchRunnerForTest()
		batcher.Start(store)
		defer batcher.Close()

		Convey("with a bunch of requests in a batch", t, func(t *T) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			nRequests := 10
			errs := make([]error, nRequests)

			wg := sync.WaitGroup{}
			wg.Add(nRequests)

			go func() {
				for i := 0; i < nRequests; i++ {
					go func(i int) {
						defer wg.Done()
						_, err := batcher.TryNotify(ctx, &swarming.NotifyTasksRequest{})
						errs[i] = err
					}(i)
				}
			}()
			batcher.TBatchWait(nRequests)

			Convey("when the context is cancelled, the whole batch unwinds.", t, func(t *T) {
				cancel()
				wg.Wait()
				for _, err := range errs {
					So(t, err, ShouldEqual(context.Canceled))
				}
			})
		})
	})
}

func TestBatcherBehavior(t *testing.T) {
	Convey("Given a testing context with a scheduler pool, and a batcher for that pool", t, func(t *T) {
		ctx := gaetesting.TestingContext()
		ctx, _ = testclock.UseTime(ctx, time.Now())
		ctx = eventlog.Use(ctx, &eventlog.NullBQInserter{})
		poolID := "pool 1"
		store := nodestore.For(poolID)
		store.Create(ctx, time.Now())

		batcher := state.NewBatchRunnerForTest()
		batcher.Start(store)
		defer batcher.Close()

		Convey("a batch of requests can run, with notifications coming before assignments.", t, func(t *T) {
			nTasks := 5
			labels := make([]string, nTasks)
			// Give each bot-task pair a unique dimension.
			for i := range labels {
				labels[i] = uuid.New().String()
			}
			assignements := make([]*swarming.AssignTasksResponse, nTasks)
			now := tutils.TimestampProto(time.Now())

			wg := sync.WaitGroup{}
			for i := 0; i < nTasks; i++ {
				wg.Add(2)
				// Run nTasks assignment requests concurrently.
				go func(i int) {
					defer wg.Done()
					req := &swarming.AssignTasksRequest{
						IdleBots: []*swarming.IdleBot{
							{
								BotId:      fmt.Sprintf("%d", i),
								Dimensions: []string{labels[i]},
							},
						},
						Time: now,
					}
					resp, err := batcher.TryAssign(ctx, req)
					if err != nil {
						panic(err)
					}
					assignements[i] = resp
				}(i)
				// Also run nTasks task notifications concurrently.
				go func(i int) {
					defer wg.Done()
					req := &swarming.NotifyTasksRequest{
						Notifications: []*swarming.NotifyTasksItem{
							{
								Task: &swarming.TaskSpec{
									EnqueuedTime: now,
									Id:           fmt.Sprintf("%d", i),
									State:        swarming.TaskSpec_PENDING,
									Slices: []*swarming.SliceSpec{
										{
											Dimensions: []string{labels[i]},
										},
									},
								},
								Time: now,
							},
						},
					}
					resp, err := batcher.TryNotify(ctx, req)
					if err != nil {
						panic(err)
					}
					if resp == nil {
						panic("unexpectedly nil response")
					}
				}(i)
			}
			batcher.TBatchWait(2 * nTasks)
			batcher.TBatchStart()
			wg.Wait()
			// All tasks should be assigned to their corresponding idle bot.
			for _, a := range assignements {
				So(t, a.Assignments, ShouldHaveLength(1))
				So(t, a.Assignments[0].BotId, ShouldEqual(a.Assignments[0].TaskId))
			}
		})
	})
}
