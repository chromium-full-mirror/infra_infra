// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"context"
	"database/sql"
	"fmt"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
)

func CountDevices(ctx context.Context, dbConn *sql.DB, filter string, realms []string) (*fleetconsolerpc.CountDevicesResponse, error) {
	query, err := buildCountDevicesQuery(ctx, filter, realms)
	if err != nil {
		logging.Errorf(ctx, "failed to construct the query: %s", err)
		return nil, err
	}

	rows, err := dbConn.QueryContext(ctx, query.Statement, query.Parameters...)
	if err != nil {
		logging.Errorf(ctx, "failed to read from the DB: %s", err)
		return nil, fmt.Errorf("DB QueryContext: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("failed to get the device metrics")
	}

	var results DeviceMetricsDAO
	if err = rows.Scan(
		&results.Total,
		&results.Leased,
		&results.Available,
		&results.Ready,
		&results.NeedManualRepair,
		&results.NeedRepair,
		&results.RepairFailed,
	); err != nil {
		return nil, fmt.Errorf("CountDevices: %w", err)
	}

	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("CountDevices: %w", err)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("CountDevices: %w", err)
	}

	// Currently we use lease state as a task state,
	// but still needs to be confirmed whether they 100% much or not.
	return &fleetconsolerpc.CountDevicesResponse{
		Total: results.Total,
		TaskState: &fleetconsolerpc.TaskStateCounts{
			Busy: results.Leased,
			Idle: results.Available,
		},
		DeviceState: &fleetconsolerpc.DeviceStateCounts{
			Ready:            results.Ready,
			NeedManualRepair: results.NeedManualRepair,
			NeedRepair:       results.NeedRepair,
			RepairFailed:     results.RepairFailed,
		},
	}, nil
}
