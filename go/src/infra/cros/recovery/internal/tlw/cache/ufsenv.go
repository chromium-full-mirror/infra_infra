// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"context"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"

	ufsModels "infra/unifiedfleet/api/v1/models"
	ufsapi "infra/unifiedfleet/api/v1/rpc"
	ufsutil "infra/unifiedfleet/app/util"
)

const refreshInterval = time.Hour

// NewUFSEnv creates an instance of Environment for caching services registered
// in UFS.
// It caches the result to prevent frequent access to UFS. It updates the cache
// regularly.
func NewUFSEnv(c UFSClient) (Environment, error) {
	e := &ufsEnv{client: c, zones: make(map[string]ufsModels.Zone)}
	if err := e.refresh(); err != nil {
		return nil, fmt.Errorf("NewUFSEnv: %w", err)
	}
	return e, nil
}

type ufsEnv struct {
	client   UFSClient
	expireMu sync.Mutex
	expire   time.Time
	subnets  []Subnet
	// cacheZones is a map of a UFS zone to its caching services.
	cacheZones map[ufsModels.Zone][]CachingService
	// zones caches the zone for a machine (server or SU) name.
	zones   map[string]ufsModels.Zone
	zonesMu sync.Mutex
}

func (e *ufsEnv) Subnets() []Subnet {
	if err := e.refresh(); err != nil {
		log.Printf("UFSEnv: fallback to cached subnets due to refresh failure: %s", err)
	}
	return e.subnets
}

func (e *ufsEnv) CacheZones() map[ufsModels.Zone][]CachingService {
	if err := e.refresh(); err != nil {
		log.Printf("UFSEnv: fallback to cached due to refresh failure: %s", err)
	}
	return e.cacheZones
}

