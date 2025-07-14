// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"strings"

	"google.golang.org/grpc"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/devicemanagerclient"
	"go.chromium.org/infra/fleetconsole/internal/ufsclient"
	"go.chromium.org/infra/libs/skylab/buildbucket"
)

// NewFleetConsoleFrontend creates a new fleet console frontend.
func NewFleetConsoleFrontend() fleetconsolerpc.FleetConsoleServer {
	return &FleetConsoleFrontend{}
}

// FleetConsoleFrontend is the fleet console frontend.
type FleetConsoleFrontend struct {
	fleetconsolerpc.UnimplementedFleetConsoleServer

	cloudProject            string
	bbclient                buildbucket.Client
	adminServiceAddress     string
	inventoryServiceAddress string
	inventoryNamespace      string
	cipdVersion             buildbucket.CIPDVersion

	deviceManagerClient func(context.Context, bool) (*devicemanagerclient.Client, error)
	ufsClient           func(context.Context, bool) (ufsclient.Client, error)
}

func (frontend *FleetConsoleFrontend) IsProdEnvironment() bool {
	return frontend.cloudProject != "" && !strings.HasSuffix(frontend.cloudProject, "dev")
}

// InstallServices installs services into the server.
func InstallServices(consoleFrontend fleetconsolerpc.FleetConsoleServer, srv grpc.ServiceRegistrar) {
	fleetconsolerpc.RegisterFleetConsoleServer(srv, consoleFrontend)
}

// SetDeviceManagerClient sets the device manager client.
func SetDeviceManagerClient(consoleFrontend *FleetConsoleFrontend, deviceManagerClient func(context.Context, bool) (*devicemanagerclient.Client, error)) {
	consoleFrontend.deviceManagerClient = deviceManagerClient
}

// SetUFSClient sets the UFS client.
func SetUFSClient(consoleFrontend *FleetConsoleFrontend, ufsClient func(context.Context, bool) (ufsclient.Client, error)) {
	consoleFrontend.ufsClient = ufsClient
}

// SetCloudProject sets the cloud project.
func SetCloudProject(consoleFrontend *FleetConsoleFrontend, cloudProject string) {
	consoleFrontend.cloudProject = cloudProject
}

// SetBBClient sets the buildbucket client.
func SetBBClient(consoleFrontend *FleetConsoleFrontend, bbclient buildbucket.Client) {
	consoleFrontend.bbclient = bbclient
}

// SetAdminServiceAddress sets the admin service address.
func SetAdminServiceAddress(consoleFrontend *FleetConsoleFrontend, adminServiceAddress string) {
	consoleFrontend.adminServiceAddress = adminServiceAddress
}

// SetInventoryServiceAddress sets the inventory service address.
func SetInventoryServiceAddress(consoleFrontend *FleetConsoleFrontend, inventoryServiceAddress string) {
	consoleFrontend.inventoryServiceAddress = inventoryServiceAddress
}

// SetInventoryNamespace sets the inventory namespace.
func SetInventoryNamespace(consoleFrontend *FleetConsoleFrontend, inventoryNamespace string) {
	consoleFrontend.inventoryNamespace = inventoryNamespace
}

// SetCIPDVersion sets the CIPD version.
func SetCIPDVersion(consoleFrontend *FleetConsoleFrontend, cipdVersion buildbucket.CIPDVersion) {
	consoleFrontend.cipdVersion = cipdVersion
}
