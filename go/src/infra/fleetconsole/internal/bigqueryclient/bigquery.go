// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bigqueryclient contains utils to connect to Big Query
package bigqueryclient

import (
	"context"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server/auth"
)

// NewBQClient creates a BQ client based on cloud project.
func NewBQClient(ctx context.Context, cloudProject string) (*bigquery.Client, error) {
	tokenSource, err := auth.GetTokenSource(ctx, auth.AsSelf, auth.WithScopes(auth.CloudOAuthScopes...))
	if err != nil {
		return nil, errors.Annotate(err, "NewBQClient: failed to get AsSelf credentails").Err()
	}
	client, err := bigquery.NewClient(
		ctx, cloudProject,
		option.WithTokenSource(tokenSource),
	)
	if err != nil {
		logging.Errorf(ctx, "NewBQClient: cannot set up BigQuery client: %s", err)
		return nil, err
	}
	return client, nil
}
