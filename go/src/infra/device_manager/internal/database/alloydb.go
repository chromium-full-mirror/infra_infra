// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"go.chromium.org/luci/common/logging"

	"infra/device_manager/internal/config"
)

type DatabaseConfig struct {
	DBHost string
	DBPort string
	DBName string
	DBUser string

	// Not the actual password but just the secret string used by SecretStore.
	DBPasswordSecret string

	// Database connection pool configs.
	ConnMaxLifetime time.Duration
	MaxIdleConns    int
	MaxOpenConns    int
}

type Client struct {
	Conn   *sql.DB
	Config DatabaseConfig
}

// ConnectDB creates a connection to the database using a TCP socket.
func ConnectDB(ctx context.Context, dbConfig DatabaseConfig) (*sql.DB, error) {
	// Use a TCP socket.
	db, err := connectTCPSocket(ctx, dbConfig)
	if err != nil {
		logging.Errorf(ctx, "ConnectDB: unable to connect: %s", err)
		return nil, err
	}

	return db, nil
}

// connectTCPSocket initializes a TCP connection pool for an AlloyDB cluster.
func connectTCPSocket(ctx context.Context, dbConfig DatabaseConfig) (*sql.DB, error) {
	dbPwd, err := config.GetSecret(ctx, dbConfig.DBPasswordSecret)
	if err != nil {
		return nil, err
	}

	logging.Debugf(ctx, "connectTCPSocket: connecting as user=%s to host=%s:%s database=%s",
		dbConfig.DBUser, dbConfig.DBHost, dbConfig.DBPort, dbConfig.DBName)
	dbURI := fmt.Sprintf("host=%s user=%s password=%s port=%s database=%s",
		dbConfig.DBHost, dbConfig.DBUser, dbPwd, dbConfig.DBPort, dbConfig.DBName)

	// dbPool is the pool of database connections.
	dbPool, err := sql.Open("pgx", dbURI)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	dbPool.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)
	dbPool.SetMaxIdleConns(dbConfig.MaxIdleConns)
	dbPool.SetMaxOpenConns(dbConfig.MaxOpenConns)

	return dbPool, nil
}
