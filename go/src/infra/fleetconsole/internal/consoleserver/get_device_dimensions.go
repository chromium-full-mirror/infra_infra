// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/database/devicesdb"
)

// GetDeviceDimensions returns dimensions of all devices
func (frontend *FleetConsoleFrontend) GetDeviceDimensions(ctx context.Context, req *emptypb.Empty) (_ *fleetconsolerpc.GetDeviceDimensionsResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	labels, err := devicesdb.GetLabels(ctx, frontend.dbConnection)
	if err != nil {
		logging.Errorf(ctx, "failed to fetch label values: %s", err)
		return nil, err
	}

	baseDimensions, err := devicesdb.GetBaseDimensions(ctx, frontend.dbConnection)
	if err != nil {
		logging.Errorf(ctx, "failed to fetch base dimension values: %s", err)
		return nil, err
	}

	return &fleetconsolerpc.GetDeviceDimensionsResponse{
		BaseDimensions: baseDimensions,
		Labels:         labels,
	}, nil
}
