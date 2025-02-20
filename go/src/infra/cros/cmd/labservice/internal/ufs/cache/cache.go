// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cache provides functionality to map DUTs to caching servers.
package cache

import (
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	ufsapi "infra/unifiedfleet/api/v1/rpc"
)

func NewLocator() *Locator {
	return &Locator{
		subnets: newSubnetsFinder(),
		zones:   newZonesFinder(),
	}
}

// Locator helps to find a caching server for any given DUT.
// It tries to use UFS zone and falls back to subnets to match the given DUT
// with a caching server.
// It caches intermediate results, e.g. IP addresses, UFS zones, etc.
type Locator struct {
	// preferredServices is the services preferred to use. It supersedes
	// other ways to locate the services.
	preferredServices []address
	subnets           *subnetsFinder
	zones             *zonesFinder
}

// SetPreferredServices sets preferred services form a string slice.
func (l *Locator) SetPreferredServices(services []string) error {
	r := make([]address, len(services))
	for i, s := range services {
		a, err := parseAddress(s)
		if err != nil {
			return fmt.Errorf("set preferred services %q: %w", s, err)
		}
		r[i] = *a
	}
	l.preferredServices = r
	return nil
}

// FindCacheServer returns the ip address of a cache server mapped to a dut.
func (l *Locator) FindCacheServer(dutName string, client ufsapi.FleetClient) (*labapi.IpEndpoint, error) {
	dutName = extractDutName(dutName)
	if dutName == "" {
		return nil, fmt.Errorf("find cache server for %q: dut-name is empty ", dutName)
	}
	cs, err := l.findPreferredServer(dutName)
	if err == nil {
		return cs, nil
	}
	log.Printf("Find cache server for %q: try zone based: %s", dutName, err)
	cs, err = l.findCacheServerByZone(dutName, client)
	if err == nil {
		return cs, nil
	}
	log.Printf("Find cache server: fall back to subnet based: %s", err)
	cs, err = l.findCacheServerBySubnet(dutName, client)
	if err != nil {
		return nil, fmt.Errorf("find cache server for %q: %w", dutName, err)
	}
	return cs, nil
}

var (
	// Peripheral suffixes used with dut-name to create peripheral names.
	peripheralNameSuffixes = []string{
		"-router",
		"-pcap",
		"-btpeer1",
		"-btpeer2",
		"-btpeer3",
		"-btpeer4",
		"-btpeer5",
		"-btpeer6",
	}
)

// extractDutName extracts dut-name from peripherals names.
// If the name is peripheral, then it will have one of the suffixes.
func extractDutName(name string) string {
	for _, s := range peripheralNameSuffixes {
		if strings.HasSuffix(name, s) {
			return strings.TrimSuffix(name, s)
		}
	}
	return name
}

func (l *Locator) findPreferredServer(dutName string) (*labapi.IpEndpoint, error) {
	if len(l.preferredServices) == 0 {
		return nil, fmt.Errorf("find preferred cache server for %q: no preferred servers", dutName)
	}
	be := chooseBackend(l.preferredServices, dutName)
	return &labapi.IpEndpoint{
		Address: be.Ip,
		Port:    be.Port,
	}, nil
}

func (l *Locator) findCacheServerByZone(dutName string, client ufsapi.FleetClient) (*labapi.IpEndpoint, error) {
	z, err := l.zones.getZoneForSU(dutName, client)
	if err != nil {
		return nil, fmt.Errorf("find cache server by zone for %q: %w", dutName, err)
	}
	cs, ok := l.zones.getCacheZones(client)[z]
	if !ok {
		return nil, fmt.Errorf("find cache server by zone for %q: no cache server for zone %q", dutName, z)
	}
	be := chooseBackend(cs, dutName)

	return &labapi.IpEndpoint{
		Address: be.Ip,
		Port:    be.Port,
	}, nil
}

func (l *Locator) findCacheServerBySubnet(dutName string, client ufsapi.FleetClient) (*labapi.IpEndpoint, error) {
	subnets, err := l.subnets.getSubnets(client)
	if err != nil {
		return nil, fmt.Errorf("find cache server by subnet: %w", err)
	}

	sn, err := findSubnet(dutName, subnets)
	if err != nil {
		return nil, fmt.Errorf("find cache server by subnet: %w", err)
	}

	be := chooseBackend(sn.Backends, dutName)
	return &labapi.IpEndpoint{
		Address: be.Ip,
		Port:    be.Port,
	}, nil
}

func findSubnet(dutName string, subnets []Subnet) (Subnet, error) {
	addr, err := lookupHost(dutName)
	if err != nil {
		return Subnet{}, status.Errorf(codes.NotFound, fmt.Sprintf("FindCacheServer: lookup IP of %q: %s", dutName, err.Error()))
	}
	log.Printf("FindCacheServer: the IP of %q is %q", dutName, addr)

	ip := net.ParseIP(addr)
	for _, s := range subnets {
		if s.IPNet.Contains(ip) {
			return s, nil
		}
	}
	return Subnet{}, fmt.Errorf("%q is not in any cache subnets (all subnets: %v)", addr, subnets)
}

// chooseBackend finds one healthy backend from given backends according to
// the requested `hostname` using 'mod N' algorithm.
func chooseBackend(backends []address, hostname string) address {
	return backends[hash(hostname)%len(backends)]
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

// parseAddress parses an address string in format of
// "[http://]<server>:<port>" into an address.
func parseAddress(addr string) (*address, error) {
	a := addr
	var port int32
	if strings.HasPrefix(a, "http://") {
		port = 80
		a = a[7:] // Remove the prefix.
	}
	parts := strings.Split(a, ":")
	parts[0] = strings.TrimSpace(parts[0])
	if parts[0] == "" {
		return nil, fmt.Errorf("parse address %q: empty server part", addr)
	}
	l := len(parts)
	switch {
	case l == 1 && port == 0:
		return nil, fmt.Errorf("parse address %q: no scheme or port specified", addr)
	case l == 1 && port != 0:
		return &address{Ip: parts[0], Port: port}, nil
	case l == 2:
		p, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("parse address %q: port must be a number", addr)
		}
		return &address{Ip: parts[0], Port: int32(p)}, nil
	case l > 2:
		return nil, fmt.Errorf("parse address %q: format must be [http://]<server>[:port]", addr)
	}
	return nil, fmt.Errorf("parse address %q: unknown error", addr)
}
