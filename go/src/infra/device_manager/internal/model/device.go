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
	"strings"
	"time"

	"github.com/jackc/pgconn"

	lucierr "go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/device_manager/internal/database"
	"go.chromium.org/infra/libs/skylab/inventory/swarming"
)

// Error types for Device model operations
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

	// IsActive marks if a Device should be schedulable on its own (actively
	// schedulable). DUTs part of a SchedulingUnit as defined in UFS will be
	// marked inactive.
	IsActive bool
	// CreatedTime is alwasys not NULL, so we don't have to use sql.NullTime.
	CreatedTime                  time.Time
	LastUpdatedTime              time.Time // Deprecated. Use NullableLastUpdatedTime instead.
	LastUpdatedTimeNullable      sql.NullTime
	LastNotificationTime         time.Time // Deprecated. Use NullableLastNotificationTime instead.
	LastNotificationTimeNullable sql.NullTime
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
func (s *SchedulableLabels) Scan(value any) error {
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
			dut_id,
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
		// Use DUT ID type (Asset Tag).
		query += `
			WHERE dut_id=$1;`
	case IDTypeHostname:
		// Use hostname which is how they are stored in DB.
		query += `
			WHERE id=$1;`
	default:
		return Device{}, fmt.Errorf("GetDeviceByID: unsupported Device ID type: %s", idType)
	}

	err := db.QueryRowContext(ctx, query, deviceID).Scan(
		&device.ID,
		&device.DutID,
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
			&device.DutID,
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

// ExpireLeasesCron expires all leases that haven't been modified since the
// given timestamp, and returns the expired lease IDs and associated device IDs
// within a cron job.
func ExpireLeasesCron(ctx context.Context, tx *sql.Tx, t time.Time) (leaseIDs, deviceIDs []string, err error) {
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
func buildListDevicesQuery(ctx context.Context, pageToken database.PageToken, pageSize int, filter string) (string, []any, error) {
	var queryArgs []any
	query := `
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

// UpdateDeviceToAvailable updates a Device to available in a transaction.
//
// UpdateDeviceToAvailable requires both id and dut_id. This is to ensure the
// Device's unique hostname to asset tag pairing. The function uses COALESCE to
// only update fields with provided values. If there is no value provided, then
// it will use the current value of the device field in the db.
func UpdateDeviceToAvailable(ctx context.Context, tx *sql.Tx, device Device) (*Device, error) {
	if device.SchedulableLabels == nil {
		return nil, fmt.Errorf("update device %q to available: empty schedulabe labels", device.ID)
	}
	if device.DutID == "" {
		return nil, fmt.Errorf("update device %q to available: empty dut_id", device.ID)
	}
	// Marshal labels and set to null
	labelBytes, err := json.Marshal(device.SchedulableLabels)
	if err != nil {
		return nil, fmt.Errorf("update device %q to available: %w", device.ID, err)
	}
	var (
		updatedDevice        Device
		createdTime          sql.NullTime
		lastUpdatedTime      sql.NullTime
		lastNotificationTime sql.NullTime
		query                = `
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
				last_notification_time;`
	)

	logging.Debugf(ctx, "UpdateDeviceToAvailable: %s", query)
	err = tx.QueryRowContext(ctx, query,
		device.ID,
		device.DutID,
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
		logging.Errorf(ctx, "UpdateDeviceToAvailable: failed to update Device %s to DB: %s", updatedDevice.ID, err)
		return nil, fmt.Errorf("update device to available: %w", err)
	}

	logging.Debugf(ctx, "UpdateDeviceToAvailable: Device %s updated successfully", updatedDevice.ID)
	return &updatedDevice, nil
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
				dut_id=$1`
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

		return Device{}, err
	}

	logging.Debugf(ctx, "UpdateDeviceToLeased: Device %s updated successfully", updatedDevice.ID)
	return updatedDevice, nil
}

// BulkUpdateDevicesToLeased marks a list of Devices as leased.
//
// BulkUpdateDevicesToLeased takes a list of Device IDs (either dut_id asset tag
// or hostname) and marks them as LEASED. The query only tries to mark
// DEVICE_STATE_AVAILABLE Devices. Anything no found by the query is considered
// to be already leased. A list of updated Devices and associated errors are
// returned.
//
// TODO (justinsuen): Need a way to know which ones are not found.
func BulkUpdateDevicesToLeased(ctx context.Context, tx *sql.Tx, deviceIDs []string, idType DeviceIDType) (map[string]*Device, map[string]error, error) {
	var (
		// A map for DUT ID to updated Device
		updateSuccess = map[string]*Device{}
		// A map for DUT ID to error
		updateErrs = map[string]error{}
		query      = `
			UPDATE
				"Devices"
			SET
				device_state='DEVICE_STATE_LEASED',
				last_updated_time=NOW()
			WHERE
				dut_id IN (%s)
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
				last_notification_time;`
	)

	// Put quotes around IDs and prepopulate errors
	var deviceIDsQuoted []string
	for _, deviceID := range deviceIDs {
		// Dedup the device IDs and use the exact count to verify the update result.
		if _, ok := updateErrs[deviceID]; !ok {
			updateErrs[deviceID] = ErrDeviceAlreadyLeased
			deviceIDsQuoted = append(deviceIDsQuoted, fmt.Sprintf("'%s'", deviceID))
		} else {
			logging.Debugf(ctx, "BulkUpdateDevicesToLeased: duplicated device ID found: %s", deviceID)
		}
	}

	logging.Debugf(ctx, "BulkUpdateDeviceToLeased: update statement: %s\n with Devices %+v", query, deviceIDs)
	query = fmt.Sprintf(query, strings.Join(deviceIDsQuoted, ", "))
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		logging.Debugf(ctx, "BulkUpdateDeviceToLeased: rolling back: %s", err)
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, nil, fmt.Errorf("bulk update devices to leased: rollback: %w", rollbackErr)
		}
		return nil, nil, fmt.Errorf("bulk update devices to leased: execute query: %w", err)
	}
	defer rows.Close()

	// Process Devices and populate actually leased Devices
	for rows.Next() {
		var (
			updatedDevice        Device
			createdTime          sql.NullTime
			lastUpdatedTime      sql.NullTime
			lastNotificationTime sql.NullTime
		)
		err := rows.Scan(
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
			logging.Errorf(ctx, "BulkUpdateDeviceToLeased: failed to update Device to DB: %w", err)
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				logging.Debugf(ctx, "BulkUpdateDeviceToLeased: SQLSTATE:", pgErr.Code)
				logging.Debugf(ctx, "BulkUpdateDeviceToLeased:", pgErr.Message)
			}
			continue
		}
		if d, ok := updateSuccess[updatedDevice.DutID]; ok {
			logging.Errorf(ctx, "BulkUpdateDeviceToLeased: duplicated DUT id: %v vs %v", d, updatedDevice)
			continue
		}
		updateSuccess[updatedDevice.DutID] = &updatedDevice
		updateErrs[updatedDevice.DutID] = nil
	}
	return updateSuccess, updateErrs, nil
}

