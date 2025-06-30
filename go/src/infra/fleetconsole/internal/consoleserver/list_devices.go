// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/sqldb"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/database/devicesdb"
)

const maxPageSize int = 1000

// ListDevices lists devices from the db.
func (frontend *FleetConsoleFrontend) ListDevices(ctx context.Context, req *fleetconsolerpc.ListDevicesRequest) (_ *fleetconsolerpc.ListDevicesResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	offset, err := pageTokenToOffset(req.PageToken, req.Filter, req.OrderBy)
	if err != nil {
		logging.Errorf(ctx, "failed to extract page token: %s", err)
		return nil, err
	}

	pageSize := maxPageSize
	if req.PageSize != 0 {
		pageSize = min(int(req.PageSize), maxPageSize)
	}

	realms, err := devicesdb.GetUserRealms(ctx, frontend.cloudProject)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errors.Annotate(err, "failed to get user realms").Tag(grpcutil.CanceledTag).Err()
		}

		logging.Errorf(ctx, "failed to get user realms: %s", err)
		return nil, err
	}

	results, hasMoreData, err := devicesdb.List(ctx, sqldb.MustGetDB(ctx), req.Filter, req.OrderBy, offset, pageSize, realms)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errors.Annotate(err, "failed to list devices").Tag(grpcutil.CanceledTag).Err()
		}

		logging.Errorf(ctx, "failed to list devices: %s", err)
		return nil, err
	}

	var nextPageToken string
	if hasMoreData {
		nextPageToken, err = offsetToPageToken(offset+pageSize, req.Filter, req.OrderBy)
		if err != nil {
			logging.Errorf(ctx, "failed to encode next page token: %s", err)
			return nil, err
		}
	}

	return &fleetconsolerpc.ListDevicesResponse{
		Devices:       results,
		NextPageToken: nextPageToken,
	}, nil
}
