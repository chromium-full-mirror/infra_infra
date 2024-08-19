// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventoryclient

import (
	"context"
	"net/http"
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/retry"
	"go.chromium.org/luci/common/retry/transient"
	"go.chromium.org/luci/grpc/prpc"

	invV2Api "infra/appengine/cros/lab_inventory/api/v1"
	"infra/libs/skylab/inventory"
)

// Client defines the common interface for the inventory client used by
// various command line tools.
type Client interface {
	GetDutInfo(context.Context, string, bool) (*inventory.DeviceUnderTest, error)
	FilterDUTHostnames(context.Context, []string) ([]string, error)
}

// V2Client is an API client for the inventory V2 service.
type V2Client struct {
	ic invV2Api.InventoryClient
}

// NewInventoryClient creates a new instance of inventory client.
func NewInventoryClient(hc *http.Client,
	inventoryService string,
	options *prpc.Options,
) *V2Client {
	return &V2Client{
		ic: invV2Api.NewInventoryPRPCClient(&prpc.Client{
			C:       hc,
			Host:    inventoryService,
			Options: options,
		}),
	}
}

// GetDutInfo gets the dut information from inventory v2 service.
func (client *V2Client) GetDutInfo(ctx context.Context, id string, byHostname bool) (*inventory.DeviceUnderTest, error) {
	devID := &invV2Api.DeviceID{Id: &invV2Api.DeviceID_ChromeosDeviceId{ChromeosDeviceId: id}}
	if byHostname {
		devID = &invV2Api.DeviceID{Id: &invV2Api.DeviceID_Hostname{Hostname: id}}
	}
	rsp, err := client.ic.GetCrosDevices(ctx, &invV2Api.GetCrosDevicesRequest{
		Ids: []*invV2Api.DeviceID{devID},
	})
	if err != nil {
		return nil, errors.Annotate(err, "get dutinfo for %s", id).Err()
	}
	if len(rsp.FailedDevices) > 0 {
		result := rsp.FailedDevices[0]
		return nil, errors.Reason("failed to get device %s: %s", result.Hostname, result.ErrorMsg).Err()
	}
	if len(rsp.Data) != 1 {
		return nil, errors.Reason("no info returned for %s", id).Err()
	}
	return invV2Api.AdaptToV1DutSpec(rsp.Data[0])
}

// FilterDUTHostnames produces a list of only the DUT hostnames that exist.
func (client *V2Client) FilterDUTHostnames(ctx context.Context, hostnames []string) ([]string, error) {
	var out []string
	// The RPC will fail if no hostnames are provided, so return early instead.
	if len(hostnames) == 0 {
		return out, nil
	}
	req := &invV2Api.GetCrosDevicesRequest{}
	for _, hostname := range hostnames {
		req.Ids = append(req.Ids, &invV2Api.DeviceID{Id: &invV2Api.DeviceID_Hostname{Hostname: hostname}})
	}
	rsp, err := client.ic.GetCrosDevices(ctx, req)
	if err != nil {
		return nil, errors.Annotate(err, "failed to get DUT information").Err()
	}
	for _, item := range rsp.Data {
		hostname := item.GetLabConfig().GetDut().GetHostname()
		out = append(out, hostname)

	}
	return out, nil
}

// Set up the client-side retry strategy for inventory APIs.
// Slow down the retry to not flood the external APIs.
var transientErrorRetriesTemplate = retry.ExponentialBackoff{
	Limited: retry.Limited{
		Delay:   200 * time.Millisecond,
		Retries: 3,
	},
	Multiplier: 4,
	MaxDelay:   5 * time.Second,
}

// transientErrorRetries returns a retry.Factory to use on transient errors on
// outbound requests.
func transientErrorRetries() retry.Factory {
	next := func() retry.Iterator {
		it := transientErrorRetriesTemplate
		return &it
	}
	return transient.Only(next)
}
