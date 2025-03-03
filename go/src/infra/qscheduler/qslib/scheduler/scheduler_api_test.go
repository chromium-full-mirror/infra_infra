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

package scheduler_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.chromium.org/luci/common/data/stringset"
	. "go.chromium.org/luci/common/testing/truth/convey/facade"

	"go.chromium.org/infra/qscheduler/qslib/scheduler"
)

type Priority = scheduler.Priority

var FreeBucket = scheduler.FreeBucket

// TestMatchAndUnassign tests that the scheduler correctly matches
// requests with idle workers, if they are available, and that the
// Unassign call reverses this assignment.
func TestMatchAndUnassign(t *testing.T) {
	Convey("Given 2 tasks and 2 idle workers", t, func(t *T) {
		ctx := context.Background()
		tm := time.Now().Add(-10 * time.Hour)
		t1 := time.Now().Add(-10 * time.Hour)
		t2 := time.Now().Add(-47 * time.Hour)
		s := scheduler.New(tm)
		w1 := scheduler.WorkerID("w1")
		w2 := scheduler.WorkerID("w2")
		r1 := scheduler.RequestID("r1")
		r2 := scheduler.RequestID("r2")
		s.MarkIdle(ctx, w1, stringset.New(0), tm, scheduler.NullEventSink)
		s.MarkIdle(ctx, w2, stringset.NewFromSlice("label1"), tm, scheduler.NullEventSink)
		// r1 should remain in the queue after unassign.
		s.AddRequest(ctx, scheduler.NewTaskRequest(r1, "a1", stringset.NewFromSlice("label1"), nil, t1), t1, nil, scheduler.NullEventSink)
		// r2 should disappear since it has hit the hard timeout(46H).
		s.AddRequest(ctx, scheduler.NewTaskRequest(r2, "a1", stringset.NewFromSlice("label2"), nil, t2), t2, nil, scheduler.NullEventSink)
		c := scheduler.NewAccountConfig(0, nil, 0, nil, false, "")
		s.AddAccount(ctx, "a1", c, []float32{2, 0, 0})
		Convey("when scheduling jobs", t, func(t *T) {
			muts := s.RunOnce(ctx, scheduler.NullEventSink)
			Convey("then both jobs should be matched, with provisionable label used as tie-breaker", t, func(t *T) {
				expects := []*scheduler.Assignment{
					{Type: scheduler.AssignmentIdleWorker, Priority: 0, RequestID: r1, WorkerID: w2, Time: tm},
					{Type: scheduler.AssignmentIdleWorker, Priority: 0, RequestID: r2, WorkerID: w1, Time: tm},
				}
				So(t, muts, ShouldResemble(expects))
				So(t, s.IsAssigned(r1, w2), ShouldBeTrue)
				So(t, s.IsAssigned(r2, w1), ShouldBeTrue)
				So(t, s.IsAssigned(r1, w1), ShouldBeFalse)
				So(t, s.IsAssigned(r2, w2), ShouldBeFalse)
			})
			Convey("then scheduling jobs again results in no new assignments.", t, func(t *T) {
				muts := s.RunOnce(ctx, scheduler.NullEventSink)
				So(t, muts, ShouldBeEmpty)
			})
			Convey("when jobs are unassigned", t, func(t *T) {
				err := s.Unassign(ctx, r1, w2, t1, scheduler.NullEventSink)
				So(t, err, ShouldBeNil)
				err = s.Unassign(ctx, r2, w1, t2, scheduler.NullEventSink)
				So(t, err, ShouldBeNil)
				Convey("then they are no longer assigned.", t, func(t *T) {
					So(t, s.IsAssigned(r1, w2), ShouldBeFalse)
					So(t, s.IsAssigned(r2, w1), ShouldBeFalse)
				})
				Convey("then they can be matched again when scheduling jobs if the task did not expire.", t, func(t *T) {
					s.MarkIdle(ctx, w1, stringset.New(0), tm, scheduler.NullEventSink)
					s.MarkIdle(ctx, w2, stringset.NewFromSlice("label1"), tm, scheduler.NullEventSink)
					muts := s.RunOnce(ctx, scheduler.NullEventSink)
					So(t, muts, ShouldHaveLength(1))
				})
			})
		})
	})
}

