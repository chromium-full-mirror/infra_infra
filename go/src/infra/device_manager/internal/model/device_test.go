// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/testing/typed"

	"infra/device_manager/internal/database"
)

func TestGetDeviceByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	baseQuery := `
		SELECT
			id,
			dut_id,
			device_address,
			device_type,
			device_state,
			schedulable_labels,
			created_time,
			last_updated_time,
			is_active
		FROM "Devices"`

	timeNow := time.Now()
	validCases := []struct {
		name           string
		idType         DeviceIDType
		expectedDevice Device
		err            error
	}{
		{
			name:   "GetDeviceByID: valid return; search by hostname",
			idType: IDTypeHostname,
			expectedDevice: Device{
				ID:            "test-device-1",
				DutID:         "test-dut-id-1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				DeviceState:   "DEVICE_STATE_AVAILABLE",
				SchedulableLabels: SchedulableLabels{
					"dut_id": LabelValues{
						Values: []string{"test-dut-id-1"},
					},
					"label-test": LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				CreatedTime:     timeNow,
				LastUpdatedTime: timeNow,
				IsActive:        true,
			},
			err: nil,
		},
		{
			name:   "GetDeviceByID: valid return; search by DUT ID",
			idType: IDTypeDutID,
			expectedDevice: Device{
				ID:            "test-device-1",
				DutID:         "test-dut-id-1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				DeviceState:   "DEVICE_STATE_AVAILABLE",
				SchedulableLabels: SchedulableLabels{
					"dut_id": LabelValues{
						Values: []string{"test-dut-id-1"},
					},
					"label-test": LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				CreatedTime:     timeNow,
				LastUpdatedTime: timeNow,
				IsActive:        true,
			},
			err: nil,
		},
	}

	for _, tt := range validCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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

			rows := sqlmock.NewRows([]string{
				"id",
				"dut_id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"created_time",
				"last_updated_time",
				"is_active"}).
				AddRow(
					"test-device-1",
					"test-dut-id-1",
					"1.1.1.1:1",
					"DEVICE_TYPE_PHYSICAL",
					"DEVICE_STATE_AVAILABLE",
					`{"dut_id":{"Values":["test-dut-id-1"]},"label-test":{"Values":["test-value-1"]}}`,
					timeNow,
					timeNow,
					true)

			query := baseQuery
			var idVal string
			switch tt.idType {
			case IDTypeDutID:
				query += `
					WHERE dut_id=$1;`
				idVal = "test-dut-id-1"
			case IDTypeHostname:
				query += `
					WHERE id=$1;`
				idVal = "test-device-1"
			default:
				t.Errorf("unexpected error: id type %s is not supported", tt.idType)
			}

			mock.ExpectQuery(regexp.QuoteMeta(query)).
				WithArgs(idVal).
				WillReturnRows(rows)

			device, err := GetDeviceByID(ctx, db, tt.idType, idVal)
			if !errors.Is(err, tt.err) {
				t.Errorf("unexpected error: %v; want: %v", err, tt.err)
			}
			if diff := typed.Got(device).Want(tt.expectedDevice).Diff(); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}

	failedCases := []struct {
		name           string
		idType         DeviceIDType
		expectedDevice Device
		err            error
	}{
		{
			name:           "invalid request; search by hostname, no device name match",
			idType:         IDTypeHostname,
			expectedDevice: Device{},
			err:            fmt.Errorf("GetDeviceByID: failed to get Device"),
		},
		{
			name:           "invalid request; search by DUT ID, no device name match",
			idType:         IDTypeDutID,
			expectedDevice: Device{},
			err:            fmt.Errorf("GetDeviceByID: failed to get Device"),
		},
	}

	for _, tt := range failedCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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

			query := baseQuery
			switch tt.idType {
			case IDTypeDutID:
				query += `
					WHERE dut_id=$1;`
			case IDTypeHostname:
				query += `
					WHERE id=$1;`
			default:
				t.Errorf("unexpected error: id type %s is not supported", tt.idType)
			}

			mock.ExpectQuery(regexp.QuoteMeta(query)).
				WithArgs("test-device-1").
				WillReturnError(fmt.Errorf("GetDeviceByID: failed to get Device"))

			device, err := GetDeviceByID(ctx, db, tt.idType, "test-device-1")
			if err.Error() != tt.err.Error() {
				t.Errorf("unexpected error: %s", err)
			}
			if diff := typed.Got(device).Want(tt.expectedDevice).Diff(); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}

