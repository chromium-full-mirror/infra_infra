// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package costserver

import (
	"context"

	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
)

// Ping takes a PingRequest which is empty and returns a PingResponse which is empty.
func (f *FleetCostFrontend) Ping(context.Context, *fleetcostAPI.PingRequest) (*fleetcostAPI.PingResponse, error) {
	return &fleetcostAPI.PingResponse{}, nil
}
