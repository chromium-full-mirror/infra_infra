// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package model

import (
	"context"
	"database/sql"
	"time"

	"go.chromium.org/luci/common/logging"
)

// ExtendLeaseRequest contains a single row from the ExtendLeaseRequests table
// in the database.
type ExtendLeaseRequest struct {
	ID             string
	LeaseID        string
	IdempotencyKey string
	ExtendDuration int64
	RequestTime    time.Time
	ExpirationTime time.Time
}

// CreateExtendLeaseRequest creates a ExtendLeaseRequest in the database.
func CreateExtendLeaseRequest(ctx context.Context, tx *sql.Tx, request ExtendLeaseRequest) error {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO "ExtendLeaseRequests"
			(id, lease_id, idempotency_key, extend_duration, request_time,
				expiration_time)
		VALUES
			($1, $2, $3, $4, NOW(), $5);`,
		request.ID,
		request.LeaseID,
		request.IdempotencyKey,
		request.ExtendDuration,
		request.ExpirationTime,
	)
	if err != nil {
		logging.Errorf(ctx, "CreateExtendLeaseRequest: error inserting into ExtendLeaseRequests: %s", err)
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			logging.Errorf(ctx, "CreateExtendLeaseRequest: unable to rollback: %v", rollbackErr)
		}
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logging.Errorf(ctx, "CreateExtendLeaseRequest: error getting rows affected: %s", err)
	}

	logging.Debugf(ctx, "CreateExtendLeaseRequest: ExtendLeaseRequest %s created successfully (%d row affected)", request.ID, rowsAffected)
	return nil
}

// GetExtendLeaseRequestByIdemKey gets a ExtendLeaseRequest from the database by idempotency key.
func GetExtendLeaseRequestByIdemKey(ctx context.Context, db *sql.DB, idemKey string) (ExtendLeaseRequest, error) {
	var (
		record         ExtendLeaseRequest
		requestTime    sql.NullTime
		expirationTime sql.NullTime
	)

	err := db.QueryRowContext(ctx, `
		SELECT
			id,
			lease_id,
			idempotency_key,
			extend_duration,
			request_time,
			expiration_time
		FROM "ExtendLeaseRequests"
		WHERE idempotency_key=$1;`, idemKey).Scan(
		&record.ID,
		&record.LeaseID,
		&record.IdempotencyKey,
		&record.ExtendDuration,
		&requestTime,
		&expirationTime,
	)
	if err != nil {
		logging.Debugf(ctx, "GetExtendLeaseRequestByIdemKey: failed to get ExtendLeaseRequest with Idempotency Key %s: %s", idemKey, err)
		return record, err
	}

	// Handle possible null times
	if requestTime.Valid {
		record.RequestTime = requestTime.Time
	}
	if expirationTime.Valid {
		record.ExpirationTime = expirationTime.Time
	}

	logging.Debugf(ctx, "GetExtendLeaseRequestByIdemKey: success: %v", record)
	return record, nil
}
