// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"google.golang.org/grpc"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/devicemanagerclient"
	"infra/fleetconsole/internal/ufsclient"
)

// NewFleetConsoleFrontend creates a new fleet console frontend.
func NewFleetConsoleFrontend() fleetconsolerpc.FleetConsoleServer {
	return &FleetConsoleFrontend{}
}

// FleetConsoleFrontend is the fleet console frontend.
type FleetConsoleFrontend struct {
	fleetconsolerpc.UnimplementedFleetConsoleServer

	cloudProject        string
	deviceManagerClient func(context.Context) (*devicemanagerclient.Client, error)
	ufsClient           func(context.Context, string) (ufsclient.Client, error)
}

// InstallServices installs services into the server.
func InstallServices(consoleFrontend fleetconsolerpc.FleetConsoleServer, srv grpc.ServiceRegistrar) {
	fleetconsolerpc.RegisterFleetConsoleServer(srv, consoleFrontend)
}

// SetDeviceManagerClient sets the device manager client.
func SetDeviceManagerClient(consoleFrontend *FleetConsoleFrontend, deviceManagerClient func(context.Context) (*devicemanagerclient.Client, error)) {
	consoleFrontend.deviceManagerClient = deviceManagerClient
}

// SeUFSClient sets the UFS client.
func SetUFSClient(consoleFrontend *FleetConsoleFrontend, ufsClient func(context.Context, string) (ufsclient.Client, error)) {
	consoleFrontend.ufsClient = ufsClient
}

// SetCloudProject sets the cloud project
func SetCloudProject(consoleFrontend *FleetConsoleFrontend, cloudProject string) {
	consoleFrontend.cloudProject = cloudProject
}
