// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/datastore"

	rpb "infra/appengine/rotation-proxy/proto"
)

var person1 = &rpb.OncallPerson{Email: "person1@google.com"}
var person2 = &rpb.OncallPerson{Email: "person2@google.com"}
var person3 = &rpb.OncallPerson{Email: "person3@google.com"}
var person4 = &rpb.OncallPerson{Email: "person4@google.com"}
var person5 = &rpb.OncallPerson{Email: "person5@google.com"}
var oncallsShift1 = []*rpb.OncallPerson{person1, person2}
var startTimeShift1 = &timestamp.Timestamp{Seconds: 111, Nanos: 0}
var endTimeShift1 = &timestamp.Timestamp{Seconds: 222, Nanos: 0}
var oncallsShift2 = []*rpb.OncallPerson{person3, person4}
var startTimeShift2 = &timestamp.Timestamp{Seconds: 333, Nanos: 0}
var endTimeShift2 = &timestamp.Timestamp{Seconds: 555, Nanos: 0}
var oncallsShift3 = []*rpb.OncallPerson{person5}
var startTimeShift3 = &timestamp.Timestamp{Seconds: 777, Nanos: 0}
var endTimeShift3 = &timestamp.Timestamp{Seconds: 777, Nanos: 0}

var rotation1 = &rpb.Rotation{
	Name: "rotation1",
	Shifts: []*rpb.Shift{
		{
			Oncalls:   oncallsShift1,
			StartTime: startTimeShift1,
			EndTime:   endTimeShift1,
		},
		{
			Oncalls:   oncallsShift2,
			StartTime: startTimeShift2,
			EndTime:   endTimeShift2,
		},
	},
}

func TestBatchUpdateRotations(t *testing.T) {
	ctx := memory.Use(context.Background())
	cl := testclock.New(testclock.TestTimeUTC)
	currentTime := time.Unix(10000, 0).UTC()
	cl.Set(currentTime)
	ctx = clock.Set(ctx, cl)

	server := &RotationProxyServer{}

	ftt.Run("batch update rotations new rotation", t, func(t *ftt.Test) {
		// TODO(nqmtuan): Figure out how can we set datastore to deal with
		// multiple entity groups in testing.
		// Currently, it is complaining about enabling XG=true, which should not
		// be required, since we are using firestore in datastore mode.
		request := &rpb.BatchUpdateRotationsRequest{
			Requests: []*rpb.UpdateRotationRequest{
				{Rotation: rotation1},
			},
		}
		response, err := server.BatchUpdateRotations(ctx, request)
		datastore.GetTestable(ctx).CatchupIndexes()

		assert.Loosely(t, err, should.BeNil)

		// Checking response
		rotations := response.Rotations
		assert.Loosely(t, len(rotations), should.Equal(1))
		assert.Loosely(t, rotations[0], should.Equal(rotation1))

		// Checking data in datastore
		q := datastore.NewQuery("Rotation")
		dsRotations := []*Rotation{}
		err = datastore.GetAll(ctx, q, &dsRotations)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(dsRotations), should.Equal(1))
		diff := cmp.Diff(rotation1, &dsRotations[0].Proto, cmp.Comparer(proto.Equal))
		assert.Loosely(t, diff, should.BeEmpty)
		assert.Loosely(t, dsRotations[0].ExpiryAt, should.Match(currentTime.Add(7*24*time.Hour)))
	})

	ftt.Run("batch update rotations should delete previous shifts", t, func(t *ftt.Test) {
		rotation1Updated := &rpb.Rotation{
			Name: "rotation1",
			Shifts: []*rpb.Shift{
				{
					Oncalls:   oncallsShift3,
					StartTime: startTimeShift3,
					EndTime:   endTimeShift3,
				},
			},
		}
		request := &rpb.BatchUpdateRotationsRequest{
			Requests: []*rpb.UpdateRotationRequest{
				{Rotation: rotation1Updated},
			},
		}
		_, err := server.BatchUpdateRotations(ctx, request)
		datastore.GetTestable(ctx).CatchupIndexes()
		assert.Loosely(t, err, should.BeNil)
		q := datastore.NewQuery("Rotation")
		dsRotations := []*Rotation{}
		err = datastore.GetAll(ctx, q, &dsRotations)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(dsRotations), should.Equal(1))
		diff := cmp.Diff(rotation1Updated, &dsRotations[0].Proto, cmp.Comparer(proto.Equal))
		assert.Loosely(t, diff, should.BeEmpty)
		assert.Loosely(t, dsRotations[0].ExpiryAt, should.Match(currentTime.Add(7*24*time.Hour)))
	})
}