// TestMatchAccountless tests that requests without a valid account are matched at the lowest
// possible priority.
func TestMatchAccountless(t *testing.T) {
	Convey("Given a state with an idle worker", t, func(t *T) {
		ctx := context.Background()
		tm := time.Unix(0, 0)
		s := scheduler.New(tm)
		wid := scheduler.WorkerID("worker")
		s.MarkIdle(ctx, wid, nil, tm, scheduler.NullEventSink)

		Convey("and a request with no account", t, func(t *T) {
			rid := scheduler.RequestID("req")
			s.AddRequest(ctx, scheduler.NewTaskRequest(rid, "", nil, nil, tm), tm, nil, scheduler.NullEventSink)
			Convey("when scheduling is run", t, func(t *T) {
				muts := s.RunOnce(ctx, scheduler.NullEventSink)
				Convey("then the request is matched at lowest priority.", t, func(t *T) {
					So(t, muts, ShouldHaveLength(1))
					So(t, muts[0].Priority, ShouldEqual(scheduler.FreeBucket))
					So(t, muts[0].RequestID, ShouldEqual(rid))
					So(t, muts[0].WorkerID, ShouldEqual(wid))
				})
			})
		})
	})
}

// TestMatchProvisionableLabel tests that scheduler correctly matches provisionable
// label, even when a worker has more provisionable labels than tasks.
func TestMatchProvisionableLabel(t *testing.T) {
	Convey("Given 500 tasks with provisionable label 'a' and 1 task with provisionable label 'b'", t, func(t *T) {
		ctx := context.Background()
		tm := time.Unix(0, 0)
		aid := scheduler.AccountID("account1")
		reqB := scheduler.RequestID("reqb")
		s := scheduler.New(tm)
		s.AddAccount(ctx, aid, scheduler.NewAccountConfig(1, nil, 1, nil, false, ""), []float32{1})
		for i := range 500 {
			id := scheduler.RequestID(fmt.Sprintf("t%d", i))
			s.AddRequest(ctx, scheduler.NewTaskRequest(id, aid, stringset.NewFromSlice("a"), nil, tm), tm, nil, scheduler.NullEventSink)
		}
		s.AddRequest(ctx, scheduler.NewTaskRequest(reqB, aid, stringset.NewFromSlice("b"), nil, tm), tm, nil, scheduler.NullEventSink)

		Convey("and an idle worker with labels 'b' and 'c'", t, func(t *T) {
			wid := scheduler.WorkerID("workerID")
			s.MarkIdle(ctx, wid, stringset.NewFromSlice("b", "c"), tm, scheduler.NullEventSink)

			Convey("when scheduling jobs", t, func(t *T) {
				muts := s.RunOnce(ctx, scheduler.NullEventSink)

				Convey("then worker is matched to the task with label 'b'.", t, func(t *T) {
					So(t, muts, ShouldHaveLength(1))
					So(t, muts[0].RequestID, ShouldEqual(reqB))
					So(t, muts[0].WorkerID, ShouldEqual(wid))
				})
			})
		})
	})
}

func TestBaseLabelMatch(t *testing.T) {
	Convey("Given a state with 1 worker, and 1 request that has base labels not satisfied by the worker", t, func(t *T) {
		ctx := context.Background()
		tm := time.Unix(0, 0)
		s := scheduler.New(tm)
		var aid scheduler.AccountID = "AccountID"
		var wid scheduler.WorkerID = "WorkerID"
		var rid scheduler.RequestID = "RequestID"
		s.AddAccount(ctx, aid, scheduler.NewAccountConfig(0, nil, 0, nil, false, ""), []float32{1})
		s.MarkIdle(ctx, wid, nil, tm, scheduler.NullEventSink)
		s.AddRequest(ctx, scheduler.NewTaskRequest(rid, aid, nil, stringset.NewFromSlice("unsatisfied_label"), tm), tm, nil, scheduler.NullEventSink)
		Convey("when scheduling jobs", t, func(t *T) {
			m := s.RunOnce(ctx, scheduler.NullEventSink)
			Convey("no requests should be assigned to workers.", t, func(t *T) {
				So(t, m, ShouldBeEmpty)
			})
		})
	})
}

