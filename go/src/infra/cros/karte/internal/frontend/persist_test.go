// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/luci/gae/service/datastore"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/internal/identifiers"
	"go.chromium.org/infra/cros/karte/internal/scalars"
	"go.chromium.org/infra/cros/karte/internal/testsupport"
)

// TestPersistObservations tests persisting observations.
func TestPersistObservations(t *testing.T) {
	t.Parallel()

	const kind = "c98f39d2-592b-4700-b6ee-874ce8f6edc2"
	const metricKind = "abf5fa64-69e5-4983-83be-0366c3d4a4f8"

	t.Run("test persisting observation", func(t *testing.T) {
		ctx := testsupport.NewTestingContext(context.Background())
		k := NewKarteFrontend("").(*karteFrontend)
		fake := &fakeClient{}
		a, err := k.CreateAction(ctx, &kartepb.CreateActionRequest{
			Action: &kartepb.Action{
				Kind:      kind,
				StartTime: scalars.ConvertTimeToTimestampPtr(time.Unix(1, 0).UTC()),
				StopTime:  scalars.ConvertTimeToTimestampPtr(time.Unix(1, 0).UTC()),
				SealTime:  scalars.ConvertTimeToTimestampPtr(time.Unix(1, 0).UTC()),
			},
		})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if a.Name == "" {
			t.Error("expected name not to be empty")
		}
		if a.Kind != kind {
			t.Errorf("expected a.Kind %q to equal kind %q", a.Kind, kind)
		}
		if diff := cmp.Diff(a.SealTime, scalars.ConvertTimeToTimestampPtr(time.Unix(1, 0)), protocmp.Transform()); diff != "" {
			t.Errorf("unexpected diff (-want +got): %s", diff)
		}
		o, err := k.CreateObservation(ctx, &kartepb.CreateObservationRequest{
			Observation: &kartepb.Observation{
				ActionName: a.Name,
				MetricKind: metricKind,
			},
		})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if o.MetricKind != metricKind {
			t.Errorf("expected a.MetricKind %q to equal kind %q", o.MetricKind, metricKind)
		}
		if o.ActionName != a.Name {
			t.Errorf("expected o.ActionName %q to equal a.Name %q", o.ActionName, a.Name)
		}
		_, err = k.persistActionRangeImpl(ctx, fake, &kartepb.PersistActionRangeRequest{
			StartVersion: "zzzz",
			StopVersion:  "zzzz",
			StartTime:    scalars.ConvertTimeToTimestampPtr(time.Unix(0, 0).UTC()),
			StopTime:     scalars.ConvertTimeToTimestampPtr(time.Unix(100, 0).UTC()),
		})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if diff := cmp.Diff(fake.size(), 2); diff != "" {
			t.Errorf("unexpected diff: %s", diff)
		}
		if diff := cmp.Diff(fake.observationsSize(), 1); diff != "" {
			t.Errorf("unexpected diff: %s", diff)
		}
	})

	t.Run("test persisting multiple observations associated with single action", func(t *testing.T) {
		ctx := testsupport.NewTestingContext(context.Background())
		ctx = identifiers.Use(ctx, identifiers.NewDefault())
		k := NewKarteFrontend("").(*karteFrontend)
		const times = 10
		fake := &fakeClient{}
		a, err := k.CreateAction(ctx, &kartepb.CreateActionRequest{
			Action: &kartepb.Action{
				Kind:      kind,
				StartTime: scalars.ConvertTimeToTimestampPtr(time.Unix(1, 0).UTC()),
				StopTime:  scalars.ConvertTimeToTimestampPtr(time.Unix(1, 0).UTC()),
				SealTime:  scalars.ConvertTimeToTimestampPtr(time.Unix(1, 0).UTC()),
			},
		})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if a.Kind != kind {
			t.Errorf("expected a.Kind %q to equal kind %q", a.Kind, kind)
		}
		if diff := cmp.Diff(a.SealTime, scalars.ConvertTimeToTimestampPtr(time.Unix(1, 0)), protocmp.Transform()); diff != "" {
			t.Errorf("unexpected diff (-want +got): %s", diff)
		}
		for i := 0; i < times; i++ {
			o, err := k.CreateObservation(ctx, &kartepb.CreateObservationRequest{
				Observation: &kartepb.Observation{
					ActionName: a.Name,
					MetricKind: metricKind,
				},
			})
			if err != nil {
				t.Errorf("unexpected error: %s", err)
			}
			if o.MetricKind != metricKind {
				t.Errorf("expected a.MetricKind %q to equal kind %q", o.MetricKind, metricKind)
			}
			if o.ActionName != a.Name {
				t.Errorf("expected o.ActionName %q to equal a.Name %q", o.ActionName, a.Name)
			}
		}
		count, err := datastore.Count(ctx, datastore.NewQuery(ObservationKind))
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if diff := cmp.Diff(count, int64(times)); diff != "" {
			t.Errorf("unexpected diff (-want +got): %s", diff)
		}
		resp, err := k.persistActionRangeImpl(ctx, fake, &kartepb.PersistActionRangeRequest{
			StartTime: scalars.ConvertTimeToTimestampPtr(time.Unix(0, 0).UTC()),
			StopTime:  scalars.ConvertTimeToTimestampPtr(time.Unix(100, 0).UTC()),
		})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if diff := cmp.Diff(resp.GetCreatedRecords(), int32(1)); diff != "" {
			t.Errorf("unexpected diff: %s", diff)
		}
		if diff := cmp.Diff(fake.size(), 1+times); diff != "" {
			t.Errorf("unexpected diff: %s", diff)
		}
		if diff := cmp.Diff(fake.observationsSize(), times); diff != "" {
			t.Errorf("unexpected diff: %s", diff)
		}
	})
}

func TestSplitTimeRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		start   time.Time
		stop    time.Time
		entries int
		out     []timeRangePair
		ok      bool
	}{
		{
			name:    "(1,0) to (3,0)",
			start:   time.Unix(1, 0).UTC(),
			stop:    time.Unix(3, 0).UTC(),
			entries: 2,
			out: []timeRangePair{
				{
					start: time.Unix(1, 0).UTC(),
					stop:  time.Unix(2, 0).UTC(),
				},
				{
					start: time.Unix(2, 0).UTC(),
					stop:  time.Unix(3, 0).UTC(),
				},
			},
			ok: true,
		},
		{
			name:    "(1,0) to (3,0)",
			start:   time.Unix(1, 0).UTC(),
			stop:    time.Unix(3, 0).UTC(),
			entries: 1,
			out: []timeRangePair{
				{
					start: time.Unix(1, 0).UTC(),
					stop:  time.Unix(3, 0).UTC(),
				},
			},
			ok: true,
		},
		{
			name:    "(1,0) to (4,0)",
			start:   time.Unix(1, 0).UTC(),
			stop:    time.Unix(4, 0).UTC(),
			entries: 3,
			out: []timeRangePair{
				{
					start: time.Unix(1, 0).UTC(),
					stop:  time.Unix(2, 0).UTC(),
				},
				{
					start: time.Unix(2, 0).UTC(),
					stop:  time.Unix(3, 0).UTC(),
				},
				{
					start: time.Unix(3, 0).UTC(),
					stop:  time.Unix(4, 0).UTC(),
				},
			},
			ok: true,
		},
		{
			name:    "(3,0) to (1,0) should fail",
			start:   time.Unix(3, 0).UTC(),
			stop:    time.Unix(1, 0).UTC(),
			entries: 2,
			out:     nil,
			ok:      false,
		},
	}

	for i, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.out
			actual, err := splitTimeRange(tt.start, tt.stop, tt.entries)
			ok := err == nil
			if diff := cmp.Diff(expected, actual, cmp.AllowUnexported(timeRangePair{})); diff != "" {
				t.Errorf("case %d: unexpected diff (-want +got): %s", i, diff)
			}

			if ok && !tt.ok {
				t.Errorf("case %d unexpectedly succeeded", i)
			}
			if !ok && tt.ok {
				t.Errorf("case %d unexpectedly failed: %s", i, err)
			}
		})
	}
}
