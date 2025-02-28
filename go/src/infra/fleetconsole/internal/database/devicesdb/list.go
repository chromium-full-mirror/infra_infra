// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/utils"
)

func List(ctx context.Context, dbConn *sql.DB, filter, orderby string, offset, pageSize int) ([]*fleetconsolerpc.Device, bool, error) {
	// Fetch one extra row to check whether there is more data available
	query, err := buildListDevicesQuery(offset, pageSize+1, filter, orderby)
	hasMoreData := false
	if err != nil {
		logging.Errorf(ctx, "failed to construct the query: %s", err)
		return nil, true, err
	}

	rows, err := dbConn.QueryContext(ctx, query.Statement, query.Parameters...)
	if err != nil {
		logging.Errorf(ctx, "failed to read from the DB: %s", err)
		return nil, true, fmt.Errorf("DB QueryContext: %w", err)
	}
	defer rows.Close()

	var results []*DeviceDAO
	for rows.Next() {
		var (
			device      DeviceDAO
			host        string
			portDB      string
			deviceType  string
			deviceState string
			labelsJSON  []byte
		)
		err := rows.Scan(
			&device.Id,
			&device.DutId,
			&host,
			&portDB,
			&deviceType,
			&deviceState,
			&labelsJSON,
		)

		if err != nil {
			return nil, hasMoreData, fmt.Errorf("ListDevices: %w", err)
		}

		port := 0
		if portDB != "" {
			port, err = strconv.Atoi(portDB)

			if err != nil {
				return nil, hasMoreData, fmt.Errorf("invalid port value: %s", portDB)
			}
		}

		device.Type = deviceType
		device.State = deviceState
		device.Address = &DeviceAddressDAO{
			Host: host,
			Port: int32(port),
		}
		device.DeviceSpec = &DeviceSpecDAO{
			Labels: make(map[string]*LabelValuesDAO),
		}
		err = json.Unmarshal(labelsJSON, &device.DeviceSpec.Labels)
		if err != nil {
			return nil, hasMoreData, fmt.Errorf("ListDevices: %w", err)
		}

		results = append(results, &device)
	}

	if err := rows.Close(); err != nil {
		return nil, hasMoreData, fmt.Errorf("ListDevices: %w", err)
	}

	if err := rows.Err(); err != nil {
		return nil, hasMoreData, fmt.Errorf("ListDevices: %w", err)
	}

	// As mentioned earlier one extra row is fetched.
	if len(results) > pageSize {
		hasMoreData = true
		results = results[:pageSize]
	}

	return utils.Map(results, ToListDevicesDevice), hasMoreData, nil
}
