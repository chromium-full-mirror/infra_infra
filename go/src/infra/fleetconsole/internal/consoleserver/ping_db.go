// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/sqldb"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
)

// PingDB pings the database.
func (frontend *FleetConsoleFrontend) PingDB(ctx context.Context, req *fleetconsolerpc.PingDBRequest) (_ *fleetconsolerpc.PingDBResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()
	logging.Infof(ctx, "beginning of ping db call")

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := sqldb.MustGetDB(ctx).PingContext(ctx); err != nil {
		logging.Errorf(ctx, "ping db call failed: %s", err)
		return nil, errors.Annotate(err, "pinging db").Err()
	}
	logging.Infof(ctx, "successful end of ping db call")
	return &fleetconsolerpc.PingDBResponse{}, nil
}
