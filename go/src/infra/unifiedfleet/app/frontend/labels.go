// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/grpcutil"

	ufspb "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

func (*FleetServerImpl) GetDeviceLabels(ctx context.Context, req *ufsAPI.GetDeviceLabelsRequest) (rsp *ufspb.DeviceLabels, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	return nil, errors.New("GetDeviceLabels not implemented")
}

func (*FleetServerImpl) ListDeviceLabels(ctx context.Context, req *ufsAPI.ListDeviceLabelsRequest) (rsp *ufsAPI.ListDeviceLabelsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	return nil, errors.New("ListDeviceLabels not implemented")
}
