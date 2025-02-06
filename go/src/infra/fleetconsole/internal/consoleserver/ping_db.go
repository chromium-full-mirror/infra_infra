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

	"infra/fleetconsole/api/fleetconsolerpc"
)

// PingDB pings the database.
func (frontend *FleetConsoleFrontend) PingDB(ctx context.Context, req *fleetconsolerpc.PingDBRequest) (_ *fleetconsolerpc.PingDBResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()
	logging.Infof(ctx, "beginning of ping db call")
	logging.Infof(ctx, "db secret source: %q", frontend.dbConnectionSource)

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
	defer cancel()
	if err := frontend.dbConnection.PingContext(ctx); err != nil {
		logging.Errorf(ctx, "ping db call failed: %s", err)
		return nil, errors.Annotate(err, "pinging db").Err()
	}
	logging.Infof(ctx, "successful end of ping db call")
	return &fleetconsolerpc.PingDBResponse{}, nil
}
