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
	"go.chromium.org/infra/fleetconsole/internal/database/android_repair_metrics_db"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

// CountDevices lists devices from the db.
func (frontend *FleetConsoleFrontend) CountRepairMetrics(ctx context.Context, req *fleetconsolerpc.CountRepairMetricsRequest) (_ *fleetconsolerpc.CountRepairMetricsResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	result, err := queryDbCountRepairMetrics(ctx, sqldb.MustGetDB(ctx), req.Filter)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errors.Annotate(err, "failed to count repair metrics").Tag(grpcutil.CanceledTag).Err()
		}
		logging.Errorf(ctx, "failed to Count repair metrics: %s", err)
		return nil, err
	}

	return result, nil
}

func queryDbCountRepairMetrics(ctx context.Context, db *sql.DB, filters string) (*fleetconsolerpc.CountRepairMetricsResponse, error) {
	totalHosts, offlineHosts, err := queryHostsCount(ctx, db, filters)
	if err != nil {
		return nil, errors.Annotate(err, "queryHostsCount").Err()
	}

	totalDevices, offlineDevices, err := queryDevicesCount(ctx, db, filters)
	if err != nil {
		return nil, errors.Annotate(err, "queryHostsCount").Err()
	}

	return &fleetconsolerpc.CountRepairMetricsResponse{
		TotalHosts:     totalHosts,
		OfflineHosts:   offlineHosts,
		TotalDevices:   totalDevices,
		OfflineDevices: offlineDevices,
	}, nil
}

func queryHostsCount(ctx context.Context, db *sql.DB, filters string) (int32, int32, error) {
	//TODO (pietroscutta): filter
	qBuilder := queryutils.NewQueryBuilder(androidrepairmetricsdb.AndroidHostsTable).
		WithRawSelectClause(`
			SELECT
				COUNT(*) as total_hosts,
				COUNT(CASE WHEN state = 'OFFLINE' THEN 1 ELSE NULL END) AS offline_hosts
		`)

	query, err := qBuilder.Build(nil)
	if err != nil {
		return 0, 0, errors.Annotate(err, "failed to build query").Err()
	}

	var totalHosts, offlineHosts int32
	err = db.QueryRowContext(ctx, query.Statement, query.Parameters...).Scan(&totalHosts, &offlineHosts)
	if err != nil {
		return 0, 0, errors.Annotate(err, "failed to run query").Err()
	}

	return totalHosts, offlineHosts, nil
}

func queryDevicesCount(ctx context.Context, db *sql.DB, filters string) (int32, int32, error) {
	//TODO (pietroscutta): filter
	qBuilder := queryutils.NewQueryBuilder(androidrepairmetricsdb.AndroidDevicesTable).
		WithRawSelectClause(`
			SELECT
				COUNT(*) as total_devices,
				COUNT(CASE WHEN state = 'OFFLINE' THEN 1 ELSE NULL END) AS offline_devices
		`)

	query, err := qBuilder.Build(nil)
	if err != nil {
		return 0, 0, errors.Annotate(err, "failed to build query").Err()
	}

	var totalDevices, offlineDevices int32
	err = db.QueryRowContext(ctx, query.Statement, query.Parameters...).Scan(&totalDevices, &offlineDevices)
	if err != nil {
		return 0, 0, errors.Annotate(err, "failed to run query").Err()
	}

	return totalDevices, offlineDevices, nil
}
