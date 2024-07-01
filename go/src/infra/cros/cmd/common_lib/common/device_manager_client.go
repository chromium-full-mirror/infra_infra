// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	DMDevURL     = "device-lease-service-dev-thnumbwdvq-uc.a.run.app"
	DMProdURL    = "device-lease-service-prod-bbx5lsj5jq-uc.a.run.app"
	DMLeasesPort = 443
)

type DeviceManagerClient struct {
	client api.DeviceLeaseServiceClient
	ctx    context.Context
}

func NewDeviceManagerClient(ctx context.Context, pool string) (*DeviceManagerClient, error) {
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
		return nil, errors.Annotate(err, "setting up DM client").Err()
	}
	prpcClient := &prpc.Client{
		C:    hc,
		Host: fmt.Sprintf("%s:%d", baseURL, DMLeasesPort),
	}
	c := api.NewDeviceLeaseServiceClient(prpcClient)
	return &DeviceManagerClient{
		client: c,
		ctx:    ctx,
	}, nil
}

// Extend extends the lease with the given ID by the given duration, and returns
// the new deadline.
func (d *DeviceManagerClient) Extend(ctx context.Context, leaseID string, dur time.Duration) (time.Time, error) {
	req := &api.ExtendLeaseRequest{
		LeaseId:        leaseID,
		ExtendDuration: durationpb.New(dur),
		IdempotencyKey: uuid.New().String(),
	}
	res, err := d.client.ExtendLease(ctx, req)
	if err != nil {
		return time.Time{}, errors.Annotate(err, "making ExtendLease request to Device Manager").Err()
	}
	return res.ExpirationTime.AsTime(), nil
}

// Release releases the lease with the given ID.
func (d *DeviceManagerClient) Release(ctx context.Context, leaseID string) error {
	req := &api.ReleaseDeviceRequest{LeaseId: leaseID}
	_, err := d.client.ReleaseDevice(ctx, req)
	if err != nil {
		return errors.Annotate(err, "making ReleaseDevice request to Device Manager").Err()
	}
	return nil
}
