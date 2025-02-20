// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"context"

	atp "go.chromium.org/infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
)

type InvocationState int

func (w InvocationState) String() string {
	switch w {
	case InvocationUnspecified:
		return "unspecified"
	case InvocationQueued:
		return "queued"
	case InvocationRunning:
		return "running"
	case InvocationError:
		return "error"
	case InvocationCompleted:
		return "completed"
	case InvocationCancelled:
		return "cancelled"
	case InvocationPending:
		return "pending"
	case InvocationSkipped:
		return "skipped"
	default:
		return ""
	}
}

const (
	InvocationUnspecified InvocationState = iota
	InvocationQueued
	InvocationRunning
	InvocationError
	InvocationCompleted
	InvocationCancelled
	InvocationPending
	InvocationSkipped
)

// InvocationService handles API calls related to invocations.
type InvocationService interface {
	Get(ctx context.Context, resourceID string) (*atp.Invocation, error)
	Insert(invocation *atp.Invocation) (*atp.Invocation, error)
	Update(resourceID string, invocation *atp.Invocation) (*atp.Invocation, error)
	Patch(resourceID string, invocation *atp.Invocation) (*atp.Invocation, error)
	List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*atp.InvocationListResponse, error)
}

// InvocationServiceImpl is the RPC implementation of InvocationService.
type InvocationServiceImpl struct {
	client *atp.InvocationService
}

// Get implementation for invocations.
func (w *InvocationServiceImpl) Get(ctx context.Context, resourceID string) (*atp.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Get(resourceID)
	return retry(ctx, call.Do)
}

// Insert implementation for invocations.
func (w *InvocationServiceImpl) Insert(invocation *atp.Invocation) (*atp.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Insert(invocation)

	return call.Do()
}

// Update implementation for invocations.
func (w *InvocationServiceImpl) Update(resourceID string, invocation *atp.Invocation) (*atp.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Update(resourceID, invocation)

	return call.Do()
}

// Patch implementation for invocations.
func (w *InvocationServiceImpl) Patch(resourceID string, invocation *atp.Invocation) (*atp.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Patch(resourceID, invocation)

	return call.Do()
}

// List implementation for invocations.
func (w *InvocationServiceImpl) List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*atp.InvocationListResponse, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.List().InvocationId(invocationID)
	if options.PageToken != "" {
		call = call.PageToken(options.PageToken)
	}

	maxResults := defaultMaxResults
	if options.MaxResults > 0 {
		maxResults = options.MaxResults
	}

	return call.MaxResults(maxResults).Do()
}
