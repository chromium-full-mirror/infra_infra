// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"infra/fleetconsole/api/fleetconsolerpc"
)

// PingDB pings the database.
func (frontend *FleetConsoleFrontend) PingDB(ctx context.Context, req *fleetconsolerpc.PingDBRequest) (*fleetconsolerpc.PingDBResponse, error) {
	err := frontend.dbConnection.Ping()
	if err != nil {
		return nil, errors.Annotate(err, "pinging db").Err()
	}
	return &fleetconsolerpc.PingDBResponse{}, nil
}
