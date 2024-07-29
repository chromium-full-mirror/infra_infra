// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"flag"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/openid"
	"go.chromium.org/luci/server/cron"
	"go.chromium.org/luci/server/module"
	"go.chromium.org/luci/server/secrets"

	"infra/device_manager/internal/database"
	"infra/device_manager/internal/frontend"
	"infra/device_manager/internal/jobs"
)

func main() {
	modules := []module.Module{
		cron.NewModuleFromFlags(),
		secrets.NewModuleFromFlags(),
	}

	dbHost := flag.String(
		"db-host",
		"device_manager_db",
		"The DB host location to connect to.",
	)

	dbPort := flag.String(
		"db-port",
		"5432",
		"The DB port number to connect to.",
	)

	dbName := flag.String(
		"db-name",
		"device_manager_db",
		"The DB name to connect to.",
	)

	dbUser := flag.String(
		"db-user",
		"postgres",
		"The DB user to connect as.",
	)

	dbPasswordSecret := flag.String(
		"db-password-secret",
		"devsecret-text://password",
		"The DB password location for Secret Store to use.",
	)

	server.Main(nil, modules, func(srv *server.Server) error {
		logging.Debugf(srv.Context, "main: initializing server")

		ctx := context.Background()
		c, err := google.FindDefaultCredentials(ctx, compute.ComputeScope)
		if err != nil {
			return err
		}

		// This allows auth to use Identity tokens.
		srv.SetRPCAuthMethods([]auth.Method{
			// The primary authentication method.
			&openid.GoogleIDTokenAuthMethod{
				AudienceCheck: openid.AudienceMatchesHost,
				SkipNonJWT:    true, // pass OAuth2 access tokens through
			},
			// Backward compatibility for RPC Explorer and old clients.
			&auth.GoogleOAuth2Method{
				Scopes: []string{"https://www.googleapis.com/auth/userinfo.email"},
			},
		})

		srv.ConfigurePRPC(func(p *prpc.Server) {
			p.AccessControl = prpc.AllowOriginAll
		})

		logging.Debugf(srv.Context, "main: installing services")

		deviceLeaseServer := frontend.NewServer()
		dbConfig := database.DatabaseConfig{
			DBHost:           *dbHost,
			DBPort:           *dbPort,
			DBName:           *dbName,
			DBUser:           *dbUser,
			DBPasswordSecret: *dbPasswordSecret,
		}

		err = frontend.SetUpDBClient(srv.Context, deviceLeaseServer, dbConfig)
		if err != nil {
			return err
		}

		err = frontend.SetUpPubSubClient(srv.Context, deviceLeaseServer, srv.Options.CloudProject)
		if err != nil {
			return err
		}

		frontend.InstallServices(deviceLeaseServer, srv)
		cron.RegisterHandler("import-ufs-devices", func(ctx context.Context) error {
			return jobs.ImportUFSDevices(ctx, deviceLeaseServer.ServiceClients, c.ProjectID)
		})
		cron.RegisterHandler("expire-leases", func(ctx context.Context) error {
			return jobs.ExpireLeases(ctx, deviceLeaseServer.ServiceClients)
		})

		logging.Debugf(srv.Context, "main: initialization finished")

		return nil
	})
}
