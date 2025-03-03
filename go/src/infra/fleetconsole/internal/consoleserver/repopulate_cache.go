// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sync"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/database/devicesdb"
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
	"go.chromium.org/infra/fleetconsole/internal/devicemanagerclient"
	"go.chromium.org/infra/fleetconsole/internal/ufsclient"
	"go.chromium.org/infra/fleetconsole/internal/utils"
	ufsmodel "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

// The sql library doesn't support more than this number of parameters
const maxQueryParametersCount = 65535
const parametersPerDevice = 8

// RepopulateCache repopulates the AlloyDB cache.
func (frontend *FleetConsoleFrontend) RepopulateCache(ctx context.Context, req *fleetconsolerpc.RepopulateCacheRequest) (_ *fleetconsolerpc.RepopulateCacheResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	deviceManagerClient, err := frontend.deviceManagerClient(ctx, frontend.cloudProject)
	if err != nil {
		return nil, err
	}
	// Device manager always uses prod ufs even in it's dev environment,
	// this means that we also need to use ufs prod in our dev, otherwise
	// we will not be able to match deviceManager devices with ufs devices
	ufsClient, err := frontend.ufsClient(ctx, "fleet-console-prod")
	if err != nil {
		return nil, err
	}

	devices, err := getAllDevices(ctx, deviceManagerClient, ufsClient)
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

func getAllDevices(ctx context.Context, deviceManagerClient *devicemanagerclient.Client, ufsClient ufsclient.Client) ([]*devicesdb.DeviceDAO, error) {
	var wg sync.WaitGroup

	wg.Add(1)
	states := make(map[string]api.DeviceState)
	var dmErr error
	go func() {
		devices, errInner := devicemanagerclient.GetAllDmDevices(ctx, deviceManagerClient)
		if errInner != nil {
			dmErr = errInner
			return
		}

		for _, d := range devices {
			states[d.Id] = d.State
		}
		wg.Done()
	}()

	wg.Add(1)
	var devicesUfs []*ufsmodel.DeviceLabels
	var ufsErr error
	go func() {
		devicesUfs, ufsErr = ufsclient.GetAllUfsDevices(ctx, ufsClient)
		wg.Done()
	}()
	wg.Wait()

	if dmErr != nil || ufsErr != nil {
		return nil, fmt.Errorf("Got an error while fetching data, dmErr = %v, ufsErr = %v", dmErr, ufsErr)
	}

	devices := make([]*devicesdb.DeviceDAO, len(devicesUfs))
	for i, d := range devicesUfs {
		state := states[devicesdb.IdFromUfsName(d.Name)].String()
		devices[i] = devicesdb.FromUfsDevice(d, state)
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
			labels,
			realm
		)
		VALUES %s
		ON CONFLICT (id) DO UPDATE SET
			dut_id=EXCLUDED.dut_id,
			host=EXCLUDED.host,
			port=EXCLUDED.port,
			type=EXCLUDED.type,
			state=EXCLUDED.state,
			labels=EXCLUDED.labels,
			realm=EXCLUDED.realm
		`

	for devicesChunk := range slices.Chunk(devices, maxQueryParametersCount/parametersPerDevice) {
		args := utils.FlatMap(devicesChunk, func(d *devicesdb.DeviceDAO) []any { return d.DeviceAsDBArguments() })

		_, err := dbConnection.ExecContext(ctx,
			fmt.Sprintf(q, queryutils.ValuesString(len(args), parametersPerDevice)),
			args...)
		if err != nil {
			logging.Warningf(ctx, "Failed to write device %v\n", err)
		}
	}
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
		fmt.Sprintf(`DELETE FROM "Devices" where id NOT IN (%s)`, queryutils.ValuesString(len(devices), 1)),
		deviceIds...,
	)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	logging.Infof(ctx, "Deleted %d devices\n", n)
	return err
}
