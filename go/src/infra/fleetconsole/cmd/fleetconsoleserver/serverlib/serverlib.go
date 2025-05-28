// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package serverlib contains the main server loop and the modules used.
package serverlib

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/rpcacl"
	"go.chromium.org/luci/server/cron"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"
	"go.chromium.org/luci/server/secrets"
	"go.chromium.org/luci/server/sqldb"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/cmd/fleetconsoleserver/flags"
	"go.chromium.org/infra/fleetconsole/internal/consoleserver"
	"go.chromium.org/infra/fleetconsole/internal/devicemanagerclient"
	"go.chromium.org/infra/fleetconsole/internal/ufsclient"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
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
		cron.NewModuleFromFlags(),
		sqldb.NewModuleFromFlags(),
	}
}

var ACLMap rpcacl.Map = map[string]string{
	"/fleetconsole.FleetConsole/CleanExit":                                  "googlers",
	"/fleetconsole.FleetConsole/CountDevices":                               "googlers",
	"/fleetconsole.FleetConsole/ExportDevicesToCSV":                         "googlers",
	"/fleetconsole.FleetConsole/GetDeviceDimensions":                        "googlers",
	"/fleetconsole.FleetConsole/ListDevices":                                "googlers",
	"/fleetconsole.FleetConsole/ListResourceRequests":                       "googlers",
	"/fleetconsole.FleetConsole/GetResourceRequestsMultiselectFilterValues": "googlers",
	"/fleetconsole.FleetConsole/CountResourceRequests":                      "googlers",
	"/fleetconsole.FleetConsole/Ping":                                       "googlers",
	"/fleetconsole.FleetConsole/PingDB":                                     "googlers",
	"/fleetconsole.FleetConsole/PingBigQuery":                               "googlers",
	"/fleetconsole.FleetConsole/PingDeviceManager":                          "googlers",
	"/fleetconsole.FleetConsole/PingUfs":                                    "googlers",
	"/fleetconsole.FleetConsole/RepopulateCache":                            "mdb/fleet-console-eng",
	"/discovery.Discovery/Describe":                                         rpcacl.All,
	"/grpc.health.v1.Health/Watch":                                          rpcacl.All,
	"/grpc.health.v1.Health/Check":                                          rpcacl.All,
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

	cron.RegisterHandler("ping-db", func(ctx context.Context) error {
		_, err := consoleFrontend.PingDB(ctx, &fleetconsolerpc.PingDBRequest{})
		return err
	})
	cron.RegisterHandler("repopulate-cache", func(ctx context.Context) error {
		_, err := consoleFrontend.RepopulateCache(ctx, &fleetconsolerpc.RepopulateCacheRequest{})
		return err
	})

	logging.Infof(srv.Context, "End initialization of console server.")
	return nil
}

var allowedOrigins = []*regexp.Regexp{
	// Currently we are safe to allow localhost, especially as we are not storing auth state in a cookie.
	// In future, it would be preferable to only allow localhost for dev environment and not on prod environment.
	regexp.MustCompile(`^http://localhost$`),
	regexp.MustCompile(`^http://localhost:8080$`),
	regexp.MustCompile(`^https://ci[.]chromium[.]org$`),
	regexp.MustCompile(`^https://(staging[.])?luci[.]app$`),
	regexp.MustCompile(`^https://([A-Za-z0-9-_]+-dot-)?luci-milo(-dev)?[.]appspot[.]com$`),
}

func controlPRPCAccess(ctx context.Context, origin string) prpc.AccessControlDecision {
	for _, re := range allowedOrigins {
		if re.MatchString(origin) {
			return prpc.AllowOriginAll(ctx, origin)
		}
	}
	return prpc.AccessControlDecision{
		AllowCrossOriginRequests: false,
		AllowCredentials:         false,
	}
}

func ConfigureCORS(ctx context.Context, srv *server.Server) {
	srv.ConfigurePRPC(func(prpcSrv *prpc.Server) {
		prpcSrv.AccessControl = controlPRPCAccess
	})
}

func GetDeviceManagerClient(ctx context.Context, isProd bool) (*devicemanagerclient.Client, error) {
	deviceManagerAddr := devicemanagerclient.DMDevURL
	if isProd {
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

func GetUfsClient(ctx context.Context, isProd bool) (ufsclient.Client, error) {
	ufsAddr := ufsclient.UfsDevURL
	if isProd {
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
