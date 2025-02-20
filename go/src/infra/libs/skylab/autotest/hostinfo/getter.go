// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostinfo

import (
	"context"
	"fmt"

	grpc "google.golang.org/grpc"

	"go.chromium.org/chromiumos/infra/proto/go/lab_platform"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
	models "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// InventoryClient is a client that knows how to resolve a ChromeosDeviceDataRequest contains hostname
// to ChromeOSDeviceData that contains information about the DUT V1.
type InventoryClient interface {
	GetChromeOSDeviceData(context.Context, *ufsAPI.GetChromeOSDeviceDataRequest, ...grpc.CallOption) (*models.ChromeOSDeviceData, error)
}

// AdminClient is a client that knows how to respond to the GetStableVersion RPC call.
// Its prototypical implementation is fleet.InventoryClient.
type AdminClient interface {
	GetRecoveryVersion(ctx context.Context, in *fleet.GetRecoveryVersionRequest, opts ...grpc.CallOption) (*fleet.GetRecoveryVersionResponse, error)
}

// Getter is a container for the clients needed to construct the host_info_store contents for a given hostname.
type Getter struct {
	ic InventoryClient
	ac AdminClient
}

// NewGetter constructs a getter in the default configuration.
func NewGetter(
	ic InventoryClient,
	ac AdminClient,
) *Getter {
	g := &Getter{}
	g.ic = ic
	g.ac = ac
	return g
}

// GetContentsForHostname gets the entire hostinfostore file contents for a given hostname
// as a string.
func (g *Getter) GetContentsForHostname(ctx context.Context, hostname string) (string, error) {
	if hostname == "" {
		return "", fmt.Errorf("hostname cannot be empty")
	}
	if g.ic == nil {
		return "", fmt.Errorf("no Inventory client for dut-info")
	}
	if g.ac == nil {
		return "", fmt.Errorf("no Inventory client for stable version")
	}
	req := &ufsAPI.GetChromeOSDeviceDataRequest{
		Hostname: hostname,
	}
	crosDeviceData, err := g.ic.GetChromeOSDeviceData(ctx, req)
	if err != nil {
		return "", err
	}
	di := crosDeviceData.GetDutV1()
	hi := ConvertDut(di)
	// Devboard device does not have stable version.
	// Only obtains stable version when device is not a devboard.
	if crosDeviceData.GetMachine().GetDevboard() == nil {
		version, err := g.GetStableVersion(ctx, "", hostname, "", "", nil)
		if err != nil {
			return "", err
		}
		hi.StableVersions = versionToMap(version)
	}
	bytes, err := MarshalIndent(hi)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// GetStableVersion gets the stable version info.
func (g *Getter) GetStableVersion(ctx context.Context, deviceType, hostname, board, model string, pools []string) (*lab_platform.StableVersion, error) {
	if g.ac == nil {
		return nil, fmt.Errorf("no Inventory client for stable version")
	}
	if hostname == "" {
		hostname = "shivas-device"
	}

	res, err := g.ac.GetRecoveryVersion(ctx, &fleet.GetRecoveryVersionRequest{
		DeviceName: hostname,
		DeviceType: deviceType,
		Model:      model,
		Board:      board,
		Pools:      pools,
	})
	if err != nil {
		return nil, err
	}
	if res.GetVersion() == nil {
		return nil, fmt.Errorf("response does not contains a version info")
	}
	return res.GetVersion(), nil
}

func versionToMap(ver *lab_platform.StableVersion) map[string]string {
	return map[string]string{
		"OsVersion":     ver.GetOsVersion(),
		"OsImagePath":   ver.GetOsImagePath(),
		"FwRoVersion":   ver.GetFirmwareRoVersion(),
		"FwRoImagePath": ver.GetFirmwareRoImagePath(),
	}
}
