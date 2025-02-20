// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ufsclients contains ready-made UFS clients that can be conveniently instantiated from other services
// or from the command line.
package ufsclients

import (
	"context"
	"net/http"

	"google.golang.org/grpc/metadata"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"

	"go.chromium.org/infra/cmdsupport/cmdlib"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// NewUFSClientFromServer creates a client to UFS from a LUCI server service.
//
// Note that this uses auth.AsSelf to authenticate to UFS. As of 2025-01-23 this is what most
// services do in practice, but it is an assumption.
//
// TODO(gregorynisbet): check the UFS namespace and reject requests if there is no namespace.
func NewUFSClientFromServer(ctx context.Context, ufsHostname string, prpcOptions *prpc.Options) (ufsAPI.FleetClient, error) {
	if ufsHostname == "" {
		return nil, errors.New("ufsHostname cannot be empty")
	}
	transport, err := auth.GetRPCTransport(ctx, auth.AsSelf, auth.WithScopes(auth.CloudOAuthScopes...))
	if err != nil {
		return nil, errors.Annotate(err, "NewUFSClientFromServer: failed to get RPC transport to UFS service").Err()
	}
	httpClient := &http.Client{
		Transport: transport,
	}
	prpcClient := &prpc.Client{
		C:       httpClient,
		Host:    ufsHostname,
		Options: prpcOptions,
	}
	return ufsAPI.NewFleetPRPCClient(prpcClient), nil
}

// SetUFSNamespace sets the UFS namespace in the context.
//
// TODO(gregorynisbet): namespace validation
func SetUFSNamespace(ctx context.Context, namespace string) context.Context {
	md := metadata.Pairs(ufsUtil.Namespace, namespace)
	return metadata.NewOutgoingContext(ctx, md)
}

// NewUFSClientFromCLI creates a UFS client that authenticates using mechanisms available to
// CLI tools using standard mechanisms in the LUCI, infra, and infra_internal repos.
func NewUFSClientFromCLI(ctx context.Context, ufsHostname string, authFlags *authcli.Flags, prpcOptions *prpc.Options) (ufsAPI.FleetClient, error) {
	httpClient, err := cmdlib.NewHTTPClient(ctx, authFlags)
	if err != nil {
		return nil, errors.Annotate(err, "NewUFSClientFromCLI").Err()
	}
	prpcClient := &prpc.Client{
		C:       httpClient,
		Host:    ufsHostname,
		Options: prpcOptions,
	}
	return ufsAPI.NewFleetPRPCClient(prpcClient), nil
}
