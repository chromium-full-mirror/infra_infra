// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapiqa

import (
	"context"

	ablib "infra/cros/cmd/common_lib/android_api"
	abqa "infra/cros/cmd/common_lib/ants-qa/androidbuildinternal/v3_qa_atp"
)

// WorkUnitService handles API calls related to workunits.
type WorkUnitService interface {
	Get(resourceID string) (*abqa.WorkUnit, error)
	Insert(workunit *abqa.WorkUnit) (*abqa.WorkUnit, error)
	Update(resourceID string, workunit *abqa.WorkUnit) (*abqa.WorkUnit, error)
	Patch(resourceID string, workunit *abqa.WorkUnit) (*abqa.WorkUnit, error)
	List(ctx context.Context, invocationID string, options ablib.AndroidBuildAPIOptions) (*abqa.WorkUnitListResponse, error)
}

// WorkUnitServiceImpl is the RPC implementation of WorkUnitService.
type WorkUnitServiceImpl struct {
	client *abqa.WorkunitService
}

// Get implementation for workunits.
func (w *WorkUnitServiceImpl) Get(resourceID string) (*abqa.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Get(resourceID)

	return call.Do()
}

// Insert implementation for workunits.
func (w *WorkUnitServiceImpl) Insert(workunit *abqa.WorkUnit) (*abqa.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Insert(workunit)

	return call.Do()
}

// Update implementation for workunits.
func (w *WorkUnitServiceImpl) Update(resourceID string, workunit *abqa.WorkUnit) (*abqa.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Update(resourceID, workunit)

	return call.Do()
}

// Patch implementation for workunits.
func (w *WorkUnitServiceImpl) Patch(resourceID string, workunit *abqa.WorkUnit) (*abqa.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Patch(resourceID, workunit)

	return call.Do()
}

// List implementation for workunits.
func (w *WorkUnitServiceImpl) List(ctx context.Context, invocationID string, options ablib.AndroidBuildAPIOptions) (*abqa.WorkUnitListResponse, error) {
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
