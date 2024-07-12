// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package buildbucket

import (
	"context"
	"net/http"
	"sort"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	. "go.chromium.org/luci/common/testing/assertions"
	"go.chromium.org/luci/grpc/prpc"

	schedulingapi "infra/libs/fleet/scheduling/api"
)

// TestAsMap tests structbuilder-compatibility.
//
// Make sure that we only have keys of a type that structbuilder understands.
//
// We will be more conservative than structbuilder and reject everything that isn't a bool or a string.
//
// Keep the function deterministic by sorting the keys before we check for
// values that have an unsupported type.
func TestAsMap(t *testing.T) {
	t.Parallel()
	zero := Params{}
	zeroMap := zero.AsMap()

	// Keep the function deterministic by sorting the keys before we check for
	// values that have an unsupported type.
	keys := make([]string, 0, len(zeroMap))
	for k := range zeroMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := zeroMap[k]
		switch v := v.(type) {
		case bool, string:
			// do nothing
		default:
			t.Errorf("key %q has value %v with unsupported type %T", k, v, v)
		}
	}
}

type FakeClient struct {
	startID int64
}

func (c *FakeClient) ScheduleLabpackTask(ctx context.Context, params *ScheduleLabpackTaskParams, _ string) (string, int64, error) {
	id := c.startID
	c.startID++
	return "", id, nil
}

func (c *FakeClient) CreateLabpackTask(ctx context.Context, params *ScheduleLabpackTaskParams, _ schedulingapi.TaskSchedulingAPI) (string, int64, error) {
	id := c.startID
	c.startID++
	return "", id, nil
}

func (c *FakeClient) BuildURL(buildID int64) string {
	panic("BuildURL should not be called!")
}

type FakeSchedulingAPI struct {
	shouldUseDM bool
}

func (s *FakeSchedulingAPI) ScheduleTask(_ context.Context, _ *schedulingapi.ScheduleTaskRequest) (*schedulingapi.Task, error) {
	return &schedulingapi.Task{Id: 42, Url: "test-url-from-scheduke"}, nil
}

func (s *FakeSchedulingAPI) CancelTasks(_ context.Context, _ *schedulingapi.CancelTasksRequest) error {
	return nil
}

func (s *FakeSchedulingAPI) ShouldUseDM(_ context.Context) (bool, error) {
	return s.shouldUseDM, nil
}

// TestScheduleTask tests whether schedule task accepts or rejects its arguments, basically.
func TestScheduleTask(t *testing.T) {
	t.Parallel()
	Convey("test schedule task with stubbed BB wrapper and stubbed inactive scheduling API", t, func() {
		ctx := context.Background()
		Convey("nil params", func() {
			_, _, err := ScheduleTask(ctx, &FakeClient{}, CIPDProd, nil, "fake service")
			So(err, ShouldNotBeNil)
			So(err, ShouldErrLike, "schedule task")
		})
		Convey("audit-rpm", func() {
			_, bbid, err := ScheduleTask(ctx, &FakeClient{startID: 3}, CIPDProd, &Params{
				BuilderName: "audit-rpm",
			}, "fake service")
			So(err, ShouldBeNil)
			So(bbid, ShouldEqual, 3)
		})
	})
}

// CreateTask tests whether schedule task accepts or rejects its arguments, basically.
func TestCreateTask(t *testing.T) {
	t.Parallel()
	Convey("test schedule task with real BB wrapper and stubbed active scheduling API", t, func() {
		ctx := context.Background()
		hc := &http.Client{}
		bc, err := NewClient(ctx, hc, prpc.DefaultOptions())
		So(err, ShouldBeNil)
		sc := &FakeSchedulingAPI{
			shouldUseDM: true,
		}
		Convey("audit-rpm", func() {
			url, bbid, err := CreateTask(ctx, bc, sc, CIPDProd, &Params{
				BuilderName: "audit-rpm",
			}, "fake service")
			So(err, ShouldBeNil)
			So(url, ShouldEqual, "test-url-from-scheduke")
			// No BBID returned since Scheduke doesn't generate them immediately.
			So(bbid, ShouldEqual, 0)
		})
	})
}
