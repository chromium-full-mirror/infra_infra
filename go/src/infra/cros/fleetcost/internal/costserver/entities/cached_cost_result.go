// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package entities

import (
	"fmt"
	"time"

	"go.chromium.org/luci/gae/service/datastore"

	fleetcostpb "infra/cros/fleetcost/api/models"
)

// CachedCostResultKind is the datastore type for a cached DUT.
const CachedCostResultKind = "CachedCostResultKind"

// CachedCostResultEntity is the datastore type for cached DUT cost.
type CachedCostResultEntity struct {
	_kind string `gae:"$kind,CachedCostResultKind"`
	// The hostname or DeviceID are the two fields that are
	// useful for looking up a device.
	Hostname string `gae:"hostname"`
	DeviceID string `gae:"deviceid"`
	// ExpirationTime is the time that the cache entry is no longer valid.
	ExpirationTime time.Time               `gae:"expiration_time"`
	CostResult     *fleetcostpb.CostResult `gae:"cost_result"`
	// deprecated! Do not use.
	CostReport *fleetcostpb.CostReport `gae:"cost_report"`
}

// For better hygiene, and to give us flexibility in the future to read old schemas, we make CachedDUTEntity a PropertyLoadSaver.
var _ datastore.PropertyLoadSaver = &CachedCostResultEntity{}

// This interface assertion is more critical. The ID of a CachedDUTEntity is computed from the hostname and the deviceid.
var _ datastore.MetaGetterSetter = &CachedCostResultEntity{}

// Silence staticcheck warning about unused field.
var _ = CachedCostResultEntity{}._kind

// GetAllMeta returns all the meta keys. Just use the default implementation, it's fine.
func (entity *CachedCostResultEntity) GetAllMeta() datastore.PropertyMap {
	return datastore.GetPLS(entity).GetAllMeta()
}

// SetMeta always returns false. This communicates to the LUCI ORM that our meta keys cannot be changed.
//
// This works better than not-implementing the method for reasons that I don't completely understand and don't really want to get into here.
func (entity *CachedCostResultEntity) SetMeta(key string, value any) bool {
	return false
}

// GetMeta gets a meta key. Compute the ID, fall back to the default implementation.
//
// Use the entity and the hostname in order as the ID. This isn't IDeal (get it?) because we're realistically just using one or the other.
func (entity *CachedCostResultEntity) GetMeta(key string) (any, bool) {
	if key == "id" {
		return fmt.Sprintf("v1;%s;%s", entity.Hostname, entity.DeviceID), true
	}
	return datastore.GetPLS(entity).GetMeta(key)
}

// Save saves an entity.
func (entity *CachedCostResultEntity) Save(withMeta bool) (datastore.PropertyMap, error) {
	return datastore.GetPLS(entity).Save(withMeta)
}

// Load loads an entity.
func (entity *CachedCostResultEntity) Load(propertyMap datastore.PropertyMap) error {
	return datastore.GetPLS(entity).Load(propertyMap)
}
