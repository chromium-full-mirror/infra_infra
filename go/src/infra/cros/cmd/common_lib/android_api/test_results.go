// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"context"

	ab_qa_atp "infra/cros/cmd/common_lib/ants-qa/androidbuildinternal/v3_qa_atp"
)

// TestResultService handles API calls related to testResults.
type TestResultService interface {
	Get(resourceID int64) (*ab_qa_atp.TestResult, error)
	Insert(testResult *ab_qa_atp.TestResult) (*ab_qa_atp.TestResult, error)
	BatchInsert(ctx context.Context, invocationID string, request *ab_qa_atp.TestResultBatchInsertRequest) (*ab_qa_atp.TestResultBatchInsertResponse, error)
	Update(resourceID int64, testResult *ab_qa_atp.TestResult) (*ab_qa_atp.TestResult, error)
	List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*ab_qa_atp.TestResultListResponse, error)
}

// TestResultServiceImpl is the RPC implementation of TestResultService.
type TestResultServiceImpl struct {
	client *ab_qa_atp.TestresultService
}

// Get implementation for testResults.
func (w *TestResultServiceImpl) Get(resourceID int64) (*ab_qa_atp.TestResult, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Get(resourceID)

	return call.Do()
}

// Insert implementation for testResults.
func (w *TestResultServiceImpl) Insert(testResult *ab_qa_atp.TestResult) (*ab_qa_atp.TestResult, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Insert(testResult)

	return call.Do()
}

// BatchInsert implementation for testResults.
func (w *TestResultServiceImpl) BatchInsert(ctx context.Context, invocationID string, request *ab_qa_atp.TestResultBatchInsertRequest) (*ab_qa_atp.TestResultBatchInsertResponse, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Batchinsert(request)

	return call.Context(ctx).InvocationId(invocationID).Do()
}

// Update implementation for testResults.
func (w *TestResultServiceImpl) Update(resourceID int64, testResult *ab_qa_atp.TestResult) (*ab_qa_atp.TestResult, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Update(resourceID, testResult)

	return call.Do()
}

// List implementation for testResults.
func (w *TestResultServiceImpl) List(ctx context.Context, invocationID string, options AndroidBuildAPIOptions) (*ab_qa_atp.TestResultListResponse, error) {
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
