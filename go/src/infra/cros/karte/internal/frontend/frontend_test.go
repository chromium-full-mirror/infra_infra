// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"sync"
	"testing"
	"time"

	cloudBQ "cloud.google.com/go/bigquery"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/internal/identifiers"
	"go.chromium.org/infra/cros/karte/internal/scalars"
	"go.chromium.org/infra/cros/karte/internal/testsupport"
)

const invalidProjectID = "invalid project ID -- 5509d052-1fec-4ff6-bb2f-bb4e98951520"

// TestCreateAction makes sure that CreateAction returns the action it created and that the action is present in datastore.
func TestCreateAction(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	k := NewKarteFrontend("")
	resp, err := k.CreateAction(ctx, &kartepb.CreateActionRequest{
		Action: &kartepb.Action{
			Name:       "",
			Kind:       "ssh-attempt",
			CreateTime: scalars.ConvertTimeToTimestampPtr(time.Unix(1, 2).UTC()),
		},
	})
	expected := &kartepb.Action{
		Name:       "zzzzUzzzzzzzzzJ00000zzzzzk",
		Kind:       "ssh-attempt",
		SealTime:   scalars.ConvertTimeToTimestampPtr(time.Unix(1+12*60*60, 2).UTC()),
		CreateTime: scalars.ConvertTimeToTimestampPtr(time.Unix(1, 2).UTC()),
	}
	if err != nil {
		t.Error(err)
	}
	if diff := cmp.Diff(expected, resp, protocmp.Transform()); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
	// Here we inspect the contents of datastore.
	q, err := newActionEntitiesQuery("", "")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	datastoreActionEntities, _, err := q.Next(ctx, 0)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if datastoreActionEntities == nil {
		t.Errorf("action entities should not be nil")
	}
	switch len(datastoreActionEntities) {
	case 0:
		t.Errorf("datastore should not be empty")
	case 1:
	default:
		t.Errorf("datastore should not have more than 1 item")
	}
}

// TestRejectActionWithUserDefinedName tests that an action with a user-defined name is rejected.
func TestRejectActionWithUserDefinedName(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	k := NewKarteFrontend("")
	resp, err := k.CreateAction(ctx, &kartepb.CreateActionRequest{
		Action: &kartepb.Action{
			Name: "aaaaa",
			Kind: "ssh-attempt",
		},
	})
	if resp != nil {
		t.Errorf("unexpected response: %s", resp.String())
	}
	if err == nil {
		t.Errorf("expected response to be rejected")
	}
}

// TestCreateActionWithNoTime tests that creating an action without a time succeeds and supplies the current time.
// See b/206651512 for details.
func TestCreateActionWithNoTime(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	// Set a test clock to an arbitrary time to make sure that the correct time is supplied.
	testClock := testclock.New(time.Unix(3, 4).UTC())
	ctx = clock.Set(ctx, testClock)
	ctx = identifiers.Use(ctx, identifiers.NewDefault())

	k := NewKarteFrontend("")

	resp, err := k.CreateAction(ctx, &kartepb.CreateActionRequest{
		Action: &kartepb.Action{
			Name: "",
			Kind: "ssh-attempt",
		},
	})

	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if resp == nil {
		t.Errorf("resp should not be nil")
	}
	expected := time.Unix(3, 4).UTC()
	actual := scalars.ConvertTimestampPtrToTime(resp.GetCreateTime())
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("unexpected diff: %s", diff)
	}
}

