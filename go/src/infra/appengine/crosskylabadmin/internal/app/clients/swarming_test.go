// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clients

import (
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/duration"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/data/strpair"
	"go.chromium.org/luci/common/testing/typed"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
)

func TestGetStateDimension(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input []*swarmingv2.StringListPair
		want  fleet.DutState
	}{
		{"missing key", nil, fleet.DutState_DutStateInvalid},
		{"normal", []*swarmingv2.StringListPair{
			{Key: "dut_state", Value: []string{"ready"}},
		}, fleet.DutState_Ready},
		{"multiple values", []*swarmingv2.StringListPair{
			{Key: "dut_state", Value: []string{"ready", "repair_failed"}},
		}, fleet.DutState_DutStateInvalid},
		{"multiple pairs", []*swarmingv2.StringListPair{
			{Key: "dut_state", Value: []string{"ready"}},
			{Key: "dut_state", Value: []string{"repair_failed"}},
		}, fleet.DutState_Ready},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := GetStateDimension(c.input)
			if got != c.want {
				t.Errorf("getStateDimension(%#v) = %#v; want %#v", c.input, got, c.want)
			}
		})
	}
}

func TestTimeSinceBotTaskN(t *testing.T) {
	t.Parallel()
	now := time.Date(2016, 1, 2, 3, 4, 5, 0, time.UTC)
	assert := func(t *testing.T, got, want *duration.Duration) {
		t.Helper()
		switch want {
		case nil:
			if got != nil {
				t.Errorf("Got %#v instead of nil", got)
			}
		default:
			if got == nil {
				t.Errorf("Got nil instead of %#v", want)
				return
			}
			if got.Seconds != want.Seconds || got.Nanos != want.Nanos {
				t.Errorf("Got %#v instead of %#v", got, want)
			}
		}
	}
	cases := []struct {
		desc  string
		now   time.Time
		input *swarmingv2.TaskResultResponse
		want  *duration.Duration
	}{
		{
			desc: "completed",
			input: &swarmingv2.TaskResultResponse{
				State:       swarmingv2.TaskState_COMPLETED,
				CompletedTs: mustParseTime("2016-01-02T03:04:01.000000009"),
			},
			want: &duration.Duration{Seconds: 3, Nanos: 999999991},
		},
		{
			desc: "running",
			input: &swarmingv2.TaskResultResponse{
				State: swarmingv2.TaskState_RUNNING,
			},
			want: &duration.Duration{Seconds: 0, Nanos: 0},
		},
		{
			desc: "killed",
			input: &swarmingv2.TaskResultResponse{
				State:       swarmingv2.TaskState_KILLED,
				AbandonedTs: mustParseTime("2016-01-02T03:04:01.000000009"),
			},
			want: &duration.Duration{Seconds: 3, Nanos: 999999991},
		},
		{
			desc: "expired",
			input: &swarmingv2.TaskResultResponse{
				State: swarmingv2.TaskState_EXPIRED,
			},
			want: nil,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.desc, func(t *testing.T) {
			t.Parallel()
			got, err := TimeSinceBotTaskN(c.input, now)
			if err != nil {
				t.Fatalf("TimeSinceBotTaskN returned unexpected error: %s", err)
			}
			assert(t, got, c.want)
		})
	}
}

func TestTaskDoneTime(t *testing.T) {
	t.Parallel()
	cases := []struct {
		desc  string
		input *swarmingv2.TaskResultResponse
		want  time.Time
	}{
		{
			desc: "completed",
			input: &swarmingv2.TaskResultResponse{
				State:       swarmingv2.TaskState_COMPLETED,
				CompletedTs: mustParseTime("2016-01-02T10:04:05.999999999"),
			},
			want: time.Date(2016, 1, 2, 10, 4, 5, 999999999, time.UTC),
		},
		{
			desc: "timed out",
			input: &swarmingv2.TaskResultResponse{
				State:       swarmingv2.TaskState_TIMED_OUT,
				CompletedTs: mustParseTime("2016-01-02T10:04:05.999999999"),
			},
			want: time.Date(2016, 1, 2, 10, 4, 5, 999999999, time.UTC),
		},
		{
			desc: "running",
			input: &swarmingv2.TaskResultResponse{
				State: swarmingv2.TaskState_RUNNING,
			},
			want: time.Time{},
		},
		{
			desc: "killed",
			input: &swarmingv2.TaskResultResponse{
				State:       swarmingv2.TaskState_KILLED,
				AbandonedTs: mustParseTime("2016-01-02T10:04:05.999999999"),
			},
			want: time.Date(2016, 1, 2, 10, 4, 5, 999999999, time.UTC),
		},
		{
			desc: "expired",
			input: &swarmingv2.TaskResultResponse{
				State: swarmingv2.TaskState_EXPIRED,
			},
			want: time.Time{},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.desc, func(t *testing.T) {
			t.Parallel()
			got, err := TaskDoneTime(c.input)
			if err != nil {
				t.Fatalf("TaskDoneTime returned unexpected error: %s", err)
			}
			if !got.Equal(c.want) {
				t.Errorf("TaskDoneTime(%#v) = %s; want %s", c.input, got.Format(time.RFC3339Nano),
					c.want.Format(time.RFC3339Nano))
			}
		})
	}
}

