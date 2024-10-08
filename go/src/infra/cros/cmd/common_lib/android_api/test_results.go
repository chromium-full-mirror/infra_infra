// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"context"

	"infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
)

// TestResultService handles API calls related to testResults.
type TestResultService interface {
	Get(resourceID int64) (*androidbuildinternal.TestResult, error)
	Insert(testResult *androidbuildinternal.TestResult) (*androidbuildinternal.TestResult, error)
	BatchInsert(request *androidbuildinternal.TestResultBatchInsertRequest) (*androidbuildinternal.TestResultBatchInsertResponse, error)
	Update(resourceID int64, testResult *androidbuildinternal.TestResult) (*androidbuildinternal.TestResult, error)
	List(ctx context.Context, testResultID string, options AndroidBuildAPIOptions) (*androidbuildinternal.TestResultListResponse, error)
}

// TestResultServiceImpl is the RPC implementation of TestResultService.
type TestResultServiceImpl struct {
	client *androidbuildinternal.TestresultService
}

// Get implmentation for testResults.
func (w *TestResultServiceImpl) Get(resourceID int64) (*androidbuildinternal.TestResult, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Get(resourceID)

	return call.Do()
}

// Insert implementation for testResults.
func (w *TestResultServiceImpl) Insert(testResult *androidbuildinternal.TestResult) (*androidbuildinternal.TestResult, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Insert(testResult)

	return call.Do()
}

// BatchInsert implementation for testResults.
func (w *TestResultServiceImpl) BatchInsert(request *androidbuildinternal.TestResultBatchInsertRequest) (*androidbuildinternal.TestResultBatchInsertResponse, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Batchinsert(request)

	return call.Do()
}

// Update implementation for testResults.
func (w *TestResultServiceImpl) Update(resourceID int64, testResult *androidbuildinternal.TestResult) (*androidbuildinternal.TestResult, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.Update(resourceID, testResult)

	return call.Do()
}

// List implmentation for testResults.
func (w *TestResultServiceImpl) List(ctx context.Context, testResultID string, options AndroidBuildAPIOptions) (*androidbuildinternal.TestResultListResponse, error) {
	if w.client == nil {
		return nil, errInit
	}

	call := w.client.List().TestResultId(testResultID)
	if options.PageToken != "" {
		call = call.PageToken(options.PageToken)
	}

	maxResults := defaultMaxResults
	if options.MaxResults > 0 {
		maxResults = options.MaxResults
	}

	return call.MaxResults(maxResults).Do()
}
