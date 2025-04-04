// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/sqldb"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
)

func TestCountDevices(t *testing.T) {
	t.Parallel()

	ctx := auth.WithState(context.Background(), &authtest.FakeState{
		FakeDB: authtest.NewFakeDB(),
	})
	consoleFrontend := NewFleetConsoleFrontend().(*FleetConsoleFrontend)
	SetCloudProject(consoleFrontend, "test")

	t.Run("CountDevices", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
		}
		defer func() {
			mock.ExpectClose()
			err = db.Close()
			if err != nil {
				t.Fatalf("failed to close db: %s", err)
			}
		}()
		ctx = sqldb.UseDB(ctx, db)

		mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					COUNT(*) AS total,
					COUNT(CASE WHEN state = 'DEVICE_STATE_LEASED' THEN 1 ELSE NULL END) AS leased,
					COUNT(CASE WHEN state = 'DEVICE_STATE_AVAILABLE' THEN 1 ELSE NULL END) AS available,
					COUNT(CASE WHEN labels -> 'dut_state' -> 'Values' ? 'ready' THEN 1 ELSE NULL END) AS ready,
					COUNT(CASE WHEN labels -> 'dut_state' -> 'Values' ? 'needs_manual_repair' THEN 1 ELSE NULL END) AS needs_manual_repair,
					COUNT(CASE WHEN labels -> 'dut_state' -> 'Values' ? 'needs_repair' THEN 1 ELSE NULL END) AS needs_repair,
					COUNT(CASE WHEN labels -> 'dut_state' -> 'Values' ? 'repair_failed' THEN 1 ELSE NULL END) AS repair_failed
				FROM "Devices"
				WHERE (realm IN ($1) OR realm IS NULL) ;`)).WithArgs("").WillReturnRows(
			sqlmock.NewRows(
				[]string{"total", "leased", "available", "ready", "needs_manual_repair", "needs_repair", "repair_failed"},
			).AddRow(190, 90, 100, 60, 50, 40, 30))

		response, err := consoleFrontend.CountDevices(ctx, &fleetconsolerpc.CountDevicesRequest{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, response, should.Match(&fleetconsolerpc.CountDevicesResponse{
			Total: 190,
			TaskState: &fleetconsolerpc.TaskStateCounts{
				Busy: 90,
				Idle: 100,
			},
			DeviceState: &fleetconsolerpc.DeviceStateCounts{
				Ready:            60,
				NeedManualRepair: 50,
				NeedRepair:       40,
				RepairFailed:     30,
			},
		}))
	})
}
