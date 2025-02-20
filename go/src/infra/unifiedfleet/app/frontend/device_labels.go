// Copyright 2024 The Chromium Authors
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

func (*FleetServerImpl) GetDeviceLabels(ctx context.Context, req *ufsAPI.GetDeviceLabelsRequest) (rsp *ufspb.DeviceLabels, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	ns := util.GetNamespaceFromCtx(ctx)
	logging.Infof(ctx, "querying namespace %q", ns)
	req.Hostname = util.RemovePrefix(req.GetHostname())
	labels, err := controller.GetDeviceLabels(ctx, req.Hostname)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	labels.Name = util.AddPrefix(util.DeviceLabelsCollection, labels.Name)
	return labels, nil
}

func (*FleetServerImpl) ListDeviceLabels(ctx context.Context, req *ufsAPI.ListDeviceLabelsRequest) (rsp *ufsAPI.ListDeviceLabelsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err := ufsAPI.ValidateListRequest(req); err != nil {
		return nil, err
	}
	pageSize := util.GetPageSize(req.PageSize)
	ns := util.GetNamespaceFromCtx(ctx)
	logging.Infof(ctx, "querying namespace %q", ns)
	result, nextPageToken, err := controller.ListDeviceLabels(ctx, pageSize, req.PageToken, req.Filter, false)
	if err != nil {
		return nil, err
	}
	// https://aip.dev/122 - as per AIP guideline
	for _, labels := range result {
		labels.Name = util.AddPrefix(util.DeviceLabelsCollection, labels.Name)
	}
	return &ufsAPI.ListDeviceLabelsResponse{
		Labels:        result,
		NextPageToken: nextPageToken,
	}, nil
}
