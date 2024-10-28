// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package devicemanagerclient is the client lib for device manager.
package devicemanagerclient

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	// In the device_manager library, please ONLY depend on the constants that are not specific to Scheduke.
	"infra/device_manager/client"
)

const (
	// DMDevURL is the URL of the dev leasing service.
	DMDevURL = client.DMDevURL
	// DMProdURL is the URL of the prod leasing service.
	DMProdURL = client.DMProdURL
	// DMLeasesPort is the port to use to lease stuff.
	DMLeasesPort = client.DMLeasesPort
)

// Client is a client for Device Manager.
type Client struct {
	Leaser api.DeviceLeaseServiceClient
}

// NewClient makes a new client.
func NewClient(ctx context.Context, baseURL string) (*Client, error) {
	authOpts := chromeinfra.SetDefaultAuthOptions(auth.Options{
		UseIDTokens: true,
		Audience:    fmt.Sprintf("https://%s", baseURL),
	})
	authenticator := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts)
	httpClient, err := authenticator.Client()
	if err != nil {
		return nil, errors.Annotate(err, "setting up DM PRPC client").Err()
	}
	prpcClient := &prpc.Client{
		C:    httpClient,
		Host: fmt.Sprintf("%s:%d", baseURL, DMLeasesPort),
	}
	return &Client{
		Leaser: api.NewDeviceLeaseServiceClient(prpcClient),
	}, nil
}
