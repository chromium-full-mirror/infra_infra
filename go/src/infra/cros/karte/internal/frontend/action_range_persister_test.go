// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.chromium.org/luci/gae/service/datastore"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/internal/testsupport"
)

// TestActionRangePersisterInsufficientInput tests that invoking actionRangePersister without
// specifying a time range fails with an error message telling you that the action that you took
// is perhaps technically valid, but is so likely to be an error that we're rejecting it for you
// as a favor.
func TestActionRangePersisterInsufficientInput(t *testing.T) {
	t.Parallel()

	_, err := makeQuery(&actionRangePersistOptions{})
	if err == nil {
		t.Errorf("expected make query to fail, but it didn't")
	} else if ok := strings.Contains(err.Error(), "rejecting likely erroneous call"); !ok {
		t.Errorf("unexpected error message %s", err)
	}
}

// TestActionRangePersisterSmokeTest tests that attempting to persist data in cases where no data
// actually exists, successfully does nothing.
func TestActionRangePersisterSmokeTest(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())

	a := &actionRangePersistOptions{
		startID: time.Unix(0, 0).UTC(),
		stopID:  time.Unix(20, 0).UTC(),
	}
	q, err := makeQuery(a)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if _, _, err := persistActions(ctx, a, q.Query); err != nil {
		t.Errorf("unexpected error %s", err)
	}
	count, err := datastore.Count(ctx, datastore.NewQuery(ActionKind))
	if count != 0 {
		t.Errorf("unexpected count %d", count)
	}
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if err := persistObservations(ctx, a); err != nil {
		t.Errorf("unexpected error %s", err)
	}
	count, err = datastore.Count(ctx, datastore.NewQuery(ObservationKind))
	if count != 0 {
		t.Errorf("unexpected count %d", count)
	}
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
}

// TestActionRangePersister tests persisting two actions and two observations.
func TestActionRangePersister(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	fake := &fakeClient{}
	k := NewKarteFrontend("")

	action1 := func() string {
		resp, err := k.CreateAction(ctx, &kartepb.CreateActionRequest{
			Action: &kartepb.Action{
				Name: "",
				Kind: "ssh-attempt",
			},
		})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		return resp.GetName()
	}()
	if action1 == "" {
		t.Error("action1 should not empty")
	}

	action2 := func() string {
		resp, err := k.CreateAction(ctx, &kartepb.CreateActionRequest{
			Action: &kartepb.Action{
				Name: "",
				Kind: "ssh-attempt",
			},
		})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		return resp.GetName()
	}()
	if action2 == "" {
		t.Error("action2 should not be empty")
	}

	observation1 := func() string {
		resp, err := k.CreateObservation(ctx, &kartepb.CreateObservationRequest{
			Observation: &kartepb.Observation{
				ActionName: action1,
			},
		})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		return resp.GetName()
	}()
	if observation1 == "" {
		t.Error("observation1 should not be empty")
	}

	observation2 := func() string {
		resp, err := k.CreateObservation(ctx, &kartepb.CreateObservationRequest{
			Observation: &kartepb.Observation{
				ActionName: action2,
			},
		})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		return resp.GetName()
	}()
	if observation2 == "" {
		t.Error("observation2 should not be empty")
	}
	a := &actionRangePersistOptions{
		startID: time.Unix(1, 0).UTC(),
		stopID:  time.Unix(100, 0).UTC(),
		bq:      fake,
	}
	q, err := makeQuery(a)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	_, _, err = persistActions(ctx, a, q.Query)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	count, err := datastore.Count(ctx, datastore.NewQuery(ActionKind))
	if count != 2 {
		t.Errorf("unexpected count: %d", count)
	}
	if err != nil {
		t.Errorf("unexpected err: %s", err)
	}
	if err := persistObservations(ctx, a); err != nil {
		t.Errorf("unexpected err: %s", err)
	}
	count, err = datastore.Count(ctx, datastore.NewQuery(ObservationKind))
	if count != 2 {
		t.Errorf("unexpected count: %d", count)
	}
	if err != nil {
		t.Errorf("unexpected err: %s", err)
	}
	// These two checks down here are the highest-value checks. They check the total number of
	// bigquery records produced and the number of those records that are observations, respectively.
	if count := fake.observationsSize(); count != 2 {
		t.Errorf("unexpected observation size: %d", count)
	}
	if count := fake.size(); count != 4 {
		t.Errorf("unexpected total entity count: %d", count)
	}
}
