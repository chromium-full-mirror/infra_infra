// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/database/devicesdb"
)

func (frontend *FleetConsoleFrontend) CountDevices(ctx context.Context, req *fleetconsolerpc.CountDevicesRequest) (_ *fleetconsolerpc.CountDevicesResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	realms, err := devicesdb.GetUserRealms(ctx, frontend.cloudProject)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errors.Annotate(err, "failed to fetch user realms").Tag(grpcutil.CanceledTag).Err()
		}

		logging.Errorf(ctx, "failed to fetch user realms: %s", err)
		return nil, err
	}

	result, err := devicesdb.CountDevices(ctx, frontend.dbConnection, req.GetFilter(), realms)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errors.Annotate(err, "failed to count devices").Tag(grpcutil.CanceledTag).Err()
		}

		logging.Errorf(ctx, "failed to count devices: %s", err)
		return nil, err
	}

	return result, nil

}
