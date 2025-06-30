// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"database/sql"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/sqldb"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	androidrepairmetricsdb "go.chromium.org/infra/fleetconsole/internal/database/android_repair_metrics_db"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

// GetDeviceDimensions returns dimensions of all devices
func (frontend *FleetConsoleFrontend) GetRepairMetricsDimensions(ctx context.Context, req *emptypb.Empty) (_ *fleetconsolerpc.GetRepairMetricsDimensionsResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	db := sqldb.MustGetDB(ctx)

	dimensionsMap := make(map[string]*fleetconsolerpc.GetRepairMetricsDimensionsResponse_Values)

	columns := []*queryutils.Column{androidrepairmetricsdb.Priority, androidrepairmetricsdb.LabName, androidrepairmetricsdb.HostGroup, androidrepairmetricsdb.RunTarget}

	for _, column := range columns {
		values, err := repairMetricsSelectDistinct(ctx, db, column.ExternalName)
		if err != nil {
			return nil, err
		}

		dimensionsMap[column.ExternalName] = values

	}

	return &fleetconsolerpc.GetRepairMetricsDimensionsResponse{
		Dimensions: dimensionsMap,
	}, nil
}

func repairMetricsSelectDistinct(ctx context.Context, db *sql.DB, column string) (*fleetconsolerpc.GetRepairMetricsDimensionsResponse_Values, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT DISTINCT %s FROM android_repair_metrics;", column))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		err = rows.Scan(&value)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &fleetconsolerpc.GetRepairMetricsDimensionsResponse_Values{
		Values: values,
	}, nil
}
