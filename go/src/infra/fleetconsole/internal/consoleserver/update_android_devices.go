// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"database/sql"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/sqldb"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	updateandroiddevices "go.chromium.org/infra/fleetconsole/internal/consoleserver/update_android_devices"
)

// UpdateAndroidDevices receives a LabResource message and process it.
// Maybe this shouldn't be part of FleetConsoleFrontend
func (frontend *FleetConsoleFrontend) UpdateAndroidDevices(ctx context.Context, req *fleetconsolerpc.UpdateAndroidDevicesRequest) (_ *fleetconsolerpc.UpdateAndroidDevicesResponse, err error) {
	defer func() {
		if err != nil {
			logging.Errorf(ctx, "UpdateAndroidDevices: %v", err)
		}
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()

	db := sqldb.MustGetDB(ctx)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.Annotate(err, "failed to begin transaction").Err()
	}
	defer func() {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			logging.Errorf(ctx, "failed to rollback transaction: %v", rollbackErr)
		}
	}()

	err = updateandroiddevices.UpdateAndroidDevices(ctx, tx, req.Host)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, errors.Annotate(err, "failed to commit transaction").Err()
	}

	return &fleetconsolerpc.UpdateAndroidDevicesResponse{}, nil
}
