// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgconn"

	lucierr "go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/device_manager/internal/database"
)

// Error types for DeviceLeaseRecord model operations
var (
	ErrLeaseIdemKeyAlreadyExists = errors.New("idempotency key for lease on Device already used")
)

// DeviceLeaseRecordData is used to pass data to the HTML template.
type DeviceLeaseRecordData struct {
	Records []DeviceLeaseRecord
}

// DeviceLeaseRecord contains a single row from the DeviceLeaseRecords table in
// the database.
type DeviceLeaseRecord struct {
	ID             string
	IdempotencyKey string
	DutID          string
	DeviceID       string
	DeviceAddress  string
	DeviceType     string
	OwnerID        string

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
			last_updated_time;`,
		record.ID,
		record.IdempotencyKey,
		record.DutID,
		record.DeviceID,
		record.DeviceAddress,
		record.DeviceType,
		record.OwnerID,
		leaseDur,
	).Scan(
		&newRecord.ID,
		&newRecord.IdempotencyKey,
		&newRecord.DutID,
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

// BulkCreateDeviceLeaseRecords creates multiple DeviceLeaseRecords in the DB.
//
// BulkCreateDeviceLeaseRecords takes a list of DeviceLeaseRecord models and
// attempts to bulk insert them into the database. On conflict of the
// idempotency key, it will do nothing.
func BulkCreateDeviceLeaseRecords(ctx context.Context, tx *sql.Tx, records []DeviceLeaseRecord, leaseDurs []time.Duration) (map[string]*DeviceLeaseRecord, map[string]error, error) {
	var (
		// A map for DUT ID to created lease record
		createSuccess = map[string]*DeviceLeaseRecord{}
		// A map for DUT ID to error
		createErrs = map[string]error{}
		query      = `
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
			VALUES %s
			ON CONFLICT (idempotency_key)
			DO NOTHING
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
				last_updated_time;`
	)

	// Populate temporary table and errors.
	var (
		valueStrings []string
		valueArgs    []interface{}
	)
	for i, r := range records {
		l := len(valueArgs)
		valueStrings = append(
			valueStrings,
			fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, NOW(), NOW() + $%d, NOW())", l+1, l+2, l+3, l+4, l+5, l+6, l+7, l+8),
		)
		valueArgs = append(
			valueArgs,
			r.ID,
			r.IdempotencyKey,
			r.DutID,
			r.DeviceID,
			r.DeviceAddress,
			r.DeviceType,
			r.OwnerID,
			leaseDurs[i],
		)
		createErrs[r.DutID] = ErrLeaseIdemKeyAlreadyExists
	}

	if len(valueStrings) == 0 {
		return nil, nil, errors.New("BulkCreateDeviceLeaseRecords: no new leases to be created")
	}

	stmt := fmt.Sprintf(query, strings.Join(valueStrings, ","))
	logging.Debugf(ctx, "BulkCreateDeviceLeaseRecords: insert statement: %s\n with value arguments: %+v", stmt, valueArgs)
	rows, err := tx.QueryContext(ctx, stmt, valueArgs...)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, nil, fmt.Errorf("unable to rollback: %w", rollbackErr)
		}
		return nil, nil, fmt.Errorf("failed to insert batch into DeviceLeaseRecords table: %w", err)
	}
	defer rows.Close()

	// Process Devices and populate actually leased Devices
	for rows.Next() {
		var (
			newRecord       DeviceLeaseRecord
			leasedTime      sql.NullTime
			expirationTime  sql.NullTime
			lastUpdatedTime sql.NullTime
		)
		err := rows.Scan(
			&newRecord.ID,
			&newRecord.IdempotencyKey,
			&newRecord.DutID,
			&newRecord.DeviceID,
			&newRecord.DeviceAddress,
			&newRecord.DeviceType,
			&newRecord.OwnerID,
			&leasedTime,
			&expirationTime,
			&lastUpdatedTime,
		)

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

		if err != nil {
			logging.Errorf(ctx, "BulkCreateDeviceLeaseRecords: failed to create lease record: %w", err)
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				logging.Debugf(ctx, "BulkCreateDeviceLeaseRecords: SQLSTATE:", pgErr.Code)
				logging.Debugf(ctx, "BulkCreateDeviceLeaseRecords:", pgErr.Message)
			}
			continue
		}
		createSuccess[newRecord.DutID] = &newRecord
		createErrs[newRecord.DutID] = nil
	}
	return createSuccess, createErrs, nil
}

// GetDeviceLeaseRecordByID gets a DeviceLeaseRecord from the database by name.
func GetDeviceLeaseRecordByID(ctx context.Context, db *sql.DB, recordID string) (*DeviceLeaseRecord, error) {
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
		WHERE id=$1;`, recordID).Scan(
		&record.ID,
		&record.IdempotencyKey,
		&record.DutID,
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
		logging.Debugf(ctx, "GetDeviceLeaseRecordByID: failed to get DeviceLeaseRecord %s: %s", recordID, err)
		return nil, fmt.Errorf("get device lease record by id %q: %w", recordID, err)
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
	return &record, nil
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
		WHERE idempotency_key=$1;`, idemKey).Scan(
		&record.ID,
		&record.IdempotencyKey,
		&record.DutID,
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
		logging.Debugf(ctx, "GetDeviceLeaseRecordByIdemKey: failed to get DeviceLeaseRecord with Idempotency Key %s: %s", idemKey, err)
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
			&lease.DutID,
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
			dut_id,
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
func ReleaseLease(ctx context.Context, tx *sql.Tx, leaseID string) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE
			"DeviceLeaseRecords"
		SET
			released_time=NOW(),
			last_updated_time=NOW()
		WHERE
			id=$1;`,
		leaseID,
	)
	if err != nil {
		logging.Errorf(ctx, "ReleaseLease: failed to release DeviceLeaseRecord %s: %s", leaseID, err)
		return fmt.Errorf("release lease %q: %w", leaseID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logging.Errorf(ctx, "ReleaseLease: error getting rows affected: %s", err)
		return fmt.Errorf("release lease %q: %w", leaseID, err)
	}

	logging.Debugf(ctx, "ReleaseLease: DeviceLeaseRecord %s released successfully (%d row affected)", leaseID, rowsAffected)
	return nil
}

// ExpireLeases expires all leases that haven't been modified since the given
// timestamp, and returns the expired lease IDs and associated device IDs.
func ExpireLeases(ctx context.Context, tx *sql.Tx, t time.Time) (releasedLeases []DeviceLeaseRecord, err error) {
	query := `
		UPDATE "DeviceLeaseRecords"
		SET
			released_time = NOW(),
			last_updated_time = NOW()
		WHERE
			expiration_time <= $1 AND
			released_time IS NULL
		RETURNING
			id,
			dut_id,
			device_id;`

	releasedLeaseRows, err := tx.QueryContext(ctx, query, t)
	if err != nil {
		err = lucierr.Annotate(err, "executing PSQL query to release leases").Err()
		return nil, err
	}
	defer releasedLeaseRows.Close()

	// Read expired lease IDs and their associated device IDs
	for releasedLeaseRows.Next() {
		var rec DeviceLeaseRecord
		err := releasedLeaseRows.Scan(
			&rec.ID,
			&rec.DutID,
			&rec.DeviceID,
		)
		if err != nil {
			err = lucierr.Annotate(err, "reading released leases").Err()
			return nil, err
		}
		releasedLeases = append(releasedLeases, rec)
	}
	return releasedLeases, nil
}
