// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"time"

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

	connMaxLifetime := *flag.Duration(
		"db-conn-max-lifetime",
		time.Minute,
		"The maximum amount of time a connection may be reused. Use Duration formatting i.e. 1m, 120s, etc.",
	)

	maxIdleConns := *flag.Int(
		"db-max-idle-conns",
		50,
		"The maximum number of connections in the idle connection pool.",
	)

	maxOpenConns := *flag.Int(
		"db-max-open-conns",
		50,
		"The maximum number of open connections to the database.",
	)

	publishWorkersN := flag.Int(
		"publish-workers",
		50,
		"The number of workers set up for publishing events.",
	)

	updateBatchSize := flag.Int(
		"update-batch-size",
		1000,
		"The maximum batch size for each update.",
	)

	maxUpdateWaitTime := *flag.Int(
		"max-update-wait",
		100,
		"The maximum wait time before launching an update DB command (in milliseconds).",
	)

	server.Main(nil, modules, func(srv *server.Server) error {
		logging.Debugf(srv.Context, "main: setting up clients")
		deviceLeaseServer := frontend.NewServer()
		dbConfig := &database.DatabaseConfig{
			DBHost:           *dbHost,
			DBPort:           *dbPort,
			DBName:           *dbName,
			DBUser:           *dbUser,
			DBPasswordSecret: *dbPasswordSecret,
			ConnMaxLifetime:  connMaxLifetime,
			MaxIdleConns:     maxIdleConns,
			MaxOpenConns:     maxOpenConns,
		}

		dbClient, err := frontend.NewDBClient(srv.Context, dbConfig)
		if err != nil {
			return fmt.Errorf("main: %w", err)
		}
		deviceLeaseServer.ServiceClients.DBClient = dbClient

		err = frontend.SetUpPubSubClient(srv.Context, deviceLeaseServer, srv.Options.CloudProject)
		if err != nil {
			return err
		}
		logging.Debugf(srv.Context, "main: setup complete; now run SendNotification continuously")

		notifierOpts := controller.NotifierOpts{
			PublishWorkersN:   publishWorkersN,
			UpdateBatchSize:   updateBatchSize,
			MaxUpdateWaitTime: time.Duration(maxUpdateWaitTime) * time.Millisecond,
		}

		srv.RunInBackground("device_manager.notifier", func(ctx context.Context) {
			for ctx.Err() == nil {
				controller.SendNotifications(
					ctx,
					deviceLeaseServer.ServiceClients.DBClient.Conn,
					deviceLeaseServer.ServiceClients.PubSubClient,
					&notifierOpts,
				)
				time.Sleep(time.Second)
			}
		})
		return nil
	})
}
