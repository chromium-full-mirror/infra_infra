// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/grpcutil"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// PingUfs pings UFS.
func (frontend *FleetConsoleFrontend) PingUfs(ctx context.Context, req *fleetconsolerpc.PingUfsRequest) (_ *fleetconsolerpc.PingUfsResponse, err error) {
	defer func() { err = grpcutil.GRPCifyAndLogErr(ctx, err) }()
	ufsClient, err := frontend.ufsClient(ctx, frontend.cloudProject)
	if err != nil {
		return nil, errors.Annotate(err, "ping ufs").Err()
	}

	_, err2 := ufsClient.ListMachineLSEs(ctx, &ufsAPI.ListMachineLSEsRequest{PageSize: 1})
	if err2 != nil {
		return nil, errors.Annotate(err2, "ping ufs").Err()
	}
	return &fleetconsolerpc.PingUfsResponse{}, nil
}
