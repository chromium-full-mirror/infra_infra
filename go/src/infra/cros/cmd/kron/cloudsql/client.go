// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cloudsql implements the interface to interact with the Cloud SQL API.
package cloudsql

import (
	"context"
	"fmt"
	"net"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"go.chromium.org/infra/cros/cmd/kron/common"
	"go.chromium.org/infra/cros/cmd/kron/secretmanager"
)

type Client interface {
	Connect(ctx context.Context, user, password, databaseName, connectionName string) error
	Read(ctx context.Context, query string, handleScanRows func(rows pgx.Rows) (any, error)) (any, error)
	Exec(ctx context.Context, sqlCommand string, insertArgs ...any) (int, error)
}
type sqlClient struct {
	dbPool *pgxpool.Pool
	dbName string
}

// Connect initiates a connection to the given PSQL database.
func (c *sqlClient) Connect(ctx context.Context, username, password, databaseName, connectionName string) error {
	// Populate connection string with login and location information.
	connectionString := fmt.Sprintf("user=%s password=%s database=%s", username, password, databaseName)
	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return err
	}

	// Create connection interface with Cloud SQL
	cloudSQLDialer, err := cloudsqlconn.NewDialer(ctx)
	if err != nil {
		return err
	}
	// Use the Cloud SQL connector to handle connecting to the instance.
	// This approach does NOT require the Cloud SQL proxy.
	config.ConnConfig.DialFunc = func(ctx context.Context, network, instance string) (net.Conn, error) {
		return cloudSQLDialer.Dial(ctx, connectionName)
	}
	// Initiate the connection to the pool.
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return err
	}
	c.dbPool = pool

	return nil
}

// Read queries the PSQL db. The user provides handleScanRows to deal with the
// generic types used in the sql adapter code.
//
// NOTE: it is assumed that the database name needs to be inserted into the SQL
// query and will be retrieved by the client during the instantiation.
func (c *sqlClient) Read(ctx context.Context, query string, handleScanRows func(rows pgx.Rows) (any, error)) (any, error) {
	if c.dbPool == nil {
		return nil, fmt.Errorf("the dbPool was not initialized and connected")
	}

	// Place the database name in the SELECT statement.
	query = fmt.Sprintf(query, c.dbName)

	rows, err := c.dbPool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	// Safe to double close if the handle function below decides to close as
	// well.
	defer rows.Close()

	// Since the PSQl adapter works with generic types it's up to the caller of
	// this function to determine how to ingest the row returned.
	return handleScanRows(rows)
}

// Exec executes the command with the given value args. It returns how many rows
// were affected.
//
// NOTE: it is assumed that the database name needs to be inserted into the SQL
// query and will be retrieved by the client during the instantiation.
func (c *sqlClient) Exec(ctx context.Context, sqlCommand string, insertArgs ...any) (int, error) {
	if c.dbPool == nil {
		return 0, fmt.Errorf("the dbPool was not initialized and connected")
	}

	// Place the database name in the INSERT statement.
	sqlCommand = fmt.Sprintf(sqlCommand, c.dbName)

	commandTag, err := c.dbPool.Exec(ctx, sqlCommand, insertArgs...)
	if err != nil {
		return 0, err
	}

	return int(commandTag.RowsAffected()), nil
}

// InitBuildsClient initialize a PSQL client for for the kron-builds table.
func InitBuildsClient(ctx context.Context, isProd, isWriter bool) (Client, error) {
	client := &sqlClient{}

	var projectNumber, usernameVersion, passwordVersion, dbNameVersion, connectionNameVersion int
	var usernameSecret, passwordSecret string

	// Select the secret for the given account requested.
	if isWriter {
		usernameSecret = common.KronWriterUsernameSecret
		passwordSecret = common.KronWriterPasswordSecret
	} else {
		usernameSecret = common.KronReaderUsernameSecret
		passwordSecret = common.KronReaderPasswordSecret
	}

	// Set the per project values.
	if isProd {
		if isWriter {
			usernameVersion = common.KronWriterUsernameSecretVersionProd
			passwordVersion = common.KronWriterPasswordSecretVersionProd
		} else {
			usernameVersion = common.KronReaderUsernameSecretVersionProd
			passwordVersion = common.KronReaderPasswordSecretVersionProd
		}

		projectNumber = common.ProdProjectNumber
		dbNameVersion = common.KronBuildsDBNameSecretVersionProd
		connectionNameVersion = common.KronBuildsConnectionNameSecretVersionProd
	} else {
		if isWriter {
			usernameVersion = common.KronWriterUsernameSecretVersionStaging
			passwordVersion = common.KronWriterPasswordSecretVersionStaging
		} else {
			usernameVersion = common.KronReaderUsernameSecretVersionStaging
			passwordVersion = common.KronReaderPasswordSecretVersionStaging

		}

		projectNumber = common.StagingProjectNumber
		dbNameVersion = common.KronBuildsDBNameSecretVersionStaging
		connectionNameVersion = common.KronBuildsConnectionNameSecretVersionStaging
	}

	// Get username from Cloud Secret Manager.
	username, err := secretmanager.GetSecret(ctx, usernameSecret, projectNumber, usernameVersion)
	if err != nil {
		return nil, err
	}
	// Get password from Cloud Secret Manager.
	password, err := secretmanager.GetSecret(ctx, passwordSecret, projectNumber, passwordVersion)
	if err != nil {
		return nil, err
	}

	// Get DB name from Cloud Secret Manager.
	databaseName, err := secretmanager.GetSecret(ctx, common.KronBuildsDBNameSecret, projectNumber, dbNameVersion)
	if err != nil {
		return nil, err
	}
	// Store the database name for later use.
	client.dbName = databaseName

	// Get connection name  from Cloud Secret Manager.
	connectionName, err := secretmanager.GetSecret(ctx, common.KronBuildsConnectionNameSecret, projectNumber, connectionNameVersion)
	if err != nil {
		return nil, err
	}

	// Connection the client to the database.
	err = client.Connect(ctx, username, password, databaseName, connectionName)
	if err != nil {
		return nil, err
	}

	return client, nil
}
