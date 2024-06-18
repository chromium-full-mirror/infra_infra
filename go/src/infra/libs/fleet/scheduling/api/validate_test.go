// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	. "go.chromium.org/luci/common/testing/assertions"
)

func TestValidate_ScheduleTaskRequest(t *testing.T) {
	Convey("ScheduleTaskRequest Validate", t, func() {
		Convey("Valid request - successful path", func() {
			req := &ScheduleTaskRequest{
				Config: &Config{
					Backend: &Config_SchedukeBackend_{
						SchedukeBackend: &Config_SchedukeBackend{
							Env:  Config_SchedukeBackend_ENV_LOCAL,
							Pool: "test-pool",
						},
					},
				},
			}
			err := req.Validate()
			So(err, ShouldBeNil)
		})
		Convey("Invalid request - empty request", func() {
			req := &ScheduleTaskRequest{}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err, ShouldErrLike, "invalid argument: no config found")
		})
		Convey("Invalid request - empty SchedukeBackend", func() {
			req := &ScheduleTaskRequest{
				Config: &Config{},
			}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err, ShouldErrLike, "invalid argument: bad backend: want scheduke backend")
		})
	})
}

func TestValidate_CancelTasksRequest(t *testing.T) {
	Convey("CancelTasksRequest Validate", t, func() {
		Convey("Valid request - successful path", func() {
			req := &CancelTasksRequest{
				Config: &Config{
					Backend: &Config_SchedukeBackend_{
						SchedukeBackend: &Config_SchedukeBackend{
							Env:  Config_SchedukeBackend_ENV_LOCAL,
							Pool: "test-pool",
						},
					},
				},
			}
			err := req.Validate()
			So(err, ShouldBeNil)
		})
		Convey("Invalid request - empty request", func() {
			req := &CancelTasksRequest{}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err, ShouldErrLike, "invalid argument: no config found")
		})
		Convey("Invalid request - empty SchedukeBackend", func() {
			req := &CancelTasksRequest{
				Config: &Config{},
			}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err, ShouldErrLike, "invalid argument: bad backend: want scheduke backend")
		})
	})
}
