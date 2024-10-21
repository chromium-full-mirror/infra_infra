// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"context"
	"io"

	ab_qa_atp "infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
)

// TestArtifactsService handles API calls related to testResults.
type TestArtifactsService interface {
	Get(resourceID string) (*ab_qa_atp.BuildArtifactMetadata, error)
	Update(resourceID string, reader io.Reader, metadata *ab_qa_atp.BuildArtifactMetadata) (*ab_qa_atp.BuildArtifactMetadata, error)
	List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*ab_qa_atp.TestArtifactListResponse, error)
}

// TestArtifactsServiceImpl is the RPC implementation of TestArtifactsService.
type TestArtifactsServiceImpl struct {
	client *ab_qa_atp.TestartifactService
}

// Get implmentation for test artifact metadata.
func (w *TestArtifactsServiceImpl) Get(resourceID string) (*ab_qa_atp.BuildArtifactMetadata, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Get(resourceID)

	return call.Do()
}

// Update implementation for test artifact metadata.
func (w *TestArtifactsServiceImpl) Update(resourceID string, reader io.Reader, metadata *ab_qa_atp.BuildArtifactMetadata) (*ab_qa_atp.BuildArtifactMetadata, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Update(resourceID, metadata)

	return call.InvocationId(metadata.InvocationId).WorkUnitId(metadata.WorkUnitId).Media(reader).Do()
}

// List implmentation for test artifact.
func (w *TestArtifactsServiceImpl) List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*ab_qa_atp.TestArtifactListResponse, error) {
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
