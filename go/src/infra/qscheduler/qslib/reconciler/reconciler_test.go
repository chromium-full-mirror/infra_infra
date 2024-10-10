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

package reconciler

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kylelemons/godebug/pretty"

	"go.chromium.org/luci/common/data/stringset"
	. "go.chromium.org/luci/common/testing/truth/convey/facade"

	"infra/qscheduler/qslib/scheduler"
)

func assertAssignments(t *testing.T, description string,
	got []Assignment, want []Assignment) {
	t.Helper()
	if diff := pretty.Compare(got, want); diff != "" {
		t.Errorf(fmt.Sprintf("%s got unexpected assignment diff (-got +want): %s", description, diff))
	}
}

// TestOneAssignment tests that a scheduler assignment for a single idle
// worker is correctly assigned, and that subsequent calls after Notify
// return the correct results.
func TestOneAssignment(t *testing.T) {
	Convey("Given an empty scheduler and reconciler state", t, func(t *T) {
		ctx := context.Background()
		t0 := time.Unix(0, 0)
		t1 := time.Unix(1, 0)
		t2 := time.Unix(2, 0)
		s := scheduler.New(t0)
		r := New()

		Convey("given an idle task has been notified", t, func(t *T) {
			aid := scheduler.AccountID("Account1")
			labels := stringset.NewFromSlice("Label1")
			rid := scheduler.RequestID("Request1")
			taskUpdate := &TaskWaitingRequest{
				AccountID:           aid,
				ProvisionableLabels: labels,
				RequestID:           rid,
				Time:                t0,
				EnqueueTime:         t0,
			}

			r.NotifyTaskWaiting(ctx, s, scheduler.NullEventSink, taskUpdate)

			Convey("when AssignTasks is called for worker with task's provisionable label", t, func(t *T) {
				wid := scheduler.WorkerID("Worker1")
				as := r.AssignTasks(ctx, s, t0, scheduler.NullEventSink, &IdleWorker{ID: wid, Labels: labels})

				Convey("then it is given the assigned task with no provision required.", t, func(t *T) {
					So(t, as, ShouldHaveLength(1))
					a := as[0]
					So(t, a.RequestID, ShouldEqual(rid))
					So(t, a.WorkerID, ShouldEqual(wid))
					So(t, a.ProvisionRequired, ShouldBeFalse)
				})
			})

			Convey("when AssignTasks is called for worker without task's provisionable label", t, func(t *T) {
				wid := scheduler.WorkerID("Worker1")
				as := r.AssignTasks(ctx, s, t0, scheduler.NullEventSink, &IdleWorker{ID: wid})

				Convey("then it is given the assigned task with provision required.", t, func(t *T) {
					So(t, as, ShouldHaveLength(1))
					a := as[0]
					So(t, a.RequestID, ShouldEqual(rid))
					So(t, a.WorkerID, ShouldEqual(wid))
					So(t, a.ProvisionRequired, ShouldBeTrue)
				})

				Convey("when AssignTasks is called again for the same worker", t, func(t *T) {
					as = r.AssignTasks(ctx, s, t1, scheduler.NullEventSink, &IdleWorker{ID: wid})
					Convey("then it is given the same task.", t, func(t *T) {
						So(t, as, ShouldHaveLength(1))
						a := as[0]
						So(t, a.RequestID, ShouldEqual(rid))
						So(t, a.WorkerID, ShouldEqual(wid))
					})
				})

				matchingNotifyCases := []struct {
					desc string
					t    time.Time
				}{
					{
						"at a future time",
						t1,
					},
					{
						"at the same time",
						t0,
					},
				}
				for _, c := range matchingNotifyCases {
					Convey(fmt.Sprintf("when the task is notified on the worker %s", c.desc), t, func(t *T) {
						taskUpdate := &TaskRunningRequest{
							RequestID: rid,
							WorkerID:  wid,
							Time:      c.t,
						}
						r.NotifyTaskRunning(ctx, s, scheduler.NullEventSink, taskUpdate)

						Convey("when AssignTasks is called again for the same worker", t, func(t *T) {
							as = r.AssignTasks(ctx, s, t2, scheduler.NullEventSink, &IdleWorker{ID: wid})
							Convey("then it is no longer given the task.", t, func(t *T) {
								So(t, as, ShouldBeEmpty)
							})
						})
					})
				}

				Convey("when a different task is notified on the worker", t, func(t *T) {
					rid2 := scheduler.RequestID("Request2")
					taskUpdate := &TaskRunningRequest{
						RequestID: rid2,
						WorkerID:  wid,
						Time:      t1,
					}
					r.NotifyTaskRunning(ctx, s, scheduler.NullEventSink, taskUpdate)

					Convey("when AssignTasks is called again for the same worker", t, func(t *T) {
						as = r.AssignTasks(ctx, s, t2, scheduler.NullEventSink, &IdleWorker{ID: wid})
						Convey("then it is no longer given the task.", t, func(t *T) {
							So(t, as, ShouldBeEmpty)
						})
					})
				})

				Convey("when the task is notified on a different worker", t, func(t *T) {
					wid2 := scheduler.WorkerID("Worker2")
					taskUpdate := &TaskRunningRequest{
						RequestID: rid,
						WorkerID:  wid2,
						Time:      t1,
					}
					r.NotifyTaskRunning(ctx, s, scheduler.NullEventSink, taskUpdate)
					Convey("when AssignTasks is called again for the same worker", t, func(t *T) {
						as := r.AssignTasks(ctx, s, t2, scheduler.NullEventSink, &IdleWorker{ID: wid})
						Convey("then it is no longer given the task.", t, func(t *T) {
							So(t, as, ShouldBeEmpty)
						})
					})

				})

			})

		})
	})

}

