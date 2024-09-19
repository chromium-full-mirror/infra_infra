// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgconn"

	lucierr "go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/device_manager/internal/database"
)

// ErrDeviceNotFound defines a custom error when a Device is not found.
var (
	ErrDeviceNotFound      = errors.New("device not found")
	ErrDeviceAlreadyLeased = errors.New("device already leased")
)

// Device contains a single row from the Devices table in the database.
type Device struct {
	ID                string
	DutID             string
	DeviceAddress     string
	DeviceType        string
	DeviceState       string
	SchedulableLabels SchedulableLabels `json:"SchedulableLabels"`

	IsActive             bool
	CreatedTime          time.Time
	LastUpdatedTime      time.Time
	LastNotificationTime time.Time
}

// DeviceIDType indicates the type of ID used to identify a Device in DB.
type DeviceIDType string

const (
	IDTypeHostname DeviceIDType = "hostname"
	IDTypeDutID    DeviceIDType = "dut_id"
)

// LabelValues is the struct containing an array of label values.
type LabelValues struct {
	Values []string
}

// SchedulableLabels is made up of a label key and LabelValues.
type SchedulableLabels map[string]LabelValues

// GormDataType expresses SchedulableLabels as a gorm type to db.
func (SchedulableLabels) GormDataType() string {
	return "JSONB"
}

// Scan implements scanner interface for SchedulableLabels.
func (s *SchedulableLabels) Scan(value interface{}) error {
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		bytes = []byte(`{}`)
	}
	err := json.Unmarshal(bytes, s)
	return err
}

// Value implements Valuer interface for SchedulableLabels.
func (s SchedulableLabels) Value() (driver.Value, error) {
	bytes, err := json.Marshal(s)
	return string(bytes), err
}

// GetDeviceByID gets a Device from the database by a type of ID.
func GetDeviceByID(ctx context.Context, db *sql.DB, idType DeviceIDType, deviceID string) (Device, error) {
	var (
		device          Device
		createdTime     sql.NullTime
		lastUpdatedTime sql.NullTime
	)
	query := `
		SELECT
			id,
			device_address,
			device_type,
			device_state,
			schedulable_labels,
			created_time,
			last_updated_time,
			is_active
		FROM "Devices"`

	switch idType {
	case IDTypeDutID:
		// Use DUT ID type also known as Asset Tag.
		query += `
			WHERE
				jsonb_path_query_array(
					schedulable_labels,
					'$.dut_id.Values[0]'
				) @> to_jsonb($1::text);`
	case IDTypeHostname:
		// Use hostname which is how they are stored in DB.
		query += `
			WHERE id=$1;`
	default:
		return Device{}, fmt.Errorf("GetDeviceByID: unsupported Device ID type: %s", idType)
	}

	err := db.QueryRowContext(ctx, query, deviceID).Scan(
		&device.ID,
		&device.DeviceAddress,
		&device.DeviceType,
		&device.DeviceState,
		&device.SchedulableLabels,
		&createdTime,
		&lastUpdatedTime,
		&device.IsActive,
	)

	// TODO (b/328662436): Collect metrics on results
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			logging.Debugf(ctx, "GetDeviceByID: SQLSTATE:", pgErr.Code)
			logging.Debugf(ctx, "GetDeviceByID:", pgErr.Message)
		}
		if errors.Is(err, sql.ErrNoRows) || (errors.As(err, &pgErr) && pgErr.Code == "P0002") {
			return device, ErrDeviceNotFound
		}
		return device, err
	}

	// Handle possible null times
	if createdTime.Valid {
		device.CreatedTime = createdTime.Time
	}
	if lastUpdatedTime.Valid {
		device.LastUpdatedTime = lastUpdatedTime.Time
	}

	return device, nil
}

