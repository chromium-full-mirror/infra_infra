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

	bqpb "infra/cros/fleetcost/api/bigquery"
	"infra/cros/fleetcost/api/bigquery/bqvaluesavers"
	"infra/cros/fleetcost/internal/costserver/entities"
	"infra/libs/bqwrapper"
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
			logging.Debugf(ctx, "%s %s %s %v", projectName, "entities", "CachedCostResult", resultSaver)
			return nil
		}
		// TODO(gregorynisbet): Do not hardcode the table or dataset.
		logging.Debugf(ctx, "writing record %q", entity.Hostname)
		return bqClient.Put(ctx, projectName, "entities", "CachedCostResult", []bigquery.ValueSaver{resultSaver})
	}); err != nil {
		return errors.Annotate(err, "persisting to bigquery").Err()
	}
	return nil
}
