// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bq implements bigquery-related logic.
package bq

import (
	"context"
	"time"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/common/bq"
)

// GetPSTTimeStamp returns the PST timestamp for bq table.
func GetPSTTimeStamp(t time.Time) string {
	tz, _ := time.LoadLocation("America/Los_Angeles")
	return t.In(tz).Format("20060102")
}

// InitBQUploaderWithClient initialize a bigquery uploader with a given bigquery client.
func InitBQUploaderWithClient(ctx context.Context, client *bigquery.Client, dataset, table string) *bq.Uploader {
	up := bq.NewUploader(ctx, client, dataset, table)
	up.SkipInvalidRows = true
	up.IgnoreUnknownValues = true
	return up
}

// InitBQUploader initialize a bigquery uploader.
func InitBQUploader(ctx context.Context, project, dataset, table string) (*bq.Uploader, error) {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, err
	}
	return InitBQUploaderWithClient(ctx, client, dataset, table), nil
}
