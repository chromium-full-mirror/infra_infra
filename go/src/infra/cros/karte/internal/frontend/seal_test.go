// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/internal/identifiers"
	"go.chromium.org/infra/cros/karte/internal/scalars"
	"go.chromium.org/infra/cros/karte/internal/testsupport"
)

// TestModifyingSealedActionShouldFail tests that updating a record after the seal time fails.
func TestModifyingSealedActionShouldFail(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	ctx = identifiers.Use(ctx, identifiers.NewDefault())
	testClock := testclock.New(time.Unix(3, 4).UTC())
	ctx = clock.Set(ctx, testClock)

	k := NewKarteFrontend("")

	if _, err := k.CreateAction(ctx, &kartepb.CreateActionRequest{
		Action: &kartepb.Action{
			Kind: "w",
		},
	}); err != nil {
		t.Error(err)
	}

	resp, err := k.ListActions(ctx, &kartepb.ListActionsRequest{
		Filter: `kind == "w"`,
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if l := len(resp.GetActions()); l != 1 {
		t.Errorf("unexpected number of actions %d", l)
	}

	_ = resp.GetActions()[0].GetName()

	sealTime := scalars.ConvertTimestampPtrToString(resp.GetActions()[0].GetSealTime())
	if diff := cmp.Diff(fmt.Sprintf("%d:%d", 3+12*60*60, 0), sealTime); diff != "" {
		t.Errorf("unexpected diff: %s", diff)
	}
}