// ListDevices retrieves Devices with pagination.
func ListDevices(ctx context.Context, db *sql.DB, pageToken database.PageToken, pageSize int, filter string) ([]Device, database.PageToken, error) {
	// handle potential errors for negative page numbers or page sizes
	if pageSize <= 0 {
		pageSize = database.DefaultPageSize
	}

	query, args, err := buildListDevicesQuery(ctx, pageToken, pageSize, filter)
	if err != nil {
		return nil, "", fmt.Errorf("ListDevices: %w", err)
	}

	logging.Debugf(ctx, "ListDevices: running query: %s", query)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("ListDevices: %w", err)
	}
	defer rows.Close()

	var results []Device
	for rows.Next() {
		var (
			device          Device
			createdTime     sql.NullTime
			lastUpdatedTime sql.NullTime
		)
		err := rows.Scan(
			&device.ID,
			&device.DeviceAddress,
			&device.DeviceType,
			&device.DeviceState,
			&device.SchedulableLabels,
			&createdTime,
			&lastUpdatedTime,
			&device.IsActive,
		)
		if err != nil {
			return nil, "", fmt.Errorf("ListDevices: %w", err)
		}

		// handle possible null times
		if createdTime.Valid {
			device.CreatedTime = createdTime.Time
		}
		if lastUpdatedTime.Valid {
			device.LastUpdatedTime = lastUpdatedTime.Time
		}

		results = append(results, device)
	}

	if err := rows.Close(); err != nil {
		return nil, "", fmt.Errorf("ListDevices: %w", err)
	}

	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("ListDevices: %w", err)
	}

	// truncate results and use last Device ID as next page token
	var nextPageToken database.PageToken
	if len(results) > pageSize {
		lastDevice := results[pageSize-1]
		nextPageToken = database.EncodePageToken(ctx, lastDevice.CreatedTime.Format(time.RFC3339Nano))
		results = results[0:pageSize] // trim results to page size
	}
	return results, nextPageToken, nil
}

// ExpireLeases expires all leases that haven't been modified since the given
// timestamp, and returns the expired lease IDs and associated device IDs.
func ExpireLeases(ctx context.Context, tx *sql.Tx, t time.Time) (leaseIDs, deviceIDs []string, err error) {
	query := `
		UPDATE "DeviceLeaseRecords"
		SET
		    released_time = NOW(),
		    last_updated_time = NOW()
		WHERE
		    expiration_time <= $1 AND
		    released_time IS NULL
		RETURNING id, device_id`

	releasedLeaseRows, err := tx.QueryContext(ctx, query, t)
	if err != nil {
		err = lucierr.Annotate(err, "executing PSQL query to release leases").Err()
		return nil, nil, err
	}
	defer releasedLeaseRows.Close()

	// Read expired lease IDs and their associated device IDs
	for releasedLeaseRows.Next() {
		var (
			leaseID  string
			deviceID string
		)
		err := releasedLeaseRows.Scan(&leaseID, &deviceID)
		if err != nil {
			err = lucierr.Annotate(err, "reading released leases").Err()
			return nil, nil, err
		}
		leaseIDs = append(leaseIDs, leaseID)
		deviceIDs = append(deviceIDs, deviceID)
	}
	return leaseIDs, deviceIDs, nil
}

// buildListDevicesQuery builds a ListDevices query using given params.
func buildListDevicesQuery(ctx context.Context, pageToken database.PageToken, pageSize int, filter string) (string, []interface{}, error) {
	var queryArgs []interface{}
	query := `
		SELECT
			id,
			device_address,
			device_type,
			device_state,
			schedulable_labels,
			created_time,
			last_updated_time,
			is_active
		FROM "Devices"`

	if pageToken != "" {
		decodedTime, err := database.DecodePageToken(ctx, pageToken)
		if err != nil {
			return "", queryArgs, fmt.Errorf("buildListDevicesQuery: %w", err)
		}
		filter = fmt.Sprintf("created_time > %s%s", decodedTime, func() string {
			if filter == "" {
				return "" // No additional filter provided
			}
			return " AND " + filter
		}())
	}

	queryFilter, filterArgs := database.BuildQueryFilter(ctx, filter)
	query += queryFilter + fmt.Sprintf(`
		ORDER BY created_time
		LIMIT $%d;`, len(filterArgs)+1)
	filterArgs = append(filterArgs, pageSize+1) // fetch one extra to check for 'next page'

	return query, filterArgs, nil
}

