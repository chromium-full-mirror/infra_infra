// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

const (
	DMDevURL     = "device-lease-service-dev-thnumbwdvq-uc.a.run.app"
	DMProdURL    = "device-lease-service-prod-bbx5lsj5jq-uc.a.run.app"
	DMLeasesPort = 443
	// LeaseExtensionAmount is the default amount we extend DM leases by.
	LeaseExtensionAmount = 7 * time.Minute
	// LeaseExtensionInterval is the default interval between DM lease extensions.
	LeaseExtensionInterval = 5 * time.Minute
	// SchedukeDevPool is the pool that the DM and Scheduke dev instances manages.
	SchedukeDevPool = "schedukeTest"
)

type Client struct {
	pc api.DeviceLeaseServiceClient
}

func NewClient(ctx context.Context, pool string) (*Client, error) {
	baseURL := DMProdURL
	if pool == SchedukeDevPool {
		baseURL = DMDevURL
	}
	authOpts := chromeinfra.SetDefaultAuthOptions(auth.Options{
		UseIDTokens: true,
		// No "https://" included in the base URL since we omit it for RPC calls;
		// however, it's still needed for the auth audience.
		Audience: fmt.Sprintf("https://%s", baseURL),
	})
	a := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts)
	hc, err := a.Client()
	if err != nil {
		return nil, errors.Annotate(err, "setting up DM PRPC client").Err()
	}
	pc := &prpc.Client{
		C:    hc,
		Host: fmt.Sprintf("%s:%d", baseURL, DMLeasesPort),
	}
	c := api.NewDeviceLeaseServiceClient(pc)
	return &Client{
		pc: c,
	}, nil
}

// Extend extends the lease with the given ID by the given duration, and returns
// the new deadline.
func (c *Client) Extend(ctx context.Context, leaseID string, dur time.Duration) (time.Time, error) {
	req := &api.ExtendLeaseRequest{
		LeaseId:        leaseID,
		ExtendDuration: durationpb.New(dur),
		IdempotencyKey: uuid.New().String(),
	}
	res, err := c.pc.ExtendLease(ctx, req)
	if err != nil {
		return time.Time{}, errors.Annotate(err, "making ExtendLease request to Device Manager").Err()
	}
	return res.ExpirationTime.AsTime(), nil
}

// Release releases the lease with the given ID.
func (c *Client) Release(ctx context.Context, leaseID string) error {
	req := &api.ReleaseDeviceRequest{LeaseId: leaseID}
	_, err := c.pc.ReleaseDevice(ctx, req)
	if err != nil {
		return errors.Annotate(err, "making ReleaseDevice request to Device Manager").Err()
	}
	return nil
}
