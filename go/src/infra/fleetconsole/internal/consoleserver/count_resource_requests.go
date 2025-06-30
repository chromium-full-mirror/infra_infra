// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"time"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/bigqueryclient"
	"go.chromium.org/infra/fleetconsole/internal/consoleserver/rri"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

func (frontend *FleetConsoleFrontend) CountResourceRequests(ctx context.Context, req *fleetconsolerpc.CountResourceRequestsRequest) (_ *fleetconsolerpc.CountResourceRequestsResponse, err error) {
	bqClient, err := bigqueryclient.NewBQClient(ctx, "chrome-fleet-analytics")
	if err != nil {
		logging.Infof(ctx, "Error instantiating a new BigQuery client")
		return nil, err
	}

	var total, ffInProgress, ffComplete, msInProgress, buildInProgress, qaInProgress, configInProgress int32

	queryBuilder := queryutils.NewQueryBuilder(rri.GetResourceRequestsTable(frontend.IsProdEnvironment())).
		SetSqlLangType(queryutils.BigQueryLangType).
		WithCustomSelectClause(
			queryutils.CountAll(&total),
			queryutils.CountIf(rri.FulfillmentStatusColumn, &ffInProgress).Equals(rri.InProgressStatus),
			queryutils.CountIf(rri.FulfillmentStatusColumn, &ffComplete).Equals(rri.CompleteStatus),
			queryutils.CountIf(rri.MaterialSourcingStatusColumn, &msInProgress).Equals(rri.InProgressStatus),
			queryutils.CountIf(rri.BuildStatusColumn, &buildInProgress).Equals(rri.InProgressStatus),
			queryutils.CountIf(rri.QAStatusColumn, &qaInProgress).Equals(rri.InProgressStatus),
			queryutils.CountIf(rri.ConfigStatusColumn, &configInProgress).Equals(rri.InProgressStatus))

	query, reader, err := queryBuilder.ToBigQueryQuery(bqClient)

	if err != nil {
		logging.Errorf(ctx, "failed to build query: %s", err)
		return nil, err
	}

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
	defer cancel()
	it, err := query.Read(ctx)
	if err != nil {
		logging.Errorf(ctx, "fetching resource requests from big query failed: %s", err)
		return nil, err
	}

	err = reader.ReadNext(it)

	if err != nil {
		logging.Errorf(ctx, "fetching resource requests from big query failed: %s", err)
		return nil, err
	}

	return &fleetconsolerpc.CountResourceRequestsResponse{
		Total:            total,
		InProgress:       ffInProgress,
		Completed:        ffComplete,
		MaterialSourcing: msInProgress,
		Build:            buildInProgress,
		Qa:               qaInProgress,
		Config:           configInProgress,
	}, nil
}
