// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package datastore

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/luci/appengine/gaetesting"
)

func TestGetDevices(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	Convey("Get devices from an empty datastore", t, func() {
		Convey("Get all", func() {
			devs, err := GetAllDevices(ctx)
			So(devs, ShouldBeEmpty)
			So(err, ShouldBeNil)
		})
		Convey("Get by hostnames", func() {
			result := GetDevicesByHostnames(ctx, []string{"dut1", "labstation2"})
			So(result.Passed(), ShouldBeEmpty)
			So(result.Failed(), ShouldHaveLength, 2)
		})
	})
}
