// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"database/sql"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/sqldb"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	androidrepairmetricsdb "go.chromium.org/infra/fleetconsole/internal/database/android_repair_metrics_db"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
	"go.chromium.org/infra/fleetconsole/internal/utils"
)

// ListDevices lists devices from the db.
func (frontend *FleetConsoleFrontend) ListRepairMetrics(ctx context.Context, req *fleetconsolerpc.ListRepairMetricsRequest) (_ *fleetconsolerpc.ListRepairMetricsResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	offset, err := pageTokenToOffset(req.PageToken, req.Filter, req.OrderBy)
	if err != nil {
		logging.Errorf(ctx, "failed to extract page token: %s", err)
		return nil, err
	}

	pageSize := maxPageSize
	if req.PageSize != 0 {
		pageSize = min(int(req.PageSize), maxPageSize)
	}

	results, hasMoreData, err := queryDbListRepairMetrics(ctx, sqldb.MustGetDB(ctx), req.Filter, req.OrderBy, offset, pageSize)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errors.Annotate(err, "failed to list repair metrics").Tag(grpcutil.CanceledTag).Err()
		}
		logging.Errorf(ctx, "failed to list repair metrics: %s", err)
		return nil, err
	}

	var nextPageToken string
	if hasMoreData {
		nextPageToken, err = offsetToPageToken(offset+pageSize, req.Filter, req.OrderBy)
		if err != nil {
			logging.Errorf(ctx, "failed to encode next page token: %s", err)
			return nil, err
		}
	}

	return &fleetconsolerpc.ListRepairMetricsResponse{
		RepairMetrics: results,
		NextPageToken: nextPageToken,
	}, nil
}

func queryDbListRepairMetrics(ctx context.Context, db *sql.DB, filters string, orderby string, offset, pageSize int) ([]*fleetconsolerpc.RepairMetric, bool, error) {
	// Fetch one extra row to check whether there is more data available
	query, err := buildQueryListRepairMetrics(filters, orderby, offset, pageSize+1)
	if err != nil {
		return nil, false, err
	}

	rows, err := db.QueryContext(ctx, query.Statement, query.Parameters...)
	if err != nil {
		return nil, false, errors.Annotate(err, "failed to execute query").Err()
	}
	defer rows.Close()

	var res []*fleetconsolerpc.RepairMetric
	var priority string
	for rows.Next() {
		rm := &fleetconsolerpc.RepairMetric{}

		err = rows.Scan(
			&priority,
			&rm.LabName,
			&rm.HostGroup,
			&rm.RunTarget,
			&rm.MinimumRepairs,
			&rm.DevicesOffline,
			&rm.TotalDevices,
		)
		if err != nil {
			return nil, false, err
		}

		rm.Priority = fleetconsolerpc.RepairMetric_Priority(fleetconsolerpc.RepairMetric_Priority_value[priority])

		res = append(res, rm)
	}

	return res, len(res) > pageSize, nil
}

func buildQueryListRepairMetrics(filters string, orderby string, offset, pageSize int) (*queryutils.Query, error) {
	qBuilder, err := queryutils.NewQueryBuilder(androidrepairmetricsdb.AndroidRepairMetricsTable).WithSelectAllClause().WithWhereClause(filters, nil)
	if err != nil {
		return nil, utils.InvalidFilterError(err)
	}

	if pageSize > 0 {
		qBuilder = qBuilder.WithOffsetPagination(offset, pageSize)
	}

	qBuilder, err = qBuilder.WithOrderByClause(orderby, "minimum_repairs")
	if err != nil {
		return nil, utils.InvalidOrderByError(err)
	}

	query, err := qBuilder.Build(nil)
	if err != nil {
		return nil, errors.Annotate(err, "failed to build query").Err()
	}

	return query, nil
}
