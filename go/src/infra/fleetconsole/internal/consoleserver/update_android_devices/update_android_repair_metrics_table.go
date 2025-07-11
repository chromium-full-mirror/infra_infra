// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateandroiddevices

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	androidrepairmetricsdb "go.chromium.org/infra/fleetconsole/internal/database/android_repair_metrics_db"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

func updateAndroidRepairMetricsTable(
	ctx context.Context,
	tx *sql.Tx,
	runTargetLabNameHostGroup runTargetsLabNamesHostGroups,
	priority fleetconsolerpc.RepairMetric_Priority, minimumRepairs int, devicesOffline int, totalDevices int,
) error {
	q, err := queryutils.NewInsertBuilder(androidrepairmetricsdb.AndroidRepairMetricsTable).
		AllColumns().
		Values(
			priority.String(),
			runTargetLabNameHostGroup.labName,
			runTargetLabNameHostGroup.hostGroup,
			runTargetLabNameHostGroup.runTarget,
			strconv.Itoa(minimumRepairs),
			strconv.Itoa(devicesOffline),
			strconv.Itoa(totalDevices),
		).
		OnConflict(
			queryutils.ConflictOn(
				androidrepairmetricsdb.LabName,
				androidrepairmetricsdb.HostGroup,
				androidrepairmetricsdb.RunTarget,
			).Replace(androidrepairmetricsdb.AndroidRepairMetricsTable.Columns...),
		).Build()
	if err != nil {
		return errors.Annotate(err, "failed to build query").Err()
	}

	_, err = tx.ExecContext(ctx, q.Statement, q.Parameters...)

	return errors.WrapIf(err, "failed to update android repair metrics")
}

func calculateSlo(ctx context.Context, tx *sql.Tx, runTargetLabNameHostGroup runTargetsLabNamesHostGroups) (fleetconsolerpc.RepairMetric_Priority, int, int, int) {
	qBuilder, err := queryutils.NewQueryBuilder(androidrepairmetricsdb.AndroidDevicesTable).
		WithCustomSelectClause(
			queryutils.CountIf("LOWER(state)", nil).In("gone", "offline"),
			queryutils.CountIf("LOWER(state)", nil).In("fastboot"),
			queryutils.CountIf("LOWER(state)", nil).In("unavailable", "unknown"),
			queryutils.CountIf("LOWER(state)", nil).In("available", "idle", "online"),
			queryutils.CountIf("LOWER(state)", nil).In("allocated", "busy"),
			queryutils.CountIf("LOWER(state)", nil).In("offline", "gone", "missing"),
			queryutils.CountAll(nil),
		).
		WithWhereClause(fmt.Sprintf(`lab_name = "%s" host_group = "%s" run_target = "%s"`,
			runTargetLabNameHostGroup.labName,
			runTargetLabNameHostGroup.hostGroup,
			runTargetLabNameHostGroup.runTarget,
		), nil)
	if err != nil {
		logging.Errorf(ctx, "failed to create query builder for android devices: %v", err)
		return fleetconsolerpc.RepairMetric_MISSING_DATA, -1, -1, -1
	}

	q, err := qBuilder.Build(nil)
	if err != nil {
		logging.Errorf(ctx, "failed to build query for android devices: %v", err)
		return fleetconsolerpc.RepairMetric_MISSING_DATA, -1, -1, -1
	}

	var gone, fastboot, unavailable, available, allocated, offline, total float64

	err = tx.QueryRowContext(ctx, q.Statement, q.Parameters...).
		Scan(&gone, &fastboot, &unavailable, &available, &allocated, &offline, &total)
	if err != nil {
		logging.Errorf(ctx, "failed to get android devices: %v", err)
		return fleetconsolerpc.RepairMetric_MISSING_DATA, -1, -1, -1
	}

	var maxDevicesAllocatedIn24Hours float64
	err = tx.QueryRowContext(ctx, `
		SELECT MAX(max_devices_allocated)
		FROM android_daily_max_devices
		WHERE
			lab_name = $1 AND
			host_group = $2 AND
			run_target = $3 AND
			period_start >= NOW() - INTERVAL '14 days'
		`,
		runTargetLabNameHostGroup.labName, runTargetLabNameHostGroup.hostGroup, runTargetLabNameHostGroup.runTarget,
	).Scan(&maxDevicesAllocatedIn24Hours)
	if err != nil {
		logging.Errorf(ctx, "failed to get 14 days android_daily_max_devices: %v", err)
		return fleetconsolerpc.RepairMetric_MISSING_DATA, -1, -1, -1
	}

	if allocated >= maxDevicesAllocatedIn24Hours {
		logging.Errorf(ctx, "Something has gone wrong, allocated should never be greater than maxDevicesAllocatedIn24Hours")
	}

	priority := calculatePriority(offline, total, maxDevicesAllocatedIn24Hours)

	// Minimum number of repairs to have 8% of devices not offline
	minimumRepairs := int(math.Ceil((offline/total - 0.08) * total))
	if minimumRepairs < 0 {
		minimumRepairs = 0
	}

	return priority, minimumRepairs, int(offline), int(total)
}

// Calculates the priority level used by flops to decide what should be fixed first.
// The specific formulas and numbers are used to replicate the functionality of go/las-explore
func calculatePriority(offline float64, total float64, maxDevicesAllocatedIn24Hours float64) fleetconsolerpc.RepairMetric_Priority {
	if offline/total >= 0.1 && offline/total+maxDevicesAllocatedIn24Hours/total >= 0.8 {
		return fleetconsolerpc.RepairMetric_BREACHED
	} else if offline/total > 0 {
		return fleetconsolerpc.RepairMetric_WATCH
	} else if offline/total == 1 && maxDevicesAllocatedIn24Hours/total < 0.2 {
		return fleetconsolerpc.RepairMetric_DEVICES_REMOVED
	} else {
		return fleetconsolerpc.RepairMetric_NICE
	}
}
