// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package external

import (
	"context"
	"net/http"

	"google.golang.org/grpc/metadata"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"

	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

const UFSServiceURI = "ufs.api.cr.dev"

// NewUFSClient creates a new client to access UFS.
func NewUFSClient(ctx context.Context, ufsHostname string) (ufsAPI.FleetClient, error) {
	if ufsHostname == "" {
		return nil, errors.Reason("NewUFSClient: must provide ufs service hostname").Err()
	}
	t, err := auth.GetRPCTransport(ctx, auth.AsSelf, auth.WithScopes(auth.CloudOAuthScopes...))
	if err != nil {
		return nil, errors.Annotate(err, "NewUFSClient: failed to get RPC transport to UFS service").Err()
	}
	return ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C: &http.Client{
			Transport: t,
		},
		Host: ufsHostname,
	}), nil
}

// SetupContext sets up context with a UFS namespace.
func SetupContext(ctx context.Context, namespace string) context.Context {
	md := metadata.Pairs(ufsUtil.Namespace, namespace)
	return metadata.NewOutgoingContext(ctx, md)
}