func TestGetRotation(t *testing.T) {
	ctx := memory.Use(context.Background())
	cl := testclock.New(testclock.TestTimeUTC)
	ctx = clock.Set(ctx, cl)

	server := &RotationProxyServer{}
	ftt.Run("get rotation", t, func(t *ftt.Test) {
		var rotation = &rpb.Rotation{
			Name: "rotation",
			Shifts: []*rpb.Shift{
				{
					Oncalls:   oncallsShift3,
					StartTime: startTimeShift3,
				},
				{
					Oncalls:   oncallsShift2,
					StartTime: startTimeShift2,
					EndTime:   endTimeShift2,
				},
				{
					Oncalls:   oncallsShift1,
					StartTime: startTimeShift1,
					EndTime:   endTimeShift1,
				},
			},
		}
		updateRequest := &rpb.BatchUpdateRotationsRequest{
			Requests: []*rpb.UpdateRotationRequest{
				{Rotation: rotation},
			},
		}
		_, err := server.BatchUpdateRotations(ctx, updateRequest)
		datastore.GetTestable(ctx).CatchupIndexes()
		assert.Loosely(t, err, should.BeNil)

		getRequest := &rpb.GetRotationRequest{
			Name: "rotation",
		}

		// Mock clock
		ctx, _ = testclock.UseTime(ctx, time.Unix(444, 0))

		response, err := server.GetRotation(ctx, getRequest)
		assert.Loosely(t, err, should.BeNil)
		expected := &rpb.Rotation{
			Name: "rotation",
			Shifts: []*rpb.Shift{
				{
					Oncalls:   oncallsShift2,
					StartTime: startTimeShift2,
					EndTime:   endTimeShift2,
				},
				{
					Oncalls:   oncallsShift3,
					StartTime: startTimeShift3,
				},
			},
		}
		diff := cmp.Diff(expected, response, cmp.Comparer(proto.Equal))
		assert.Loosely(t, diff, should.BeEmpty)
	})
}

func TestBatchGetRotations(t *testing.T) {
	ctx := memory.Use(context.Background())
	server := &RotationProxyServer{}
	cl := testclock.New(testclock.TestTimeUTC)
	ctx = clock.Set(ctx, cl)

	ftt.Run("batch get rotations", t, func(t *ftt.Test) {
		var rotation = &rpb.Rotation{
			Name: "rotation",
			Shifts: []*rpb.Shift{
				{
					Oncalls:   oncallsShift3,
					StartTime: startTimeShift3,
				},
				{
					Oncalls:   oncallsShift2,
					StartTime: startTimeShift2,
					EndTime:   endTimeShift2,
				},
				{
					Oncalls:   oncallsShift1,
					StartTime: startTimeShift1,
					EndTime:   endTimeShift1,
				},
			},
		}
		updateRequest := &rpb.BatchUpdateRotationsRequest{
			Requests: []*rpb.UpdateRotationRequest{
				{Rotation: rotation},
			},
		}
		_, err := server.BatchUpdateRotations(ctx, updateRequest)
		datastore.GetTestable(ctx).CatchupIndexes()
		assert.Loosely(t, err, should.BeNil)

		getRequest := &rpb.BatchGetRotationsRequest{
			Names: []string{"rotation"},
		}

		// Mock clock
		ctx, _ = testclock.UseTime(ctx, time.Unix(444, 0))

		response, err := server.BatchGetRotations(ctx, getRequest)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(response.Rotations), should.Equal(1))
		expected := &rpb.Rotation{
			Name: "rotation",
			Shifts: []*rpb.Shift{
				{
					Oncalls:   oncallsShift2,
					StartTime: startTimeShift2,
					EndTime:   endTimeShift2,
				},
				{
					Oncalls:   oncallsShift3,
					StartTime: startTimeShift3,
				},
			},
		}
		diff := cmp.Diff(expected, response.Rotations[0], cmp.Comparer(proto.Equal))
		assert.Loosely(t, diff, should.BeEmpty)
	})
}

func TestGetCurrentOncallEmails(t *testing.T) {
	ctx := memory.Use(context.Background())
	cl := testclock.New(testclock.TestTimeUTC)
	ctx = clock.Set(ctx, cl)

	server := &RotationProxyServer{}
	ftt.Run("Test get current oncall emails", t, func(t *ftt.Test) {
		var rotation = &rpb.Rotation{
			Name: "rotation",
			Shifts: []*rpb.Shift{
				{
					Oncalls:   []*rpb.OncallPerson{person5},
					StartTime: &timestamp.Timestamp{Seconds: 777, Nanos: 0},
				},
				{
					Oncalls:   []*rpb.OncallPerson{person3, person4},
					StartTime: &timestamp.Timestamp{Seconds: 333, Nanos: 0},
					EndTime:   &timestamp.Timestamp{Seconds: 555, Nanos: 0},
				},
			},
		}
		updateRequest := &rpb.BatchUpdateRotationsRequest{
			Requests: []*rpb.UpdateRotationRequest{
				{Rotation: rotation},
			},
		}
		_, err := server.BatchUpdateRotations(ctx, updateRequest)
		datastore.GetTestable(ctx).CatchupIndexes()
		assert.Loosely(t, err, should.BeNil)

		ctx, _ = testclock.UseTime(ctx, time.Unix(444, 0))
		emails, err := getCurrentOncallEmails(ctx, "rotation")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, emails, should.Match([]string{"person3@google.com", "person4@google.com"}))

		ctx, _ = testclock.UseTime(ctx, time.Unix(666, 0))
		emails, err = getCurrentOncallEmails(ctx, "rotation")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, emails, should.Match([]string{}))

		ctx, _ = testclock.UseTime(ctx, time.Unix(888, 0))
		emails, err = getCurrentOncallEmails(ctx, "rotation")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, emails, should.Match([]string{"person5@google.com"}))

		ctx, _ = testclock.UseTime(ctx, time.Unix(888, 0))
		emails, err = getCurrentOncallEmails(ctx, "anotherrotation")
		assert.Loosely(t, err, should.NotBeNil)
	})
}
