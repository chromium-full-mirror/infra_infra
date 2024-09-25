// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service provides the API handlers for ants publish.
package service

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	common_utils "go.chromium.org/chromiumos/test/publish/cmd/common-utils"
)

type AntsPublishService struct {
	RetryCount           int
	CurrentInvocationID  string
	TestResultProto      *artifact.TestResult
	TesthausURL          string
	TempDirPath          string
	BaseVariant          map[string]string
	PostProcessResponses *api.RunActivitiesResponse
}

// NewAntsPublishService creates a new publish service to interact with Ants.
// TODO(srinivashegde): Use AntsMetadata once ready.
func NewAntsPublishService(req *api.PublishRequest) (*AntsPublishService, error) {
	m, err := unpackMetadata(req)
	if err != nil {
		return nil, err
	}

	if err = common_utils.ValidateRDBPublishRequest(req, m); err != nil {
		return nil, err
	}

	retryCount := 0
	if req.GetRetryCount() > 0 {
		retryCount = int(req.GetRetryCount())
	}

	return &AntsPublishService{
		RetryCount:           retryCount,
		CurrentInvocationID:  m.GetCurrentInvocationId(),
		TestResultProto:      m.GetTestResult(),
		TesthausURL:          m.GetTesthausUrl(),
		BaseVariant:          m.GetBaseVariant(),
		PostProcessResponses: m.GetPostProcessResponses(),
	}, nil
}

// UploadToAnts uploads test results to ResultDB.
func (rps *AntsPublishService) UploadToAnts(ctx context.Context) error {
	// TODO(srinivashegde): Implement this
	return nil
}

// unpackMetadata unpacks the Any metadata field into PublishGcsMetadata
func unpackMetadata(req *api.PublishRequest) (*metadata.PublishRdbMetadata, error) {
	var m metadata.PublishRdbMetadata
	if err := req.Metadata.UnmarshalTo(&m); err != nil {
		return &m, fmt.Errorf("improperly formatted input proto metadata: %w", err)
	}
	return &m, nil
}
