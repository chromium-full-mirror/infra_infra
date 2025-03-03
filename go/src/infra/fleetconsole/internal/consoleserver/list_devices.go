// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/database/devicesdb"
	"go.chromium.org/infra/fleetconsole/internal/utils"
)

const maxPageSize int = 50

// ListDevices lists devices from the db.
func (frontend *FleetConsoleFrontend) ListDevices(ctx context.Context, req *fleetconsolerpc.ListDevicesRequest) (_ *fleetconsolerpc.ListDevicesResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()

	offset, err := listDevicesPageTokenToOffset(req)
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
		return nil, err
	}

	results, hasMoreData, err := devicesdb.List(ctx, frontend.dbConnection, req.Filter, req.OrderBy, offset, pageSize, realms)
	if err != nil {
		return nil, err
	}

	var nextPageToken string
	if hasMoreData {
		nextPageToken, err = listDevicesOffsetToPageToken(offset+pageSize, req)
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

func listDevicesPageTokenToOffset(req *fleetconsolerpc.ListDevicesRequest) (int, error) {
	return utils.PageTokenToOffset(req.GetPageToken(), map[string]string{
		"filter":   req.GetFilter(),
		"order_by": req.GetOrderBy(),
	})
}

func listDevicesOffsetToPageToken(offset int, req *fleetconsolerpc.ListDevicesRequest) (string, error) {
	return utils.OffsetToPageToken(offset, map[string]string{
		"filter":   req.GetFilter(),
		"order_by": req.GetOrderBy(),
	})
}
