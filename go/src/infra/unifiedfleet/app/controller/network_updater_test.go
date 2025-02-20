// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/luci/common/errors"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
)

// TestLogChanges is an extremely shallow test that checks whether new change events are added to the Changes field.
func TestLogChanges(t *testing.T) {
	t.Parallel()

	nu := &networkUpdater{}
	nu.logChanges([]*ufspb.ChangeEvent{
		{
			Name: "fake-change-event",
		},
	}, nil)

	want := []*ufspb.ChangeEvent{
		{
			Name: "fake-change-event",
		},
	}
	if diff := cmp.Diff(want, nu.Changes, protocmp.Transform()); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}

	if len(nu.Msgs) != 0 {
		t.Errorf("unexpected msgs %#v", nu.Msgs)
	}
}

// TestDeleteDHCPHelper tests creating a DHCPConfig for a fake device it and then deleting it through the network updater helper method
func TestDeleteDHCPHelper(t *testing.T) {
	t.Parallel()

	ctx := networkUpdaterTestingContext()

	if err := (&networkUpdater{
		Hostname: "fake-device",
	}).deleteDHCPHelper(ctx); err != nil {
		t.Error(err)
	}

	dhcpConfigs, err := getAllDHCPs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(dhcpConfigs); n != 0 {
		t.Errorf("bad dhcpConfigs: %#v", dhcpConfigs)
	}
}

// TestDeleteHostHelperSmokeTest tests that deleteHostHelper deleted the DHCP config, at the very least.
func TestDeleteHostHelperSmokeTest(t *testing.T) {
	t.Parallel()

	ctx := networkUpdaterTestingContext()

	if err := (&networkUpdater{Hostname: "fake-device"}).deleteHostHelper(
		ctx,
		&ufspb.DHCPConfig{
			Hostname: "fake-device",
		},
	); err != nil {
		t.Error(err)
	}

	dhcpConfigs, err := getAllDHCPs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(dhcpConfigs); n != 0 {
		t.Errorf("bad dhcpConfigs: %#v", dhcpConfigs)
	}
}

// TestGetFreeIPHelperSimple tests the happy path of getFreeIPHelper.
// We have a Vlan that has some space left and we want an IP from it.
func TestGetFreeIPHelperSimple(t *testing.T) {
	t.Parallel()

	ctx := networkUpdaterTestingContext()

	ip, err := getFreeIPHelper(ctx, "fake-vlan")
	if err != nil {
		t.Error(err)
	}
	// 0.0 is taken and the first ten addresses are reserved.
	want := "198.64.0.11"
	if diff := cmp.Diff(ip.GetIpv4Str(), want); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestGetSpecifiedIPSimple gets a free IP out of a Vlan that we just created.
func TestGetSpecifiedIPSimple(t *testing.T) {
	t.Parallel()

	ctx := networkUpdaterTestingContext()

	ip, err := getSpecifiedIP(ctx, "198.64.0.11")
	if err != nil {
		t.Error(err)
	}
	want := "198.64.0.11"
	if diff := cmp.Diff(ip.GetIpv4Str(), want); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestUpdateDHCPWithMac tests updating the mac address entry in a DHCP entry.
// This test tests the happy path only.
func TestUpdateDHCPWithMac(t *testing.T) {
	t.Parallel()

	ctx := networkUpdaterTestingContext()

	dhcp, err := (&networkUpdater{
		Hostname: "fake-device",
	}).updateDHCPWithMac(ctx, "aa:aa:aa:aa:aa:aa")
	if err != nil {
		t.Error(err)
	}

	if diff := cmp.Diff(dhcp.GetMacAddress(), "aa:aa:aa:aa:aa:aa"); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestRenameDHCP tests finding a DHCP entry by its hostname, and replacing it with a new hostname and mac address.
// In order to make sure we really changed the thing in datastore, we check that
// the DHCP entry that we grabbed from datastore is right, not the one returned by renameDHCP.
func TestRenameDHCP(t *testing.T) {
	t.Parallel()

	ctx := networkUpdaterTestingContext()

	_, err := (&networkUpdater{
		Hostname: "fake-device",
	}).renameDHCP(ctx, "fake-device", "new-fake-device", "aa:aa:aa:aa:aa:aa")
	if err != nil {
		t.Error(err)
	}

	dhcps, err := getAllDHCPs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(dhcps) != 1 {
		t.Errorf("there should be one dhcp entry not %d", len(dhcps))
	}

	dhcp := dhcps[0]
	if diff := cmp.Diff(dhcp.GetHostname(), "new-fake-device"); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
	if diff := cmp.Diff(dhcp.GetMacAddress(), "aa:aa:aa:aa:aa:aa"); diff != "" {
		t.Errorf("unexpected mac address (-want +got): %s", diff)
	}
}

// networkUpdaterTestingContext produces a fresh context for use in tests in this file.
//
// It creates:
// 1)  a fake device with hostname "fake-device" and no mac address.
// 2)  a Vlan called "fake-vlan" associated with "198.64.0.0/24"
//
// Adding new fake resources to our little fake world should be fine as long as all the tests in this file pass.
//
// Please do not call this function outside of network_updater_test.go.
func networkUpdaterTestingContext() context.Context {
	ctx := testingContext()

	// Create all the resources.
	_, err := configuration.BatchUpdateDHCPs(ctx, []*ufspb.DHCPConfig{{
		Hostname: "fake-device",
	}})
	if err != nil {
		panic(err)
	}
	_, err = CreateVlan(ctx, &ufspb.Vlan{
		Name:        "fake-vlan",
		VlanAddress: "198.64.0.0/24",
	})
	if err != nil {
		panic(err)
	}

	// Validate the resources in the context.
	dhcps, err := getAllDHCPs(ctx)
	if err != nil {
		panic(err)
	}
	if n := len(dhcps); n != 1 {
		panic(fmt.Sprintf("there are %d dhcps; 1 was expected", n))
	}
	return ctx
}

// getAllDHCPs is a helper method for getting DHCPConfigs out of the in-memory datastore implementation.
func getAllDHCPs(ctx context.Context) ([]*ufspb.DHCPConfig, error) {
	res, err := configuration.GetAllDHCPs(ctx)
	if err != nil {
		return nil, err
	}
	var out []*ufspb.DHCPConfig
	for _, item := range *res {
		if item == nil {
			return nil, errors.New("item cannot be nil")
		}
		if item.Err != nil {
			return nil, item.Err
		}
		dhcpConfig, ok := item.Data.(*ufspb.DHCPConfig)
		if !ok {
			return nil, errors.New("item has bad type")
		}
		out = append(out, dhcpConfig)
	}
	return out, nil
}
