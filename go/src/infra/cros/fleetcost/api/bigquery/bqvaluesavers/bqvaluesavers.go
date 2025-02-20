// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bqvaluesavers wraps saveable-to-bigquery proto types so that we
// don't have to put methods on the proto message, which is bad style.
//
// If that rule did not exist, then neither would this package.
package bqvaluesavers

import (
	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/common/errors"

	bqpb "go.chromium.org/infra/cros/fleetcost/api/bigquery"
)

// ResultSaver saves a cost result.
type ResultSaver struct {
	CostResult *bqpb.CostResult
}

// Check that result saver is a bigquery.ValueSaver
var _ bigquery.ValueSaver = &ResultSaver{}

// Save produces a bigquery record that can be saved.
func (resultSaver *ResultSaver) Save() (row map[string]bigquery.Value, insertID string, err error) {
	if resultSaver.CostResult.GetName() == "" {
		return nil, "", errors.Reason("ResultSaver: name cannot be empty").Err()
	}

	row = make(map[string]bigquery.Value)
	row["name"] = resultSaver.CostResult.GetName()
	row["namespace"] = resultSaver.CostResult.GetNamespace()
	row["hourly_cloud_cost"] = resultSaver.CostResult.GetHourlyCloudCost()
	row["hourly_dedicated_cost"] = resultSaver.CostResult.GetHourlyDedicatedCost()
	row["hourly_shared_cost"] = resultSaver.CostResult.GetHourlySharedCost()
	row["hourly_total_cost"] = resultSaver.CostResult.GetHourlyTotalCost()

	return row, bigquery.NoDedupeID, nil
}
