// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"go.chromium.org/luci/grpc/grpcutil"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/database/devicesdb"
)

func (frontend *FleetConsoleFrontend) CountDevices(ctx context.Context, req *fleetconsolerpc.CountDevicesRequest) (_ *fleetconsolerpc.CountDevicesResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	result, err := devicesdb.CountDevices(ctx, frontend.dbConnection, req.GetFilter())
	if err != nil {
		return nil, err
	}

	return result, nil

}
