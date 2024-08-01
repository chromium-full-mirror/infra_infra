// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.chromium.org/luci/common/logging"

	"infra/device_manager/internal/database"
)

// DeviceLeaseRecordData is used to pass data to the HTML template.
type DeviceLeaseRecordData struct {
	Records []DeviceLeaseRecord
}

// DeviceLeaseRecord contains a single row from the DeviceLeaseRecords table in
// the database.
type DeviceLeaseRecord struct {
	ID              string
	IdempotencyKey  string
	DeviceID        string
	DeviceAddress   string
	DeviceType      string
	OwnerID         string
	LeasedTime      time.Time
	ReleasedTime    time.Time
	ExpirationTime  time.Time
	LastUpdatedTime time.Time
}

// CreateDeviceLeaseRecord creates a DeviceLeaseRecord in the database.
func CreateDeviceLeaseRecord(ctx context.Context, tx *sql.Tx, record DeviceLeaseRecord, leaseDur time.Duration) (DeviceLeaseRecord, error) {
	var (
		newRecord       DeviceLeaseRecord
		leasedTime      sql.NullTime
		expirationTime  sql.NullTime
		lastUpdatedTime sql.NullTime
	)
	err := tx.QueryRowContext(ctx, `
		INSERT INTO "DeviceLeaseRecords"
			(
				id,
				idempotency_key,
				device_id,
				device_address,
				device_type,
				owner_id,
			 	leased_time,
				expiration_time,
				last_updated_time
			)
		VALUES
			($1, $2, $3, $4, $5, $6, NOW(), NOW() + $7, NOW())
		RETURNING
			id,
			idempotency_key,
			device_id,
			device_address,
			device_type,
			owner_id,
			leased_time,
			expiration_time,
			last_updated_time;`,
		record.ID,
		record.IdempotencyKey,
		record.DeviceID,
		record.DeviceAddress,
		record.DeviceType,
		record.OwnerID,
		leaseDur,
	).Scan(
		&newRecord.ID,
		&newRecord.IdempotencyKey,
		&newRecord.DeviceID,
		&newRecord.DeviceAddress,
		&newRecord.DeviceType,
		&newRecord.OwnerID,
		&leasedTime,
		&expirationTime,
		&lastUpdatedTime,
	)
	if err != nil {
		logging.Errorf(ctx, "CreateDeviceLeaseRecord: error inserting into DeviceLeaseRecords: %s", err)
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			logging.Errorf(ctx, "CreateDeviceLeaseRecord: unable to rollback: %v", rollbackErr)
		}
		return DeviceLeaseRecord{}, err
	}

	// Handle possible null times
	if leasedTime.Valid {
		newRecord.LeasedTime = leasedTime.Time
	}
	if expirationTime.Valid {
		newRecord.ExpirationTime = expirationTime.Time
	}
	if lastUpdatedTime.Valid {
		newRecord.LastUpdatedTime = lastUpdatedTime.Time
	}

	logging.Debugf(ctx, "CreateDeviceLeaseRecord: DeviceLeaseRecord %s for Device %s created successfully", newRecord.ID, newRecord.DeviceID)
	return newRecord, nil
}

// GetDeviceLeaseRecordByID gets a DeviceLeaseRecord from the database by name.
func GetDeviceLeaseRecordByID(ctx context.Context, db *sql.DB, recordID string) (DeviceLeaseRecord, error) {
	var (
		record          DeviceLeaseRecord
		leasedTime      sql.NullTime
		releasedTime    sql.NullTime
		expirationTime  sql.NullTime
		lastUpdatedTime sql.NullTime
	)

	err := db.QueryRowContext(ctx, `
		SELECT
			id,
			idempotency_key,
			device_id,
			device_address,
			device_type,
			owner_id,
			leased_time,
			released_time,
			expiration_time,
			last_updated_time
		FROM "DeviceLeaseRecords"
		WHERE id=$1;`, recordID).Scan(
		&record.ID,
		&record.IdempotencyKey,
		&record.DeviceID,
		&record.DeviceAddress,
		&record.DeviceType,
		&record.OwnerID,
		&leasedTime,
		&releasedTime,
		&expirationTime,
		&lastUpdatedTime,
	)
	if err != nil {
		logging.Errorf(ctx, "GetDeviceLeaseRecordByID: failed to get DeviceLeaseRecord %s: %s", recordID, err)
		return record, err
	}

	// Handle possible null times
	if leasedTime.Valid {
		record.LeasedTime = leasedTime.Time
	}
	if releasedTime.Valid {
		record.ReleasedTime = releasedTime.Time
	}
	if expirationTime.Valid {
		record.ExpirationTime = expirationTime.Time
	}
	if lastUpdatedTime.Valid {
		record.LastUpdatedTime = lastUpdatedTime.Time
	}

	logging.Debugf(ctx, "GetDeviceLeaseRecordByID: success: %v", record)
	return record, nil
}

