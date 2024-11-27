// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package serverlib contains the main server loop and the modules used.
package serverlib

import (
	"context"
	"strconv"
	"strings"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/rpcacl"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"

	"infra/fleetconsole/cmd/fleetconsoleserver/flags"
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
	"/fleetconsole.FleetConsole/ListDevices":       "fleet-console-access",
	"/discovery.Discovery/Describe":                rpcacl.All,
	"/grpc.health.v1.Health/Watch":                 rpcacl.All,
}

func ServerMain(srv *server.Server) error {
	logging.Infof(srv.Context, "Begin initialization of console server.")
	consoleFrontend := consoleserver.NewFleetConsoleFrontend().(*consoleserver.FleetConsoleFrontend)
	if !srv.Options.Prod {
		ConfigureDevCORS(srv)
	}
	interceptor := rpcacl.Interceptor(ACLMap)
	srv.RegisterUnifiedServerInterceptors(interceptor)
	consoleserver.InstallServices(consoleFrontend, srv)
	consoleserver.SetDeviceManagerClient(consoleFrontend, GetDeviceManagerClient)
	logging.Infof(srv.Context, "End initialization of console server.")
	return nil
}

func ConfigureDevCORS(srv *server.Server) {
	srv.ConfigurePRPC(func(prpcSrv *prpc.Server) {
		prpcSrv.AccessControl = func(ctx context.Context, origin string) prpc.AccessControlDecision {
			if strings.HasPrefix(origin, "http://localhost:") {
				return prpc.AllowOriginAll(ctx, origin)
			}
			return prpc.AccessControlDecision{
				AllowCrossOriginRequests: false,
				AllowCredentials:         false,
			}
		}
	})
}

func GetDeviceManagerClient(ctx context.Context) (*devicemanagerclient.Client, error) {
	deviceManagerAddr := devicemanagerclient.DMProdURL
	deviceManagerPort := devicemanagerclient.DMLeasesPort
	if *flags.UseLocalDeviceManager {
		deviceManagerAddr = "localhost"
		deviceManagerPort = 8800
	}
	if *flags.DeviceManagerAddr != "" {
		res := strings.Split(*flags.DeviceManagerAddr, ":")
		deviceManagerAddr = res[0]
		port, err := strconv.Atoi(res[1])
		if err != nil {
			return nil, errors.Annotate(err, "parsing device manager port from flag").Err()
		}

		deviceManagerPort = port
	}
	logging.Infof(ctx, "Initializing device manager client with address: %s:%d", deviceManagerAddr, deviceManagerPort)
	deviceManagerClient, err := devicemanagerclient.NewClient(ctx, auth.AsSelf, deviceManagerAddr, deviceManagerPort)
	if err != nil {
		return nil, errors.Annotate(err, "configuring device manager client").Err()
	}
	return deviceManagerClient, nil
}