// UpdateDevice updates a Device in a transaction.
//
// UpdateDevice uses COALESCE to only update fields with provided values. If
// there is no value provided, then it will use the current value of the device
// field in the db.
//
// DUT ID (asset tag) will be extracted from the schedulable labels. This is the
// only way to update the field. Trying to set Device.DutID will result in no
// change.
func UpdateDevice(ctx context.Context, tx *sql.Tx, device Device) (Device, error) {
	var (
		err                  error
		updatedDevice        Device
		labelBytes           []byte
		createdTime          sql.NullTime
		lastUpdatedTime      sql.NullTime
		lastNotificationTime sql.NullTime
		query                = `
			UPDATE
				"Devices"
			SET
				dut_id=COALESCE(NULLIF($2, ''), device_address),
				device_address=COALESCE(NULLIF($3, ''), device_address),
				device_type=COALESCE(NULLIF($4, ''), device_type),
				device_state=COALESCE(NULLIF($5, ''), device_state),
				schedulable_labels=COALESCE($6::jsonb, schedulable_labels),
				last_updated_time=NOW(),
				is_active=COALESCE($7, is_active)
			WHERE
				id=$1
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
				last_notification_time;`
	)

	if device.SchedulableLabels != nil {
		// Marshal labels and set to null
		labelBytes, err = json.Marshal(device.SchedulableLabels)
		if err != nil {
			return Device{}, err
		}

		err = device.SetDutIDFromLabels(ctx)
		if err != nil {
			logging.Errorf(ctx, "UpdateDevice: failed to set DUT ID for Device %s: %s", device.ID, err)
			return Device{}, err
		}
	}

	logging.Debugf(ctx, "UpdateDevice: %s", query)
	err = tx.QueryRowContext(ctx, query,
		device.ID,
		device.DutID,
		device.DeviceAddress,
		device.DeviceType,
		device.DeviceState,
		labelBytes,
		device.IsActive,
	).Scan(
		&updatedDevice.ID,
		&updatedDevice.DutID,
		&updatedDevice.DeviceAddress,
		&updatedDevice.DeviceType,
		&updatedDevice.DeviceState,
		&updatedDevice.SchedulableLabels,
		&updatedDevice.IsActive,
		&createdTime,
		&lastUpdatedTime,
		&lastNotificationTime,
	)

	// Handle possible null times
	if createdTime.Valid {
		updatedDevice.CreatedTime = createdTime.Time
	}
	if lastUpdatedTime.Valid {
		updatedDevice.LastUpdatedTime = lastUpdatedTime.Time
	}
	if lastNotificationTime.Valid {
		updatedDevice.LastNotificationTime = lastNotificationTime.Time
	}

	if err != nil {
		logging.Errorf(ctx, "UpdateDevice: failed to update Device %s to DB: %s", updatedDevice.ID, err)
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			logging.Errorf(ctx, "UpdateDevice: unable to rollback: %v", rollbackErr)
		}
		return Device{}, err
	}

	logging.Debugf(ctx, "UpdateDevice: Device %s updated successfully", updatedDevice.ID)
	return updatedDevice, nil
}

// UpdateDeviceToLeased updates a Device to Leased in a transaction.
//
// UpdateDeviceToLeased uses COALESCE to only update fields with provided
// values. If there is no value provided, then it will use the current value of
// the device field in the db. If no Device is returned by RETURNING, that means
// the Device was not found or already leased.
func UpdateDeviceToLeased(ctx context.Context, tx *sql.Tx, device Device, idType DeviceIDType) (Device, error) {
	var (
		err                  error
		updatedDevice        Device
		createdTime          sql.NullTime
		lastUpdatedTime      sql.NullTime
		lastNotificationTime sql.NullTime
		query                = `
			UPDATE
				"Devices"
			SET
				device_state='DEVICE_STATE_LEASED',
				last_updated_time=NOW()`
	)

	switch idType {
	case IDTypeDutID:
		// Use DUT ID type also known as Asset Tag.
		query += `
			WHERE
				jsonb_path_query_array(
					schedulable_labels,
					'$.dut_id.Values[0]'
				) @> to_jsonb($1::text)`
	case IDTypeHostname:
		// Use hostname which is how they are stored in DB.
		query += `
			WHERE
				id=$1`
	default:
		return Device{}, fmt.Errorf("UpdateDeviceToLeased: unsupported Device ID type: %s", idType)
	}
	query += `
				AND device_state='DEVICE_STATE_AVAILABLE'
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
				last_notification_time;`

	logging.Debugf(ctx, "UpdateDeviceToLeased: %s", query)
	err = tx.QueryRowContext(ctx, query,
		device.ID,
	).Scan(
		&updatedDevice.ID,
		&updatedDevice.DutID,
		&updatedDevice.DeviceAddress,
		&updatedDevice.DeviceType,
		&updatedDevice.DeviceState,
		&updatedDevice.SchedulableLabels,
		&updatedDevice.IsActive,
		&createdTime,
		&lastUpdatedTime,
		&lastNotificationTime,
	)

	// Handle possible null times
	if createdTime.Valid {
		updatedDevice.CreatedTime = createdTime.Time
	}
	if lastUpdatedTime.Valid {
		updatedDevice.LastUpdatedTime = lastUpdatedTime.Time
	}
	if lastNotificationTime.Valid {
		updatedDevice.LastNotificationTime = lastNotificationTime.Time
	}

	if err != nil {
		logging.Errorf(ctx, "UpdateDeviceToLeased: failed to update Device %s to DB: %s", updatedDevice.ID, err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			logging.Debugf(ctx, "UpdateDeviceToLeased: SQLSTATE:", pgErr.Code)
			logging.Debugf(ctx, "UpdateDeviceToLeased:", pgErr.Message)
		}

		// We know Device is found but already leased because calling this method
		// requires a found Device.
		if errors.Is(err, sql.ErrNoRows) || (errors.As(err, &pgErr) && pgErr.Code == "P0002") {
			logging.Errorf(ctx, "UpdateDeviceToLeased: device is not available: %v", err)
			return Device{}, ErrDeviceAlreadyLeased
		}

		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			logging.Errorf(ctx, "UpdateDeviceToLeased: unable to rollback: %v", rollbackErr)
		}
		return Device{}, err
	}

	logging.Debugf(ctx, "UpdateDeviceToLeased: Device %s updated successfully", updatedDevice.ID)
	return updatedDevice, nil
}

