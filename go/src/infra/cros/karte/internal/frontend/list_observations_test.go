// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"testing"

	"go.chromium.org/infra/cros/karte/internal/testsupport"
)

// TestListObservationsWithFilter tests listing observations with a simple filter.
func TestListObservationsWithFilter(t *testing.T) {
	t.Parallel()
	ctx := testsupport.NewTestingContext(context.Background())
	if err := PutObservationEntities(
		ctx,
		&ObservationEntity{ID: "hi", MetricKind: "w"},
		&ObservationEntity{ID: "hi2", MetricKind: "w"},
		&ObservationEntity{ID: "hi3", MetricKind: "a"},
	); err != nil {
		t.Errorf("putting entities: %s", err)
	}
	q, err := newObservationEntitiesQuery("", "metric_kind == \"w\"")
	if err != nil {
		t.Errorf("building query: %s", err)
	}
	es, err := q.Next(ctx, 10)
	if err != nil {
		t.Errorf("running query: %s", err)
	}
	if len(es) != 2 {
		t.Errorf("unexpected entities: %v", es)
	}
}
