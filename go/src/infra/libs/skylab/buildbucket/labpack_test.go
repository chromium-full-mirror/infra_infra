// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package buildbucket

import (
	"context"
	"net/http"
	"sort"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/grpc/prpc"

	schedulingapi "go.chromium.org/infra/libs/fleet/scheduling/api"
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
	ftt.Run("test schedule task with stubbed BB wrapper and stubbed inactive scheduling API", t, func(t *ftt.Test) {
		ctx := context.Background()
		t.Run("nil params", func(t *ftt.Test) {
			_, _, err := ScheduleTask(ctx, &FakeClient{}, CIPDProd, nil, "fake service")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("schedule task"))
		})
		t.Run("audit-rpm", func(t *ftt.Test) {
			_, bbid, err := ScheduleTask(ctx, &FakeClient{startID: 3}, CIPDProd, &Params{
				BuilderName: "audit-rpm",
			}, "fake service")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, bbid, should.Equal(3))
		})
	})
}

// CreateTask tests whether schedule task accepts or rejects its arguments, basically.
func TestCreateTask(t *testing.T) {
	t.Parallel()
	ftt.Run("test schedule task with real BB wrapper and stubbed active scheduling API", t, func(t *ftt.Test) {
		ctx := context.Background()
		hc := &http.Client{}
		bc, err := NewClient(ctx, hc, prpc.DefaultOptions())
		assert.Loosely(t, err, should.BeNil)
		sc := &FakeSchedulingAPI{
			shouldUseDM: true,
		}
		t.Run("audit-rpm", func(t *ftt.Test) {
			url, bbid, err := CreateTask(ctx, bc, sc, CIPDProd, &Params{
				BuilderName: "audit-rpm",
			}, "fake service")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, url, should.Equal("test-url-from-scheduke"))
			// No BBID returned since Scheduke doesn't generate them immediately.
			assert.Loosely(t, bbid, should.BeZero)
		})
	})
}

func TestGetBuilderName(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name         string
		builderName  string
		hive         string
		expectedName string
	}{
		{
			"normal builder name",
			"repair",
			"",
			"repair",
		},
		{
			"clank builder name",
			"repair",
			"chrome-clank",
			"repair-clank",
		},
		{
			"special hive but not registered",
			"repair",
			"chrome-foo",
			"repair",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualName := BuilderNamePerHive(tc.builderName, tc.hive)
			if actualName != tc.expectedName {
				t.Errorf("unexpected builder name %s (expected %s)", actualName, tc.expectedName)
			}
		})
	}
}

func TestIfUseScheduke(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		builderName   string
		expectedValue bool
	}{
		{
			"normal builder name",
			"repair",
			true,
		},
		{
			"latest builder name",
			"repair-latest",
			true,
		},
		{
			"clank builder name",
			"repair-clank",
			false,
		},
		{
			"perf builder name",
			"repair-perf",
			false,
		},
		{
			"pinpoint builder name",
			"repair-pinpoint",
			false,
		},
		{
			"clank-latest builder name",
			"repair-clank-latest",
			false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualValue := IfUseScheduke(tc.builderName)
			if actualValue != tc.expectedValue {
				t.Errorf("unexpected value %t (expected %t)", actualValue, tc.expectedValue)
			}
		})
	}
}

func TestGetCipdVersion(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		useLatest     bool
		expectedValue CIPDVersion
	}{
		{
			"use prod",
			false,
			CIPDProd,
		},
		{
			"use latest",
			true,
			CIPDLatest,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualValue := CipdVersion(tc.useLatest)
			if actualValue != tc.expectedValue {
				t.Errorf("unexpected value %s (expected %s)", actualValue, tc.expectedValue)
			}
		})
	}
}
