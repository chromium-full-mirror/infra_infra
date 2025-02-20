// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package cache

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsapi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsutil "go.chromium.org/infra/unifiedfleet/app/util"
)

// zonesFinder finds a cache server in the same UFS zone of the given SU.
type zonesFinder struct {
	expireMu sync.Mutex
	expire   time.Time
	// cacheZones is a map of a UFS zone to its cache service addresses.
	cacheZones map[ufspb.Zone][]address

	machineToZoneMu sync.Mutex
	// machineToZone caches the zone for a machine (server or SU) name.
	machineToZone map[string]ufspb.Zone
}

func newZonesFinder() *zonesFinder {
	return &zonesFinder{
		cacheZones:    make(map[ufspb.Zone][]address),
		machineToZone: make(map[string]ufspb.Zone),
	}
}

// getCacheZones returns the up-to-date cache zones.
func (f *zonesFinder) getCacheZones(client ufsapi.FleetClient) map[ufspb.Zone][]address {
	if err := f.refreshCacheZones(client); err != nil {
		log.Printf("Cache zone finder: fallback to cached data due to refresh failure: %s", err)
	}
	return f.cacheZones
}

// refreshCacheZones makes sure the internal cache zones is up-to-date.
func (f *zonesFinder) refreshCacheZones(client ufsapi.FleetClient) error {
	n := time.Now()
	f.expireMu.Lock()
	defer f.expireMu.Unlock()

	if f.cacheZones != nil && n.Before(f.expire) {
		return nil
	}
	f.expire = n.Add(refreshInterval)

	z, err := f.fetchCacheZones(client)
	if err != nil {
		return fmt.Errorf("refresh cache zones: %w", err)
	}
	f.cacheZones = z
	return nil
}

// fetchCacheZones fetches the cache zones from the given cache services.
// When the zone is not specified for a cache service, we deduce it from the
// backend cache server.
func (f *zonesFinder) fetchCacheZones(client ufsapi.FleetClient) (map[ufspb.Zone][]address, error) {
	cacheServices, err := fetchCachingServicesFromUFS(client)
	if err != nil {
		return nil, fmt.Errorf("fetch cache zones: %w", err)
	}
	result := make(map[ufspb.Zone][]address)
	for _, s := range cacheServices {
		svc, err := cachingServiceAddr(s)
		if err != nil {
			return nil, fmt.Errorf("fetch cache zones: %w", err)
		}
		if len(s.GetServingSubnets()) > 0 {
			// Skip as the server still use the subnet based selection.
			continue
		}
		if zs := s.GetZones(); len(zs) > 0 {
			for _, z := range zs {
				result[z] = append(result[z], *svc)
			}
			continue
		}
		// Deduce zone from the backend cache server.
		// We always use the secondary node/server to get the zone because the
		// secondary node is always set even the service only has one active
		// node.
		node := s.GetSecondaryNode()
		z, err := f.getZoneForServer(node, client)
		if err != nil {
			return nil, fmt.Errorf("get cache zones of %v (using node %q): %w", svc, node, err)
		}
		result[z] = append(result[z], *svc)
	}
	for k, v := range result {
		sort.Slice(v, func(i, j int) bool {
			return v[i].Ip < v[j].Ip || (v[i].Ip == v[j].Ip && v[i].Port < v[j].Port)
		})
		log.Printf("Cache zone: %q: %#v", k, v)
	}
	return result, nil
}

// getZoneForServer returns the UFS zone for the given server name.
// Lab servers are registered as "Machine" in UFS.
func (f *zonesFinder) getZoneForServer(name string, client ufsapi.FleetClient) (ufspb.Zone, error) {
	f.machineToZoneMu.Lock()
	defer f.machineToZoneMu.Unlock()

	if z, ok := f.machineToZone[name]; ok {
		return z, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	md := metadata.Pairs("namespace", "os")
	ctx = metadata.NewOutgoingContext(ctx, md)

	m, err := client.GetMachine(ctx, &ufsapi.GetMachineRequest{
		Name: ufsutil.AddPrefix(ufsutil.MachineCollection, name),
	})
	if err != nil {
		return ufspb.Zone_ZONE_UNSPECIFIED, fmt.Errorf("get zone from name %q: %w", name, err)
	}
	f.machineToZone[name] = m.GetLocation().GetZone()
	return f.machineToZone[name], nil
}

// getZoneForSU returns the UFS zone for the given SU name.
// SUs are registered as "MachineLSE" in UFS.
func (f *zonesFinder) getZoneForSU(name string, client ufsapi.FleetClient) (ufspb.Zone, error) {
	f.machineToZoneMu.Lock()
	defer f.machineToZoneMu.Unlock()

	if z, ok := f.machineToZone[name]; ok {
		return z, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	md := metadata.Pairs("namespace", "os")
	ctx = metadata.NewOutgoingContext(ctx, md)

	lse, err := client.GetMachineLSE(ctx, &ufsapi.GetMachineLSERequest{
		Name: ufsutil.AddPrefix(ufsutil.MachineLSECollection, name),
	})
	if err != nil {
		return ufspb.Zone_ZONE_UNSPECIFIED, fmt.Errorf("get zone from name %q: %w", name, err)
	}
	f.machineToZone[name] = ufspb.Zone(ufspb.Zone_value[lse.GetZone()])
	return f.machineToZone[name], nil
}
