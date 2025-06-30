// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateandroiddevices

import (
	"context"
	"regexp"
	"strconv"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
)

func TestCalculateSlo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mockTarget := runTargetsLabNamesHostGroups{
		runTarget: "test-target",
		labName:   "test-lab",
		hostGroup: "test-group",
	}

	deviceCountsCols := []string{"gone", "fastboot", "unavailable", "available", "allocated", "offline", "total"}
	maxAllocatedCols := []string{"max"}

	t.Run("Nice SLO", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.Loosely(t, err, should.BeNil)

		mock.ExpectBegin()
		// 1 offline, 100 total, 10 max allocated
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT
			SUM(CASE WHEN LOWER(state) IN ('gone', 'offline') THEN 1 ELSE 0 END) AS v0,
			SUM(CASE WHEN LOWER(state) IN ('fastboot') THEN 1 ELSE 0 END) AS v1,
			SUM(CASE WHEN LOWER(state) IN ('unavailable', 'unknown') THEN 1 ELSE 0 END) AS v2,
			SUM(CASE WHEN LOWER(state) IN ('available', 'idle', 'online') THEN 1 ELSE 0 END) AS v3,
			SUM(CASE WHEN LOWER(state) IN ('allocated', 'busy') THEN 1 ELSE 0 END) AS v4,
			SUM(CASE WHEN LOWER(state) IN ('offline', 'gone', 'missing') THEN 1 ELSE 0 END) AS v5,
			COUNT(*) AS v6
		FROM "android_devices"
		WHERE
			((lab_name = $1) AND
			(host_group = $2) AND
			(run_target = $3))
		;
		`)).WithArgs(mockTarget.labName, mockTarget.hostGroup, mockTarget.runTarget).
			WillReturnRows(sqlmock.NewRows(deviceCountsCols).AddRow(0, 0, 0, 90, 9, 0, 100))
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT MAX(max_devices_allocated)
		FROM android_daily_max_devices
		WHERE
			lab_name = $1 AND
			host_group = $2 AND
			run_target = $3 AND
			period_start >= NOW() - INTERVAL '14 days'
		`)).WithArgs(mockTarget.labName, mockTarget.hostGroup, mockTarget.runTarget).
			WillReturnRows(sqlmock.NewRows(maxAllocatedCols).AddRow(10))

		tx, err := db.BeginTx(ctx, nil)
		assert.Loosely(t, err, should.BeNil)

		slo, minRepairs, offline, total := calculateSlo(ctx, tx, mockTarget)

		assert.Loosely(t, slo, should.Equal(fleetconsolerpc.RepairMetric_NICE))
		assert.Loosely(t, minRepairs, should.Equal(0))
		assert.Loosely(t, offline, should.Equal(0))
		assert.Loosely(t, total, should.Equal(100))
		assert.Loosely(t, mock.ExpectationsWereMet(), should.BeNil)
	})

	t.Run("Watch SLO", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.Loosely(t, err, should.BeNil)

		mock.ExpectBegin()
		// 9 offline, 100 total, 85 max allocated
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT
			SUM(CASE WHEN LOWER(state) IN ('gone', 'offline') THEN 1 ELSE 0 END) AS v0,
			SUM(CASE WHEN LOWER(state) IN ('fastboot') THEN 1 ELSE 0 END) AS v1,
			SUM(CASE WHEN LOWER(state) IN ('unavailable', 'unknown') THEN 1 ELSE 0 END) AS v2,
			SUM(CASE WHEN LOWER(state) IN ('available', 'idle', 'online') THEN 1 ELSE 0 END) AS v3,
			SUM(CASE WHEN LOWER(state) IN ('allocated', 'busy') THEN 1 ELSE 0 END) AS v4,
			SUM(CASE WHEN LOWER(state) IN ('offline', 'gone', 'missing') THEN 1 ELSE 0 END) AS v5,
			COUNT(*) AS v6
		FROM "android_devices"
		WHERE
			((lab_name = $1) AND
			(host_group = $2) AND
			(run_target = $3))
		;
		`)).WithArgs(mockTarget.labName, mockTarget.hostGroup, mockTarget.runTarget).
			WillReturnRows(sqlmock.NewRows(deviceCountsCols).AddRow(12, 0, 0, 10, 21, 11, 100))
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT MAX(max_devices_allocated)
		FROM android_daily_max_devices
		WHERE
			lab_name = $1 AND
			host_group = $2 AND
			run_target = $3 AND
			period_start >= NOW() - INTERVAL '14 days'
		`)).WithArgs(mockTarget.labName, mockTarget.hostGroup, mockTarget.runTarget).
			WillReturnRows(sqlmock.NewRows(maxAllocatedCols).AddRow(25))

		tx, _ := db.BeginTx(ctx, nil)
		slo, minRepairs, offline, total := calculateSlo(ctx, tx, mockTarget)

		assert.Loosely(t, slo, should.Equal(fleetconsolerpc.RepairMetric_WATCH))
		assert.Loosely(t, minRepairs, should.Equal(3))
		assert.Loosely(t, offline, should.Equal(11))
		assert.Loosely(t, total, should.Equal(100))
		assert.Loosely(t, mock.ExpectationsWereMet(), should.BeNil)
	})

	t.Run("Breached SLO", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.Loosely(t, err, should.BeNil)

		mock.ExpectBegin()
		// 11 offline, 100 total, 80 max allocated
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT
			SUM(CASE WHEN LOWER(state) IN ('gone', 'offline') THEN 1 ELSE 0 END) AS v0,
			SUM(CASE WHEN LOWER(state) IN ('fastboot') THEN 1 ELSE 0 END) AS v1,
			SUM(CASE WHEN LOWER(state) IN ('unavailable', 'unknown') THEN 1 ELSE 0 END) AS v2,
			SUM(CASE WHEN LOWER(state) IN ('available', 'idle', 'online') THEN 1 ELSE 0 END) AS v3,
			SUM(CASE WHEN LOWER(state) IN ('allocated', 'busy') THEN 1 ELSE 0 END) AS v4,
			SUM(CASE WHEN LOWER(state) IN ('offline', 'gone', 'missing') THEN 1 ELSE 0 END) AS v5,
			COUNT(*) AS v6
		FROM "android_devices"
		WHERE
			((lab_name = $1) AND
			(host_group = $2) AND
			(run_target = $3))
		;
		`)).WithArgs(mockTarget.labName, mockTarget.hostGroup, mockTarget.runTarget).
			WillReturnRows(sqlmock.NewRows(deviceCountsCols).AddRow(11, 0, 0, 80, 79, 11, 100))
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT MAX(max_devices_allocated)
		FROM android_daily_max_devices
		WHERE
			lab_name = $1 AND
			host_group = $2 AND
			run_target = $3 AND
			period_start >= NOW() - INTERVAL '14 days'
		`)).WithArgs(mockTarget.labName, mockTarget.hostGroup, mockTarget.runTarget).
			WillReturnRows(sqlmock.NewRows(maxAllocatedCols).AddRow(80))

		tx, _ := db.BeginTx(ctx, nil)
		slo, minRepairs, offline, total := calculateSlo(ctx, tx, mockTarget)

		assert.Loosely(t, slo, should.Equal(fleetconsolerpc.RepairMetric_BREACHED))
		assert.Loosely(t, minRepairs, should.Equal(3))
		assert.Loosely(t, offline, should.Equal(11))
		assert.Loosely(t, total, should.Equal(100))
		assert.Loosely(t, mock.ExpectationsWereMet(), should.BeNil)
	})

	t.Run("Zero Total Devices", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.Loosely(t, err, should.BeNil)

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT
			SUM(CASE WHEN LOWER(state) IN ('gone', 'offline') THEN 1 ELSE 0 END) AS v0,
			SUM(CASE WHEN LOWER(state) IN ('fastboot') THEN 1 ELSE 0 END) AS v1,
			SUM(CASE WHEN LOWER(state) IN ('unavailable', 'unknown') THEN 1 ELSE 0 END) AS v2,
			SUM(CASE WHEN LOWER(state) IN ('available', 'idle', 'online') THEN 1 ELSE 0 END) AS v3,
			SUM(CASE WHEN LOWER(state) IN ('allocated', 'busy') THEN 1 ELSE 0 END) AS v4,
			SUM(CASE WHEN LOWER(state) IN ('offline', 'gone', 'missing') THEN 1 ELSE 0 END) AS v5,
			COUNT(*) AS v6
		FROM "android_devices"
		WHERE
			((lab_name = $1) AND
			(host_group = $2) AND
			(run_target = $3))
		;
		`)).WithArgs(mockTarget.labName, mockTarget.hostGroup, mockTarget.runTarget).
			WillReturnRows(sqlmock.NewRows(deviceCountsCols).AddRow(0, 0, 0, 0, 0, 0, 0))
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT MAX(max_devices_allocated)
		FROM android_daily_max_devices
		WHERE
			lab_name = $1 AND
			host_group = $2 AND
			run_target = $3 AND
			period_start >= NOW() - INTERVAL '14 days'
		`)).WithArgs(mockTarget.labName, mockTarget.hostGroup, mockTarget.runTarget).
			WillReturnRows(sqlmock.NewRows(maxAllocatedCols).AddRow(0))

		tx, _ := db.BeginTx(ctx, nil)
		slo, minRepairs, offline, total := calculateSlo(ctx, tx, mockTarget)

		assert.Loosely(t, slo, should.Equal(fleetconsolerpc.RepairMetric_NICE))
		assert.Loosely(t, minRepairs, should.Equal(0))
		assert.Loosely(t, offline, should.Equal(0))
		assert.Loosely(t, total, should.Equal(0))
		assert.Loosely(t, mock.ExpectationsWereMet(), should.BeNil)
	})
}

func TestUpdateAndroidRepairMetricsTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("UpdateAndroidRepairMetricsTable", t, func(t *ftt.Test) {
		db, mock, err := sqlmock.New()
		assert.Loosely(t, err, should.BeNil)
		defer db.Close()

		mockTarget := runTargetsLabNamesHostGroups{
			runTarget: "test-target",
			labName:   "test-lab",
			hostGroup: "test-group",
		}

		mock.ExpectBegin()

		expectedSLO := fleetconsolerpc.RepairMetric_WATCH
		expectedMinimumRepairs := 0
		expectedDevicesOffline := 10
		expectedTotalDevices := 100

		mock.ExpectExec(regexp.QuoteMeta(`
			INSERT INTO "android_repair_metrics" (
				priority,
				lab_name,
				host_group,
				run_target,
				minimum_repairs,
				devices_offline,
				total_devices
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (lab_name, host_group, run_target) DO UPDATE SET
				priority = EXCLUDED.priority,
				lab_name = EXCLUDED.lab_name,
				host_group = EXCLUDED.host_group,
				run_target = EXCLUDED.run_target,
				minimum_repairs = EXCLUDED.minimum_repairs,
				devices_offline = EXCLUDED.devices_offline,
				total_devices = EXCLUDED.total_devices;
			`)).WithArgs(
			expectedSLO.String(),
			mockTarget.labName,
			mockTarget.hostGroup,
			mockTarget.runTarget,
			strconv.Itoa(expectedMinimumRepairs),
			strconv.Itoa(expectedDevicesOffline),
			strconv.Itoa(expectedTotalDevices),
		).WillReturnResult(sqlmock.NewResult(1, 1))

		tx, err := db.BeginTx(ctx, nil)
		assert.Loosely(t, err, should.BeNil)

		err = updateAndroidRepairMetricsTable(ctx, tx, mockTarget,
			expectedSLO,
			expectedMinimumRepairs,
			expectedDevicesOffline,
			expectedTotalDevices,
		)
		assert.Loosely(t, err, should.BeNil)

		err = mock.ExpectationsWereMet()
		assert.Loosely(t, err, should.BeNil)
	})
}
