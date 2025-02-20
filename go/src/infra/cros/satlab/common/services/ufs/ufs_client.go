// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufs

import (
	"context"

	"google.golang.org/grpc"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	s "go.chromium.org/infra/cros/satlab/common/site"
	ufsModels "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsApi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// UFSClient interface provides subset of UFS methods relevant to Satlab CLI
// not strictly necessary (could just use proto client directly) but makes it easier to test
// add more methods if additional UFS reqs needed
type UFSClient interface {
	GetMachine(context.Context, *ufsApi.GetMachineRequest, ...grpc.CallOption) (*ufsModels.Machine, error)
	GetMachineLSE(context.Context, *ufsApi.GetMachineLSERequest, ...grpc.CallOption) (*ufsModels.MachineLSE, error)
	UpdateMachineLSE(context.Context, *ufsApi.UpdateMachineLSERequest, ...grpc.CallOption) (*ufsModels.MachineLSE, error)
}

// implementation of UFS client
type clientImpl struct {
	client ufsApi.FleetClient
}

// GetMachine fetches information about the machine we request.
func (c *clientImpl) GetMachine(ctx context.Context, req *ufsApi.GetMachineRequest) (*ufsModels.Machine, error) {
	return c.client.GetMachine(ctx, req)
}

// GetMachine fetches information about the machine we request.
func (c *clientImpl) GetMachineLSE(ctx context.Context, req *ufsApi.GetMachineLSERequest, opts ...grpc.CallOption) (*ufsModels.MachineLSE, error) {
	return c.client.GetMachineLSE(ctx, req)
}

// GetMachine fetches information about the machine we request.
func (c *clientImpl) UpdateMachineLSE(ctx context.Context, req *ufsApi.UpdateMachineLSERequest, opts ...grpc.CallOption) (*ufsModels.MachineLSE, error) {
	return c.client.UpdateMachineLSE(ctx, req)
}

// NewUFSClient creates a new client to access UFS, but only exposing specific methods needed for Satlab CLI
func NewUFSClient(ctx context.Context, ufsService string, authFlags *authcli.Flags) (ufsApi.FleetClient, error) {
	httpClient, err := cmdlib.NewHTTPClient(ctx, authFlags)
	if err != nil {
		return nil, err
	}

	if ufsService == "" {
		return nil, errors.Reason("new ufs client: must provide ufs service hostname").Err()
	}

	return ufsApi.NewFleetPRPCClient(&prpc.Client{
		C:       httpClient,
		Host:    ufsService,
		Options: site.ProdDefaultPRPCOptions(),
	}), nil

}

// NewUFSClientWithDefaultOptions creates a new client to access UFS with default options.
func NewUFSClientWithDefaultOptions(ctx context.Context, ufsService string) (ufsApi.FleetClient, error) {
	if ufsService == "" {
		return nil, errors.Reason("new ufs client: must provide ufs service hostname").Err()
	}

	opts := s.GetAuthOption(ctx)
	a := auth.NewAuthenticator(ctx, auth.SilentLogin, opts)
	c, err := a.Client()
	if err != nil {
		return nil, err
	}
	return ufsApi.NewFleetPRPCClient(&prpc.Client{
		C:       c,
		Host:    ufsService,
		Options: site.ProdDefaultPRPCOptions(),
	}), nil

}
