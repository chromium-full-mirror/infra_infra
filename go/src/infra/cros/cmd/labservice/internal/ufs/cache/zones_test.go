// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsapi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

func TestGetZones(t *testing.T) {
	t.Parallel()

	c := &fakeClient{
		CachingServices: &ufsapi.ListCachingServicesResponse{
			CachingServices: []*ufspb.CachingService{
				{
					Name:  "cachingservice/200.200.200.24",
					Port:  124,
					Zones: []ufspb.Zone{ufspb.Zone_ZONE_CHROMEOS2, ufspb.Zone_ZONE_CHROMEOS4},
					State: ufspb.State_STATE_SERVING,
				},
				{
					Name:  "cachingservice/200.200.200.4",
					Port:  104,
					Zones: []ufspb.Zone{ufspb.Zone_ZONE_CHROMEOS4},
					State: ufspb.State_STATE_SERVING,
				},
				{
					Name:  "cachingservice/200.200.100.8",
					Port:  108,
					Zones: []ufspb.Zone{ufspb.Zone_ZONE_SFO36_OS},
					State: ufspb.State_STATE_SERVING,
				},
			},
		},
	}
	zones := newZonesFinder()
	got := zones.getCacheZones(c)
	want := map[ufspb.Zone][]address{
		ufspb.Zone_ZONE_CHROMEOS2: {
			{Ip: "200.200.200.24", Port: 124},
		},
		ufspb.Zone_ZONE_CHROMEOS4: {
			{Ip: "200.200.200.24", Port: 124},
			{Ip: "200.200.200.4", Port: 104},
		},
		ufspb.Zone_ZONE_SFO36_OS: {
			{Ip: "200.200.100.8", Port: 108},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("getSubnets() mismatch (-want +got):\n%s", diff)
	}
}

func TestGetZones_deduction(t *testing.T) {
	t.Parallel()

	c := &fakeClient{
		CachingServices: &ufsapi.ListCachingServicesResponse{
			CachingServices: []*ufspb.CachingService{
				{
					Name:          "cachingservice/200.200.100.8",
					Port:          108,
					SecondaryNode: "server-name",
					State:         ufspb.State_STATE_SERVING,
				},
			},
		},
		Machines: map[string]*ufspb.Machine{
			"machines/server-name": {
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS2,
				},
			},
			"machines/another-server": {
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_OS,
				},
			},
		},
	}
	zones := newZonesFinder()
	got := zones.getCacheZones(c)
	want := map[ufspb.Zone][]address{
		ufspb.Zone_ZONE_CHROMEOS2: {
			{Ip: "200.200.100.8", Port: 108},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("getSubnets() mismatch (-want +got):\n%s", diff)
	}
}
