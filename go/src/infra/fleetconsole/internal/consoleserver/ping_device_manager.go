// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/fleetconsole/api/fleetconsolerpc"
)

// PingDeviceManager pings device manager.
func (frontend *FleetConsoleFrontend) PingDeviceManager(ctx context.Context, req *fleetconsolerpc.PingDeviceManagerRequest) (*fleetconsolerpc.PingDeviceManagerResponse, error) {
	deviceManagerClient, err := frontend.deviceManagerClient(ctx, frontend.cloudProject)
	if err != nil {
		return nil, errors.Annotate(err, "ping device manager").Err()
	}
	_, err = deviceManagerClient.Leaser.ListDevices(ctx, &api.ListDevicesRequest{PageSize: 1})
	if err != nil {
		logging.Infof(ctx, "device manager: %v\n", err)
		return nil, errors.Annotate(err, "ping device manager").Err()
	}
	return &fleetconsolerpc.PingDeviceManagerResponse{}, nil
}