// UpsertDeviceFromUFS upserts a Device in a transaction.
//
// UpsertDeviceFromUFS will attempt to insert a Device pulled from UFS into the
// db. On conflict of the ID, the old device record will be updated with the new
// information except for device_address, device_type, and device_state. Those
// three will not be updated on conflict but should be inserted for new Devices.
func UpsertDeviceFromUFS(ctx context.Context, db *sql.DB, device Device) error {
	err := device.SetDutIDFromLabels(ctx)
	if err != nil {
		logging.Errorf(ctx, "UpsertDeviceFromUFS: failed to set DUT ID for Device %s: %s", device.ID, err)
		return err
	}

	result, err := db.ExecContext(ctx, `
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
			is_active=COALESCE(EXCLUDED.is_active, d.is_active);`,
		device.ID,
		device.DutID,
		device.DeviceAddress,
		device.DeviceType,
		device.DeviceState,
		device.SchedulableLabels,
		device.IsActive,
	)
	if err != nil {
		logging.Errorf(ctx, "UpsertDeviceFromUFS: failed to upsert Device %s: %s", device.ID, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logging.Errorf(ctx, "UpsertDeviceFromUFS: error getting rows affected: %s", err)
	}

	logging.Debugf(ctx, "UpsertDeviceFromUFS: Device %s upserted successfully (%d row affected)", device.ID, rowsAffected)
	return nil
}

// DUTID returns the DUT ID (i.e. Swarming asset tag) for the given device.
func (d *Device) DUTID() (string, error) {
	idLabel, ok := d.SchedulableLabels[string(IDTypeDutID)]
	if !ok || len(idLabel.Values) == 0 {
		return "", fmt.Errorf("found no DUT ID for device %v", d.ID)
	}
	if len(idLabel.Values) > 1 {
		return "", fmt.Errorf("found multiple DUT IDs for device %v", d.ID)
	}
	return idLabel.Values[0], nil
}

// SetDutIDFromLabels takes dut_id (asset tag) from the schedulable labels.
//
// SetDutIDFromLabels take the label and sets it to the Device model. If no
// labels are found, then the DUT ID will also not be set.
func (d *Device) SetDutIDFromLabels(ctx context.Context) error {
	if len(d.DutID) != 0 {
		logging.Warningf(ctx, "dut_id %s will be overridden by the schedulable label value", d.DutID)
		d.DutID = ""
	}

	if d.SchedulableLabels == nil {
		return fmt.Errorf("no labels provided")
	}

	// Extract DUT ID from labels and set DutID.
	dutIDLabel, ok := d.SchedulableLabels["dut_id"]
	if !ok {
		return fmt.Errorf("failed to get dut_id from Device %s", d.ID)
	}
	dutIDVals := dutIDLabel.Values
	switch len(dutIDVals) {
	case 1:
		logging.Debugf(ctx, "Extracted dut_id from schedulable labels: %s", dutIDVals[0])
		d.DutID = dutIDVals[0]
		return nil
	case 0:
		return fmt.Errorf("no value for DUT ID found for Device %s", d.ID)
	default:
		return fmt.Errorf("multiple values for DUT ID found for Device %s", d.ID)
	}
}