// TestQueuedAssignment tests that a scheduler assignment is queued until
// the relevant worker calls AssignTasks.
func TestQueuedAssignment(t *testing.T) {
	Convey("Given an empty scheduler and reconciler state", t, func(t *T) {
		ctx := context.Background()
		t0 := time.Now().Add(-10 * time.Hour)
		r := New()
		s := scheduler.New(t0)
		Convey("given a worker with a label is idle", t, func(t *T) {
			preferredWorkerID := scheduler.WorkerID("Worker1")
			labels := stringset.NewFromSlice("Label1")
			r.AssignTasks(ctx, s, t0, scheduler.NullEventSink, &IdleWorker{preferredWorkerID, labels})
			Convey("given a request is enqueued with that label", t, func(t *T) {
				rid := scheduler.RequestID("Request1")
				taskUpdate := &TaskWaitingRequest{
					EnqueueTime:         t0,
					Time:                t0,
					ProvisionableLabels: labels,
					RequestID:           rid,
				}
				r.NotifyTaskWaiting(ctx, s, scheduler.NullEventSink, taskUpdate)
				Convey("when a different worker without that label calls AssignTasks", t, func(t *T) {
					otherWorkerID := scheduler.WorkerID("Worker2")
					otherWorker := &IdleWorker{otherWorkerID, stringset.New(0)}
					t1 := time.Now().Add(-10 * time.Hour)
					as := r.AssignTasks(ctx, s, t1, scheduler.NullEventSink, otherWorker)
					Convey("then it is given no task.", t, func(t *T) {
						So(t, as, ShouldBeEmpty)
					})
					Convey("when the labeled worker calls AssignTasks", t, func(t *T) {
						as = r.AssignTasks(ctx, s, t1, scheduler.NullEventSink, &IdleWorker{preferredWorkerID, labels})
						Convey("it is given the task.", t, func(t *T) {
							So(t, as, ShouldHaveLength(1))
							So(t, as[0].RequestID, ShouldEqual(rid))
							So(t, as[0].WorkerID, ShouldEqual(preferredWorkerID))
							So(t, as[0].ProvisionRequired, ShouldBeFalse)
						})
						Convey("when queue timeout expires without the preferred worker picking up task", t, func(t *T) {
							t2 := t1.Add(2 * WorkerQueueTimeout)
							as := r.AssignTasks(ctx, s, t2, scheduler.NullEventSink, otherWorker)
							Convey("then the other worker is given the task.", t, func(t *T) {
								So(t, as, ShouldHaveLength(1))
								So(t, as[0].RequestID, ShouldEqual(rid))
								So(t, as[0].WorkerID, ShouldEqual(otherWorkerID))
								So(t, as[0].ProvisionRequired, ShouldBeTrue)
							})
						})
					})

				})
			})
		})

	})
}

