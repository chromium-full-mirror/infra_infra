// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/sqldb"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	androidrepairmetricsdb "go.chromium.org/infra/fleetconsole/internal/database/android_repair_metrics_db"
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

type filterBy = struct {
	uniqueLabNames   []string
	uniqueHostGroups []string
	uniqueRunTargets []string
}

func figureOutFilters(ctx context.Context, db *sql.DB, filters string) (*filterBy, error) {
	qBuilder, err := queryutils.NewQueryBuilder(androidrepairmetricsdb.AndroidRepairMetricsTable).
		WithRawSelectClause(`
			SELECT
				ARRAY_AGG(DISTINCT lab_name ORDER BY lab_name),
				ARRAY_AGG(DISTINCT host_group ORDER BY host_group),
				ARRAY_AGG(DISTINCT run_target ORDER BY run_target)
		`).WithWhereClause(filters, nil)
	if err != nil {
		return nil, err
	}

	q, err := qBuilder.Build(nil)
	if err != nil {
		return nil, err
	}

	var uniqueLabNames,
		uniqueHostGroups,
		uniqueRunTargets sql.NullString
	err = db.QueryRowContext(ctx, q.Statement, q.Parameters...).Scan(
		&uniqueLabNames,
		&uniqueHostGroups,
		&uniqueRunTargets,
	)
	if err != nil {
		return nil, err
	}

	parseArray := func(ns sql.NullString) []string {
		if !ns.Valid || ns.String == "" {
			return []string{}
		}

		// The strings are the in the form {value1,value2,...}
		return strings.Split(ns.String[1:len(ns.String)-1], ",")
	}

	return &filterBy{
		uniqueLabNames:   parseArray(uniqueLabNames),
		uniqueHostGroups: parseArray(uniqueHostGroups),
		uniqueRunTargets: parseArray(uniqueRunTargets),
	}, nil
}

func queryDbCountRepairMetrics(ctx context.Context, db *sql.DB, filters string) (*fleetconsolerpc.CountRepairMetricsResponse, error) {
	filterBy, err := figureOutFilters(ctx, db, filters)
	if err != nil {
		return nil, errors.Annotate(err, "figureOutFilters").Err()
	}

	totalHosts, offlineHosts, err := queryHostsCount(ctx, db, filterBy)
	if err != nil {
		return nil, errors.Annotate(err, "queryHostsCount").Err()
	}

	totalDevices, offlineDevices, err := queryDevicesCount(ctx, db, filterBy)
	if err != nil {
		return nil, errors.Annotate(err, "queryDevicesCount").Err()
	}

	return &fleetconsolerpc.CountRepairMetricsResponse{
		TotalHosts:     totalHosts,
		OfflineHosts:   offlineHosts,
		TotalDevices:   totalDevices,
		OfflineDevices: offlineDevices,
	}, nil
}

func queryHostsCount(ctx context.Context, db *sql.DB, filterBy *filterBy) (int32, int32, error) {
	if len(filterBy.uniqueHostGroups) == 0 {
		return 0, 0, nil
	}

	q := fmt.Sprintf(`
		SELECT
			COUNT(*) as total_hosts,
			COUNT(CASE WHEN state = 'OFFLINE' THEN 1 ELSE NULL END) AS offline_hosts
		FROM android_hosts
		WHERE host_group IN %s
	`, queryutils.ValuesString(len(filterBy.uniqueHostGroups), len(filterBy.uniqueHostGroups)))

	var totalHosts, offlineHosts int32
	err := db.QueryRowContext(ctx, q, queryutils.ToAnySlice(filterBy.uniqueHostGroups)...).Scan(&totalHosts, &offlineHosts)
	if err != nil {
		return 0, 0, errors.Annotate(err, "failed to run query").Err()
	}

	return totalHosts, offlineHosts, nil
}

func queryDevicesCount(ctx context.Context, db *sql.DB, filterBy *filterBy) (int32, int32, error) {
	if len(filterBy.uniqueHostGroups) == 0 || len(filterBy.uniqueRunTargets) == 0 || len(filterBy.uniqueLabNames) == 0 {
		return 0, 0, nil
	}

	q := fmt.Sprintf(`
		SELECT
			COUNT(*) as total_devices,
			COUNT(CASE WHEN state = 'OFFLINE' THEN 1 ELSE NULL END) AS offline_devices
		FROM android_devices
		WHERE
		host_group IN %s AND
		run_target IN %s AND
		lab_name IN %s
	`,
		// We need the offset because otherwise we reuse the same parameters
		// ($1, $2, etc.) for different IN clauses. This would lead to
		// incorrect filtering as the parameters would be bound to the
		// values from the first IN clause.
		queryutils.ValuesString(len(filterBy.uniqueHostGroups), 0), // This has no offset because its the first
		queryutils.ValuesStringWithOffset(len(filterBy.uniqueRunTargets), 0, len(filterBy.uniqueHostGroups)),
		queryutils.ValuesStringWithOffset(len(filterBy.uniqueLabNames), 0, len(filterBy.uniqueHostGroups)+len(filterBy.uniqueRunTargets)),
	)

	var totalDevices, offlineDevices int32
	err := db.QueryRowContext(ctx, q,
		queryutils.ToAnySlice(slices.Concat(
			filterBy.uniqueHostGroups,
			filterBy.uniqueRunTargets,
			filterBy.uniqueLabNames,
		))...,
	).Scan(&totalDevices, &offlineDevices)
	if err != nil {
		return 0, 0, errors.Annotate(err, "failed to run query").Err()
	}

	return totalDevices, offlineDevices, nil
}
