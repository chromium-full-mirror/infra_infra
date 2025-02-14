// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package serverlib contains the main server loop and the modules used.
package serverlib

import (
	"context"
	"net/http"
	"slices"
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
	"go.chromium.org/luci/server/secrets"

	"infra/fleetconsole/cmd/fleetconsoleserver/flags"
	"infra/fleetconsole/internal/consoleserver"
	"infra/fleetconsole/internal/devicemanagerclient"
	"infra/fleetconsole/internal/ufsclient"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
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
		// For the database info and password.
		secrets.NewModuleFromFlags(),
	}
}

var ACLMap rpcacl.Map = map[string]string{
	"/fleetconsole.FleetConsole/CleanExit":           "fleet-console-access",
	"/fleetconsole.FleetConsole/CountDevices":        "fleet-console-access",
	"/fleetconsole.FleetConsole/GetDeviceDimensions": "fleet-console-access",
	"/fleetconsole.FleetConsole/ListDevices":         "fleet-console-access",
	"/fleetconsole.FleetConsole/Ping":                "fleet-console-access",
	"/fleetconsole.FleetConsole/PingDB":              "fleet-console-access",
	"/fleetconsole.FleetConsole/PingBigQuery":        "fleet-console-access",
	"/fleetconsole.FleetConsole/PingDeviceManager":   "fleet-console-access",
	"/fleetconsole.FleetConsole/PingUfs":             "fleet-console-access",
	"/fleetconsole.FleetConsole/RepopulateCache":     "fleet-console-access",
	"/discovery.Discovery/Describe":                  rpcacl.All,
	"/grpc.health.v1.Health/Watch":                   rpcacl.All,
	"/grpc.health.v1.Health/Check":                   rpcacl.All,
}

func ServerMain(srv *server.Server) error {
	logging.Infof(srv.Context, "Begin initialization of console server.")
	consoleFrontend := consoleserver.NewFleetConsoleFrontend().(*consoleserver.FleetConsoleFrontend)
	ConfigureCORS(srv.Context, srv)
	interceptor := rpcacl.Interceptor(ACLMap)
	srv.RegisterUnifiedServerInterceptors(interceptor)
	consoleserver.SetCloudProject(consoleFrontend, srv.Options.CloudProject)
	consoleserver.InstallServices(consoleFrontend, srv)
	consoleserver.SetDeviceManagerClient(consoleFrontend, GetDeviceManagerClient)
	consoleserver.SetUFSClient(consoleFrontend, GetUfsClient)
	consoleserver.MustSetDBConnection(srv.Context, consoleFrontend)
	logging.Infof(srv.Context, "End initialization of console server.")
	return nil
}

func ConfigureCORS(ctx context.Context, srv *server.Server) {
	srv.ConfigurePRPC(func(prpcSrv *prpc.Server) {
		prpcSrv.AccessControl = func(ctx context.Context, origin string) prpc.AccessControlDecision {
			// currently we are safe to allow localhost, especially as we are not storing auth state in a cookie
			// in future it would be preferable to only allow localhost for dev environment and not on prod environment
			allowedDomains := []string{"localhost:", "luci-milo-dev.appspot.com", "luci-milo.appspot.com", "ci.chromium.org"}
			allowedSuffixes := []string{"luci-milo-dev.appspot.com"}

			matches := slices.ContainsFunc(allowedDomains, func(address string) bool {
				return strings.HasPrefix(origin, "https://"+address) || strings.HasPrefix(origin, "http://"+address)
			})

			matchesSuffix := slices.ContainsFunc(allowedSuffixes, func(address string) bool {
				return strings.HasSuffix(origin, address)
			})

			if matches || matchesSuffix {
				return prpc.AllowOriginAll(ctx, origin)
			}
			return prpc.AccessControlDecision{
				AllowCrossOriginRequests: false,
				AllowCredentials:         false,
			}
		}
	})
}

func GetDeviceManagerClient(ctx context.Context, cloudProject string) (*devicemanagerclient.Client, error) {
	deviceManagerAddr := devicemanagerclient.DMDevURL
	if isProdEnvironment(cloudProject) {
		deviceManagerAddr = devicemanagerclient.DMProdURL
	}
	deviceManagerPort := devicemanagerclient.DMLeasesPort
	if *flags.UseLocalDeviceManager {
		logging.Infof(ctx, "using local device manager")
		deviceManagerAddr = "localhost"
		deviceManagerPort = 8800
	}
	if *flags.DeviceManagerAddr != "" {
		logging.Infof(ctx, "parsing device manager from flag: %s", *flags.DeviceManagerAddr)
		res := strings.Split(*flags.DeviceManagerAddr, ":")
		deviceManagerAddr = res[0]
		port, err := strconv.Atoi(res[1])
		if err != nil {
			return nil, errors.Annotate(err, "parsing device manager port from flag").Err()
		}

		deviceManagerPort = port
	}
	logging.Infof(ctx, "Initializing device manager client with address: %s:%d", deviceManagerAddr, deviceManagerPort)
	deviceManagerClient, err := devicemanagerclient.NewClient(ctx, auth.AsCredentialsForwarder, deviceManagerAddr, deviceManagerPort, *flags.UseLocalDeviceManager)
	if err != nil {
		logging.Errorf(ctx, "encountered error while initializing device manager: %s", err)
		return nil, errors.Annotate(err, "configuring device manager client").Err()
	}
	return deviceManagerClient, nil
}

func GetUfsClient(ctx context.Context, cloudProject string) (ufsclient.Client, error) {
	ufsAddr := ufsclient.UfsDevURL
	if isProdEnvironment(cloudProject) {
		ufsAddr = ufsclient.UfsProdURL
	}
	ufsPort := ufsclient.UfsPort
	if *flags.UseLocalUfs {
		ufsAddr = "localhost"
		ufsPort = 8800
	}
	if *flags.UfsAddr != "" {
		res := strings.Split(*flags.UfsAddr, ":")
		ufsAddr = res[0]
		port, err := strconv.Atoi(res[1])
		if err != nil {
			return nil, errors.Annotate(err, "parsing ufs port from flag").Err()
		}
		ufsPort = port
	}

	// This stanza is copied from the fleet cost service.
	t, err := auth.GetRPCTransport(ctx, auth.AsSelf, auth.WithScopes(auth.CloudOAuthScopes...))
	if err != nil {
		return nil, errors.Annotate(err, "setting up UFS client").Err()
	}
	httpClient := &http.Client{
		Transport: t,
	}
	prpcClient := &prpc.Client{
		C:    httpClient,
		Host: ufsAddr,
	}
	ufsClient := ufsAPI.NewFleetPRPCClient(prpcClient)

	logging.Infof(ctx, "Initializing ufs client with address: %s:%d", ufsAddr, ufsPort)
	return ufsClient, nil
}

func isProdEnvironment(cloudProject string) bool {
	return !strings.HasSuffix(cloudProject, "dev")
}