func TestPreemption(t *testing.T) {
	Convey("Given an empty scheduler and reconciler state", t, func(t *T) {
		ctx := context.Background()
		t0 := time.Unix(0, 0)
		r := New()
		s := scheduler.New(t0)

		Convey("given task & idle worker, and AssignTasks called and worker running task", t, func(t *T) {
			oldRequest := scheduler.RequestID("Request1")
			taskUpdate := &TaskWaitingRequest{
				EnqueueTime: t0,
				Time:        t0,
				RequestID:   oldRequest,
			}
			r.NotifyTaskWaiting(ctx, s, scheduler.NullEventSink, taskUpdate)

			wid := scheduler.WorkerID("Worker1")
			r.AssignTasks(ctx, s, t0, scheduler.NullEventSink, &IdleWorker{ID: wid})

			// Note: This is more of a test of the scheduler's behavior than the
			// reconciler, but it is a precondition for the rest of the test cases.
			So(t, s.IsAssigned(oldRequest, wid), ShouldBeTrue)

			Convey("given a new request with higher priority", t, func(t *T) {
				aid := scheduler.AccountID("Account1")
				s.AddAccount(ctx, aid, scheduler.NewAccountConfig(0, nil, 0, nil, false, ""), []float32{1})
				t1 := time.Unix(1, 0)
				newRequest := scheduler.RequestID("Request2")
				taskUpdate := &TaskWaitingRequest{
					AccountID:   aid,
					EnqueueTime: t1,
					Time:        t1,
					RequestID:   newRequest,
				}
				r.NotifyTaskWaiting(ctx, s, scheduler.NullEventSink, taskUpdate)

				Convey("when AssignTasks called with no idle workers & scheduler preempts old request with new one", t, func(t *T) {
					r.AssignTasks(ctx, s, t1, scheduler.NullEventSink)

					// Note: This is more of a test of the scheduler's behavior than the
					// reconciler, but it is a precondition for the rest of the test cases.
					So(t, s.IsAssigned(newRequest, wid), ShouldBeTrue)

					Convey("when GetCancellations is called", t, func(t *T) {
						c := r.Cancellations(ctx)
						Convey("then it returns a cancellation for the old request on that worker.", t, func(t *T) {
							So(t, c, ShouldHaveLength(1))
							So(t, c[0].RequestID, ShouldEqual(string(oldRequest)))
							So(t, c[0].WorkerID, ShouldEqual(string(wid)))
						})
					})

					Convey("when Notify is called to inform that the old request is cancelled", t, func(t *T) {
						t2 := time.Unix(2, 0)
						r.NotifyTaskAbsent(ctx, s, scheduler.NullEventSink, &TaskAbsentRequest{RequestID: oldRequest, Time: t2})
						Convey("when GetCancellations is called", t, func(t *T) {
							c := r.Cancellations(ctx)
							Convey("then it returns nothing.", t, func(t *T) {
								So(t, c, ShouldBeEmpty)
							})
						})
					})

					Convey("when AssignTasks is called for the intended worker", t, func(t *T) {
						t2 := time.Unix(2, 0)
						as := r.AssignTasks(ctx, s, t2, scheduler.NullEventSink, &IdleWorker{wid, stringset.New(0)})
						Convey("then it returns the preempting request.", t, func(t *T) {
							So(t, as, ShouldHaveLength(1))
							So(t, as[0].RequestID, ShouldEqual(newRequest))
							So(t, as[0].WorkerID, ShouldEqual(wid))
						})
					})

					Convey("when AssignTasks is called for a different worker prior to ACK of the cancellation", t, func(t *T) {
						t2 := time.Unix(2, 0)
						wid2 := scheduler.WorkerID("Worker2")
						as := r.AssignTasks(ctx, s, t2, scheduler.NullEventSink, &IdleWorker{wid2, stringset.New(0)})
						Convey("then it returns nothing.", t, func(t *T) {
							So(t, as, ShouldHaveLength(0))
						})
					})

					Convey("when the cancellation is ACKed", t, func(t *T) {
						t2 := time.Unix(2, 0)

						taskUpdate := &TaskWaitingRequest{
							EnqueueTime: t0,
							Time:        t0,
							RequestID:   oldRequest,
						}
						r.NotifyTaskWaiting(ctx, s, scheduler.NullEventSink, taskUpdate)

						Convey("when AssignTasks is called for a different worker", t, func(t *T) {

							wid2 := scheduler.WorkerID("Worker2")
							as := r.AssignTasks(ctx, s, t2, scheduler.NullEventSink, &IdleWorker{wid2, stringset.New(0)})
							Convey("then it returns the previously cancelled request.", t, func(t *T) {
								So(t, as, ShouldHaveLength(1))
								So(t, as[0].RequestID, ShouldEqual(oldRequest))
								So(t, as[0].WorkerID, ShouldEqual(wid2))
							})
						})

						Convey("when AssignTasks is called for the intended worker and a different worker simultaneously", t, func(t *T) {
							wid2 := scheduler.WorkerID("Worker2")
							as := r.AssignTasks(ctx, s, t2, scheduler.NullEventSink, &IdleWorker{wid, stringset.New(0)}, &IdleWorker{wid2, stringset.New(0)})
							Convey("then intended worker receives preempting request, other receives preempted request.", t, func(t *T) {
								So(t, as, ShouldHaveLength(2))
								a1 := Assignment{RequestID: newRequest, WorkerID: wid}
								a2 := Assignment{RequestID: oldRequest, WorkerID: wid2}
								asm := make(map[scheduler.WorkerID]Assignment)
								for _, a := range as {
									asm[a.WorkerID] = a
								}
								So(t, asm[a1.WorkerID], ShouldResemble(a1))
								So(t, asm[a2.WorkerID], ShouldResemble(a2))
							})
						})
					})
				})
			})
		})
	})
}

