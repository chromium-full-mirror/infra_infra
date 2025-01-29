// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/grpcutil"

	"infra/fleetconsole/api/fleetconsolerpc"
)

// PingDB pings the database.
func (frontend *FleetConsoleFrontend) PingDB(ctx context.Context, req *fleetconsolerpc.PingDBRequest) (_ *fleetconsolerpc.PingDBResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()
	if err := frontend.dbConnection.Ping(); err != nil {
		return nil, errors.Annotate(err, "pinging db").Err()
	}
	return &fleetconsolerpc.PingDBResponse{}, nil
}
