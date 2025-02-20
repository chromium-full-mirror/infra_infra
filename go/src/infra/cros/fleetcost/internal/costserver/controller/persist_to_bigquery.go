// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	bqpb "go.chromium.org/infra/cros/fleetcost/api/bigquery"
	"go.chromium.org/infra/cros/fleetcost/api/bigquery/bqvaluesavers"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/entities"
	"go.chromium.org/infra/libs/bqwrapper"
)

// PersistToBigquery persists everything to BigQuery.
func PersistToBigquery(ctx context.Context, projectName string, bqClient bqwrapper.BQIf, readonly bool) error {
	query := datastore.NewQuery(entities.CachedCostResultKind)
	if err := datastore.Run(ctx, query, func(entity *entities.CachedCostResultEntity) error {
		dedicated := entity.CostResult.GetDedicatedCost()
		shared := entity.CostResult.GetSharedCost()
		cloud := entity.CostResult.GetCloudServiceCost()
		total := dedicated + shared + cloud
		resultSaver := &bqvaluesavers.ResultSaver{
			CostResult: &bqpb.CostResult{
				Name:                entity.Hostname,
				Namespace:           "OS",
				HourlyTotalCost:     total,
				HourlyDedicatedCost: dedicated,
				HourlySharedCost:    shared,
				HourlyCloudCost:     cloud,
			},
		}
		if readonly {
			logging.Debugf(ctx, "fake write: %s %s %s %v", projectName, "entities", "cost_result", resultSaver)
			return nil
		}
		// TODO(gregorynisbet): Do not hardcode the table or dataset.
		logging.Debugf(ctx, "writing record %q", entity.Hostname)
		if err := bqClient.Put(ctx, projectName, "entities", "cost_result", []bigquery.ValueSaver{resultSaver}); err != nil {
			logging.Errorf(ctx, "error writing record for hostname %q: %s", entity.Hostname, err)
		}
		return nil
	}); err != nil {
		return errors.Annotate(err, "persisting to bigquery").Err()
	}
	return nil
}
