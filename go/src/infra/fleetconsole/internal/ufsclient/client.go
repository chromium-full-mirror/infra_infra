// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ufsclient is the client lib for UFS.
package ufsclient

import (
	"context"

	"go.chromium.org/luci/server/auth"

	"infra/fleetconsole/internal/site"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
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
type Client struct {
	FleetClient ufsAPI.FleetClient
}

// NewClient makes a new client.
func NewClient(ctx context.Context, rpcAuthorityKind auth.RPCAuthorityKind, baseURL string, port int, insecure bool) (*Client, error) {
	prpcClient, err := site.NewAuthenticatedClient(ctx, rpcAuthorityKind, baseURL, port, insecure)

	if err != nil {
		return nil, err
	}

	return &Client{
		FleetClient: ufsAPI.NewFleetPRPCClient(prpcClient),
	}, nil
}
