// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package devicemanagerclient is the client lib for device manager.
package devicemanagerclient

import (
	"context"
	"fmt"
	"net/http"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"

	// In the device_manager library, please ONLY depend on the constants that are not specific to Scheduke.
	"infra/device_manager/client"
	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/cmd/fleetconsoleserver/flags"
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
func NewClient(ctx context.Context, rpcAuthorityKind auth.RPCAuthorityKind, hostname string, port int) (*Client, error) {
	var opts []auth.RPCOption
	switch rpcAuthorityKind {
	case auth.AsCredentialsForwarder:
		// do nothing
	case auth.AsSelf:
		opts = []auth.RPCOption{auth.WithIDToken()}
	default:
		opts = []auth.RPCOption{auth.WithScopes(auth.CloudOAuthScopes...)}
	}
	t, err := auth.GetRPCTransport(ctx, rpcAuthorityKind, opts...)
	if err != nil {
		return nil, errors.Annotate(err, "setting up auth").Err()
	}
	httpClient := &http.Client{
		Transport: t,
	}
	prpcClient := &prpc.Client{
		C: httpClient,
		Options: &prpc.Options{
			Insecure: *flags.UseLocalDeviceManager,
		},
		Host: fmt.Sprintf("%s:%d", hostname, port),
	}
	return &Client{
		Leaser: api.NewDeviceLeaseServiceClient(prpcClient),
	}, nil
}

func MapDevices(devices []*api.Device) []*fleetconsolerpc.Device {
	var mappedDevices []*fleetconsolerpc.Device
	for _, device := range devices {
		mappedDevices = append(mappedDevices, mapDevice(device))
	}
	return mappedDevices
}

func mapDevice(device *api.Device) *fleetconsolerpc.Device {
	return &fleetconsolerpc.Device{
		Id:    device.Id,
		DutId: device.DutId,
		Address: &fleetconsolerpc.DeviceAddress{
			Host: device.Address.Host,
			Port: device.Address.Port,
		},
		Type:  fleetconsolerpc.DeviceType(device.Type),
		State: fleetconsolerpc.DeviceState(device.State),
		DeviceSpec: &fleetconsolerpc.DeviceSpec{
			Labels: mapLabels(device.HardwareReqs.SchedulableLabels),
		},
	}
}

func mapLabels(labels map[string]*api.HardwareRequirements_LabelValues) map[string]*fleetconsolerpc.LabelValues {
	mappedLabels := make(map[string]*fleetconsolerpc.LabelValues)
	for k, v := range labels {
		mappedLabels[k] = &fleetconsolerpc.LabelValues{
			Values: v.Values,
		}
	}
	return mappedLabels
}
