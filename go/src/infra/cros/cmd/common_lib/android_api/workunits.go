// Copyright 2024 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"context"

	ab_qa_atp "infra/cros/cmd/common_lib/ants-qa/androidbuildinternal/v3_qa_atp"
)

// WorkUnitService handles API calls related to workunits.
type WorkUnitService interface {
	Get(resourceID string) (*ab_qa_atp.WorkUnit, error)
	Insert(workunit *ab_qa_atp.WorkUnit) (*ab_qa_atp.WorkUnit, error)
	Update(resourceID string, workunit *ab_qa_atp.WorkUnit) (*ab_qa_atp.WorkUnit, error)
	Patch(resourceID string, workunit *ab_qa_atp.WorkUnit) (*ab_qa_atp.WorkUnit, error)
	List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*ab_qa_atp.WorkUnitListResponse, error)
}

// WorkUnitServiceImpl is the RPC implementation of WorkUnitService.
type WorkUnitServiceImpl struct {
	client *ab_qa_atp.WorkunitService
}

// Get implementation for workunits.
func (w *WorkUnitServiceImpl) Get(resourceID string) (*ab_qa_atp.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Get(resourceID)

	return call.Do()
}

// Insert implementation for workunits.
func (w *WorkUnitServiceImpl) Insert(workunit *ab_qa_atp.WorkUnit) (*ab_qa_atp.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Insert(workunit)

	return call.Do()
}

// Update implementation for workunits.
func (w *WorkUnitServiceImpl) Update(resourceID string, workunit *ab_qa_atp.WorkUnit) (*ab_qa_atp.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Update(resourceID, workunit)

	return call.Do()
}

// Patch implementation for workunits.
func (w *WorkUnitServiceImpl) Patch(resourceID string, workunit *ab_qa_atp.WorkUnit) (*ab_qa_atp.WorkUnit, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Patch(resourceID, workunit)

	return call.Do()
}

// List implementation for workunits.
func (w *WorkUnitServiceImpl) List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*ab_qa_atp.WorkUnitListResponse, error) {
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

// NewWorkUnit is a helper function to generate a new WorkUnit.
func NewWorkUnit(parentWUId, invocationID, name string, childRunNumber, childShardNumber, childAttemptNumber int) *ab_qa_atp.WorkUnit {
	return &ab_qa_atp.WorkUnit{
		InvocationId:       invocationID,
		ParentId:           parentWUId,
		Name:               name,
		State:              "RUNNING",
		ChildRunNumber:     int64(childRunNumber),
		ChildShardNumber:   int64(childShardNumber),
		ChildAttemptNumber: int64(childAttemptNumber),
	}
}
