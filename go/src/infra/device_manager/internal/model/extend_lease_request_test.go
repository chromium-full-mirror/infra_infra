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

func TestCreateExtendLeaseRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("CreateExtendLeaseRequest", t, func(t *ftt.Test) {
		t.Run("CreateExtendLeaseRequest: valid insert", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestGetExtendLeaseRequestByIdemKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("GetExtendLeaseRequestByIdemKey", t, func(t *ftt.Test) {
		t.Run("GetExtendLeaseRequestByIdemKey: valid return", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, record, should.Equal(ExtendLeaseRequest{
				ID:             "test-extend-record-1",
				LeaseID:        "test-lease-record-1",
				IdempotencyKey: "fe20140c-b1aa-4953-90fc-d15677df0c6a",
				ExtendDuration: 600,
				RequestTime:    timeNow,
				ExpirationTime: timeNow.Add(time.Minute * 10),
			}))
		})
		t.Run("GetExtendLeaseRequestByIdemKey: no record found", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.ErrLike("no rows in result set"))
			assert.Loosely(t, record, should.Equal(ExtendLeaseRequest{}))
		})
	})
}
