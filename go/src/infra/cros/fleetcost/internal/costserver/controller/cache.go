// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/gae/service/datastore"

	models "infra/cros/fleetcost/api/models"
	"infra/cros/fleetcost/internal/costserver/entities"
)

// cacheTTL is how long a cache entity lives.
var cacheTTL = 4 * time.Hour

// StoreCachedCostResult stores a cached cost result.
func StoreCachedCostResult(ctx context.Context, hostname string, result *models.CostResult) error {
	return datastore.Put(ctx, &entities.CachedCostResultEntity{
		Hostname:       hostname,
		CostResult:     result,
		ExpirationTime: time.Now().UTC().Add(cacheTTL),
	})
}

// ReadCachedCostResult reads a cached cost result if it's before the deadline.
func ReadCachedCostResult(ctx context.Context, hostname string) (*models.CostResult, error) {
	deadline := time.Now().UTC()
	query := datastore.NewQuery(entities.CachedCostResultKind).Eq("hostname", hostname)
	var ents []*entities.CachedCostResultEntity
	if err := datastore.GetAll(ctx, query, &ents); err != nil {
		return nil, err
	}
	if len(ents) == 0 {
		return nil, datastore.ErrNoSuchEntity
	}
	if len(ents) != 1 {
		return nil, fmt.Errorf("cache inconsitency: found %d records for hostname %q", len(ents), hostname)
	}
	if ents[0].ExpirationTime.After(deadline) {
		return ents[0].CostResult, nil
	}
	return nil, fmt.Errorf("expiration time is too early: %s is after %s", ents[0].ExpirationTime.String(), deadline.String())
}
