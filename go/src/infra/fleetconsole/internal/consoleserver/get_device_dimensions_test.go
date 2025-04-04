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
	"google.golang.org/protobuf/types/known/emptypb"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/sqldb"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/testutils"
)

func TestGetDeviceDimensions(t *testing.T) {
	t.Parallel()

	ctx := auth.WithState(context.Background(), &authtest.FakeState{
		FakeDB: authtest.NewFakeDB(),
	})
	consoleFrontend := NewFleetConsoleFrontend().(*FleetConsoleFrontend)
	SetCloudProject(consoleFrontend, "test")

	t.Run("GetDeviceDimensions", func(t *testing.T) {
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

		mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT labels
				FROM "Devices"
				WHERE (realm IN ($1) OR realm IS NULL) ;`)).WithArgs("").WillReturnRows(
			testutils.MockSQLRowsFromCSVForCol(t, csvFilePath, false, "labels"))

		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT DISTINCT id
			FROM "Devices"
			WHERE (realm IN ($1) OR realm IS NULL) ;`)).WithArgs("").WillReturnRows(
			testutils.MockSQLRowsFromCSVForCol(t, csvFilePath, true, "id"))

		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT DISTINCT dut_id
			FROM "Devices"
			WHERE (realm IN ($1) OR realm IS NULL) ;`)).WithArgs("").WillReturnRows(
			testutils.MockSQLRowsFromCSVForCol(t, csvFilePath, true, "dut_id"))

		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT DISTINCT host
			FROM "Devices"
			WHERE (realm IN ($1) OR realm IS NULL) ;`)).WithArgs("").WillReturnRows(
			testutils.MockSQLRowsFromCSVForCol(t, csvFilePath, true, "host"))

		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT DISTINCT port
			FROM "Devices"
			WHERE (realm IN ($1) OR realm IS NULL) ;`)).WithArgs("").WillReturnRows(
			testutils.MockSQLRowsFromCSVForCol(t, csvFilePath, true, "port"))

		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT DISTINCT type
			FROM "Devices"
			WHERE (realm IN ($1) OR realm IS NULL) ;`)).WithArgs("").WillReturnRows(
			testutils.MockSQLRowsFromCSVForCol(t, csvFilePath, true, "type"))

		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT DISTINCT state
			FROM "Devices"
			WHERE (realm IN ($1) OR realm IS NULL) ;`)).WithArgs("").WillReturnRows(
			testutils.MockSQLRowsFromCSVForCol(t, csvFilePath, true, "state"))

		response, err := consoleFrontend.GetDeviceDimensions(ctx, &emptypb.Empty{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, response, should.Match(&fleetconsolerpc.GetDeviceDimensionsResponse{
			BaseDimensions: map[string]*fleetconsolerpc.LabelValues{
				"id": {
					Values: []string{"device1", "device2", "device5"},
				},
				"dut_id": {
					Values: []string{"C111111", "C222222", "C555555"},
				},
				"host": {
					Values: []string{"1.1.1.1"},
				},
				"port": {
					Values: []string{"8888"},
				},
				"type": {
					Values: []string{},
				},
				"state": {
					Values: []string{"DEVICE_STATE_AVAILABLE", "DEVICE_STATE_LEASED"},
				},
			},
			Labels: map[string]*fleetconsolerpc.LabelValues{
				"model_name": {
					Values: []string{"Chromebook Plus", "LaptopPro X", "Random model"},
				},
				"storage_type": {
					Values: []string{"HDD", "SSD"},
				},
				"os_version": {
					Values: []string{"macOS Ventura"},
				},
				"memory_gb": {
					Values: []string{"8"},
				},
				"graphics_card": {
					Values: []string{"NVIDIA GeForce RTX 3060"},
				},
				"wifi_generation": {
					Values: []string{"Wi-Fi 5"},
				},
			},
		}))
	})
}
