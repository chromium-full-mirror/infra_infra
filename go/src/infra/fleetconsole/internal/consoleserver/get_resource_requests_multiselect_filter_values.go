// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"sort"
	"time"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/bigqueryclient"
	"go.chromium.org/infra/fleetconsole/internal/consoleserver/rri"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

// GetResourceRequestsMultiselectFilterValues lists resource requests.
func (frontend *FleetConsoleFrontend) GetResourceRequestsMultiselectFilterValues(ctx context.Context, req *fleetconsolerpc.GetResourceRequestsMultiselectFilterValuesRequest) (*fleetconsolerpc.GetResourceRequestsMultiselectFilterValuesResponse, error) {
	logging.Infof(ctx, "GetResourceRequestsMultiselectFilterValues called")

	bqClient, err := bigqueryclient.NewBQClient(ctx, "chrome-fleet-analytics")
	if err != nil {
		logging.Infof(ctx, "Error instantiating a new BigQuery client")
		return nil, err
	}

	var rrId, resourceDetails any

	queryBuilder := queryutils.NewQueryBuilder(rri.GetResourceRequestsTable(frontend.IsProdEnvironment()))
	queryBuilder = queryBuilder.SetSqlLangType(queryutils.BigQueryLangType)

	queryBuilder = queryBuilder.WithCustomSelectDistinctClause(queryutils.Select(rri.RrIDColumn, &rrId), queryutils.Select(rri.ResourceDetailsColumn, &resourceDetails))

	query, reader, err := queryBuilder.ToBigQueryQuery(bqClient)

	if err != nil {
		logging.Errorf(ctx, "failed to build query: %s", err)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	it, err := query.Read(ctx)
	if err != nil {
		logging.Errorf(ctx, "fetching resource requests from big query failed: %s", err)
		return nil, err
	}

	rrIdList := make([]string, 0)
	resourceDetailsList := make([]string, 0)

	for {
		err = reader.ReadNext(it)
		if err != nil {
			break
		}
		rrIdList = append(rrIdList, rrId.(string))
		resourceDetailsList = append(resourceDetailsList, resourceDetails.(string))
	}

	sort.Strings(rrIdList)
	sort.Strings(resourceDetailsList)

	return &fleetconsolerpc.GetResourceRequestsMultiselectFilterValuesResponse{
		RrIds:           rrIdList,
		ResourceDetails: resourceDetailsList,
	}, nil
}
