// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
)

// TestGetFreeIPSimple tests getting a free IP address form a mostly empty Vlan.
func TestGetFreeIPSimple(t *testing.T) {
	t.Parallel()

	ctx := testingContext()

	const originalIP = "192.64.0.0"
	// We reserve 192.64.0.0 unconditionally,
	// followed by the first 10 addresses 192.64.0.{1..10}
	// Thus the first address that's free for our use is 192.64.0.11
	const firstFreeIP = "192.64.0.11"

	_, err := CreateVlan(ctx, &ufspb.Vlan{
		Name:        "fake-vlan",
		VlanAddress: fmt.Sprintf("%s/24", originalIP),
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := getFreeIP(ctx, "fake-vlan", 1)
	if err != nil {
		t.Error(err)
	}
	if len(res) != 1 {
		t.Fatalf("res has bad length %d: %#v", len(res), res)
	}

	if diff := cmp.Diff(res[0].GetIpv4Str(), firstFreeIP); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestGetFreeIPWithGaps is a gross, somewhat invasive test that intentionally mixes a few levels of abstraction.
//
// We create a vlan. This creates a bunch of not-occupied not-reserved IP addresses.
// We then remove ALL of the not-reserved IP addresses.
// Then we grab two IP addresses, just to make sure that we're getting the right IP addresses.
func TestGetFreeIPWithGaps(t *testing.T) {
	t.Parallel()

	ctx := testingContext()

	const originalIP = "192.64.0.0"
	// We reserve 192.64.0.0 unconditionally,
	// followed by the first 10 addresses 192.64.0.{1..10}
	// Thus the first address that's free for our use is 192.64.0.11
	const firstFreeIP = "192.64.0.11"
	const secondFreeIP = "192.64.0.12"

	const expectedReservedNumber = 12

	vlan, err := CreateVlan(ctx, &ufspb.Vlan{
		Name:        "fake-vlan",
		VlanAddress: fmt.Sprintf("%s/28", originalIP),
	})
	if n := vlan.GetReservedIpNum(); n != expectedReservedNumber {
		t.Fatalf("bad number of reserved ips (got: %d, want: %d)", n, expectedReservedNumber)
	}
	if err != nil {
		t.Fatal(err)
	}
	q := datastore.NewQuery(configuration.IPKind).Eq("reserved", false).KeysOnly(true)
	if err := datastore.Run(ctx, q, func(key *datastore.Key) error {
		return datastore.Delete(ctx, key)
	}); err != nil {
		t.Fatal(err)
	}

	ips, err := getFreeIP(ctx, "fake-vlan", 2)
	if err != nil {
		t.Error(err)
	}
	got := ips[0].Ipv4Str
	if diff := cmp.Diff(got, firstFreeIP); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
	got = ips[1].Ipv4Str
	if diff := cmp.Diff(got, secondFreeIP); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestGetFreeIPFromPartiallyPreallocatedVlan tests getting free IP addresses from a partially preallocated vlan.
//
// This means that some of the IP addresses in the vlan are occupied (which gets tested first) and some are not.
func TestGetFreeIPFromPartiallyPreallocatedVlan(t *testing.T) {
	t.Parallel()

	ctx := testingContext()

	const originalIP = "192.64.0.0"
	const firstFreeIP = "192.64.0.11"
	const secondFreeIP = "192.64.0.12"

	const expectedReservedNumber = 12

	vlan, err := CreateVlan(ctx, &ufspb.Vlan{
		Name:        "fake-vlan",
		VlanAddress: fmt.Sprintf("%s/28", originalIP),
	})
	if n := vlan.GetReservedIpNum(); n != expectedReservedNumber {
		t.Fatalf("bad number of reserved ips (got: %d, want: %d)", n, expectedReservedNumber)
	}
	if err != nil {
		t.Fatal(err)
	}
	q := datastore.NewQuery(configuration.IPKind).Eq("reserved", false)
	if err := datastore.Run(ctx, q, func(e *configuration.IPEntity) error {
		if e.IPv4Str != firstFreeIP {
			return datastore.Delete(ctx, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ips, err := getFreeIP(ctx, "fake-vlan", 2)
	if err != nil {
		t.Error(err)
	}
	got := ips[0].Ipv4Str
	if diff := cmp.Diff(got, firstFreeIP); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
	got = ips[1].Ipv4Str
	if diff := cmp.Diff(got, secondFreeIP); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}
