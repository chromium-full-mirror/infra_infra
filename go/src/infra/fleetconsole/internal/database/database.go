// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package database connects to a postgres or alloydb database.
package database

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"

	"go.chromium.org/luci/common/errors"
)

func Connect(dbURI string) (*sql.DB, error) {
	dbPool, err := sql.Open("pgx", dbURI)
	if err != nil {
		return nil, errors.Annotate(err, "connect to db").Err()
	}
	return dbPool, nil
}
