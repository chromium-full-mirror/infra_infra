// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/database/devicesdb"
	"go.chromium.org/infra/fleetconsole/internal/devicemanagerclient"
	"go.chromium.org/infra/fleetconsole/internal/utils"
)

// The sql library doesn't support more than this number of parameters
const maxQueryParametersCount = 65535
const parametersPerDevice = 7

func (frontend *FleetConsoleFrontend) RepopulateCache(ctx context.Context, req *fleetconsolerpc.RepopulateCacheRequest) (_ *fleetconsolerpc.RepopulateCacheResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	deviceManagerClient, err := frontend.deviceManagerClient(ctx, frontend.cloudProject)
	devices, err := getAllDevices(ctx, deviceManagerClient)
	if err != nil {
		return nil, err
	}
	logging.Infof(ctx, "Got %d devices", len(devices))

	saveDevices(ctx, frontend.dbConnection, devices)

	err = deleteOtherDevices(ctx, frontend.dbConnection, devices)
	if err != nil {
		logging.Warningf(ctx, "Error while deleting old devices", err)
	}

	return &fleetconsolerpc.RepopulateCacheResponse{}, nil
}

func getAllDevices(ctx context.Context, deviceManagerClient *devicemanagerclient.Client) ([]*devicesdb.DeviceDAO, error) {
	var devices []*devicesdb.DeviceDAO
	nextPageToken := ""
	for {
		res, err := deviceManagerClient.Leaser.ListDevices(ctx, &api.ListDevicesRequest{
			PageToken: nextPageToken,
		})
		if err != nil {
			return nil, err
		}

		devices = append(devices, utils.Map[*api.Device, *devicesdb.DeviceDAO](res.Devices, devicesdb.FromDeviceManagerDevice)...)

		nextPageToken = res.GetNextPageToken()
		if nextPageToken == "" {
			break
		}
	}

	return devices, nil
}

func saveDevices(ctx context.Context, dbConnection *sql.DB, devices []*devicesdb.DeviceDAO) {
	q := `INSERT INTO "Devices" (
			id,
			dut_id,
			host,
			port,
			type,
			state,
			labels
		)
		VALUES %s
		ON CONFLICT (id) DO UPDATE SET
			dut_id=EXCLUDED.dut_id,
			host=EXCLUDED.host,
			port=EXCLUDED.port,
			type=EXCLUDED.type,
			state=EXCLUDED.state,
			labels=EXCLUDED.labels
		`

	for devicesChunk := range slices.Chunk(devices, maxQueryParametersCount/parametersPerDevice) {
		args := utils.FlatMap(devicesChunk, func(d *devicesdb.DeviceDAO) []any { return d.DeviceAsDBArguments() })

		_, err := dbConnection.ExecContext(ctx,
			fmt.Sprintf(q, getValuesString(len(args), parametersPerDevice)),
			args...)
		if err != nil {
			logging.Warningf(ctx, "Failed to write device %v\n", err)
		}
	}
}

// Returns a string in the shape of "($1, $2, $3), ($4, $5, $6)"
// lenValues is the total number of values (6 in the example above)
// numberOfArgs is the number of args in each parenthesis (3 in the example above)
//
// If lenValues is not cleanly divisible by numberOfArgs the remaining values will be ignored:
// E.G: getValuesString(5, 2) = "($1, $2), ($3, $4)"
func getValuesString(lenValues int, numberOfArgs int) string {
	values := make([]string, lenValues/numberOfArgs)

	for i := 0; i < lenValues/numberOfArgs; i++ {
		inner := make([]string, numberOfArgs)
		for j := 0; j < numberOfArgs; j++ {
			inner[j] = fmt.Sprintf("$%d", j+i*numberOfArgs+1)
		}
		values[i] = "(" + strings.Join(inner, ", ") + ")"
	}
	return strings.Join(values, ", ")
}

func deleteOtherDevices(ctx context.Context, dbConnection *sql.DB, devices []*devicesdb.DeviceDAO) error {
	if len(devices) > maxQueryParametersCount {
		return fmt.Errorf("cannot perform delete with more than %d devices, got %d", maxQueryParametersCount, len(devices))
	}

	deviceIds := make([]any, len(devices))
	for i, d := range devices {
		deviceIds[i] = d.Id
	}

	res, err := dbConnection.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM "Devices" where id NOT IN (%s)`, getValuesString(len(devices), 1)),
		deviceIds...,
	)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	logging.Infof(ctx, "Deleted %d devices\n", n)
	return err
}
