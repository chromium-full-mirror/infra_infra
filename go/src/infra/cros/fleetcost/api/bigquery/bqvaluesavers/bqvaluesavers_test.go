// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
// Package bqvaluesavers defines entities that can be saved to BigQuery.

// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bqvaluesavers defines entities that can be saved to BigQuery.
package bqvaluesavers_test

import (
	"testing"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	bqpb "go.chromium.org/infra/cros/fleetcost/api/bigquery"
	"go.chromium.org/infra/cros/fleetcost/api/bigquery/bqvaluesavers"
)

// TestResultSaverSimple tests saving a ResultSaver.
func TestResultSaverSimple(t *testing.T) {
	t.Parallel()

	resultSaver := &bqvaluesavers.ResultSaver{
		CostResult: &bqpb.CostResult{
			Name:                "some name",
			Namespace:           "OS",
			HourlyTotalCost:     14.56 + 0.01 + 0.02,
			HourlyDedicatedCost: 14.56,
			HourlySharedCost:    0.01,
			HourlyCloudCost:     0.02,
		},
	}

	row, _, err := resultSaver.Save()
	if err != nil {
		t.Errorf("error saving result: %s", err)
	}

	expected := map[string]bigquery.Value{
		"name":                  "some name",
		"namespace":             "OS",
		"hourly_total_cost":     14.56 + 0.01 + 0.02,
		"hourly_dedicated_cost": 14.56,
		"hourly_shared_cost":    0.01,
		"hourly_cloud_cost":     0.02,
	}

	assert.That(t, row, should.Match(expected))
}