// GetDeviceLeaseRecordByIdemKey gets a DeviceLeaseRecord from the database by idempotency key.
func GetDeviceLeaseRecordByIdemKey(ctx context.Context, db *sql.DB, idemKey string) (DeviceLeaseRecord, error) {
	var (
		record          DeviceLeaseRecord
		leasedTime      sql.NullTime
		releasedTime    sql.NullTime
		expirationTime  sql.NullTime
		lastUpdatedTime sql.NullTime
	)

	err := db.QueryRowContext(ctx, `
		SELECT
			id,
			idempotency_key,
			device_id,
			device_address,
			device_type,
			owner_id,
			leased_time,
			released_time,
			expiration_time,
			last_updated_time
		FROM "DeviceLeaseRecords"
		WHERE idempotency_key=$1;`, idemKey).Scan(
		&record.ID,
		&record.IdempotencyKey,
		&record.DeviceID,
		&record.DeviceAddress,
		&record.DeviceType,
		&record.OwnerID,
		&leasedTime,
		&releasedTime,
		&expirationTime,
		&lastUpdatedTime,
	)
	if err != nil {
		logging.Errorf(ctx, "GetDeviceLeaseRecordByIdemKey: failed to get DeviceLeaseRecord with Idempotency Key %s: %s", idemKey, err)
		return record, err
	}

	// Handle possible null times
	if leasedTime.Valid {
		record.LeasedTime = leasedTime.Time
	}
	if releasedTime.Valid {
		record.ReleasedTime = releasedTime.Time
	}
	if expirationTime.Valid {
		record.ExpirationTime = expirationTime.Time
	}
	if lastUpdatedTime.Valid {
		record.LastUpdatedTime = lastUpdatedTime.Time
	}

	logging.Debugf(ctx, "GetDeviceLeaseRecordByIdemKey: success: %v", record)
	return record, nil
}

// ListLeases retrieves DeviceLeaseRecords with pagination.
func ListLeases(ctx context.Context, db *sql.DB, pageToken database.PageToken, pageSize int, filter string) ([]DeviceLeaseRecord, database.PageToken, error) {
	// handle potential errors for negative page numbers or page sizes
	if pageSize <= 0 {
		pageSize = database.DefaultPageSize
	}

	query, args, err := buildListLeasesQuery(ctx, pageToken, pageSize, filter)
	if err != nil {
		return nil, "", fmt.Errorf("ListLeases: %w", err)
	}

	logging.Debugf(ctx, "ListLeases: running query: %s", query)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("ListLeases: %w", err)
	}
	defer rows.Close()

	var results []DeviceLeaseRecord
	for rows.Next() {
		var (
			lease           DeviceLeaseRecord
			leasedTime      sql.NullTime
			releasedTime    sql.NullTime
			expirationTime  sql.NullTime
			lastUpdatedTime sql.NullTime
		)

		err := rows.Scan(
			&lease.ID,
			&lease.DeviceID,
			&lease.DeviceAddress,
			&lease.DeviceType,
			&lease.OwnerID,
			&leasedTime,
			&releasedTime,
			&expirationTime,
			&lastUpdatedTime,
		)
		if err != nil {
			return nil, "", fmt.Errorf("ListLeases: %w", err)
		}

		// handle possible null times
		if leasedTime.Valid {
			lease.LeasedTime = leasedTime.Time
		}
		if releasedTime.Valid {
			lease.ReleasedTime = releasedTime.Time
		}
		if expirationTime.Valid {
			lease.ExpirationTime = expirationTime.Time
		}
		if lastUpdatedTime.Valid {
			lease.LastUpdatedTime = lastUpdatedTime.Time
		}

		results = append(results, lease)
	}

	if err := rows.Close(); err != nil {
		return nil, "", fmt.Errorf("ListLeases: %w", err)
	}

	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("ListLeases: %w", err)
	}

	// truncate results and use last Device ID as next page token
	var nextPageToken database.PageToken
	if len(results) > pageSize {
		lastDevice := results[pageSize-1]
		nextPageToken = database.EncodePageToken(ctx, lastDevice.LeasedTime.Format(time.RFC3339Nano))
		results = results[0:pageSize] // trim results to page size
	}
	return results, nextPageToken, nil
}

