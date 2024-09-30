// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package androidapi

import (
	"context"

	"infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
)

// InvocationService handles API calls related to invocations.
type InvocationService interface {
	Get(resourceID string) (*androidbuildinternal.Invocation, error)
	Insert(invocation *androidbuildinternal.Invocation) (*androidbuildinternal.Invocation, error)
	Update(resourceID string, invocation *androidbuildinternal.Invocation) (*androidbuildinternal.Invocation, error)
	Patch(resourceID string, invocation *androidbuildinternal.Invocation) (*androidbuildinternal.Invocation, error)
	List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*androidbuildinternal.InvocationListResponse, error)
}

// InvocationServiceImpl is the RPC implementation of InvocationService.
type InvocationServiceImpl struct {
	client *androidbuildinternal.InvocationService
}

// Get implmentation for invocations.
func (w *InvocationServiceImpl) Get(resourceID string) (*androidbuildinternal.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Get(resourceID)

	return call.Do()
}

// Insert implmentation for invocations.
func (w *InvocationServiceImpl) Insert(invocation *androidbuildinternal.Invocation) (*androidbuildinternal.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Insert(invocation)

	return call.Do()
}

// Update implmentation for invocations.
func (w *InvocationServiceImpl) Update(resourceID string, invocation *androidbuildinternal.Invocation) (*androidbuildinternal.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Update(resourceID, invocation)

	return call.Do()
}

// Patch implmentation for invocations.
func (w *InvocationServiceImpl) Patch(resourceID string, invocation *androidbuildinternal.Invocation) (*androidbuildinternal.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Patch(resourceID, invocation)

	return call.Do()
}

// List implmentation for invocations.
func (w *InvocationServiceImpl) List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*androidbuildinternal.InvocationListResponse, error) {
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
