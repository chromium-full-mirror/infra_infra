// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package analytics

import (
	"context"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"google.golang.org/api/option"

	"go.chromium.org/luci/common/logging"

	"infra/cros/cmd/common_lib/common"
)

const saProject = "chromeos-test-platform-data"
const dataset = "analytics"
const trInstanceTable = "TRMetrics"

type TrInstanceBqData struct {
	Date          civil.DateTime
	BBID          string
	Status        string
	Pool          string
	SuiteName     string
	AnalyticsName string
	CIPDLabel     string
	Bucket        string
	Duration      float64
	IsLed         bool
	Summary       string
	IsDynamic     bool
}

// TrAnalyticsBQClient will build the client for the CTP BQ tables, using the default CTP SA
func TrAnalyticsBQClient(ctx context.Context) *bigquery.Client {
	var err error
	defer func(err error) {
		if err != nil {
			logging.Infof(ctx, "Unable to make BQ client: %s", err)
		}
	}(err)
	dockerKeyFile, err := common.LocateFile([]string{common.LabDockerKeyFileLocation, common.VmLabDockerKeyFileLocation})
	if err != nil {
		return nil
	}
	c, err := bigquery.NewClient(ctx, saProject,
		option.WithCredentialsFile(dockerKeyFile))
	return c
}

// InsertTrMetrics will insert the CTP Analytics Data into the CTPv2Metrics Table.
func InsertTrInstanceMetrics(BQClient *bigquery.Client, data []*TrInstanceBqData) error {
	ctx := context.Background()
	inserter := BQClient.Dataset(dataset).Table(trInstanceTable).Inserter()
	if err := inserter.Put(ctx, data); err != nil {
		return err
	}
	return nil
}

// SoftInsertTrInstanceMetrics inserts metrics to the BQClient. Do not fail on errors.
func SoftInsertTrInstanceMetrics(ctx context.Context, BQClient *bigquery.Client, data *TrInstanceBqData) {
	if BQClient != nil {
		err := InsertTrInstanceMetrics(BQClient, []*TrInstanceBqData{data})
		if err != nil {
			logging.Infof(ctx, "Error during BQ write: %s", err)
		}
		logging.Infof(ctx, "Successful write")
	} else {
		logging.Infof(ctx, "Skipped BQ write as no client provided.")
	}
}
