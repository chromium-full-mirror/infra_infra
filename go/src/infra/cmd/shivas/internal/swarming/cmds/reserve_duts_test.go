// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"context"
	"testing"

	"infra/cmd/shivas/site"
)

// TestScheduleReserveBuilder tests that scheduling a repair builder produces the correct taskID.
// This test does NOT emulate the buildbucket client on a deep level.
func TestScheduleReserveBuilder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &fakeClient{}
	r := &reserveDuts{
		session: "admin-session:bla bla",
		config:  "task-config",
	}
	_, taskID, err := r.scheduleReserveBuilder(ctx, client, nil, site.Environment{}, "fake-labstation1")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	expected := int64(1)
	actual := taskID
	if taskID != expected {
		t.Errorf("unexpected %v by got %v", expected, actual)
	}
}
