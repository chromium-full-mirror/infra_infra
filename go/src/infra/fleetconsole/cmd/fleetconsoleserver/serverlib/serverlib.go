// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package serverlib contains the main server loop and the modules used.
package serverlib

import (
	"context"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/rpcacl"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"

	"infra/fleetconsole/internal/consoleserver"
	"infra/fleetconsole/internal/devicemanagerclient"
)

func Options() *server.Options {
	return &server.Options{
		OpenIDRPCAuthEnable: true,
	}
}

// Modules is the slice of luci server modules used by the fleet console.
func Modules() []module.Module {
	return []module.Module{
		gaeemulation.NewModuleFromFlags(),
	}
}

var ACLMap rpcacl.Map = map[string]string{
	"/fleetconsole.FleetConsole/Ping":              "fleet-console-access",
	"/fleetconsole.FleetConsole/PingDeviceManager": "fleet-console-access",
	"/grpc.health.v1.Health/Watch":                 rpcacl.All,
}

func ServerMain(srv *server.Server) error {
	logging.Infof(srv.Context, "Begin initialization of console server.")
	consoleFrontend := consoleserver.NewFleetConsoleFrontend().(*consoleserver.FleetConsoleFrontend)
	interceptor := rpcacl.Interceptor(ACLMap)
	srv.RegisterUnifiedServerInterceptors(interceptor)
	consoleserver.InstallServices(consoleFrontend, srv)
	consoleserver.SetDeviceManagerClient(consoleFrontend, func(context.Context) (*devicemanagerclient.Client, error) {
		deviceManagerClient, err := devicemanagerclient.NewClient(srv.Context, auth.AsSelf, devicemanagerclient.DMProdURL)
		if err != nil {
			return nil, errors.Annotate(err, "configuring device manager client").Err()
		}
		return deviceManagerClient, nil
	})
	logging.Infof(srv.Context, "End initialization of console server.")
	return nil
}
