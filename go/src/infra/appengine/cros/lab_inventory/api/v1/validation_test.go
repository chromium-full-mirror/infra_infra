// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/chromiumos/infra/proto/go/lab"
)

func TestGetDevicesValidation(t *testing.T) {
	t.Parallel()

	hostname := DeviceID{
		Id: &DeviceID_Hostname{
			Hostname: "the_hostname",
		},
	}
	id := DeviceID{
		Id: &DeviceID_ChromeosDeviceId{
			ChromeosDeviceId: "UUID:123",
		},
	}
	Convey("Delete devices from storage backend", t, func() {
		Convey("Empty request", func() {
			req := GetCrosDevicesRequest{}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "must specify device ID(s)")
		})
		Convey("Happy path", func() {
			req := GetCrosDevicesRequest{
				Ids: []*DeviceID{&hostname, &id},
			}
			err := req.Validate()
			So(err, ShouldBeNil)
		})
	})
}

func TestUpdateDutStatusValidation(t *testing.T) {
	t.Parallel()
	state1 := lab.DutState{
		Id: &lab.ChromeOSDeviceID{Value: "UUID:01"},
	}
	dutMeta1 := DutMeta{
		ChromeosDeviceId: "UUID:11",
	}
	Convey("Update DUT status", t, func() {
		Convey("empty request", func() {
			req := &UpdateDutsStatusRequest{}
			err := req.Validate()
			So(err, ShouldNotBeNil)
		})

		Convey("zero devices", func() {
			req := &UpdateDutsStatusRequest{States: nil}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "no devices to update")
		})

		Convey("Request has two identical entries", func() {
			req := &UpdateDutsStatusRequest{States: []*lab.DutState{&state1, &state1}}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "Duplicated id found")
		})
		Convey("Request has two identical dut metas", func() {
			req := &UpdateDutsStatusRequest{
				States:   []*lab.DutState{&state1},
				DutMetas: []*DutMeta{&dutMeta1, &dutMeta1},
			}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "Duplicated id found in meta")
		})
		Convey("Request has unmatched meta and state", func() {
			req := &UpdateDutsStatusRequest{
				States:   []*lab.DutState{&state1},
				DutMetas: []*DutMeta{&dutMeta1},
			}
			err := req.Validate()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "Cannot update meta without valid dut states")
		})
		Convey("Happy path", func() {
			req := &UpdateDutsStatusRequest{States: []*lab.DutState{&state1}}
			err := req.Validate()
			So(err, ShouldBeNil)
		})
	})
}
