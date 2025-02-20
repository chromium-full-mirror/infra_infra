// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufs

import (
	"context"

	"google.golang.org/grpc"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	models "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsapi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// Client exposes a deliberately chosen subset of the UFS functionality.
type Client interface {
	GetMachineLSE(context.Context, *ufsapi.GetMachineLSERequest, ...grpc.CallOption) (*models.MachineLSE, error)
	GetMachine(context.Context, *ufsapi.GetMachineRequest, ...grpc.CallOption) (*models.Machine, error)
	CheckFleetTestsPolicy(context.Context, *ufsapi.CheckFleetTestsPolicyRequest, ...grpc.CallOption) (*ufsapi.CheckFleetTestsPolicyResponse, error)
}

// newUFSClient returns a new client to interact with the Unified Fleet System.
func NewUFSClient(ctx context.Context, ufsService string, authFlags *authcli.Flags) (Client, error) {
	httpClient, err := cmdlib.NewHTTPClient(ctx, authFlags)
	if err != nil {
		return nil, err
	}
	return ufsapi.NewFleetPRPCClient(&prpc.Client{
		C:       httpClient,
		Host:    ufsService,
		Options: site.DefaultPRPCOptions,
	}), nil
}
