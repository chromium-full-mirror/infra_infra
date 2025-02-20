// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package entities

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/datastore"

	"go.chromium.org/infra/appengine/chrome-test-health/datastorage"
)

var (
	ErrNotFound = errors.New("FinditConfigRoot not found")
)

type FinditConfigRoot struct {
	Key     *datastore.Key
	Current int `datastore:"current"`
}

// Get function fetches the FinditConfigRoot entity from the datastore.
// Note that there is a single FinditConfigRoot entity present in the datastore
// and this function fetches that. The purpose of this entity is to maintain
// the version of the latest code coverage configuration.
func (f *FinditConfigRoot) Get(ctx context.Context, client datastorage.IDataClient) error {
	records := []FinditConfigRoot{}
	if err := client.Query(ctx, &records, "FinditConfigRoot", nil, nil, 1); err != nil {
		return fmt.Errorf("FinditConfigRoot: %w", err)
	}
	if len(records) == 0 {
		return ErrNotFound
	}
	*f = records[0]
	return nil
}
