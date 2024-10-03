// Copyright 2024 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"context"

	"infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
)

// WorkUnitService handles API calls related to workunits.
type WorkUnitService interface {
	Get(resourceID string) (*androidbuildinternal.WorkUnit, error)
	Insert(workunit *androidbuildinternal.WorkUnit) (*androidbuildinternal.WorkUnit, error)
	Update(resourceID string, workunit *androidbuildinternal.WorkUnit) (*androidbuildinternal.WorkUnit, error)
	Patch(resourceID string, workunit *androidbuildinternal.WorkUnit) (*androidbuildinternal.WorkUnit, error)
	List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*androidbuildinternal.WorkUnitListResponse, error)
}

// WorkUnitServiceImpl is the RPC implementation of WorkUnitService.
type WorkUnitServiceImpl struct {
	client *androidbuildinternal.WorkunitService
}

// Get implementation for workunits.
func (w *WorkUnitServiceImpl) Get(resourceID string) (*androidbuildinternal.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Get(resourceID)

	return call.Do()
}

// Insert implementation for workunits.
func (w *WorkUnitServiceImpl) Insert(workunit *androidbuildinternal.WorkUnit) (*androidbuildinternal.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Insert(workunit)

	return call.Do()
}

// Update implementation for workunits.
func (w *WorkUnitServiceImpl) Update(resourceID string, workunit *androidbuildinternal.WorkUnit) (*androidbuildinternal.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Update(resourceID, workunit)

	return call.Do()
}

// Patch implementation for workunits.
func (w *WorkUnitServiceImpl) Patch(resourceID string, workunit *androidbuildinternal.WorkUnit) (*androidbuildinternal.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Patch(resourceID, workunit)

	return call.Do()
}

// List implementation for workunits.
func (w *WorkUnitServiceImpl) List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*androidbuildinternal.WorkUnitListResponse, error) {
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
