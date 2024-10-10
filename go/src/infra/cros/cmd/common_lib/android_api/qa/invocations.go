// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapiqa

import (
	"context"

	ablib "infra/cros/cmd/common_lib/android_api"
	abqa "infra/cros/cmd/common_lib/ants-qa/androidbuildinternal/v3_qa_atp"
)

// InvocationService handles API calls related to invocations.
type InvocationService interface {
	Get(resourceID string) (*abqa.Invocation, error)
	Insert(invocation *abqa.Invocation) (*abqa.Invocation, error)
	Update(resourceID string, invocation *abqa.Invocation) (*abqa.Invocation, error)
	Patch(resourceID string, invocation *abqa.Invocation) (*abqa.Invocation, error)
	List(ctx context.Context, invocationID string, options ablib.AndroidBuildAPIOptions) (*abqa.InvocationListResponse, error)
}

// InvocationServiceImpl is the RPC implementation of InvocationService.
type InvocationServiceImpl struct {
	client *abqa.InvocationService
}

// Get implementation for invocations.
func (w *InvocationServiceImpl) Get(resourceID string) (*abqa.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Get(resourceID)

	return call.Do()
}

// Insert implementation for invocations.
func (w *InvocationServiceImpl) Insert(invocation *abqa.Invocation) (*abqa.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Insert(invocation)

	return call.Do()
}

// Update implementation for invocations.
func (w *InvocationServiceImpl) Update(resourceID string, invocation *abqa.Invocation) (*abqa.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Update(resourceID, invocation)

	return call.Do()
}

// Patch implementation for invocations.
func (w *InvocationServiceImpl) Patch(resourceID string, invocation *abqa.Invocation) (*abqa.Invocation, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Patch(resourceID, invocation)

	return call.Do()
}

// List implementation for invocations.
func (w *InvocationServiceImpl) List(ctx context.Context, invocationID string, options ablib.AndroidBuildAPIOptions) (*abqa.InvocationListResponse, error) {
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
