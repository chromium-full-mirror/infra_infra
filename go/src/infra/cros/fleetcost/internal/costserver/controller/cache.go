// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"time"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	models "go.chromium.org/infra/cros/fleetcost/api/models"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/entities"
)

// cacheTTL is how long a cache entity lives.
var cacheTTL = 4 * time.Hour

// StoreCachedCostResult stores a cached cost result.
func StoreCachedCostResult(ctx context.Context, hostname string, result *models.CostResult) error {
	return datastore.Put(ctx, &entities.CachedCostResultEntity{
		Hostname:       hostname,
		CostResult:     result,
		ExpirationTime: clock.Get(ctx).Now().UTC().Add(cacheTTL),
	})
}

// ReadValidCachedCostResult reads a cached cost result if it's before the deadline.
func ReadValidCachedCostResult(ctx context.Context, hostname string) (*entities.CachedCostResultEntity, error) {
	now := clock.Get(ctx).Now().UTC()
	entity := &entities.CachedCostResultEntity{
		Hostname: hostname,
	}
	// Funny story, this used to be a datastore.NewQuery, but that failed when I was testing it
	// because no cache entities existed. That gave us the cool error shown below:
	//
	// > fleet cost: get cost result: rpc error: code = FailedPrecondition desc = The query requires an ASC or DESC index for kind CachedCostResultKind and property hostname.
	//
	if err := datastore.Get(ctx, entity); err != nil {
		return entity, errors.Annotate(err, "error looking up cached cost result").Err()
	}
	if entity.ExpirationTime.After(now) {
		return entity, nil
	}
	// In cases where the entry is too old, just successfully return nothing, but log that we found an entry
	// that is indeed too old.
	logging.Infof(ctx, "expiration time is too early: %s is after %s", entity.ExpirationTime.String(), now.String())
	return nil, nil
}
