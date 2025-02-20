// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/gae/service/datastore"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/internal/scalars"
	"go.chromium.org/infra/cros/karte/internal/testsupport"
)

// TestGetMostRecentAction tests that we can get the most recent action of any kind in the datastore db.
func TestGetMostRecentAction(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())

	datastore.GetTestable(ctx).Consistent(true)
	k := NewKarteFrontend("")

	_, err := k.CreateAction(
		ctx,
		&kartepb.CreateActionRequest{
			Action: &kartepb.Action{
				Kind:       "foo",
				CreateTime: scalars.ConvertTimeToTimestampPtr(time.Unix(1, 2).UTC()),
			},
		},
	)
	if err != nil {
		t.Errorf("failed to insert: %s", err)
	}

	_, err = k.CreateAction(
		ctx,
		&kartepb.CreateActionRequest{
			Action: &kartepb.Action{
				Kind:       "bar",
				CreateTime: scalars.ConvertTimeToTimestampPtr(time.Unix(1, 2).UTC()),
			},
		},
	)
	if err != nil {
		t.Errorf("failed to insert: %s", err)
	}

	resp, err := k.ListActions(ctx, &kartepb.ListActionsRequest{
		PageSize: 1,
		Filter:   "",
	})
	if err != nil {
		t.Errorf("unexpected error while fetching actions: %s", err)
	}

	const expected = "bar"
	actual := resp.GetActions()[0].GetKind()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestGetMostRecentActionInKind tests that we can get the most recent action of a given kind.
func TestGetMostRecentActionInKind(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	k := NewKarteFrontend("")

	_, err := k.CreateAction(
		ctx,
		&kartepb.CreateActionRequest{
			Action: &kartepb.Action{
				Kind:       "ssh-attempt",
				FailReason: "1",
				CreateTime: scalars.ConvertTimeToTimestampPtr(time.Unix(1, 2).UTC()),
			},
		},
	)
	if err != nil {
		t.Errorf("failed to insert")
	}

	_, err = k.CreateAction(
		ctx,
		&kartepb.CreateActionRequest{
			Action: &kartepb.Action{
				Kind:       "ssh-attempt",
				FailReason: "2",
				CreateTime: scalars.ConvertTimeToTimestampPtr(time.Unix(1, 2).UTC()),
			},
		},
	)
	if err != nil {
		t.Errorf("failed to insert")
	}

	_, err = k.CreateAction(
		ctx,
		&kartepb.CreateActionRequest{
			Action: &kartepb.Action{
				Kind:       "flash-firmware",
				FailReason: "3",
				CreateTime: scalars.ConvertTimeToTimestampPtr(time.Unix(1, 2).UTC()),
			},
		},
	)
	if err != nil {
		t.Errorf("failed to insert")
	}

	resp, err := k.ListActions(ctx, &kartepb.ListActionsRequest{
		PageSize: 1,
		Filter:   `kind == "ssh-attempt"`,
	})
	if err != nil {
		t.Errorf("unexpected error while fetching actions: %s", err)
	}

	const expected = "2"
	var actual string
	if len(resp.GetActions()) > 0 {
		actual = resp.GetActions()[0].GetFailReason()
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}
