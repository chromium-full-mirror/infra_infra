// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"fmt"
	"hash/fnv"
	"log"
	"net"

	ufsmodels "infra/unifiedfleet/api/v1/models"
	ufsapi "infra/unifiedfleet/api/v1/rpc"
)

// Environment is the runtime dependencies, e.g. networking, etc. of the
// implementation. The main goal of it is for unit test.
type Environment interface {
	// Subnets returns the caching subnets.
	// The slice returned may be shared, so do not modify it.
	// This function is concurrency safe.
	Subnets() []Subnet
	// CacheZones returns the caching zones.
	// A caching zone is a series of caching services serving a UFS zone.
	// The map returned may be shared, so do not modify it.
	// This function is concurrency safe.
	CacheZones() map[ufsmodels.Zone][]CachingService

	// GetZoneForServer returns UFS zone for the given server name.
	GetZoneForServer(name string) (ufsmodels.Zone, error)
	// GetZoneForDUT returns UFS zone for the given DUT name.
	GetZoneForDUT(name string) (ufsmodels.Zone, error)
}

// CachingService represents a caching service.
type CachingService string

// Subnet is a network in labs (i.e. test VLAN).
// DUTs can only access caching backends in the same Subnet.
type Subnet struct {
	IPNet    *net.IPNet
	Backends []string
}

// NewEnv creates new instance of Environment according to inputs.
func NewEnv(preferredCachingServices string, ufsClient ufsapi.FleetClient) (Environment, error) {
	env, err := NewPreferredEnv(preferredCachingServices)
	if err == nil {
		log.Printf("new cache env: created preferred env using %q", preferredCachingServices)
		return env, nil
	}

	log.Printf("new cache env: no preferred services specified, try to load from UFS")
	env, err = NewUFSEnv(ufsClient)
	if err != nil {
		return nil, fmt.Errorf("new env: %w", err)
	}
	return env, nil
}

// Frontend manages caching backends and assigns backends for client requests.
type Frontend struct {
	env Environment
}

// NewFrontend creates a new cache frontend.
func NewFrontend(env Environment) *Frontend {
	return &Frontend{env: env}
}

// AssignBackend assigns a healthy backend to the request from `dutName` on
// `filename`.
// This function is concurrency safe.
func (f *Frontend) AssignBackend(dutName, filename string) (string, error) {
	log.Printf("Assign caching backend: try UFS zone based")
	b, errZone := f.assignBackendByZone(dutName, filename)
	if errZone == nil {
		return b, nil
	}
	log.Printf("Assign caching backend: fall back to subnet based: %s", errZone)
	b, errSubnet := f.assignBackendBySubnet(dutName, filename)
	if errSubnet != nil {
		return "", fmt.Errorf("assign backend: %w (fall back to subnet based): %w", errZone, errSubnet)
	}
	return b, nil
}

func (f *Frontend) assignBackendByZone(dutName, filename string) (string, error) {
	z, err := f.env.GetZoneForDUT(dutName)
	if err != nil {
		return "", fmt.Errorf("assign backend by zone for %q: %w", dutName, err)
	}
	cs, ok := f.env.CacheZones()[z]
	if !ok {
		return "", fmt.Errorf("assign backend by zone for %q: zone %q has no caching services", dutName, z)
	}
	// TODO(guocb): eliminate the type conversion after we fully deprecate the
	// subnet based selection.
	s := make([]string, len(cs))
	for i, c := range cs {
		s[i] = string(c)
	}
	return findOneBackend(filename, s), nil

}
func (f *Frontend) assignBackendBySubnet(dutName, filename string) (string, error) {
	dutAddr, err := lookupHost(dutName)
	if err != nil {
		return "", fmt.Errorf("assign backend to %q: %w", dutName, err)
	}
	// Get cache backends serving the DUT subnet.
	subnet, ok := f.findSubnet(net.ParseIP(dutAddr))
	if !ok {
		return "", fmt.Errorf("DUT %q(%q) is not in any cache subnets (all subnets: %v)", dutName, dutAddr, f.env.Subnets())
	}
	// Get a cache backend according to the hash value of 'filename'.
	return findOneBackend(filename, subnet.Backends), nil
}

func (f *Frontend) findSubnet(ip net.IP) (*Subnet, bool) {
	for i := range f.env.Subnets() {
		if f.env.Subnets()[i].IPNet.Contains(ip) {
			return &f.env.Subnets()[i], true
		}
	}
	return nil, false
}

// findOneBackend finds one healthy backend from the current subnet according to
// the requested `filename` using 'mod N' algorithm.
func findOneBackend(filename string, backends []string) string {
	return backends[hash(filename)%len(backends)]
}

// hash returns integer hash value of the input string.
// We use the hash value to map to a backend according to a specified algorithm.
// We choose FNV hashing because we concern more on computation speed, not for
// cryptography.
func hash(s string) int {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int(h.Sum32())
}
