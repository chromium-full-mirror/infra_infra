// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/controller"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// GetOwnershipData returns the ownership data for a given host.
func (fs *FleetServerImpl) GetOwnershipData(ctx context.Context, req *ufsAPI.GetOwnershipDataRequest) (response *ufspb.OwnershipData, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	ownership, err := controller.GetOwnershipData(ctx, req.GetHostname())
	if err == nil {
		return ownership, nil
	}
	logging.Errorf(ctx, "Error while querying ownership data : %v", err)
	return nil, err
}

// ListOwnershipData returns the ownership data entries.
func (fs *FleetServerImpl) ListOwnershipData(ctx context.Context, req *ufsAPI.ListOwnershipDataRequest) (response *ufsAPI.ListOwnershipDataResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := ufsAPI.ValidateListRequest(req); err != nil {
		return nil, err
	}
	pageSize := util.GetPageSize(req.PageSize)
	result, nextPageToken, err := controller.ListOwnershipConfigs(ctx, pageSize, req.PageToken, req.Filter, req.KeysOnly)
	if err != nil {
		return nil, err
	}
	return &ufsAPI.ListOwnershipDataResponse{
		OwnershipData: result,
		NextPageToken: nextPageToken,
	}, nil
}
