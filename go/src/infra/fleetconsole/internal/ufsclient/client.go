// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ufsclient is the client lib for UFS.
package ufsclient

import (
	"context"

	"google.golang.org/grpc/metadata"

	ufsmodel "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

const (
	// UfsDevURL is the URL of the dev ufs instance.
	UfsDevURL = "staging.ufs.api.cr.dev"
	// UfsProdURL is the URL of the prod ufs instance.
	UfsProdURL = "ufs.api.cr.dev"
	// UfsPort is the port for ufs.
	UfsPort = 443
)

// Client is a client for UFS.
type Client = ufsAPI.FleetClient

func SetUfsNameSpace(ctx context.Context, nameSpace string) context.Context {
	md := metadata.Pairs(ufsUtil.Namespace, nameSpace)
	return metadata.NewOutgoingContext(ctx, md)
}

func GetAllUfsDevices(ctx context.Context, ufsClient Client) ([]*ufsmodel.DeviceLabels, error) {
	var devices []*ufsmodel.DeviceLabels
	nextPageToken := ""
	ctx = SetUfsNameSpace(ctx, ufsUtil.OSNamespace)
	for {
		res, err := ufsClient.ListDeviceLabels(ctx, &ufspb.ListDeviceLabelsRequest{
			PageSize:  1000,
			PageToken: nextPageToken,
		})
		if err != nil {
			return nil, err
		}

		devices = append(devices, res.Labels...)

		nextPageToken = res.GetNextPageToken()
		if nextPageToken == "" {
			break
		}
	}

	return devices, nil
}
