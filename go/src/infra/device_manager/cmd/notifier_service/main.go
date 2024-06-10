// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"flag"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/module"
	"go.chromium.org/luci/server/secrets"

	"infra/device_manager/internal/controller"
	"infra/device_manager/internal/database"
	"infra/device_manager/internal/frontend"
)

func main() {
	modules := []module.Module{
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
		logging.Debugf(srv.Context, "main: setting up clients")
		deviceLeaseServer := frontend.NewServer()
		dbConfig := database.DatabaseConfig{
			DBHost:           *dbHost,
			DBPort:           *dbPort,
			DBName:           *dbName,
			DBUser:           *dbUser,
			DBPasswordSecret: *dbPasswordSecret,
		}

		err := frontend.SetUpDBClient(srv.Context, deviceLeaseServer, dbConfig)
		if err != nil {
			return err
		}

		err = frontend.SetUpPubSubClient(srv.Context, deviceLeaseServer, srv.Options.CloudProject)
		if err != nil {
			return err
		}
		logging.Debugf(srv.Context, "main: setup complete; now run sendNotification continuously")

		for {
			controller.SendNotifications(
				srv.Context,
				deviceLeaseServer.ServiceClients.DBClient.Conn,
				deviceLeaseServer.ServiceClients.PubSubClient,
			)
		}
	})
}