// TestCreateActionWithSwarmingAndBuildbucketID tests creating a new action with an swarming ID and a buildbucket ID and reading it back.
func TestCreateActionWithSwarmingAndBuildbucketID(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	testClock := testclock.New(time.Unix(3, 4).UTC())
	ctx = clock.Set(ctx, testClock)
	ctx = identifiers.Use(ctx, identifiers.NewNaive())

	k := NewKarteFrontend("")

	expected := []*kartepb.Action{
		{
			Name:           "zzzzUzzzzzzzzzk00000zzzzzk",
			Kind:           "ssh-attempt",
			SwarmingTaskId: "a",
			BuildbucketId:  "b",
			CreateTime:     scalars.ConvertTimeToTimestampPtr(time.Unix(3, 0).UTC()),
			SealTime:       scalars.ConvertTimeToTimestampPtr(time.Unix(3+12*60*60, 0).UTC()),
		},
	}

	_, err := k.CreateAction(ctx, &kartepb.CreateActionRequest{
		Action: &kartepb.Action{
			Name:           "",
			Kind:           "ssh-attempt",
			SwarmingTaskId: "a",
			BuildbucketId:  "b",
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	resp, err := k.ListActions(ctx, &kartepb.ListActionsRequest{
		Filter: `kind == "ssh-attempt"`,
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	actual := resp.GetActions()

	if diff := cmp.Diff(expected, actual, protocmp.Transform()); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestCreateObservation makes sure that that CreateObservation fails because
// it isn't implemented.
func TestCreateObservation(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	k := NewKarteFrontend("")
	_, err := k.CreateObservation(ctx, &kartepb.CreateObservationRequest{})
	if err == nil {
		t.Error("expected Create Observation to fail")
	}
}

// TestListActionsSmokeTest tests that ListActions does not error.
func TestListActionsSmokeTest(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	k := NewKarteFrontend("")
	resp, err := k.ListActions(ctx, &kartepb.ListActionsRequest{})
	if resp == nil {
		t.Errorf("expected resp to not be nil")
	}
	if len(resp.GetActions()) != 0 {
		t.Errorf("expected actions to be trivial")
	}
	if err != nil {
		t.Errorf("expected error to be nil not %s", err)
	}
}

// TestListActions tests that ListActions errors.
func TestListActions(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	if err := PutActionEntities(
		ctx,
		&ActionEntity{
			ID: "aaaa",
		},
	); err != nil {
		t.Error(err)
	}
	k := NewKarteFrontend("")
	resp, err := k.ListActions(ctx, &kartepb.ListActionsRequest{})
	if err != nil {
		t.Errorf("expected error to be nil not %s", err)
	}
	if resp == nil {
		t.Errorf("expected resp to not be nil")
	}
	if resp.GetActions() == nil {
		t.Errorf("expected actions to not be nil")
	}
	if len(resp.GetActions()) != 1 {
		t.Errorf("expected len(actions) to be 1 not %d", len(resp.GetActions()))
	}
}

// TestListObservations tests that ListObservations errors.
func TestListObservations(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	k := NewKarteFrontend("")
	resp, err := k.ListObservations(ctx, &kartepb.ListObservationsRequest{})
	if resp == nil {
		t.Errorf("expected resp to not be nil")
	}
	if err != nil {
		t.Errorf("expected error to be nil not %s", err)
	}
}

// fakeClient mimics the real bigquery client, which is thread safe.
// See the link below for more details.
//
// https://github.com/googleapis/google-cloud-go/issues/4673
type fakeClient struct {
	mutex        sync.Mutex
	items        [][]cloudBQ.ValueSaver
	observations [][]cloudBQ.ValueSaver
}

func (c *fakeClient) getInserter(dataset string, table string) bqInserter {
	return func(ctx context.Context, item []cloudBQ.ValueSaver) error {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		c.items = append(c.items, item)
		if table == "observations" {
			c.observations = append(c.observations, item)
		}
		return nil
	}
}

// size returns the total number of items.
func (c *fakeClient) size() int {
	out := 0
	for _, row := range c.items {
		out += len(row)
	}
	return out
}

// observationsSize returns the total number of observations.
func (c *fakeClient) observationsSize() int {
	out := 0
	for _, row := range c.observations {
		out += len(row)
	}
	return out
}

// TestPersistActionRangeImpl_SmokeTest tests that persisting a range of actions
// returns a non-error response given an empty dataset
func TestPersistActionRangeImpl_SmokeTest(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	k := NewKarteFrontend("").(*karteFrontend)
	fake := &fakeClient{}

	_, err := k.persistActionRangeImpl(ctx, fake, &kartepb.PersistActionRangeRequest{
		StartTime: scalars.ConvertTimeToTimestampPtr(time.Unix(1, 0).UTC()),
		StopTime:  scalars.ConvertTimeToTimestampPtr(time.Unix(2, 0).UTC()),
	})
	if err != nil {
		t.Errorf("expected resp to be nil not %s", err)
	}
}

// TestAlignedIntervalStrictlyInPast tests that we produce an interval that is aligned with
// the time.Unix(0, 0) and has the specified width.
func TestAlignedIntervalStrictlyInPast(t *testing.T) {
	t.Parallel()

	type input struct {
		t time.Time
		d time.Duration
	}

	type output struct {
		start time.Time
		end   time.Time
	}

	cases := []struct {
		name string
		in   input
		out  output
		ok   bool
	}{
		{
			name: "sad path -- non-UTC time",
			in: input{
				t: time.Unix(0, 0).Local(),
				d: time.Second,
			},
			out: output{},
			ok:  false,
		},
		{
			name: "(0,0) rounding down",
			in: input{
				t: time.Unix(0, 0).UTC(),
				d: time.Second,
			},
			out: output{
				start: time.Unix(-1, 0).UTC(),
				end:   time.Unix(0, 0).UTC(),
			},
			ok: true,
		},
		{
			// In this example, we should round down from (0,1) to (0,0) BEFORE we
			// jump one second into the past.
			name: "(0,1) rounding down",
			in: input{
				t: time.Unix(0, 1).UTC(),
				d: time.Second,
			},
			out: output{
				start: time.Unix(-1, 0).UTC(),
				end:   time.Unix(0, 0).UTC(),
			},
			ok: true,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			start, end, err := makeAlignedIntervalStrictlyInPast(tt.in.t, tt.in.d)
			if tt.ok {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
			} else {
				if err == nil {
					t.Error("expected alignedIntervalStrictlyInPast to return an error but it didn't")
				}
			}

			expected := tt.out
			actual := output{
				start: start,
				end:   end,
			}
			if diff := cmp.Diff(expected, actual, cmp.AllowUnexported(output{})); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}
