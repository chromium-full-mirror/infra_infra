// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/sqldb"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/testutils"
)

func TestListDevices(t *testing.T) {
	t.Parallel()

	ctx := auth.WithState(context.Background(), &authtest.FakeState{
		FakeDB: authtest.NewFakeDB(),
	})
	consoleFrontend := NewFleetConsoleFrontend().(*FleetConsoleFrontend)
	SetCloudProject(consoleFrontend, "test")

	t.Run("ListDevices", func(t *testing.T) {
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

		csvFilePath := filepath.Join("testdata", "devices.csv")
		rows := testutils.MockSQLRowsFromCSV(t, csvFilePath, nil)

		mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT id, dut_id, host, port, type, state, labels
				FROM "Devices"
				WHERE (realm IN ($1) OR realm IS NULL)
				ORDER BY id
				LIMIT 3
				OFFSET 0;`)).WithArgs("").WillReturnRows(rows)

		response, err := consoleFrontend.ListDevices(ctx, &fleetconsolerpc.ListDevicesRequest{
			PageSize: 2,
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, response, should.Match(&fleetconsolerpc.ListDevicesResponse{
			Devices: []*fleetconsolerpc.Device{
				{
					Id:      "device1",
					DutId:   "C111111",
					Address: &fleetconsolerpc.DeviceAddress{},
					State:   fleetconsolerpc.DeviceState_DEVICE_STATE_LEASED,
					DeviceSpec: &fleetconsolerpc.DeviceSpec{
						Labels: map[string]*fleetconsolerpc.LabelValues{
							"model_name": {
								Values: []string{"LaptopPro X"},
							},
							"storage_type": {
								Values: []string{"SSD", "HDD"},
							},
							"os_version": {
								Values: []string{"macOS Ventura"},
							},
						},
					},
				},
				{
					Id:    "device2",
					DutId: "C222222",
					Address: &fleetconsolerpc.DeviceAddress{
						Host: "1.1.1.1",
						Port: 8888,
					},
					State: fleetconsolerpc.DeviceState_DEVICE_STATE_AVAILABLE,
					DeviceSpec: &fleetconsolerpc.DeviceSpec{
						Labels: map[string]*fleetconsolerpc.LabelValues{
							"model_name": {
								Values: []string{"Chromebook Plus"},
							},
							"memory_gb": {
								Values: []string{"8"},
							},
						},
					},
				},
			},
			NextPageToken: "CAISDTMzbmlpaHpqNHV4NDU",
		}))
	})
}