func TestConvertToDimensions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		desc string
		in   *SwarmingCreateTaskArgs
		want []*swarmingv2.StringPair
	}{
		{
			desc: "empty collection",
			in:   &SwarmingCreateTaskArgs{},
			want: nil,
		},
		{
			desc: "with DutName",
			in: &SwarmingCreateTaskArgs{
				Pool:    "pool1",
				DutName: "dut1",
			},
			want: []*swarmingv2.StringPair{
				{
					Key:   "pool",
					Value: "pool1",
				},
				{
					Key:   "dut_name",
					Value: "dut1",
				},
			},
		},
		{
			desc: "with BotId",
			in: &SwarmingCreateTaskArgs{
				Pool:  "pool1",
				BotID: "bot1",
			},
			want: []*swarmingv2.StringPair{
				{
					Key:   "pool",
					Value: "pool1",
				},
				{
					Key:   "id",
					Value: "bot1",
				},
			},
		},
		{
			desc: "with dut_id",
			in: &SwarmingCreateTaskArgs{
				Pool:  "pool1",
				DutID: "dut_id1",
			},
			want: []*swarmingv2.StringPair{
				{
					Key:   "pool",
					Value: "pool1",
				},
				{
					Key:   "dut_id",
					Value: "dut_id1",
				},
			},
		},
		{
			desc: "priority to bot id",
			in: &SwarmingCreateTaskArgs{
				Pool:     "pool1",
				BotID:    "bot1",
				DutName:  "dut1",
				DutID:    "dut_id1",
				DutState: "some_state",
			},
			want: []*swarmingv2.StringPair{
				{
					Key:   "pool",
					Value: "pool1",
				},
				{
					Key:   "id",
					Value: "bot1",
				},
				{
					Key:   "dut_state",
					Value: "some_state",
				},
			},
		},
		{
			desc: "priority to dut id",
			in: &SwarmingCreateTaskArgs{
				Pool:     "pool1",
				DutName:  "dut1",
				DutID:    "dut_id1",
				DutState: "some_state",
			},
			want: []*swarmingv2.StringPair{
				{
					Key:   "pool",
					Value: "pool1",
				},
				{
					Key:   "dut_id",
					Value: "dut_id1",
				},
				{
					Key:   "dut_state",
					Value: "some_state",
				},
			},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.desc, func(t *testing.T) {
			t.Parallel()
			got, err := convertToDimensions(c.in)
			if err != nil {
				if c.want != nil {
					t.Fatalf("TaskDoneTime returned unexpected error: %s", err)
				}
			}
			diff := typed.Diff(got, c.want)
			if diff != "" {
				t.Errorf("Test faild for %#v = %s", c.desc, diff)
			}
		})
	}
}

// mustParseTime is a helper function that parses a time according to the swarming time format.
//
// It panics if the time is invalid or not in the swarming time format.
func mustParseTime(timeString string) *timestamppb.Timestamp {
	theTime, err := time.Parse(SwarmingTimeLayout, timeString)
	if err != nil {
		panic(err)
	}
	return timestamppb.New(theTime)
}

// TestAsPairs tests converting a map to StringPairs.
func TestAsPairs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  strpair.Map
		output []*swarmingv2.StringPair
	}{
		{
			name:   "empty",
			input:  nil,
			output: nil,
		},
		{
			name:  "singleton",
			input: strpair.Map{"k": []string{"a", "b"}},
			output: []*swarmingv2.StringPair{
				{
					Key:   "k",
					Value: "a",
				},
				{
					Key:   "k",
					Value: "b",
				},
			},
		},
		{
			name: "doubleton",
			input: strpair.Map{
				"k1": []string{"a1", "b1"},
				"k2": []string{"a2", "b2"},
			},
			output: []*swarmingv2.StringPair{
				{
					Key:   "k1",
					Value: "a1",
				},
				{
					Key:   "k1",
					Value: "b1",
				},
				{
					Key:   "k2",
					Value: "a2",
				},
				{
					Key:   "k2",
					Value: "b2",
				},
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := asPairs(tt.input)
			if diff := typed.Diff(got, tt.output); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}
