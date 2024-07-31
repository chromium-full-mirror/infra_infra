// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"

	ufsModels "infra/unifiedfleet/api/v1/models"
	ufsapi "infra/unifiedfleet/api/v1/rpc"
	ufsutil "infra/unifiedfleet/app/util"
)

type fakeUFSClient struct {
	ufsapi.FleetClient
	services    []*ufsModels.CachingService
	machineLSEs map[string]*ufsModels.MachineLSE
	machines    map[string]*ufsModels.Machine
}

func (c fakeUFSClient) ListCachingServices(context.Context, *ufsapi.ListCachingServicesRequest, ...grpc.CallOption) (*ufsapi.ListCachingServicesResponse, error) {
	return &ufsapi.ListCachingServicesResponse{
		CachingServices: c.services,
	}, nil
}

func (c fakeUFSClient) GetMachineLSE(_ context.Context, req *ufsapi.GetMachineLSERequest, o ...grpc.CallOption) (*ufsModels.MachineLSE, error) {
	n := ufsutil.RemovePrefix(req.GetName())
	return c.machineLSEs[n], nil
}

func (c fakeUFSClient) GetMachine(_ context.Context, req *ufsapi.GetMachineRequest, o ...grpc.CallOption) (*ufsModels.Machine, error) {
	n := ufsutil.RemovePrefix(req.GetName())
	return c.machines[n], nil
}
func TestSubnets_multipleSubnets(t *testing.T) {
	t.Parallel()
	c := &fakeUFSClient{services: []*ufsModels.CachingService{
		{
			Name:           "cachingservice/1.1.1.1",
			Port:           8001,
			ServingSubnets: []string{"1.1.1.0/24", "1.1.2.0/24"},
			State:          ufsModels.State_STATE_SERVING,
		},
	}}
	env, err := NewUFSEnv(c)
	if err != nil {
		t.Fatalf("NewUFSEnv(fakeClient) failed: %s", err)
	}
	want := []Subnet{
		{
			IPNet:    &net.IPNet{IP: net.IPv4(1, 1, 1, 0), Mask: net.CIDRMask(24, 32)},
			Backends: []string{"http://1.1.1.1:8001"},
		},
		{
			IPNet:    &net.IPNet{IP: net.IPv4(1, 1, 2, 0), Mask: net.CIDRMask(24, 32)},
			Backends: []string{"http://1.1.1.1:8001"},
		},
	}
	got := env.Subnets()
	less := func(a, b Subnet) bool { return a.IPNet.String() < b.IPNet.String() }
	if diff := cmp.Diff(want, got, cmpopts.SortSlices(less)); diff != "" {
		t.Errorf("Subnets() returned unexpected diff (-want +got):\n%s", diff)
	}
}

