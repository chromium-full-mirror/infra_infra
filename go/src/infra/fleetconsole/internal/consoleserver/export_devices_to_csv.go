// Copyright 2025 The Chromium Authors
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

func (frontend *FleetConsoleFrontend) ExportDevicesToCSV(ctx context.Context, req *fleetconsolerpc.ExportDevicesToCSVRequest) (*fleetconsolerpc.ExportDevicesToCSVResponse, error) {

	realms, err := devicesdb.GetUserRealms(ctx, frontend.cloudProject)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errors.Annotate(err, "failed to get user realms").Tag(grpcutil.CanceledTag).Err()
		}

		logging.Errorf(ctx, "failed to get user realms: %s", err)
		return nil, err
	}

	csvData, err := devicesdb.ExportCSV(ctx, sqldb.MustGetDB(ctx), req.Columns, req.Filter, req.OrderBy, realms)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errors.Annotate(err, "failed to export devices to CSV").Tag(grpcutil.CanceledTag).Err()
		}

		logging.Errorf(ctx, "failed to export devices to CSV: %s", err)
		return nil, err
	}

	return &fleetconsolerpc.ExportDevicesToCSVResponse{
		CsvData: csvData,
	}, nil

}
