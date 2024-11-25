// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"google.golang.org/grpc"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/devicemanagerclient"
)

// NewFleetConsoleFrontend creates a new fleet console frontend.
func NewFleetConsoleFrontend() fleetconsolerpc.FleetConsoleServer {
	return &FleetConsoleFrontend{}
}

// FleetConsoleFrontend is the fleet console frontend.
type FleetConsoleFrontend struct {
	fleetconsolerpc.UnimplementedFleetConsoleServer

	deviceManagerClient func(context.Context) (*devicemanagerclient.Client, error)
}

// InstallServices installs services into the server.
func InstallServices(consoleFrontend fleetconsolerpc.FleetConsoleServer, srv grpc.ServiceRegistrar) {
	fleetconsolerpc.RegisterFleetConsoleServer(srv, consoleFrontend)
}

// SetDeviceManagerClient sets the device manager client.
func SetDeviceManagerClient(consoleFrontend *FleetConsoleFrontend, deviceManagerClient func(context.Context) (*devicemanagerclient.Client, error)) {
	consoleFrontend.deviceManagerClient = deviceManagerClient
}
