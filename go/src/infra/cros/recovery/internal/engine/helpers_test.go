// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package engine

import (
	"context"

	"go.chromium.org/infra/cros/recovery/logger/metrics"
)

// FakeMetrics implementation that stores all actions.
//
// NOTE: For this type, create and update BOTH APPEND ITEMS.
//
//	It does NOT emulate the real semantics of update.
type fakeMetrics struct {
	actions []*metrics.Action
}

// Check that fakeMetrics satisfies the metrics interface.
var _ metrics.Metrics = &fakeMetrics{}

// NewFakeMetrics makes a new fake metrics instance.
func newFakeMetrics() *fakeMetrics {
	return &fakeMetrics{}
}

// Create a new action by appending it.
func (m *fakeMetrics) Create(ctx context.Context, action *metrics.Action) error {
	m.actions = append(m.actions, action)
	return nil
}

// Update an action by appending it. Do not remove the original.
func (m *fakeMetrics) Update(ctx context.Context, action *metrics.Action) error {
	m.actions = append(m.actions, action)
	return nil
}

// Search is not implemented.
func (m *fakeMetrics) Search(ctx context.Context, q *metrics.Query) (*metrics.QueryResult, error) {
	panic("not implemented")
}
