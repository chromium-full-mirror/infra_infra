// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package datastore

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/chromiumos/infra/proto/go/lab"
)

func makeServo(servoHost, serial string, port int) *lab.Servo {
	return &lab.Servo{
		ServoHostname: servoHost,
		ServoPort:     int32(port),
		ServoSerial:   serial,
		ServoType:     "v3",
	}
}

func TestCheckDuplicates(t *testing.T) {
	t.Parallel()
	Convey("Check duplicates by port", t, func() {
		servos := []*lab.Servo{
			makeServo("host1", "Ser2", 2),
			makeServo("host1", "Ser3", 3),
			makeServo("host1", "Ser4", 4),
			makeServo("host1", "Ser5", 5),
		}
		Convey("No duplicates", func() {
			servo := makeServo("host1", "Ser1", 1)
			err := checkDuplicatePort(servo, servos)
			So(err, ShouldBeNil)
			err = checkDuplicateSerial(servo, servos)
			So(err, ShouldBeNil)
		})
		Convey("has duplicate by port", func() {
			servo := makeServo("host1", "Ser1", 3)
			err := checkDuplicatePort(servo, servos)
			So(err, ShouldNotBeNil)
			err = checkDuplicateSerial(servo, servos)
			So(err, ShouldBeNil)
		})
		Convey("has duplicate by serial number", func() {
			servo := makeServo("host1", "Ser3", 1)
			err := checkDuplicatePort(servo, servos)
			So(err, ShouldBeNil)
			err = checkDuplicateSerial(servo, servos)
			So(err, ShouldNotBeNil)
		})
	})
}