func TestListDevices(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("ListDevices", t, func(t *ftt.Test) {
		t.Run("ListDevices: valid return; page token returned", func(t *ftt.Test) {
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

			var (
				pageSize = 1
				timeNow  = time.Now()
			)

			createdTime, err := time.Parse("2006-01-02 15:04:05", "2024-01-01 12:00:00")
			assert.Loosely(t, err, should.BeNil)

			rows := sqlmock.NewRows([]string{
				"id",
				"dut_id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"created_time",
				"last_updated_time",
				"is_active"}).
				AddRow(
					"test-device-1",
					"test-dut-id-1",
					"1.1.1.1:1",
					"DEVICE_TYPE_PHYSICAL",
					"DEVICE_STATE_AVAILABLE",
					`{"dut_id":{"Values":["test-dut-id-1"]},"label-test":{"Values":["test-value-1"]}}`,
					createdTime,
					timeNow,
					true).
				AddRow(
					"test-device-2",
					"test-dut-id-2",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"dut_id":{"Values":["test-dut-id-2"]},"label-test":{"Values":["test-value-2"]}}`,
					createdTime,
					timeNow,
					false)

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					dut_id,
					device_address,
					device_type,
					device_state,
					schedulable_labels,
					created_time,
					last_updated_time,
					is_active
				FROM "Devices"
				ORDER BY created_time
				LIMIT $1;`)).
				WithArgs(pageSize + 1).
				WillReturnRows(rows)

			devices, nextPageToken, err := ListDevices(ctx, db, "", pageSize, "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nextPageToken, should.Match(database.PageToken("MjAyNC0wMS0wMVQxMjowMDowMFo=")))
			assert.Loosely(t, devices, should.Match([]Device{
				{
					ID:            "test-device-1",
					DutID:         "test-dut-id-1",
					DeviceAddress: "1.1.1.1:1",
					DeviceType:    "DEVICE_TYPE_PHYSICAL",
					DeviceState:   "DEVICE_STATE_AVAILABLE",
					SchedulableLabels: SchedulableLabels{
						"dut_id": LabelValues{
							Values: []string{"test-dut-id-1"},
						},
						"label-test": LabelValues{
							Values: []string{"test-value-1"},
						},
					},
					CreatedTime:     createdTime,
					LastUpdatedTime: timeNow,
					IsActive:        true,
				},
			}))

			decodedToken, err := database.DecodePageToken(ctx, database.PageToken(nextPageToken))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, decodedToken, should.Match(createdTime.Format(time.RFC3339Nano)))
		})
		t.Run("ListDevices: valid return; no page token returned", func(t *ftt.Test) {
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

			var (
				pageSize = 2
				timeNow  = time.Now()
			)

			createdTime, err := time.Parse("2006-01-02 15:04:05", "2024-01-01 12:00:00")
			assert.Loosely(t, err, should.BeNil)

			rows := sqlmock.NewRows([]string{
				"id",
				"dut_id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"created_time",
				"last_updated_time",
				"is_active"}).
				AddRow(
					"test-device-1",
					"test-dut-id-1",
					"1.1.1.1:1",
					"DEVICE_TYPE_PHYSICAL",
					"DEVICE_STATE_AVAILABLE",
					`{"dut_id":{"Values":["test-dut-id-1"]},"label-test":{"Values":["test-value-1"]}}`,
					createdTime,
					timeNow,
					true).
				AddRow(
					"test-device-2",
					"test-dut-id-2",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"dut_id":{"Values":["test-dut-id-2"]},"label-test":{"Values":["test-value-2"]}}`,
					createdTime,
					timeNow,
					false)

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					dut_id,
					device_address,
					device_type,
					device_state,
					schedulable_labels,
					created_time,
					last_updated_time,
					is_active
				FROM "Devices"
				ORDER BY created_time
				LIMIT $1;`)).
				WithArgs(pageSize + 1).
				WillReturnRows(rows)

			devices, nextPageToken, err := ListDevices(ctx, db, "", pageSize, "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nextPageToken, should.Equal(database.PageToken("")))
			assert.Loosely(t, devices, should.Match([]Device{
				{
					ID:            "test-device-1",
					DutID:         "test-dut-id-1",
					DeviceAddress: "1.1.1.1:1",
					DeviceType:    "DEVICE_TYPE_PHYSICAL",
					DeviceState:   "DEVICE_STATE_AVAILABLE",
					SchedulableLabels: SchedulableLabels{
						"dut_id": LabelValues{
							Values: []string{"test-dut-id-1"},
						},
						"label-test": LabelValues{
							Values: []string{"test-value-1"},
						},
					},
					CreatedTime:     createdTime,
					LastUpdatedTime: timeNow,
					IsActive:        true,
				},
				{
					ID:            "test-device-2",
					DutID:         "test-dut-id-2",
					DeviceAddress: "2.2.2.2:2",
					DeviceType:    "DEVICE_TYPE_VIRTUAL",
					DeviceState:   "DEVICE_STATE_LEASED",
					SchedulableLabels: SchedulableLabels{
						"dut_id": LabelValues{
							Values: []string{"test-dut-id-2"},
						},
						"label-test": LabelValues{
							Values: []string{"test-value-2"},
						},
					},
					CreatedTime:     createdTime,
					LastUpdatedTime: timeNow,
					IsActive:        false,
				},
			}))
		})
		t.Run("ListDevices: valid request using page token", func(t *ftt.Test) {
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

			var (
				pageSize  = 1
				pageToken = "MjAyNC0wMS0wMVQxMjowMDowMFo="
				timeNow   = time.Now()
			)

			createdTime, err := time.Parse("2006-01-02 15:04:05", "2024-01-01 12:00:00")
			assert.Loosely(t, err, should.BeNil)

			// only add rows after test-device-1
			rows := sqlmock.NewRows([]string{
				"id",
				"dut_id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"created_time",
				"last_updated_time",
				"is_active"}).
				AddRow(
					"test-device-2",
					"test-dut-id-2",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"dut_id":{"Values":["test-dut-id-2"]},"label-test":{"Values":["test-value-2"]}}`,
					createdTime,
					timeNow,
					false)

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					dut_id,
					device_address,
					device_type,
					device_state,
					schedulable_labels,
					created_time,
					last_updated_time,
					is_active
				FROM "Devices"
				WHERE created_time > $1
				ORDER BY created_time
				LIMIT $2;`)).
				WithArgs(createdTime.Format(time.RFC3339Nano), pageSize+1).
				WillReturnRows(rows)

			devices, nextPageToken, err := ListDevices(ctx, db, database.PageToken(pageToken), pageSize, "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nextPageToken, should.Equal(database.PageToken("")))
			assert.Loosely(t, devices, should.Match([]Device{
				{
					ID:            "test-device-2",
					DutID:         "test-dut-id-2",
					DeviceAddress: "2.2.2.2:2",
					DeviceType:    "DEVICE_TYPE_VIRTUAL",
					DeviceState:   "DEVICE_STATE_LEASED",
					SchedulableLabels: SchedulableLabels{
						"dut_id": LabelValues{
							Values: []string{"test-dut-id-2"},
						},
						"label-test": LabelValues{
							Values: []string{"test-value-2"},
						},
					},
					CreatedTime:     createdTime,
					LastUpdatedTime: timeNow,
					IsActive:        false,
				},
			}))
		})
	})
}

