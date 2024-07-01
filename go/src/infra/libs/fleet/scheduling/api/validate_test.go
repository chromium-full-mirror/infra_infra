// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	. "go.chromium.org/luci/common/testing/assertions"
)

func TestValidate_ScheduleTaskRequest(t *testing.T) {
	Convey("ScheduleTaskRequest Validate", t, func() {
		Convey("Valid request - successful path", func() {
			req := &ScheduleTaskRequest{
				DeviceName: "foo-device",
				BuildbucketRequest: &buildbucketpb.ScheduleBuildRequest{
					Builder: &buildbucketpb.BuilderID{
						Project: "foo",
						Bucket:  "bar",
						Builder: "baz",
					},
				},
			}
			err := req.Validate()
			So(err, ShouldBeNil)
		})
		Convey("Invalid request - nil BB request", func() {
			req := &ScheduleTaskRequest{
				DeviceName: "foo-device",
			}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err, ShouldErrLike, "invalid argument: no builder specified in BB request")
		})
		Convey("Invalid request - empty BB request", func() {
			req := &ScheduleTaskRequest{
				BuildbucketRequest: &buildbucketpb.ScheduleBuildRequest{
					Builder: &buildbucketpb.BuilderID{},
				},
				DeviceName: "foo-device",
			}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err, ShouldErrLike, "invalid argument: no builder specified in BB request")
		})
		Convey("Invalid request - empty device name", func() {
			req := &ScheduleTaskRequest{
				BuildbucketRequest: &buildbucketpb.ScheduleBuildRequest{
					Builder: &buildbucketpb.BuilderID{
						Project: "foo",
						Bucket:  "bar",
						Builder: "baz",
					},
				},
			}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err, ShouldErrLike, "invalid format: no device name")
		})
	})
}

func TestValidate_CancelTasksRequest(t *testing.T) {
	Convey("CancelTasksRequest Validate", t, func() {
		Convey("Valid request - successful path", func() {
			req := &CancelTasksRequest{
				TaskIds: []int64{1, 2},
			}
			err := req.Validate()
			So(err, ShouldBeNil)
		})
		Convey("Invalid request - nil task IDs", func() {
			req := &CancelTasksRequest{}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err, ShouldErrLike, "invalid argument: no task IDs")
		})
		Convey("Invalid request - no task IDs", func() {
			req := &CancelTasksRequest{
				TaskIds: []int64{},
			}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err, ShouldErrLike, "invalid argument: no task IDs")
		})
	})
}