// UpsertDeviceFromUFS upserts a Device in a transaction.
//
// UpsertDeviceFromUFS will attempt to insert a Device pulled from UFS into the
// db. On conflict of the ID, the old device record will be updated with the new
// information except for device_address, device_type, and device_state. Those
// three will not be updated on conflict but should be inserted for new Devices.
func UpsertDeviceFromUFS(ctx context.Context, db *sql.DB, device Device) error {
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

// ApplySwarmingDims applies swarming Dimensions to the device object.
//
// Mostly it set the scheduleable labels and the dut id field. If no dut_id dim
// are found, then the DUT ID will also not be set.
func (d *Device) ApplySwarmingDims(ctx context.Context, dims swarming.Dimensions) error {
	if dims == nil {
		// It's doubtful that if dims can be nil. But just regard it as OK here to
		// keep the same behavior of the old code.
		logging.Warningf(ctx, "%q:%q dims is nil", d.DutID, d.ID)
		return nil
	}
	d.SchedulableLabels = swarmingDimsToLabels(dims)
	newDutID, err := getDutIDLabel(d.SchedulableLabels)
	if err != nil {
		logging.Warningf(ctx, "dut_id %q of %q is cleared: %s", d.DutID, d.ID, err)
		d.DutID = ""
		return fmt.Errorf("apply swarming dims to labels: %w", err)
	}
	if d.DutID != "" && d.DutID != newDutID {
		logging.Warningf(ctx, "dut_id %q of %q will be overridden to %q by the scheduleable label value", d.DutID, d.ID, newDutID)
	}
	d.DutID = newDutID
	return nil
}

// swarmingDimsToLabels converts SwarmingDimensions to Device Manager
// SchedulableLabels.
func swarmingDimsToLabels(dims swarming.Dimensions) SchedulableLabels {
	schedLabels := make(SchedulableLabels)
	for k, v := range dims {
		schedLabels[k] = LabelValues{
			Values: v,
		}
	}
	return schedLabels
}

func getDutIDLabel(labels SchedulableLabels) (string, error) {
	if labels == nil {
		return "", fmt.Errorf("get dut_id label: the scheduleable labels is nil")
	}

	// Extract DUT ID from labels and set DutID.
	dutIDLabel, ok := labels["dut_id"]
	if !ok {
		return "", fmt.Errorf("get dut_id label: no dut_id in labels")
	}
	dutIDVals := dutIDLabel.Values
	switch len(dutIDVals) {
	case 1:
		return dutIDVals[0], nil
	case 0:
		return "", fmt.Errorf("get dut_id label: dut_id is empty in labels")
	default:
		return "", fmt.Errorf("get dut_id label: multiple dut_id values: %v", dutIDVals)
	}
}