// buildListLeasesQuery builds a ListLeases query using given params.
func buildListLeasesQuery(ctx context.Context, pageToken database.PageToken, pageSize int, filter string) (string, []interface{}, error) {
	var queryArgs []interface{}
	query := `
		SELECT
			id,
			device_id,
			device_address,
			device_type,
			owner_id,
			leased_time,
			released_time,
			expiration_time,
			last_updated_time
		FROM "DeviceLeaseRecords"`

	if pageToken != "" {
		decodedTime, err := database.DecodePageToken(ctx, pageToken)
		if err != nil {
			return "", queryArgs, fmt.Errorf("buildListLeasesQuery: %w", err)
		}
		filter = fmt.Sprintf("leased_time > %s%s", decodedTime, func() string {
			if filter == "" {
				return "" // No additional filter provided
			}
			return " AND " + filter
		}())
	}

	queryFilter, filterArgs := database.BuildQueryFilter(ctx, filter)
	query += queryFilter + fmt.Sprintf(`
		ORDER BY leased_time
		LIMIT $%d;`, len(filterArgs)+1)
	filterArgs = append(filterArgs, pageSize+1) // fetch one extra to check for 'next page'

	return query, filterArgs, nil
}

// ExtendLease updates a lease record in a transaction.
//
// ExtendLease uses COALESCE to only update fields with provided values. If
// there is no value provided, then it will use the current value of the device
// field in the db.
func ExtendLease(ctx context.Context, tx *sql.Tx, leaseRec DeviceLeaseRecord) error {
	// Handle possible null times
	var expirationTime sql.NullTime
	if !leaseRec.ExpirationTime.IsZero() {
		expirationTime.Time = leaseRec.ExpirationTime
		expirationTime.Valid = true
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE
			"DeviceLeaseRecords"
		SET
			expiration_time=COALESCE($2, expiration_time),
			last_updated_time=NOW()
		WHERE
			id=$1;`,
		leaseRec.ID,
		expirationTime,
	)
	if err != nil {
		logging.Errorf(ctx, "ExtendLease: failed to extend DeviceLeaseRecord %s: %s", leaseRec.ID, err)
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			logging.Errorf(ctx, "ExtendLease: unable to rollback: %v", rollbackErr)
		}
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logging.Errorf(ctx, "ExtendLease: error getting rows affected: %s", err)
	}

	logging.Debugf(ctx, "ExtendLease: DeviceLeaseRecord %s extended successfully (%d row affected)", leaseRec.ID, rowsAffected)
	return nil
}

// ReleaseLease releases a lease record in a transaction.
func ReleaseLease(ctx context.Context, tx *sql.Tx, leaseRec DeviceLeaseRecord) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE
			"DeviceLeaseRecords"
		SET
			released_time=NOW(),
			last_updated_time=NOW()
		WHERE
			id=$1;`,
		leaseRec.ID,
	)
	if err != nil {
		logging.Errorf(ctx, "ReleaseLease: failed to release DeviceLeaseRecord %s: %s", leaseRec.ID, err)
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			logging.Errorf(ctx, "ReleaseLease: unable to rollback: %v", rollbackErr)
		}
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logging.Errorf(ctx, "ReleaseLease: error getting rows affected: %s", err)
	}

	logging.Debugf(ctx, "ReleaseLease: DeviceLeaseRecord %s released successfully (%d row affected)", leaseRec.ID, rowsAffected)
	return nil
}