func Test_buildListDevicesQuery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name       string
		pageToken  database.PageToken
		pageSize   int
		filter     string
		wantQuery  string
		wantArgs   []interface{}
		wantErrStr string
	}{
		{
			name:      "empty filter, page token",
			pageToken: "",
			pageSize:  10,
			filter:    "",
			wantQuery: `
		SELECT
			id,
			dut_id,
			device_address,
			device_type,
			device_state,
			schedulable_labels,
			created_time,
			last_updated_time,
			is_active
		FROM "Devices"
		ORDER BY created_time
		LIMIT $1;`,
			wantArgs:   []interface{}{11},
			wantErrStr: "",
		},
		{
			name:       "bad page token",
			pageToken:  "1",
			pageSize:   10,
			filter:     "is_active = true",
			wantQuery:  "",
			wantArgs:   nil,
			wantErrStr: "DecodePageToken: illegal base64 data",
		},
		{
			name:      "valid request",
			pageToken: "MjAyNC0wNS0xNVQyMTo0NzoyMy44MjIyODNa",
			pageSize:  10,
			filter:    "is_active = true",
			wantQuery: `
		SELECT
			id,
			dut_id,
			device_address,
			device_type,
			device_state,
			schedulable_labels,
			created_time,
			last_updated_time,
			is_active
		FROM "Devices"
		WHERE created_time > $1 AND is_active = $2
		ORDER BY created_time
		LIMIT $3;`,
			wantArgs:   []interface{}{"2024-05-15T21:47:23.822283Z", "true", 11},
			wantErrStr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotQuery, gotArgs, gotErr := buildListDevicesQuery(ctx, tt.pageToken, tt.pageSize, tt.filter)

			if gotErr != nil && tt.wantErrStr == "" {
				t.Errorf("buildListDevicesQuery() gotErr = %v, want %v", gotErr, tt.wantErrStr)
			}
			if gotErr != nil && !strings.Contains(gotErr.Error(), tt.wantErrStr) {
				t.Errorf("buildListDevicesQuery() gotErr = %v, want %v", gotErr, tt.wantErrStr)
			}
			if gotQuery != tt.wantQuery {
				t.Errorf("buildListDevicesQuery() gotQuery = %v, want %v", gotQuery, tt.wantQuery)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Errorf("buildListDevicesQuery() gotArgs = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}

func TestUpdateDeviceToAvailable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("UpdateDeviceToAvailable", t, func(t *ftt.Test) {
		t.Run("UpdateDeviceToAvailable: valid update", func(t *ftt.Test) {
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

			mock.ExpectBegin()

			var txOpts *sql.TxOptions
			tx, err := db.BeginTx(ctx, txOpts)
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub db transaction", err)
			}

			timeNow := time.Now()
			rows := sqlmock.NewRows([]string{
				"id",
				"dut_id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"is_active",
				"created_time",
				"last_updated_time",
				"last_notification_time"}).
				AddRow(
					"test-device-1",
					"test-dut-id",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"dut_id":{"Values":["test-dut-id"]},"label-test":{"Values":["test-value-1"]}}`,
					false,
					timeNow,
					timeNow,
					timeNow)

			labelBytes, err := json.Marshal(SchedulableLabels{
				"dut_id": LabelValues{
					Values: []string{"test-dut-id"},
				},
				"label-test": LabelValues{
					Values: []string{"test-value-1"},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, string(labelBytes), should.Match(`{"dut_id":{"Values":["test-dut-id"]},"label-test":{"Values":["test-value-1"]}}`))

			mock.ExpectQuery(regexp.QuoteMeta(`
				UPDATE
					"Devices"
				SET
					device_state='DEVICE_STATE_AVAILABLE',
					schedulable_labels=COALESCE($3::jsonb, schedulable_labels),
					is_active=COALESCE($4, is_active),
					last_updated_time=NOW()
				WHERE
					id=$1
					AND dut_id=$2
				RETURNING
					id,
					dut_id,
					device_address,
					device_type,
					device_state,
					schedulable_labels,
					is_active,
					created_time,
					last_updated_time,
					last_notification_time;`)).
				WithArgs(
					"test-device-1",
					"test-dut-id",
					labelBytes,
					false).
				WillReturnRows(rows)

			updatedDevice, err := UpdateDeviceToAvailable(ctx, tx, Device{
				ID:            "test-device-1",
				DutID:         "test-dut-id-no-change", // this should not change DUT ID
				DeviceAddress: "2.2.2.2:2",
				DeviceType:    "DEVICE_TYPE_VIRTUAL",
				DeviceState:   "DEVICE_STATE_LEASED",
				SchedulableLabels: SchedulableLabels{
					"dut_id": LabelValues{
						Values: []string{"test-dut-id"},
					},
					"label-test": LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: false,
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, *updatedDevice, should.Match(Device{
				ID:            "test-device-1",
				DutID:         "test-dut-id",
				DeviceAddress: "2.2.2.2:2",
				DeviceType:    "DEVICE_TYPE_VIRTUAL",
				DeviceState:   "DEVICE_STATE_LEASED",
				SchedulableLabels: SchedulableLabels{
					"dut_id": LabelValues{
						Values: []string{"test-dut-id"},
					},
					"label-test": LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive:             false,
				CreatedTime:          timeNow,
				LastUpdatedTime:      timeNow,
				LastNotificationTime: timeNow,
			}))
		})
	})
}

func TestUpdateDeviceToLeased(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("UpdateDeviceToLeased", t, func(t *ftt.Test) {
		t.Run("UpdateDeviceToLeased: valid update using DUT ID", func(t *ftt.Test) {
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

			mock.ExpectBegin()

			var txOpts *sql.TxOptions
			tx, err := db.BeginTx(ctx, txOpts)
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub db transaction", err)
			}

			timeNow := time.Now()
			rows := sqlmock.NewRows([]string{
				"id",
				"dut_id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"is_active",
				"created_time",
				"last_updated_time",
				"last_notification_time"}).
				AddRow(
					"test-device-1",
					"test-dut-id",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"dut_id":{"Values":["test-dut-id"]},"label-test":{"Values":["test-value-1"]}}`,
					false,
					timeNow,
					timeNow,
					timeNow)

			labelBytes, err := json.Marshal(SchedulableLabels{
				"dut_id": LabelValues{
					Values: []string{"test-dut-id"},
				},
				"label-test": LabelValues{
					Values: []string{"test-value-1"},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, string(labelBytes), should.Match(`{"dut_id":{"Values":["test-dut-id"]},"label-test":{"Values":["test-value-1"]}}`))

			mock.ExpectQuery(regexp.QuoteMeta(`
				UPDATE
					"Devices"
				SET
					device_state='DEVICE_STATE_LEASED',
					last_updated_time=NOW()
				WHERE
					dut_id=$1
					AND device_state='DEVICE_STATE_AVAILABLE'
					AND is_active=TRUE
				RETURNING
					id,
					dut_id,
					device_address,
					device_type,
					device_state,
					schedulable_labels,
					is_active,
					created_time,
					last_updated_time,
					last_notification_time;`)).
				WithArgs(
					"test-dut-id").
				WillReturnRows(rows)

			updatedDevice, err := UpdateDeviceToLeased(ctx, tx, Device{
				ID: "test-dut-id",
			}, IDTypeDutID)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, updatedDevice, should.Match(Device{
				ID:            "test-device-1",
				DutID:         "test-dut-id",
				DeviceAddress: "2.2.2.2:2",
				DeviceType:    "DEVICE_TYPE_VIRTUAL",
				DeviceState:   "DEVICE_STATE_LEASED",
				SchedulableLabels: SchedulableLabels{
					"dut_id": LabelValues{
						Values: []string{"test-dut-id"},
					},
					"label-test": LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive:             false,
				CreatedTime:          timeNow,
				LastUpdatedTime:      timeNow,
				LastNotificationTime: timeNow,
			}))
		})
		t.Run("UpdateDeviceToLeased: valid update using Hostname", func(t *ftt.Test) {
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

			mock.ExpectBegin()

			var txOpts *sql.TxOptions
			tx, err := db.BeginTx(ctx, txOpts)
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub db transaction", err)
			}

			timeNow := time.Now()
			rows := sqlmock.NewRows([]string{
				"id",
				"dut_id",
				"device_address",
				"device_type",
				"device_state",
				"schedulable_labels",
				"is_active",
				"created_time",
				"last_updated_time",
				"last_notification_time"}).
				AddRow(
					"test-device-1",
					"test-dut-id",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"dut_id":{"Values":["test-dut-id"]},"label-test":{"Values":["test-value-1"]}}`,
					false,
					timeNow,
					timeNow,
					timeNow)

			labelBytes, err := json.Marshal(SchedulableLabels{
				"dut_id": LabelValues{
					Values: []string{"test-dut-id"},
				},
				"label-test": LabelValues{
					Values: []string{"test-value-1"},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, string(labelBytes), should.Match(`{"dut_id":{"Values":["test-dut-id"]},"label-test":{"Values":["test-value-1"]}}`))

			mock.ExpectQuery(regexp.QuoteMeta(`
				UPDATE
					"Devices"
				SET
					device_state='DEVICE_STATE_LEASED',
					last_updated_time=NOW()
				WHERE
					id=$1
					AND device_state='DEVICE_STATE_AVAILABLE'
					AND is_active=TRUE
				RETURNING
					id,
					dut_id,
					device_address,
					device_type,
					device_state,
					schedulable_labels,
					is_active,
					created_time,
					last_updated_time,
					last_notification_time;`)).
				WithArgs(
					"test-device-1").
				WillReturnRows(rows)

			updatedDevice, err := UpdateDeviceToLeased(ctx, tx, Device{
				ID: "test-device-1",
			}, IDTypeHostname)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, updatedDevice, should.Match(Device{
				ID:            "test-device-1",
				DutID:         "test-dut-id",
				DeviceAddress: "2.2.2.2:2",
				DeviceType:    "DEVICE_TYPE_VIRTUAL",
				DeviceState:   "DEVICE_STATE_LEASED",
				SchedulableLabels: SchedulableLabels{
					"dut_id": LabelValues{
						Values: []string{"test-dut-id"},
					},
					"label-test": LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive:             false,
				CreatedTime:          timeNow,
				LastUpdatedTime:      timeNow,
				LastNotificationTime: timeNow,
			}))
		})
	})
}

func TestUpsertDeviceFromUFS(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("UpsertDeviceFromUFS", t, func(t *ftt.Test) {
		t.Run("UpsertDeviceFromUFS: valid upsert", func(t *ftt.Test) {
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

			mock.ExpectExec(regexp.QuoteMeta(`
				INSERT INTO "Devices" AS d
					(
						id,
						dut_id,
						device_address,
						device_type,
						device_state,
						schedulable_labels,
						last_updated_time,
						is_active
					)
				VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7)
				ON CONFLICT(id)
				DO UPDATE SET
					dut_id=COALESCE(EXCLUDED.dut_id, d.dut_id),
					schedulable_labels=COALESCE(EXCLUDED.schedulable_labels, d.schedulable_labels),
					last_updated_time=NOW(),
					is_active=COALESCE(EXCLUDED.is_active, d.is_active);`)).
				WithArgs(
					"test-device-1",
					"test-dut-id",
					"2.2.2.2:2",
					"DEVICE_TYPE_VIRTUAL",
					"DEVICE_STATE_LEASED",
					`{"dut_id":{"Values":["test-dut-id"]},"label-test":{"Values":["test-value-1"]}}`,
					false).
				WillReturnResult(sqlmock.NewResult(1, 1))

			err = UpsertDeviceFromUFS(ctx, db, Device{
				ID:            "test-device-1",
				DeviceAddress: "2.2.2.2:2",
				DeviceType:    "DEVICE_TYPE_VIRTUAL",
				DeviceState:   "DEVICE_STATE_LEASED",
				SchedulableLabels: SchedulableLabels{
					"dut_id": LabelValues{
						Values: []string{"test-dut-id"},
					},
					"label-test": LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: false,
			})
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestSetDutIDFromLabels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("SetDutIDFromLabels should set the dut_id label", t, func(t *ftt.Test) {
		t.Run("SetDutIDFromLabels: valid dut_id", func(t *ftt.Test) {
			d := Device{
				ID: "foo",
				SchedulableLabels: SchedulableLabels{
					"dut_id": LabelValues{
						Values: []string{"bar"},
					},
					"hostname": LabelValues{
						Values: []string{"baz", "lol"},
					},
				},
			}
			err := d.SetDutIDFromLabels(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, d.DutID, should.Match("bar"))
		})
		t.Run("SetDutIDFromLabels: invalid dut_id; no labels", func(t *ftt.Test) {
			d := Device{
				ID: "foo",
				SchedulableLabels: SchedulableLabels{
					"hostname": LabelValues{
						Values: []string{"baz", "lol"},
					},
				},
			}
			err := d.SetDutIDFromLabels(ctx)
			assert.Loosely(t, err, should.ErrLike("failed to get dut_id from Device foo"))
			assert.Loosely(t, d.DutID, should.Match(""))
		})
		t.Run("SetDutIDFromLabels: invalid dut_id; too many labels", func(t *ftt.Test) {
			d := Device{
				ID: "foo",
				SchedulableLabels: SchedulableLabels{
					"dut_id": LabelValues{
						Values: []string{"baz", "lol"},
					},
				},
			}
			err := d.SetDutIDFromLabels(ctx)
			assert.Loosely(t, err, should.ErrLike("multiple values for DUT ID found for Device foo"))
			assert.Loosely(t, d.DutID, should.Match(""))
		})
	})
}
