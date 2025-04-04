// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"database/sql/driver"
	"io"
	"os"
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

func TestExportDevicesToCSV(t *testing.T) {
	t.Parallel()

	ctx := auth.WithState(context.Background(), &authtest.FakeState{
		FakeDB: authtest.NewFakeDB(),
	})
	consoleFrontend := NewFleetConsoleFrontend().(*FleetConsoleFrontend)
	SetCloudProject(consoleFrontend, "test")

	columnsToExport := []*fleetconsolerpc.Column{
		{
			Name:        "id",
			DisplayName: "ID",
		},
		{
			Name:        "state",
			DisplayName: "Lease state",
		},
		{
			Name:        "model_name",
			DisplayName: "Model name",
		},
		{
			Name:        "storage_type",
			DisplayName: "Storage type",
		},
	}

	testCases := []struct {
		name                string
		ids                 []string
		columns             []*fleetconsolerpc.Column
		expectedQuery       string
		expectedArgs        []driver.Value
		expectedCSVFileName string
	}{
		{
			name:    "ExportDevicesToCSV: export all",
			ids:     nil,
			columns: columnsToExport,
			expectedQuery: regexp.QuoteMeta(`
				SELECT id, dut_id, host, port, type, state, labels
				FROM "Devices"
				WHERE (realm IN ($1) OR realm IS NULL)
				ORDER BY id ;`),
			expectedArgs:        []driver.Value{""},
			expectedCSVFileName: "export_all_csv_response.csv",
		},
		{
			name:    "ExportDevicesToCSV: export selected",
			ids:     []string{"device2"},
			columns: columnsToExport,
			expectedQuery: regexp.QuoteMeta(`
				SELECT id, dut_id, host, port, type, state, labels
				FROM "Devices"
				WHERE (id IN ($1)) AND (realm IN ($2) OR realm IS NULL)
				ORDER BY id ;`),
			expectedArgs:        []driver.Value{"device2", ""},
			expectedCSVFileName: "export_selected_csv_response.csv",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
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

			testDataDir := "testdata"
			csvFilePath := filepath.Join(testDataDir, "devices.csv")
			rows := testutils.MockSQLRowsFromCSV(t, csvFilePath, tt.ids)

			mock.ExpectQuery(tt.expectedQuery).WithArgs(tt.expectedArgs...).WillReturnRows(rows)

			expectedCSVFile, err := os.Open(filepath.Join(testDataDir, tt.expectedCSVFileName))
			if err != nil {
				t.Fatalf("failed to open CSV file: %v", err)
			}
			defer expectedCSVFile.Close()

			expectedCSVData, err := io.ReadAll(expectedCSVFile)
			if err != nil {
				t.Fatalf("failed to read the CSV: %v", err)
			}

			response, err := consoleFrontend.ExportDevicesToCSV(ctx, &fleetconsolerpc.ExportDevicesToCSVRequest{
				Columns: tt.columns,
				Ids:     tt.ids,
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.Match(&fleetconsolerpc.ExportDevicesToCSVResponse{
				CsvData: string(expectedCSVData),
			}))
		})
	}
}
