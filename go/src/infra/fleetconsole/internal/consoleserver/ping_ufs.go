// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"infra/fleetconsole/api/fleetconsolerpc"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

// PingUfs pings UFS.
func (frontend *FleetConsoleFrontend) PingUfs(ctx context.Context, req *fleetconsolerpc.PingUfsRequest) (*fleetconsolerpc.PingUfsResponse, error) {
	ufsClient, err := frontend.ufsClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "ping ufs").Err()
	}

	_, err2 := ufsClient.ListMachineLSEs(ctx, &ufsAPI.ListMachineLSEsRequest{PageSize: 1})
	if err2 != nil {
		return nil, errors.Annotate(err2, "ping ufs").Err()
	}
	return &fleetconsolerpc.PingUfsResponse{}, nil
}
