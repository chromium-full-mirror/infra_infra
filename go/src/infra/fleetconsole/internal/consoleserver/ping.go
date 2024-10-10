// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"infra/fleetconsole/api/fleetconsolerpc"
)

// Ping is the ping RPC. It responds with an empty response and never fails.
func (frontend *FleetConsoleFrontend) Ping(ctx context.Context, req *fleetconsolerpc.PingRequest) (*fleetconsolerpc.PingResponse, error) {
	return &fleetconsolerpc.PingResponse{}, nil
}
