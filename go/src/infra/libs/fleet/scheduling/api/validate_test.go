// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import (
	"testing"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestValidate_ScheduleTaskRequest(t *testing.T) {
	ftt.Run("ScheduleTaskRequest Validate", t, func(t *ftt.Test) {
		t.Run("Valid request - successful path", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Invalid request - nil BB request", func(t *ftt.Test) {
			req := &ScheduleTaskRequest{
				DeviceName: "foo-device",
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("invalid argument: no builder specified in BB request"))
		})
		t.Run("Invalid request - empty BB request", func(t *ftt.Test) {
			req := &ScheduleTaskRequest{
				BuildbucketRequest: &buildbucketpb.ScheduleBuildRequest{
					Builder: &buildbucketpb.BuilderID{},
				},
				DeviceName: "foo-device",
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("invalid argument: no builder specified in BB request"))
		})
		t.Run("Invalid request - empty device name", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("invalid format: no device name"))
		})
	})
}

func TestValidate_CancelTasksRequest(t *testing.T) {
	ftt.Run("CancelTasksRequest Validate", t, func(t *ftt.Test) {
		t.Run("Valid request - successful path", func(t *ftt.Test) {
			req := &CancelTasksRequest{
				TaskIds: []int64{1, 2},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Invalid request - nil task IDs", func(t *ftt.Test) {
			req := &CancelTasksRequest{}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("invalid argument: no task IDs"))
		})
		t.Run("Invalid request - no task IDs", func(t *ftt.Test) {
			req := &CancelTasksRequest{
				TaskIds: []int64{},
			}
			err := req.Validate()
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("invalid argument: no task IDs"))
		})
	})
}