func TestSubnets_refresh(t *testing.T) {
	t.Parallel()
	c := &fakeUFSClient{services: []*ufsModels.CachingService{
		{
			Name:           "cachingservice/1.1.1.1",
			Port:           8001,
			ServingSubnets: []string{"1.1.1.1/24"},
			State:          ufsModels.State_STATE_SERVING,
		},
	}}
	env, err := NewUFSEnv(c)
	if err != nil {
		t.Fatalf("NewUFSEnv(fakeClient) failed: %s", err)
	}
	want := []Subnet{{
		IPNet:    &net.IPNet{IP: net.IPv4(1, 1, 1, 0), Mask: net.CIDRMask(24, 32)},
		Backends: []string{"http://1.1.1.1:8001"},
	}}
	t.Run("add initial data", func(t *testing.T) {
		got := env.Subnets()
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Subnets() returned unexpected diff (-want +got):\n%s", diff)
		}
	})
	t.Run("expired data will be updated", func(t *testing.T) {
		c.services = []*ufsModels.CachingService{{
			Name:           "cachingservice/2.2.2.2",
			Port:           8002,
			ServingSubnets: []string{"2.2.2.2/24"},
			State:          ufsModels.State_STATE_SERVING,
		}}
		t.Run("Subnets won't change when not expired", func(t *testing.T) {
			got := env.Subnets()
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Subnets() returned unexpected diff (-want +got):\n%s", diff)
			}
		})
		t.Run("Subnets will change when expired", func(t *testing.T) {
			// Set the `expire` to an old time to ensure the cache is expired.
			env.(*ufsEnv).expire = time.Time{}
			gotNew := env.Subnets()
			wantNew := []Subnet{{
				IPNet:    &net.IPNet{IP: net.IPv4(2, 2, 2, 0), Mask: net.CIDRMask(24, 32)},
				Backends: []string{"http://2.2.2.2:8002"},
			}}
			if diff := cmp.Diff(wantNew, gotNew); diff != "" {
				t.Errorf("Subnets() returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	})
}

func TestZones_initialization(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		svc  []*ufsModels.CachingService
		want map[ufsModels.Zone][]CachingService
	}{
		"one server serves one zone": {
			[]*ufsModels.CachingService{
				{
					Name:  "cachingservice/1.1.1.1",
					Port:  8001,
					Zones: []ufsModels.Zone{ufsModels.Zone_ZONE_SFO36_OS},
					State: ufsModels.State_STATE_SERVING,
				},
			},
			map[ufsModels.Zone][]CachingService{
				ufsModels.Zone_ZONE_SFO36_OS: {"http://1.1.1.1:8001"},
			},
		},
		"one server serves two zones": {
			[]*ufsModels.CachingService{
				{
					Name:  "cachingservice/1.1.1.1",
					Port:  8001,
					Zones: []ufsModels.Zone{ufsModels.Zone_ZONE_CHROMEOS2, ufsModels.Zone_ZONE_CHROMEOS4},
					State: ufsModels.State_STATE_SERVING,
				},
			},
			map[ufsModels.Zone][]CachingService{
				ufsModels.Zone_ZONE_CHROMEOS2: {"http://1.1.1.1:8001"},
				ufsModels.Zone_ZONE_CHROMEOS4: {"http://1.1.1.1:8001"},
			},
		},
		"two servers serve one zones": {
			[]*ufsModels.CachingService{
				{
					Name:  "cachingservice/1.1.1.1",
					Port:  8001,
					Zones: []ufsModels.Zone{ufsModels.Zone_ZONE_SFO36_OS},
					State: ufsModels.State_STATE_SERVING,
				},
				{
					Name:  "cachingservice/1.1.1.2",
					Port:  8001,
					Zones: []ufsModels.Zone{ufsModels.Zone_ZONE_SFO36_OS},
					State: ufsModels.State_STATE_SERVING,
				},
			},
			map[ufsModels.Zone][]CachingService{
				ufsModels.Zone_ZONE_SFO36_OS: {"http://1.1.1.1:8001", "http://1.1.1.2:8001"},
			},
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := &fakeUFSClient{services: test.svc}
			env, err := NewUFSEnv(c)
			if err != nil {
				t.Fatalf("NewUFSEnv(fakeClient) failed: %s", err)
			}
			got := env.CacheZones()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Zones() returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestZones_deduction(t *testing.T) {
	t.Parallel()

	serverHostname := "server-hostname"
	c := &fakeUFSClient{
		services: []*ufsModels.CachingService{
			{
				Name:          "cachingservice/1.1.1.1",
				Port:          8001,
				SecondaryNode: serverHostname,
				State:         ufsModels.State_STATE_SERVING,
			},
		},
		machines: map[string]*ufsModels.Machine{
			serverHostname: {
				Name: serverHostname,
				Location: &ufsModels.Location{
					Zone: ufsModels.Zone_ZONE_CHROMEOS2,
				},
			},
		},
	}
	env, err := NewUFSEnv(c)
	if err != nil {
		t.Fatalf("NewUFSEnv(fakeClient) failed: %s", err)
	}
	want := map[ufsModels.Zone][]CachingService{
		ufsModels.Zone_ZONE_CHROMEOS2: {"http://1.1.1.1:8001"},
	}
	got := env.CacheZones()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Zones() returned unexpected diff (-want +got):\n%s", diff)
	}
}
