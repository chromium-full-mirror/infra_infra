// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateandroiddevices

import (
	"context"
	"database/sql"
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
)

func updateLatestDailyMaxTable(ctx context.Context, tx *sql.Tx, runTargetLabNameHostGroup runTargetsLabNamesHostGroups) (err error) {
	currentAllocatedCount, err := getCurrentAllocatedCount(ctx, tx, runTargetLabNameHostGroup)
	if err != nil {
		return err
	}

	lastestDailyMax, err := getLastestDailyMax(ctx, tx, runTargetLabNameHostGroup, currentAllocatedCount)
	if err != nil {
		return err
	}

	if lastestDailyMax.MaxDevicesAllocated < currentAllocatedCount {
		lastestDailyMax.MaxDevicesAllocated = currentAllocatedCount
	}

	if lastestDailyMax.PeriodStart.Before(time.Now().UTC().Add(-24 * time.Hour)) {
		// EXCLUDING id and period_start so we create a new row
		_, err = tx.ExecContext(ctx, `
				INSERT INTO android_daily_max_devices (
					lab_name,
					host_group,
					run_target,
					max_devices_allocated
				) VALUES ($1, $2, $3, $4)
				`,
			lastestDailyMax.LabName,
			lastestDailyMax.HostGroup,
			lastestDailyMax.RunTarget,
			lastestDailyMax.MaxDevicesAllocated,
		)
		if err != nil {
			return errors.Annotate(err, "failed to update android daily max devices with a new row").Err()
		}

		_, err = tx.ExecContext(ctx, `
				DELETE FROM android_daily_max_devices
				WHERE
					lab_name = $1 AND
					host_group = $2 AND
					run_target = $3 AND
					period_start < NOW() - INTERVAL '14 day'
				`,
			runTargetLabNameHostGroup.labName, runTargetLabNameHostGroup.hostGroup, runTargetLabNameHostGroup.runTarget)
		if err != nil {
			logging.Errorf(ctx, "failed to delete old android daily max devices: %v", err)
		}
	} else {
		// INCLUDING id and period_start to update the existing row
		_, err = tx.ExecContext(ctx, `
				INSERT INTO android_daily_max_devices (
				 	id,
					lab_name,
					host_group,
					run_target,
					max_devices_allocated,
					period_start
				) VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (id) DO UPDATE SET
					max_devices_allocated=EXCLUDED.max_devices_allocated
				`,
			lastestDailyMax.ID,
			lastestDailyMax.LabName,
			lastestDailyMax.HostGroup,
			lastestDailyMax.RunTarget,
			lastestDailyMax.MaxDevicesAllocated,
			lastestDailyMax.PeriodStart,
		)
		if err != nil {
			return errors.Annotate(err, "failed to update android daily max devices").Err()
		}
	}

	return nil
}

func getCurrentAllocatedCount(ctx context.Context, tx *sql.Tx, runTargetLabNameHostGroup runTargetsLabNamesHostGroups) (int, error) {
	currentAllocatedCount := 0
	err := tx.QueryRowContext(ctx, `
			SELECT count(*) FROM android_devices
			WHERE
				lab_name = $1 AND
				host_group = $2 AND
				run_target = $3 AND
				LOWER(state) = 'allocated'
			`,
		runTargetLabNameHostGroup.labName, runTargetLabNameHostGroup.hostGroup, runTargetLabNameHostGroup.runTarget,
	).Scan(&currentAllocatedCount)
	if err != nil {
		return 0, errors.Annotate(err, "failed to get current allocated count").Err()
	}

	return currentAllocatedCount, nil
}

func getLastestDailyMax(ctx context.Context, tx *sql.Tx, runTargetLabNameHostGroup runTargetsLabNamesHostGroups, currentAllocatedCount int) (lastestDailyMax *androidDailyMaxDevice, err error) {
	lastestDailyMax = &androidDailyMaxDevice{}
	err = tx.QueryRowContext(ctx, `
			SELECT
				id,
				lab_name,
				host_group,
				run_target,
				max_devices_allocated,
				period_start
			FROM android_daily_max_devices
			WHERE
				lab_name = $1 AND
				host_group = $2 AND
				run_target = $3
			ORDER BY period_start DESC
			LIMIT 1
			`,
		runTargetLabNameHostGroup.labName, runTargetLabNameHostGroup.hostGroup, runTargetLabNameHostGroup.runTarget,
	).Scan(&lastestDailyMax.ID, &lastestDailyMax.LabName, &lastestDailyMax.HostGroup, &lastestDailyMax.RunTarget, &lastestDailyMax.MaxDevicesAllocated, &lastestDailyMax.PeriodStart)

	if err != nil && errors.Is(err, sql.ErrNoRows) {
		lastestDailyMax = &androidDailyMaxDevice{
			LabName: sql.NullString{
				String: runTargetLabNameHostGroup.labName,
				Valid:  true,
			},
			HostGroup: sql.NullString{
				String: runTargetLabNameHostGroup.hostGroup,
				Valid:  true,
			},
			RunTarget: sql.NullString{
				String: runTargetLabNameHostGroup.runTarget,
				Valid:  true,
			},
			MaxDevicesAllocated: currentAllocatedCount,
			PeriodStart:         time.Time{},
		}
	} else if err != nil {
		return nil, errors.Annotate(err, "failed to get lastest daily max").Err()
	}

	return lastestDailyMax, nil
}

type androidDailyMaxDevice struct {
	ID                  int64
	LabName             sql.NullString
	HostGroup           sql.NullString
	RunTarget           sql.NullString
	MaxDevicesAllocated int
	PeriodStart         time.Time
}
