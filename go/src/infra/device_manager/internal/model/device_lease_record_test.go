// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package model

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestCreateDeviceLeaseRecord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("CreateDeviceLeaseRecord", t, func(t *ftt.Test) {
		t.Run("CreateDeviceLeaseRecord: valid insert", func(t *ftt.Test) {
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
				"idempotency_key",
				"dut_id",
				"device_id",
				"device_address",
				"device_type",
				"owner_id",
				"leased_time",
				"expiration_time",
				"last_updated_time"}).
				AddRow(
					"test-lease-record-1",
					"fe20140c-b1aa-4953-90fc-d15677df0c6a",
					"test-dut-id",
					"test-device-1",
					"1.1.1.1:1",
					"DEVICE_TYPE_PHYSICAL",
					"test-owner-id-1",
					timeNow,
					timeNow.Add(time.Minute),
					timeNow,
				)

			mock.ExpectQuery(regexp.QuoteMeta(`
				INSERT INTO "DeviceLeaseRecords"
					(
						id,
						idempotency_key,
						dut_id,
						device_id,
						device_address,
						device_type,
						owner_id,
						leased_time,
						expiration_time,
						last_updated_time
					)
				VALUES
					($1, $2, $3, $4, $5, $6, $7, NOW(), NOW() + $8, NOW())
				RETURNING
					id,
					idempotency_key,
					dut_id,
					device_id,
					device_address,
					device_type,
					owner_id,
					leased_time,
					expiration_time,
					last_updated_time;`)).
				WithArgs(
					"test-lease-record-1",
					"fe20140c-b1aa-4953-90fc-d15677df0c6a",
					"test-dut-id",
					"test-device-1",
					"1.1.1.1:1",
					"DEVICE_TYPE_PHYSICAL",
					"test-owner-id-1",
					time.Minute,
				).
				WillReturnRows(rows)

			newRec, err := CreateDeviceLeaseRecord(ctx, tx, DeviceLeaseRecord{
				ID:             "test-lease-record-1",
				IdempotencyKey: "fe20140c-b1aa-4953-90fc-d15677df0c6a",
				DutID:          "test-dut-id",
				DeviceID:       "test-device-1",
				DeviceAddress:  "1.1.1.1:1",
				DeviceType:     "DEVICE_TYPE_PHYSICAL",
				OwnerID:        "test-owner-id-1",
			}, time.Minute)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, newRec, should.Equal(DeviceLeaseRecord{
				ID:              "test-lease-record-1",
				IdempotencyKey:  "fe20140c-b1aa-4953-90fc-d15677df0c6a",
				DutID:           "test-dut-id",
				DeviceID:        "test-device-1",
				DeviceAddress:   "1.1.1.1:1",
				DeviceType:      "DEVICE_TYPE_PHYSICAL",
				OwnerID:         "test-owner-id-1",
				LeasedTime:      timeNow,
				ExpirationTime:  timeNow.Add(time.Minute),
				LastUpdatedTime: timeNow,
			}))
		})
	})
}

func TestGetDeviceLeaseRecordByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("GetDeviceLeaseRecordByID", t, func(t *ftt.Test) {
		t.Run("GetDeviceLeaseRecordByID: valid return", func(t *ftt.Test) {
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

			timeNow := time.Now()
			rows := sqlmock.NewRows([]string{
				"id",
				"idempotency_key",
				"dut_id",
				"device_id",
				"device_address",
				"device_type",
				"owner_id",
				"leased_time",
				"released_time",
				"expiration_time",
				"last_updated_time"}).
				AddRow(
					"test-lease-record-1",
					"fe20140c-b1aa-4953-90fc-d15677df0c6a",
					"test-dut-id",
					"test-device-1",
					"1.1.1.1:1",
					"DEVICE_TYPE_PHYSICAL",
					"test-owner-id-1",
					timeNow,
					timeNow,
					timeNow,
					timeNow,
				)

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					idempotency_key,
					dut_id,
					device_id,
					device_address,
					device_type,
					owner_id,
					leased_time,
					released_time,
					expiration_time,
					last_updated_time
				FROM "DeviceLeaseRecords"
				WHERE id=$1;`)).
				WithArgs("test-lease-record-1").
				WillReturnRows(rows)

			record, err := GetDeviceLeaseRecordByID(ctx, db, "test-lease-record-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, record, should.Equal(DeviceLeaseRecord{
				ID:              "test-lease-record-1",
				IdempotencyKey:  "fe20140c-b1aa-4953-90fc-d15677df0c6a",
				DutID:           "test-dut-id",
				DeviceID:        "test-device-1",
				DeviceAddress:   "1.1.1.1:1",
				DeviceType:      "DEVICE_TYPE_PHYSICAL",
				OwnerID:         "test-owner-id-1",
				LeasedTime:      timeNow,
				ReleasedTime:    timeNow,
				ExpirationTime:  timeNow,
				LastUpdatedTime: timeNow,
			}))
		})
		t.Run("GetDeviceLeaseRecordByID: no record found", func(t *ftt.Test) {
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
				"idempotency_key",
				"dut_id",
				"device_id",
				"device_address",
				"device_type",
				"owner_id",
				"leased_time",
				"released_time",
				"expiration_time",
				"last_updated_time"})

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					idempotency_key,
					dut_id,
					device_id,
					device_address,
					device_type,
					owner_id,
					leased_time,
					released_time,
					expiration_time,
					last_updated_time
				FROM "DeviceLeaseRecords"
				WHERE id=$1;`)).
				WithArgs("test-lease-record-1").
				WillReturnRows(rows)

			record, err := GetDeviceLeaseRecordByID(ctx, db, "test-lease-record-1")
			assert.Loosely(t, err, should.ErrLike("no rows in result set"))
			assert.Loosely(t, record, should.Equal(DeviceLeaseRecord{}))
		})
	})
}

func TestGetDeviceLeaseRecordByIdemKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("GetDeviceLeaseRecordByIdemKey", t, func(t *ftt.Test) {
		t.Run("GetDeviceLeaseRecordByIdemKey: valid return", func(t *ftt.Test) {
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

			timeNow := time.Now()
			rows := sqlmock.NewRows([]string{
				"id",
				"idempotency_key",
				"dut_id",
				"device_id",
				"device_address",
				"device_type",
				"owner_id",
				"leased_time",
				"released_time",
				"expiration_time",
				"last_updated_time"}).
				AddRow(
					"test-lease-record-1",
					"fe20140c-b1aa-4953-90fc-d15677df0c6a",
					"test-dut-id",
					"test-device-1",
					"1.1.1.1:1",
					"DEVICE_TYPE_PHYSICAL",
					"test-owner-id-1",
					timeNow,
					timeNow,
					timeNow,
					timeNow,
				)

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					idempotency_key,
					dut_id,
					device_id,
					device_address,
					device_type,
					owner_id,
					leased_time,
					released_time,
					expiration_time,
					last_updated_time
				FROM "DeviceLeaseRecords"
				WHERE idempotency_key=$1;`)).
				WithArgs("fe20140c-b1aa-4953-90fc-d15677df0c6a").
				WillReturnRows(rows)

			record, err := GetDeviceLeaseRecordByIdemKey(ctx, db, "fe20140c-b1aa-4953-90fc-d15677df0c6a")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, record, should.Equal(DeviceLeaseRecord{
				ID:              "test-lease-record-1",
				IdempotencyKey:  "fe20140c-b1aa-4953-90fc-d15677df0c6a",
				DutID:           "test-dut-id",
				DeviceID:        "test-device-1",
				DeviceAddress:   "1.1.1.1:1",
				DeviceType:      "DEVICE_TYPE_PHYSICAL",
				OwnerID:         "test-owner-id-1",
				LeasedTime:      timeNow,
				ReleasedTime:    timeNow,
				ExpirationTime:  timeNow,
				LastUpdatedTime: timeNow,
			}))
		})
		t.Run("GetDeviceLeaseRecordByIdemKey: no record found", func(t *ftt.Test) {
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
				"idempotency_key",
				"dut_id",
				"device_id",
				"device_address",
				"device_type",
				"owner_id",
				"leased_time",
				"released_time",
				"expiration_time",
				"last_updated_time"})

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					idempotency_key,
					dut_id,
					device_id,
					device_address,
					device_type,
					owner_id,
					leased_time,
					released_time,
					expiration_time,
					last_updated_time
				FROM "DeviceLeaseRecords"
				WHERE idempotency_key=$1;`)).
				WithArgs("fe20140c-b1aa-4953-90fc-d15677df0c6a").
				WillReturnRows(rows)

			record, err := GetDeviceLeaseRecordByIdemKey(ctx, db, "fe20140c-b1aa-4953-90fc-d15677df0c6a")
			assert.Loosely(t, err, should.ErrLike("no rows in result set"))
			assert.Loosely(t, record, should.Equal(DeviceLeaseRecord{}))
		})
	})
}

func TestExtendLease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("ExtendLease", t, func(t *ftt.Test) {
		t.Run("ExtendLease: valid extend", func(t *ftt.Test) {
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

			mock.ExpectExec(regexp.QuoteMeta(`
				UPDATE
					"DeviceLeaseRecords"
				SET
					expiration_time=COALESCE($2, expiration_time),
					last_updated_time=NOW()
				WHERE
					id=$1;`)).
				WithArgs(
					"test-lease-record-1",
					timeNow.Add(time.Second*600)).
				WillReturnResult(sqlmock.NewResult(1, 1))

			err = ExtendLease(ctx, tx, DeviceLeaseRecord{
				ID:             "test-lease-record-1",
				ExpirationTime: timeNow.Add(time.Second * 600),
			})
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestReleaseLease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("ReleaseLease", t, func(t *ftt.Test) {
		t.Run("ReleaseLease: valid release", func(t *ftt.Test) {
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

			mock.ExpectExec(regexp.QuoteMeta(`
				UPDATE
					"DeviceLeaseRecords"
				SET
					released_time=NOW(),
					last_updated_time=NOW()
				WHERE
					id=$1;`)).
				WithArgs(
					"test-lease-record-1").
				WillReturnResult(sqlmock.NewResult(1, 1))

			err = ReleaseLease(ctx, tx, DeviceLeaseRecord{
				ID:              "test-lease-record-1",
				ReleasedTime:    timeNow.Add(time.Second * 600),
				ExpirationTime:  timeNow.Add(time.Second * 600),
				LastUpdatedTime: timeNow,
			})
			assert.Loosely(t, err, should.BeNil)
		})
	})
}
