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
	. "github.com/smartystreets/goconvey/convey"

	. "go.chromium.org/luci/common/testing/assertions"
)

func TestCreateExtendLeaseRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	Convey("CreateExtendLeaseRequest", t, func() {
		Convey("CreateExtendLeaseRequest: valid insert", func() {
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
				INSERT INTO "ExtendLeaseRequests"
					(id, lease_id, idempotency_key, extend_duration, request_time,
						expiration_time)
				VALUES
					($1, $2, $3, $4, NOW(), $5);`)).
				WithArgs(
					"test-extend-request-1",
					"test-lease-record-1",
					"fe20140c-b1aa-4953-90fc-d15677df0c6a",
					600,
					timeNow,
				).
				WillReturnResult(sqlmock.NewResult(1, 1))

			err = CreateExtendLeaseRequest(ctx, tx, ExtendLeaseRequest{
				ID:             "test-extend-request-1",
				LeaseID:        "test-lease-record-1",
				IdempotencyKey: "fe20140c-b1aa-4953-90fc-d15677df0c6a",
				ExtendDuration: 600,
				ExpirationTime: timeNow,
			})
			So(err, ShouldBeNil)
		})
	})
}

func TestGetExtendLeaseRequestByIdemKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	Convey("GetExtendLeaseRequestByIdemKey", t, func() {
		Convey("GetExtendLeaseRequestByIdemKey: valid return", func() {
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
				"lease_id",
				"idempotency_key",
				"extend_duration",
				"request_time",
				"expiration_time"}).
				AddRow(
					"test-extend-record-1",
					"test-lease-record-1",
					"fe20140c-b1aa-4953-90fc-d15677df0c6a",
					600,
					timeNow,
					timeNow.Add(time.Minute*10),
				)

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					lease_id,
					idempotency_key,
					extend_duration,
					request_time,
					expiration_time
				FROM "ExtendLeaseRequests"
				WHERE idempotency_key=$1;`)).
				WithArgs("fe20140c-b1aa-4953-90fc-d15677df0c6a").
				WillReturnRows(rows)

			record, err := GetExtendLeaseRequestByIdemKey(ctx, db, "fe20140c-b1aa-4953-90fc-d15677df0c6a")
			So(err, ShouldBeNil)
			So(record, ShouldEqual, ExtendLeaseRequest{
				ID:             "test-extend-record-1",
				LeaseID:        "test-lease-record-1",
				IdempotencyKey: "fe20140c-b1aa-4953-90fc-d15677df0c6a",
				ExtendDuration: 600,
				RequestTime:    timeNow,
				ExpirationTime: timeNow.Add(time.Minute * 10),
			})
		})
		Convey("GetExtendLeaseRequestByIdemKey: no record found", func() {
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
				"lease_id",
				"idempotency_key",
				"extend_duration",
				"request_time",
				"expiration_time"})

			mock.ExpectQuery(regexp.QuoteMeta(`
				SELECT
					id,
					lease_id,
					idempotency_key,
					extend_duration,
					request_time,
					expiration_time
				FROM "ExtendLeaseRequests"
				WHERE idempotency_key=$1;`)).
				WithArgs("fe20140c-b1aa-4953-90fc-d15677df0c6a").
				WillReturnRows(rows)

			record, err := GetExtendLeaseRequestByIdemKey(ctx, db, "fe20140c-b1aa-4953-90fc-d15677df0c6a")
			So(err, ShouldErrLike, "no rows in result set")
			So(record, ShouldEqual, ExtendLeaseRequest{})
		})
	})
}
