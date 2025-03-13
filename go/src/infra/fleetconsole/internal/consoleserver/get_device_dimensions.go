// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/sqldb"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/database/devicesdb"
)

// GetDeviceDimensions returns dimensions of all devices
func (frontend *FleetConsoleFrontend) GetDeviceDimensions(ctx context.Context, req *emptypb.Empty) (_ *fleetconsolerpc.GetDeviceDimensionsResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	realms, err := devicesdb.GetUserRealms(ctx, frontend.cloudProject)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errors.Annotate(err, "failed to fetch user realms").Tag(grpcutil.CanceledTag).Err()
		}

		logging.Errorf(ctx, "failed to fetch user realms: %s", err)
		return nil, err
	}

	labels, err := devicesdb.GetLabels(ctx, sqldb.MustGetDB(ctx), realms)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errors.Annotate(err, "failed to fetch label values").Tag(grpcutil.CanceledTag).Err()
		}

		logging.Errorf(ctx, "failed to fetch label values: %s", err)
		return nil, err
	}

	baseDimensions, err := devicesdb.GetBaseDimensions(ctx, sqldb.MustGetDB(ctx), realms)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errors.Annotate(err, "failed to fetch base dimension values").Tag(grpcutil.CanceledTag).Err()
		}

		logging.Errorf(ctx, "failed to fetch base dimension values: %s", err)
		return nil, err
	}

	return &fleetconsolerpc.GetDeviceDimensionsResponse{
		BaseDimensions: baseDimensions,
		Labels:         labels,
	}, nil
}
