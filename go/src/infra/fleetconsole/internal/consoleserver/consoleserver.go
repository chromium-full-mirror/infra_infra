// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"database/sql"

	"google.golang.org/grpc"

	"go.chromium.org/luci/server/secrets"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/cmd/fleetconsoleserver/flags"
	"infra/fleetconsole/internal/database"
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
	deviceManagerClient func(context.Context, string) (*devicemanagerclient.Client, error)
	ufsClient           func(context.Context, string) (ufsclient.Client, error)
	// dbConnectionSource is a diagnostic field set to the source of the secret.
	dbConnectionSource string
	dbConnection       *sql.DB
}

// InstallServices installs services into the server.
func InstallServices(consoleFrontend fleetconsolerpc.FleetConsoleServer, srv grpc.ServiceRegistrar) {
	fleetconsolerpc.RegisterFleetConsoleServer(srv, consoleFrontend)
}

// SetDeviceManagerClient sets the device manager client.
func SetDeviceManagerClient(consoleFrontend *FleetConsoleFrontend, deviceManagerClient func(context.Context, string) (*devicemanagerclient.Client, error)) {
	consoleFrontend.deviceManagerClient = deviceManagerClient
}

// SetUFSClient sets the UFS client.
func SetUFSClient(consoleFrontend *FleetConsoleFrontend, ufsClient func(context.Context, string) (ufsclient.Client, error)) {
	consoleFrontend.ufsClient = ufsClient
}

// SetCloudProject sets the cloud project
func SetCloudProject(consoleFrontend *FleetConsoleFrontend, cloudProject string) {
	consoleFrontend.cloudProject = cloudProject
}

// MustSetDBConnection sets the db connection and panics if it can't retrieve the secret or connect to the database.
func MustSetDBConnection(ctx context.Context, consoleFrontend *FleetConsoleFrontend) {
	secret, err := secrets.StoredSecret(ctx, *flags.DBSecret)
	if err != nil {
		panic(err)
	}
	dbURI := string(secret.Active)
	consoleFrontend.dbConnection, err = database.Connect(dbURI)
	if err != nil {
		panic(err)
	}
	consoleFrontend.dbConnectionSource = *flags.DBSecret
}
