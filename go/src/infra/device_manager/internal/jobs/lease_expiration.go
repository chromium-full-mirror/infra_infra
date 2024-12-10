// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/device_manager/internal/controller"
	"infra/device_manager/internal/external"
	"infra/device_manager/internal/frontend"
	"infra/device_manager/internal/model"
	"infra/libs/fleet/device"
	ufsUtil "infra/unifiedfleet/app/util"
)

// ExpireLeases ends all expired Leases and release the corresponding Devices.
func ExpireLeases(ctx context.Context, serviceClients frontend.ServiceClients) error {
	// Use current time to guard against lease updates during this expiry op.
	readTime := time.Now()

	tx, err := serviceClients.DBClient.Conn.BeginTx(ctx, nil)
	if err != nil {
		err = errors.Annotate(err, "ExpireLeases: starting database transaction").Err()
		logging.Errorf(ctx, err.Error())
		return err
	}

	// Mark releases as expired, and read expired lease IDs and their associated
	// device IDs.
	leaseIDs, deviceIDs, err := model.ExpireLeasesCron(ctx, tx, readTime)
	if err != nil {
		err = errors.Annotate(err, "ExpireLeases: attempting to expire leases in DB").Err()
		logging.Errorf(ctx, err.Error())
		return err
	}

	// Try to pull updated dimensions for devices; mark as inactive if not found.
	// NOTE: this is not a batch operation as we serialize the requests to UFS, so
	// large batches of devices may lock up rows for a long time.
	deviceIDs = removeDupeDevices(deviceIDs)
	updatedDevices, err := constructUpdatedDevices(ctx, deviceIDs)
	if err != nil {
		err = errors.Annotate(err, "ExpireLeases: pulling dimensions for released devices").Err()
		logging.Errorf(ctx, err.Error())
		return err
	}

	// Mark associated devices as released.
	err = bulkReleaseDevices(ctx, tx, updatedDevices)
	if err != nil {
		err = errors.Annotate(err, "ExpireLeases: committing transaction").Err()
		logging.Errorf(ctx, err.Error())
		return err
	}

	// Log updates once we're ready to commit the transaction.
	logging.Debugf(ctx, "released %d leases: %v", len(leaseIDs), leaseIDs)
	logging.Debugf(ctx, "released %d devices: %v", len(deviceIDs), deviceIDs)

	// Commit transaction.
	if err = tx.Commit(); err != nil {
		err = errors.Annotate(err, "ExpireLeases: committing transaction").Err()
		logging.Errorf(ctx, err.Error())
		return err
	}

	return nil
}

// constructUpdatedDevices constructs Devices using updated information.
func constructUpdatedDevices(ctx context.Context, deviceIDs []string) ([]model.Device, error) {
	// Skip if no devices to update.
	if len(deviceIDs) == 0 {
		return nil, nil
	}

	ctx = external.SetupContext(ctx, ufsUtil.OSNamespace)
	client, err := external.NewUFSClient(ctx, external.UFSServiceURI)
	if err != nil {
		logging.Errorf(ctx, "failed to get UFS client: %s", err)
		return nil, err
	}

	reportFunc := func(e error) { logging.Debugf(ctx, "sanitize dimensions: %s\n", e) }
	updatedDevices := make([]model.Device, len(deviceIDs))

	// TODO: parallelize getting dims from UFS
	for i, id := range deviceIDs {
		d := model.Device{
			ID:          id,
			DeviceState: "DEVICE_STATE_AVAILABLE",
			IsActive:    true,
		}
		dims, err := device.GetOSResourceDims(ctx, client, reportFunc, id)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				d.IsActive = false
			} else {
				logging.Errorf(ctx, "failed to get dims for %s: %s", id, err)
			}
		}

		if dims != nil {
			d.SchedulableLabels = controller.SwarmingDimsToLabels(ctx, dims)
		}
		updatedDevices[i] = d
	}
	return updatedDevices, nil
}

// removeDupeDevices removes duplicate devices in case there are multiple
// unexpired leases.
func removeDupeDevices(deviceIDs []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, val := range deviceIDs {
		if _, ok := seen[val]; !ok {
			seen[val] = true
			result = append(result, val)
		}
	}
	return result
}

// bulkReleaseDevices releases a list of Devices in bulk
func bulkReleaseDevices(ctx context.Context, tx *sql.Tx, updatedDevices []model.Device) error {
	// Skip if no devices to release.
	if len(updatedDevices) == 0 {
		return nil
	}

	var (
		createQuery = `
			CREATE TEMPORARY TABLE temp_devices (
				id VARCHAR PRIMARY KEY,
				device_state VARCHAR,
				is_active BOOL,
				schedulable_labels JSONB,
				last_updated_time TIMESTAMP WITHOUT TIME ZONE
			) ON COMMIT DROP;`
		insertStmt = `
			INSERT INTO temp_devices (
				id,
				device_state,
				is_active,
				schedulable_labels,
				last_updated_time
			) VALUES %s;`
		updateQuery = `
			UPDATE "Devices" d
			SET
				device_state = td.device_state,
				is_active = td.is_active,
				schedulable_labels = td.schedulable_labels,
				last_updated_time = td.last_updated_time
			FROM temp_devices td
			WHERE d.id = td.id AND
				d.last_updated_time < td.last_updated_time;`
	)

	// Create temporary table.
	_, err := tx.ExecContext(ctx, createQuery)
	if err != nil {
		logging.Errorf(ctx, "error creating temp table: %w", err)
		return err
	}

	// Populate temporary table.
	var valueStrings []string
	var valueArgs []interface{}
	for _, device := range updatedDevices {
		l := len(valueArgs)
		valueStrings = append(
			valueStrings,
			fmt.Sprintf("($%d, $%d, $%d, $%d, NOW())", l+1, l+2, l+3, l+4),
		)
		valueArgs = append(
			valueArgs,
			device.ID,
			device.DeviceState,
			device.IsActive,
			device.SchedulableLabels,
		)
	}

	stmt := fmt.Sprintf(insertStmt, strings.Join(valueStrings, ","))
	logging.Debugf(ctx, "Insert statement: %s", stmt)
	res, err := tx.ExecContext(ctx, stmt, valueArgs...)
	if err != nil {
		logging.Errorf(ctx, "error inserting batch into temp table: %w", err)
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		logging.Errorf(ctx, err.Error())
	}
	logging.Debugf(ctx, "Inserted %d rows in temp Devices table", n)

	// Perform the bulk update and mark Devices as available.
	res, err = tx.ExecContext(ctx, updateQuery)
	if err != nil {
		logging.Errorf(ctx, "error updating Devices table: %w", err)
		return err
	}

	n, err = res.RowsAffected()
	if err != nil {
		logging.Errorf(ctx, err.Error())
	}
	logging.Debugf(ctx, "Batch updated %d rows in Devices table", n)
	return nil
}