// TestMatchRareLabel tests that the worker-to-request match quality heuristics allow a rare worker to be matched
// to its corresponding rare request, even amidst other common requests that could use that worker.
func TestMatchRareLabel(t *testing.T) {
	Convey("Given a state with 10 interchangeable workers and 1 rare-labeled worker", t, func(t *T) {
		ctx := context.Background()
		tm := time.Unix(0, 0)
		s := scheduler.New(tm)
		commonLabel := "CommonLabel"
		for i := range 10 {
			id := scheduler.WorkerID(fmt.Sprintf("CommonWorker%d", i))
			s.MarkIdle(ctx, id, stringset.NewFromSlice(commonLabel), tm, scheduler.NullEventSink)
		}
		rareLabel := "RareLabel"
		var rareWorker scheduler.WorkerID = "RareWorker"
		s.MarkIdle(ctx, rareWorker, stringset.NewFromSlice(commonLabel, rareLabel), tm, scheduler.NullEventSink)
		Convey("and 10 interchangeable requests and 1 rare-labeled request", t, func(t *T) {
			var aid scheduler.AccountID = "AccountID"
			s.AddAccount(ctx, aid, scheduler.NewAccountConfig(0, nil, 0, nil, false, ""), []float32{1})
			for i := range 10 {
				id := scheduler.RequestID(fmt.Sprintf("CommonRequest%d", i))
				s.AddRequest(ctx, scheduler.NewTaskRequest(id, aid, nil, stringset.NewFromSlice(commonLabel), tm), tm, nil, scheduler.NullEventSink)
			}
			var rareRequest scheduler.RequestID = "RareRequest"
			s.AddRequest(ctx, scheduler.NewTaskRequest(rareRequest, aid, nil, stringset.NewFromSlice(commonLabel, rareLabel), tm), tm, nil, scheduler.NullEventSink)
			Convey("when scheduling jobs", t, func(t *T) {
				muts := s.RunOnce(ctx, scheduler.NullEventSink)
				Convey("then all jobs are scheduled to workers, including the rare requests and workers.", t, func(t *T) {
					So(t, muts, ShouldHaveLength(11))
					So(t, s.IsAssigned(rareRequest, rareWorker), ShouldBeTrue)
				})
			})
		})
	})
}

// TestAddRequest ensures that AddRequest enqueues a request.
func TestAddRequest(t *testing.T) {
	ctx := context.Background()
	tm := time.Unix(0, 0)
	s := scheduler.New(tm)
	r := scheduler.NewTaskRequest("r1", "a1", nil, nil, tm)
	s.AddRequest(ctx, r, tm, nil, scheduler.NullEventSink)

	if _, ok := s.GetRequest("r1"); !ok {
		t.Errorf("AddRequest did not enqueue request.")
	}
}

func TestExpireWorker(t *testing.T) {
	Convey("Given an empty scheduler, with an an idle worker", t, func(t *T) {
		ctx := context.Background()
		tm := time.Unix(0, 0)
		s := scheduler.New(tm)
		s.MarkIdle(ctx, "worker1", nil, tm, scheduler.NullEventSink)
		So(t, s.GetWorkers(), ShouldHaveLength(1))
		Convey("when time is updated by less than expiry threshold, worker is still idle.", t, func(t *T) {
			t2 := tm.Add(150 * time.Second)
			s.UpdateTime(ctx, t2)
			So(t, s.GetWorkers(), ShouldHaveLength(1))
		})
		Convey("when time is updated by more than expiry threshold, worker is removed.", t, func(t *T) {
			t2 := tm.Add(301 * time.Second)
			s.UpdateTime(ctx, t2)
			So(t, s.GetWorkers(), ShouldBeEmpty)
		})
	})
}
