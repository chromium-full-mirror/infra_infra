// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/auth"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/luciexe/build"
)

type Row struct {
	Timestamp time.Time `bigquery:"timestamp"`
	Project   string    `bigquery:"project"`
	Bucket    string    `bigquery:"bucket"`
	Builder   string    `bigquery:"builder"`
}

func isChromeOrChromiumProject(project string) bool {
	return project == "chrome" || project == "chromium" || strings.HasPrefix(project, "chrome-") || strings.HasPrefix(project, "chromium-")
}

func retrieveChromeAndChromiumBuilders(ctx context.Context) ([]Row, error) {
	client, err := bbClient(ctx)

	if err != nil {
		return nil, errors.Fmt("Make BB client: %w", err)
	}

	result := []*buildbucketpb.BuilderItem{}
	token := ""
	for {
		req := &buildbucketpb.ListBuildersRequest{
			PageSize: 1000,
		}
		if token != "" {
			req.PageToken = token
		}
		res, _ := client.ListBuilders(ctx, req)
		if err != nil {
			return nil, err
		}

		result = append(result, res.Builders...)
		token = res.NextPageToken
		if token == "" {
			break
		}
	}

	logging.Infof(ctx, "Got %d builders", len(result))

	var rows []Row
	for _, b := range result {
		if isChromeOrChromiumProject(b.Id.Project) {
			var row Row
			row.Timestamp = time.Now()
			row.Project = b.Id.Project
			row.Bucket = b.Id.Bucket
			row.Builder = b.Id.Builder
			rows = append(rows, row)
		}
	}

	logging.Infof(ctx, "of which %d are Chrome or Chromium builders", len(rows))

	return rows, nil
}

func generate(ctx context.Context) error {
	bqClient, err := setup(ctx)
	if err != nil {
		return errors.Fmt("Setup: %w", err)
	}
	defer bqClient.Close()

	err = deleteBuilders(ctx, bqClient)
	if err != nil {
		return errors.Fmt("Delete builders: %w", err)
	}

	var rows []Row
	rows, err = retrieveChromeAndChromiumBuilders(ctx)
	if err != nil {
		logging.Errorf(ctx, "Unable to retrieve builders using ListBuilders RPC")
	}

	logging.Infof(ctx, "Write %d builders to database", len(rows))

	// Write out to BQ
	if err = writeToBigQuery(ctx, bqClient, rows); err != nil {
		return errors.Fmt("Write builders: %w", err)
	}
	return nil
}

func setup(buildCtx context.Context) (*bigquery.Client, error) {
	var err error
	step, _ := build.StartStep(buildCtx, "Setup")
	defer func() { step.End(err) }()

	bqClient, err := bigquery.NewClient(buildCtx, "cr-builder-value")
	if err != nil {
		return nil, errors.Fmt("Initializing BigQuery client: %w", err)
	}

	return bqClient, nil
}

func bbClient(buildCtx context.Context) (buildbucketpb.BuildersClient, error) {
	var err error
	step, _ := build.StartStep(buildCtx, "Make BB client")
	defer func() { step.End(err) }()

	authenticator := auth.NewAuthenticator(buildCtx, auth.SilentLogin, auth.Options{})
	httpClient, err := authenticator.Client()
	if err != nil {
		return nil, errors.Fmt("Initializing Auth: %w", err)
	}

	return buildbucketpb.NewBuildersPRPCClient(&prpc.Client{
		C:    httpClient,
		Host: "cr-buildbucket.appspot.com",
	}), nil
}

// Delete all rows that have been created on the same day
func deleteBuilders(buildCtx context.Context, bqClient *bigquery.Client) error {
	var err error
	step, _ := build.StartStep(buildCtx, "Delete builders added on the same day")
	defer func() { step.End(err) }()

	step.SetSummaryMarkdown("Delete the builders added on the same day from cr-builder-value.builder_value.builders")

	// BigQuery does not allow us to delete data of the last 90 minutes
	// https://support.google.com/gcp-kb-internal/answer/10525609?hl=en
	bqClient.Query(`
	DELETE FROM cr-builder-value.builder_value.builders
	WHERE
	    (CAST(timestamp AS DATE) = CAST(CURRENT_TIMESTAMP() AS DATE))
	    AND timestamp < TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 90 MINUTE)
	`)

	return nil
}

func writeToBigQuery(buildCtx context.Context, bqClient *bigquery.Client, rows []Row) error {
	var err error
	step, ctx := build.StartStep(buildCtx, "Write builders to BigQuery table")
	defer func() { step.End(err) }()

	step.SetSummaryMarkdown("Writing to BQ table cr-builder-value.builder_value.builders")

	inserter := bqClient.Dataset("builder_value").Table("builders").Inserter()
	if err := inserter.Put(ctx, rows); err != nil {
		return err
	}

	return nil
}