// GetZoneForServer implements the Environment interface.
func (e *ufsEnv) GetZoneForServer(name string) (ufsModels.Zone, error) {
	e.zonesMu.Lock()
	defer e.zonesMu.Unlock()

	if z, ok := e.zones[name]; ok {
		return z, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	md := metadata.Pairs("namespace", "os")
	ctx = metadata.NewOutgoingContext(ctx, md)

	m, err := e.client.GetMachine(ctx, &ufsapi.GetMachineRequest{Name: ufsutil.AddPrefix(ufsutil.MachineCollection, name)})
	if err != nil {
		return ufsModels.Zone_ZONE_UNSPECIFIED, fmt.Errorf("get zone from server name %q: %w", name, err)
	}
	e.zones[name] = m.GetLocation().GetZone()
	return e.zones[name], nil
}

// GetZoneForDUT implements the Environment interface.
func (e *ufsEnv) GetZoneForDUT(name string) (ufsModels.Zone, error) {
	// Sometimes the call to UFS is problematic, so hardcode the case for SFO36.
	if strings.HasPrefix(name, "chromeos8-") {
		log.Printf("GetZoneForDUT(%q): returned hardcode zone", name)
		return ufsModels.Zone_ZONE_SFO36_OS, nil
	}

	e.zonesMu.Lock()
	defer e.zonesMu.Unlock()

	if z, ok := e.zones[name]; ok {
		return z, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	md := metadata.Pairs("namespace", "os")
	ctx = metadata.NewOutgoingContext(ctx, md)

	lse, err := e.client.GetMachineLSE(ctx, &ufsapi.GetMachineLSERequest{
		Name: ufsutil.AddPrefix(ufsutil.MachineLSECollection, name),
	})
	if err != nil {
		return ufsModels.Zone_ZONE_UNSPECIFIED, fmt.Errorf("get zone for DUT %q: %w", name, err)
	}
	e.zones[name] = ufsModels.Zone(ufsModels.Zone_value[lse.GetZone()])
	return e.zones[name], nil
}

// getCachingSubnets returns the caching subnets from the input caching
// services.
func (e *ufsEnv) refresh() error {
	n := time.Now()
	e.expireMu.Lock()
	defer e.expireMu.Unlock()
	if n.Before(e.expire) {
		return nil
	}
	e.expire = n.Add(refreshInterval)

	cs, err := fetchCachingServicesFromUFS(e.client)
	if err != nil {
		return fmt.Errorf("refresh caching services: %w", err)
	}

	// For the caching services selected by UFS zone, we MUST NOT set the
	// ServingSubnets field.
	// Once we fully migrate all caching services to use UFS zone for selection,
	// we don't need the below two slices variable. Instead we can check the
	// caching services in the loop directly.
	var subnetBased []*ufsModels.CachingService
	var zoneBased []*ufsModels.CachingService
	for _, s := range cs {
		if state := s.GetState(); state != ufsModels.State_STATE_SERVING {
			continue
		}
		if len(s.GetServingSubnets()) > 0 {
			subnetBased = append(subnetBased, s)
		} else {
			zoneBased = append(zoneBased, s)
		}
	}
	s, err := getCachingSubnets(subnetBased)
	if err != nil {
		return fmt.Errorf("refresh caching services: %w", err)
	}
	e.subnets = s

	z, err := getCachingZones(e, zoneBased)
	if err != nil {
		return fmt.Errorf("refresh caching services: %w", err)
	}
	e.cacheZones = z
	return nil
}

func getCachingSubnets(cs []*ufsModels.CachingService) ([]Subnet, error) {
	var result []Subnet
	m := make(map[string][]string)
	for _, s := range cs {
		svc, err := cachingServiceName(s)
		if err != nil {
			return nil, fmt.Errorf("get caching subnets: %w", err)
		}
		subnets := s.GetServingSubnets()
		for _, s := range subnets {
			m[s] = append(m[s], svc)
		}
	}
	for k, v := range m {
		_, ipNet, err := net.ParseCIDR(k)
		if err != nil {
			return nil, fmt.Errorf("fetch caching subnets: parse subnet %q: %w", k, err)
		}
		sort.Strings(v)
		result = append(result, Subnet{IPNet: ipNet, Backends: v})
		log.Printf("Caching subnet: %q: %#v", k, v)
	}
	return result, nil
}

// getCachingZones get the caching zones from the given caching services.
// When the zone is not specified for a caching service, we deduce it from the
// backend caching server.
func getCachingZones(env Environment, ss []*ufsModels.CachingService) (map[ufsModels.Zone][]CachingService, error) {
	result := make(map[ufsModels.Zone][]CachingService)
	for _, s := range ss {
		name, err := cachingServiceName(s)
		svc := CachingService(name)
		if err != nil {
			return nil, fmt.Errorf("get caching zones: %w", err)
		}
		if zs := s.GetZones(); len(zs) > 0 {
			for _, z := range zs {
				result[z] = append(result[z], svc)
			}
			continue
		}
		// Deduce zone from the backend caching server.
		// We always use the secondary node/server to get the zone because the
		// secondary node is always set even the service only has one active
		// node.
		node := s.GetSecondaryNode()
		z, err := env.GetZoneForServer(node)
		if err != nil {
			return nil, fmt.Errorf("get caching zones of %q (using node %q): %w", svc, node, err)
		}
		result[z] = append(result[z], svc)
	}
	for k, v := range result {
		sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
		log.Printf("Caching zone: %q: %#v", k, v)
	}
	return result, nil
}

// cachingServiceName returns a caching service name in format of
// 'http://host:port'.
func cachingServiceName(s *ufsModels.CachingService) (string, error) {
	// The name ufsModels.CachingService has a prefix of "cachingservice/".
	nameParts := strings.Split(s.GetName(), "/")
	if len(nameParts) != 2 {
		return "", fmt.Errorf("caching service name: %q isn't in format of 'cachingservice/<name>'", s.GetName())
	}
	port := strconv.Itoa(int(s.GetPort()))
	ip, err := lookupHost(nameParts[1])
	if err != nil {
		return "", fmt.Errorf("caching service name: %w", err)
	}
	return fmt.Sprintf("http://%s", net.JoinHostPort(ip, port)), nil
}

func fetchCachingServicesFromUFS(c UFSClient) ([]*ufsModels.CachingService, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	md := metadata.Pairs("namespace", "os")
	ctx = metadata.NewOutgoingContext(ctx, md)

	resp, err := c.ListCachingServices(ctx, &ufsapi.ListCachingServicesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list caching service from UFS: %w", err)
	}
	return resp.GetCachingServices(), nil
}

// lookupHost looks up the IP address of the provided host by using the local
// resolver.
func lookupHost(hostname string) (string, error) {
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return "", fmt.Errorf("look up IP of %q: %w", hostname, err)
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("look up IP of %q: No addresses found", hostname)
	}
	return addrs[0], nil
}
