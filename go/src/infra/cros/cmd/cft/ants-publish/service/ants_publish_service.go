// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service provides the API handlers for ants publish.
package service

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	common_utils "go.chromium.org/chromiumos/test/publish/cmd/common-utils"
)

type AntsPublishService struct {
	RetryCount       int
	InvocationID     string
	ParentWorkUnitID string
	AccountID        int
}

// NewAntsPublishService creates a new publish service to interact with Ants.
func NewAntsPublishService(req *api.PublishRequest) (*AntsPublishService, error) {
	m, err := unpackMetadata(req)
	if err != nil {
		return nil, err
	}

	if err = common_utils.ValidateGenericPublishRequest(req); err != nil {
		return nil, err
	}

	if err = validateAntsPublishRequest(m); err != nil {
		return nil, err
	}

	retryCount := 0
	if req.GetRetryCount() > 0 {
		retryCount = int(req.GetRetryCount())
	}

	accountID, err := strconv.Atoi(m.GetAccountId())
	if err != nil {
		return nil, err
	}

	return &AntsPublishService{
		RetryCount:       retryCount,
		InvocationID:     m.GetAntsInvocationId(),
		ParentWorkUnitID: m.GetParentWorkUnitId(),
		AccountID:        accountID,
	}, nil
}

// UploadToAnts uploads test results to ResultDB.
func (rps *AntsPublishService) UploadToAnts(ctx context.Context) error {
	// TODO(srinivashegde): Implement this
	log.Printf("Uploading to AnTS")
	return nil
}

func validateAntsPublishRequest(metadata *metadata.PublishAntsMetadata) error {
	if metadata.GetAntsInvocationId() == "" {
		return fmt.Errorf("ants invocation id is required")
	} else if metadata.GetParentWorkUnitId() == "" {
		return fmt.Errorf("parent workunit id is required")
	} else if metadata.GetAccountId() == "" {
		return fmt.Errorf("partner account id is required")
	}

	return nil
}

// unpackMetadata unpacks the Any metadata field into PublishGcsMetadata
func unpackMetadata(req *api.PublishRequest) (*metadata.PublishAntsMetadata, error) {
	var m metadata.PublishAntsMetadata
	if err := req.Metadata.UnmarshalTo(&m); err != nil {
		return &m, fmt.Errorf("improperly formatted input proto metadata: %w", err)
	}
	return &m, nil
}
