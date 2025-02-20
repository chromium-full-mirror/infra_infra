// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"

	empty "github.com/golang/protobuf/ptypes/empty"

	"go.chromium.org/luci/grpc/grpcutil"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/controller"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// CreateDefaultWifi creates DefaultWifi entry in database.
func (fs *FleetServerImpl) CreateDefaultWifi(ctx context.Context, req *ufsAPI.CreateDefaultWifiRequest) (rsp *ufspb.DefaultWifi, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err = req.Validate(); err != nil {
		return
	}
	req.DefaultWifi.Name = req.DefaultWifiId
	rsp, err = controller.CreateDefaultWifi(ctx, req.DefaultWifi)
	if err != nil {
		return nil, err
	}
	rsp.Name = util.AddPrefix(util.DefaultWifiCollection, rsp.Name)
	return
}

// GetDefaultWifi returns the specified GetDefaultWifi.
func (fs *FleetServerImpl) GetDefaultWifi(ctx context.Context, req *ufsAPI.GetDefaultWifiRequest) (rsp *ufspb.DefaultWifi, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	rsp, err = controller.GetDefaultWifi(ctx, name)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline.
	rsp.Name = util.AddPrefix(util.DefaultWifiCollection, rsp.Name)
	return
}

// ListDefaultWifis list the DefaultWifis information from database.
func (fs *FleetServerImpl) ListDefaultWifis(ctx context.Context, req *ufsAPI.ListDefaultWifisRequest) (rsp *ufsAPI.ListDefaultWifisResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := ufsAPI.ValidateListRequest(req); err != nil {
		return nil, err
	}
	pageSize := util.GetPageSize(req.PageSize)
	result, nextPageToken, err := controller.ListDefaultWifis(ctx, pageSize, req.PageToken, req.Filter, req.KeysOnly)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline.
	for _, wifi := range result {
		wifi.Name = util.AddPrefix(util.DefaultWifiCollection, wifi.Name)
	}
	return &ufsAPI.ListDefaultWifisResponse{
		DefaultWifis:  result,
		NextPageToken: nextPageToken,
	}, nil
}

// DeleteDefaultWifi deletes the DefaultWifi from database.
func (fs *FleetServerImpl) DeleteDefaultWifi(ctx context.Context, req *ufsAPI.DeleteDefaultWifiRequest) (rsp *empty.Empty, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	name := util.RemovePrefix(req.Name)
	err = controller.DeleteDefaultWifi(ctx, name)
	return &empty.Empty{}, err
}

// UpdateDefaultWifi updates the DefaultWifi information in database.
func (fs *FleetServerImpl) UpdateDefaultWifi(ctx context.Context, req *ufsAPI.UpdateDefaultWifiRequest) (rsp *ufspb.DefaultWifi, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.DefaultWifi.Name = util.RemovePrefix(req.DefaultWifi.Name)
	wifi, err := controller.UpdateDefaultWifi(ctx, req.DefaultWifi, req.UpdateMask)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline.
	wifi.Name = util.AddPrefix(util.DefaultWifiCollection, wifi.Name)
	return wifi, err
}