func TestTaskError(t *testing.T) {
	Convey("Given an empty reconciler and scheduler state", t, func(t *T) {
		ctx := context.Background()
		t0 := time.Unix(0, 0)
		r := New()
		s := scheduler.New(t0)

		Convey("when TaskError is called for a new task", t, func(t *T) {
			taskID := scheduler.RequestID("Task1")
			err := errors.New("an frabjous error occurred")
			r.AddTaskError(taskID, err)

			Convey("when GetCancellations is called", t, func(t *T) {
				c := r.Cancellations(ctx)
				Convey("then it returns the error'ed task.", t, func(t *T) {
					So(t, c, ShouldHaveLength(1))
					So(t, c[0].RequestID, ShouldEqual(string(taskID)))
					So(t, c[0].WorkerID, ShouldEqual(""))
					So(t, c[0].ErrorMessage, ShouldContainSubstring("frabjous"))
				})
			})
			Convey("when NotifyRequest is called to abort the task", t, func(t *T) {
				r.NotifyTaskAbsent(ctx, s, scheduler.NullEventSink, &TaskAbsentRequest{RequestID: taskID, Time: t0})
				Convey("when GetCancellations is called", t, func(t *T) {
					c := r.Cancellations(ctx)
					Convey("then it returns nothing.", t, func(t *T) {
						So(t, c, ShouldBeEmpty)
					})
				})
			})
		})
	})
}
