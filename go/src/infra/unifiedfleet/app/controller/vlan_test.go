// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
	"google.golang.org/genproto/protobuf/field_mask"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/history"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func mockVlan(id, cidr string) *ufspb.Vlan {
	return &ufspb.Vlan{
		Name:          id,
		VlanAddress:   cidr,
		ResourceState: ufspb.State_STATE_UNSPECIFIED,
	}
}

func TestCreateVlan(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	configuration.BatchUpdateVlans(ctx, []*ufspb.Vlan{mockVlan("create-vlan-0", "2.2.2.2/22")})
	ftt.Run("CreateVlan", t, func(t *ftt.Test) {
		t.Run("Create vlan - with existing vlan", func(t *ftt.Test) {
			resp, err := CreateVlan(ctx, mockVlan("create-vlan-0", ""))
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("already exists in the system"))
		})

		t.Run("Create vlan - with duplicated cidr block", func(t *ftt.Test) {
			resp, err := CreateVlan(ctx, mockVlan("create-vlan-1", "2.2.2.2/22"))
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("cidr block 2.2.2.2/22 is already occupied by "))
		})

		t.Run("Create vlan - invalid cidr block", func(t *ftt.Test) {
			resp, err := CreateVlan(ctx, mockVlan("create-vlan-1", "a.b.c.d/ab"))
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("invalid CIDR block"))
		})

		t.Run("Create vlan - invalid free start ip", func(t *ftt.Test) {
			vlan1 := mockVlan("create-vlan-1", "192.168.100.0/27")
			vlan1.FreeStartIpv4Str = "1235"
			_, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("free ip 1235 is an invalid IP"))
		})

		t.Run("Create vlan - invalid free end ip", func(t *ftt.Test) {
			vlan1 := mockVlan("create-vlan-1", "192.168.100.0/27")
			vlan1.FreeEndIpv4Str = "1235"
			_, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("free ip 1235 is an invalid IP"))
		})

		t.Run("Create vlan - invalid free ip range", func(t *ftt.Test) {
			vlan1 := mockVlan("create-vlan-1", "192.168.100.0/27")
			vlan1.FreeEndIpv4Str = "192.168.100.6"
			vlan1.FreeStartIpv4Str = "192.168.100.26"
			_, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("invalid IP Range"))
		})

		t.Run("Create vlan - no range validation if end IP is missing", func(t *ftt.Test) {
			vlan1 := mockVlan("create-vlan-5", "194.168.100.0/27")
			vlan1.FreeStartIpv4Str = "194.168.100.26"
			_, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Create vlan - happy path", func(t *ftt.Test) {
			vlan1 := mockVlan("create-vlan-1", "192.168.100.0/27")
			vlan1.Zones = []ufspb.Zone{ufspb.Zone_ZONE_MTV97, ufspb.Zone_ZONE_MTV96}
			vlan1.FreeStartIpv4Str = "192.168.100.6"
			vlan1.FreeEndIpv4Str = "192.168.100.26"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksCreate, util.BrowserLabAdminRealm)
			resp, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetName(), should.Equal("create-vlan-1"))
			assert.Loosely(t, resp.GetVlanAddress(), should.Equal("192.168.100.0/27"))
			assert.Loosely(t, resp.GetCapacityIp(), should.Equal(32))
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_SERVING))
			assert.Loosely(t, resp.GetFreeStartIpv4Str(), should.Equal("192.168.100.6"))
			assert.Loosely(t, resp.GetFreeEndIpv4Str(), should.Equal("192.168.100.26"))
			assert.Loosely(t, resp.GetZones(), should.HaveLength(2))
			assert.Loosely(t, resp.GetTags(), should.BeNil)

			startIPInt, err := util.IPv4StrToInt("192.168.100.6")
			assert.Loosely(t, err, should.BeNil)
			endIPInt, err := util.IPv4StrToInt("192.168.100.26")
			assert.Loosely(t, err, should.BeNil)
			ips, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"vlan": "create-vlan-1"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(32))
			for _, ip := range ips {
				if ip.GetIpv4() < startIPInt || ip.GetIpv4() > endIPInt {
					assert.Loosely(t, ip.GetReserve(), should.BeTrue)
				} else {
					assert.Loosely(t, ip.GetReserve(), should.BeFalse)
				}
			}

			s, err := state.GetStateRecord(ctx, "vlans/create-vlan-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_SERVING))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "vlans/create-vlan-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("vlan"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "vlans/create-vlan-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Create vlan - permission deined", func(t *ftt.Test) {
			vlan1 := mockVlan("create-vlan-3", "192.168.100.0/28")
			vlan1.Zones = []ufspb.Zone{ufspb.Zone_ZONE_MTV97, ufspb.Zone_ZONE_MTV96}

			//same realm and no create permission
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksGet, util.BrowserLabAdminRealm)
			_, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))

			//different realm and no permission
			ctx = initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksCreate, util.AtlLabAdminRealm)
			_, err = CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

// TestCreateVlanTableTest tests creating vlans.
func TestCreateVlanTableTest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  *ufspb.Vlan
		output *ufspb.Vlan
		ok     bool
	}{
		{
			name: "medium IPv4 vlan",
			input: &ufspb.Vlan{
				Name:        "fake-vlan",
				VlanAddress: "127.0.0.0/26",
			},
			output: &ufspb.Vlan{
				Name:             "fake-vlan",
				VlanAddress:      "127.0.0.0/26",
				FreeStartIp:      "127.0.0.11",
				FreeEndIp:        "127.0.0.62",
				FreeStartIpv4Str: "127.0.0.11",
				FreeEndIpv4Str:   "127.0.0.62",
				ResourceState:    ufspb.State_STATE_SERVING,
				ReservedIpNum:    12,
				CapacityIp:       64,
			},
			ok: true,
		},
		{
			name: "medium IPv4 vlan with explicit bounds",
			input: &ufspb.Vlan{
				Name:        "fake-vlan",
				VlanAddress: "127.0.0.0/26",
				FreeStartIp: "127.0.0.11",
				FreeEndIp:   "127.0.0.62",
			},
			output: &ufspb.Vlan{
				Name:             "fake-vlan",
				VlanAddress:      "127.0.0.0/26",
				FreeStartIp:      "127.0.0.11",
				FreeEndIp:        "127.0.0.62",
				FreeStartIpv4Str: "127.0.0.11",
				FreeEndIpv4Str:   "127.0.0.62",
				ResourceState:    ufspb.State_STATE_SERVING,
				ReservedIpNum:    12,
				CapacityIp:       64,
			},
			ok: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := testingContext()

			got, err := CreateVlan(ctx, tt.input)
			if got != nil {
				got.UpdateTime = nil
			}

			switch {
			case err == nil && !tt.ok:
				t.Error("error is unexpectedly nil")
			case err != nil && tt.ok:
				t.Errorf("unexpected error: %s", err)
			}

			assert.That(t, got, should.Match(tt.output))
		})
	}
}

func TestUpdateVlan(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	configuration.BatchUpdateVlans(ctx, []*ufspb.Vlan{mockVlan("update-vlan-0", "4.4.4.4/27")})
	ftt.Run("UpdateVlan", t, func(t *ftt.Test) {
		t.Run("Update vlan - with existing vlan", func(t *ftt.Test) {
			resp, err := UpdateVlan(ctx, mockVlan("update-vlan-non-exist", ""), nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Update vlan - partial update invalid fields", func(t *ftt.Test) {
			resp, err := UpdateVlan(ctx, mockVlan("update-vlan-0", "2.2.2.2/22"), &field_mask.FieldMask{Paths: []string{"cidr_block"}})
			assert.Loosely(t, resp, should.BeNil)
			assert.ErrIsLike(t, err, "cidr_block/vlan_address cannot be updated")
		})

		t.Run("Update vlan - fully update cidr_block won't work", func(t *ftt.Test) {
			resp, err := UpdateVlan(ctx, mockVlan("update-vlan-0", "2.2.2.2/22"), nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetVlanAddress(), should.Equal("4.4.4.4/27"))
		})

		t.Run("Update vlan - fully update description & state", func(t *ftt.Test) {
			vlan2 := mockVlan("update-vlan-0", "4.4.4.4/30")
			vlan2.ResourceState = ufspb.State_STATE_SERVING
			vlan2.Description = "test fully update"
			resp, err := UpdateVlan(ctx, vlan2, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetDescription(), should.Equal("test fully update"))
			assert.Loosely(t, resp.GetVlanAddress(), should.Equal("4.4.4.4/27"))
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_SERVING))

			vlan, err := configuration.GetVlan(ctx, "update-vlan-0")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, vlan.GetDescription(), should.Equal("test fully update"))
			assert.Loosely(t, vlan.GetVlanAddress(), should.Equal("4.4.4.4/27"))
			assert.Loosely(t, vlan.GetResourceState(), should.Equal(ufspb.State_STATE_SERVING))
			s, err := state.GetStateRecord(ctx, "vlans/update-vlan-0")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_SERVING))
		})

		t.Run("Update vlan - partial update description", func(t *ftt.Test) {
			configuration.BatchUpdateVlans(ctx, []*ufspb.Vlan{mockVlan("update-vlan-1", "5.5.5.5/27")})
			vlan2 := mockVlan("update-vlan-1", "2.2.2.2/27")
			vlan2.Description = "test partial update"
			resp, err := UpdateVlan(ctx, vlan2, &field_mask.FieldMask{Paths: []string{"description"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetDescription(), should.Equal("test partial update"))
			assert.Loosely(t, resp.GetVlanAddress(), should.Equal("5.5.5.5/27"))
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_UNSPECIFIED))

			vlan, err := configuration.GetVlan(ctx, "update-vlan-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, vlan.GetDescription(), should.Equal("test partial update"))
			assert.Loosely(t, vlan.GetVlanAddress(), should.Equal("5.5.5.5/27"))
			assert.Loosely(t, vlan.GetResourceState(), should.Equal(ufspb.State_STATE_UNSPECIFIED))
		})

		t.Run("Update vlan - partial update state", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-2", "5.5.5.5/27")
			vlan1.Description = "before update"
			configuration.BatchUpdateVlans(ctx, []*ufspb.Vlan{vlan1})
			vlan2 := mockVlan("update-vlan-2", "2.2.2.2/27")
			vlan2.Description = "after update"
			vlan2.ResourceState = ufspb.State_STATE_SERVING
			resp, err := UpdateVlan(ctx, vlan2, &field_mask.FieldMask{Paths: []string{"resourceState"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetDescription(), should.Equal("before update"))
			assert.Loosely(t, resp.GetVlanAddress(), should.Equal("5.5.5.5/27"))
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_SERVING))

			vlan, err := configuration.GetVlan(ctx, "update-vlan-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, vlan.GetDescription(), should.Equal("before update"))
			assert.Loosely(t, vlan.GetVlanAddress(), should.Equal("5.5.5.5/27"))
			assert.Loosely(t, vlan.GetResourceState(), should.Equal(ufspb.State_STATE_SERVING))

			s, err := state.GetStateRecord(ctx, "vlans/update-vlan-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_SERVING))
		})

		t.Run("Update vlan - partial update reserved_ips - invalid ips", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-3", "6.6.6.0/27")
			vlan1.Description = "before update"
			configuration.BatchUpdateVlans(ctx, []*ufspb.Vlan{vlan1})
			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.ReservedIps = []string{"6.6.6.48"}
			_, err := UpdateVlan(ctx, vlan2, &field_mask.FieldMask{Paths: []string{"reserved_ips"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("doesn't belong to vlan"))
		})

		t.Run("Update vlan - partial update reserved_ips - happy path", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-4", "6.6.6.0/27")
			vlan1.Description = "before update"
			configuration.BatchUpdateVlans(ctx, []*ufspb.Vlan{vlan1})
			ips, _, _, _, _, err := util.ParseVlan("update-vlan-4", "6.6.6.0/27", vlan1.GetFreeStartIpv4Str(), vlan1.GetFreeEndIpv4Str())
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateIPs(ctx, ips)
			assert.Loosely(t, err, should.BeNil)

			// Before
			resIPs, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "6.6.6.14"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resIPs, should.HaveLength(1))
			assert.Loosely(t, resIPs[0].GetReserve(), should.BeFalse)

			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.ReservedIps = []string{"6.6.6.14"}
			res, err := UpdateVlan(ctx, vlan2, &field_mask.FieldMask{Paths: []string{"reserved_ips"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetReservedIps(), should.Match([]string{"6.6.6.14"}))
			resIPs, err = configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "6.6.6.14"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resIPs, should.HaveLength(1))
			assert.Loosely(t, resIPs[0].GetReserve(), should.BeTrue)
		})

		t.Run("Update vlan - partial update reserved_ips - happy path with existing old reserved_ips", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-5", "6.6.5.0/27")
			vlan1.ReservedIps = []string{"6.6.5.13"}
			configuration.BatchUpdateVlans(ctx, []*ufspb.Vlan{vlan1})
			ips, _, _, _, _, err := util.ParseVlan("update-vlan-5", "6.6.5.0/27", vlan1.GetFreeStartIpv4Str(), vlan1.GetFreeEndIpv4Str())
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateIPs(ctx, ips)
			assert.Loosely(t, err, should.BeNil)

			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.ReservedIps = nil
			res, err := UpdateVlan(ctx, vlan2, &field_mask.FieldMask{Paths: []string{"reserved_ips"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetReservedIps(), should.HaveLength(0))
			resIPs, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "6.6.5.13"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resIPs, should.HaveLength(1))
			assert.Loosely(t, resIPs[0].GetReserve(), should.BeFalse)
		})

		t.Run("Update vlan - partial update zones - happy path", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-6", "6.6.6.0/27")
			vlan1.Description = "before update"
			vlan1.Zones = []ufspb.Zone{ufspb.Zone_ZONE_ATL97}
			vlan1.Realm = util.BrowserLabAdminRealm
			configuration.BatchUpdateVlans(ctx, []*ufspb.Vlan{vlan1})

			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.Zones = []ufspb.Zone{ufspb.Zone_ZONE_ATL97, ufspb.Zone_ZONE_MTV96}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksUpdate, util.BrowserLabAdminRealm)
			res, err := UpdateVlan(ctx, vlan2, &field_mask.FieldMask{Paths: []string{"zones"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetZones(), should.HaveLength(2))
			assert.Loosely(t, res.GetZones(), should.Contain(ufspb.Zone_ZONE_ATL97))
			assert.Loosely(t, res.GetZones(), should.Contain(ufspb.Zone_ZONE_MTV96))

			vlan3 := mockVlan("update-vlan-6", "")
			res, err = UpdateVlan(ctx, vlan3, &field_mask.FieldMask{Paths: []string{"zones"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetZones(), should.HaveLength(0))
		})

		t.Run("Update vlan - partial update free ips - happy path", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-7", "7.7.7.0/27")
			vlan1.Description = "before update"
			resp, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetFreeStartIpv4Str(), should.Equal("7.7.7.11"))
			assert.Loosely(t, resp.GetFreeEndIpv4Str(), should.Equal("7.7.7.30"))
			startIPInt, err := util.IPv4StrToInt("7.7.7.11")
			assert.Loosely(t, err, should.BeNil)
			endIPInt, err := util.IPv4StrToInt("7.7.7.30")
			assert.Loosely(t, err, should.BeNil)
			ips, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"vlan": "update-vlan-7"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(32))
			for _, ip := range ips {
				if ip.GetIpv4() < startIPInt || ip.GetIpv4() > endIPInt {
					assert.Loosely(t, ip.GetReserve(), should.BeTrue)
				} else {
					assert.Loosely(t, ip.GetReserve(), should.BeFalse)
				}
			}

			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.FreeStartIpv4Str = "7.7.7.10"
			vlan2.FreeEndIpv4Str = "7.7.7.29"
			resp, err = UpdateVlan(ctx, vlan2, &field_mask.FieldMask{Paths: []string{"free_start_ip", "free_end_ip"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetFreeStartIpv4Str(), should.Equal("7.7.7.10"))
			assert.Loosely(t, resp.GetFreeEndIpv4Str(), should.Equal("7.7.7.29"))

			startIPInt, err = util.IPv4StrToInt("7.7.7.10")
			assert.Loosely(t, err, should.BeNil)
			endIPInt, err = util.IPv4StrToInt("7.7.7.29")
			assert.Loosely(t, err, should.BeNil)
			ips, err = configuration.QueryIPByPropertyName(ctx, map[string]string{"vlan": "update-vlan-7"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(32))
			for _, ip := range ips {
				if ip.GetIpv4() < startIPInt || ip.GetIpv4() > endIPInt {
					assert.Loosely(t, ip.GetReserve(), should.BeTrue)
				} else {
					assert.Loosely(t, ip.GetReserve(), should.BeFalse)
				}
			}

			// Reset the start/end ip range with reserved ips
			vlan3 := mockVlan("update-vlan-7", "")
			vlan3.ReservedIps = []string{"7.7.7.16"}
			resp, err = UpdateVlan(ctx, vlan3, &field_mask.FieldMask{Paths: []string{"free_start_ip", "free_end_ip", "reserved_ips"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetFreeStartIpv4Str(), should.Equal("7.7.7.11"))
			assert.Loosely(t, resp.GetFreeEndIpv4Str(), should.Equal("7.7.7.30"))
			startIPInt, err = util.IPv4StrToInt("7.7.7.11")
			assert.Loosely(t, err, should.BeNil)
			endIPInt, err = util.IPv4StrToInt("7.7.7.30")
			assert.Loosely(t, err, should.BeNil)
			ips, err = configuration.QueryIPByPropertyName(ctx, map[string]string{"vlan": "update-vlan-7"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(32))
			for _, ip := range ips {
				if ip.GetIpv4() < startIPInt || ip.GetIpv4() > endIPInt {
					assert.Loosely(t, ip.GetReserve(), should.BeTrue)
				} else if ip.GetIpv4Str() == "7.7.7.16" {
					assert.Loosely(t, ip.GetReserve(), should.BeTrue)
				} else {
					assert.Loosely(t, ip.GetReserve(), should.BeFalse)
				}
			}

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "ips/update-vlan-7/117901066")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("ip.reserved"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("true"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("false"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("ip.reserved"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("false"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("true"))
		})

		t.Run("Update vlan - partial update free ips - invalid ip ranges", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-7.1", "7.7.8.0/27")
			configuration.BatchUpdateVlans(ctx, []*ufspb.Vlan{vlan1})
			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.FreeEndIpv4Str = "1235"
			_, err := UpdateVlan(ctx, vlan2, &field_mask.FieldMask{Paths: []string{"free_start_ip", "free_end_ip"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("free ip 1235 is an invalid IP"))
		})

		t.Run("Update vlan: permission denied", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-8", "192.168.100.0/28")
			vlan1.Zones = []ufspb.Zone{ufspb.Zone_ZONE_MTV97, ufspb.Zone_ZONE_MTV96}
			vlan1.Realm = util.BrowserLabAdminRealm
			_, err := configuration.CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)

			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.Description = "updating"
			//same realm and no update permission
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksGet, util.BrowserLabAdminRealm)
			_, err = UpdateVlan(ctx, vlan2, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))

			//different realm
			ctx = initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksUpdate, util.AtlLabAdminRealm)
			_, err = UpdateVlan(ctx, vlan2, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update vlan(different Zone without permission): permission denied", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-9", "192.168.100.0/28")
			vlan1.Zones = []ufspb.Zone{ufspb.Zone_ZONE_MTV97, ufspb.Zone_ZONE_MTV96}
			vlan1.Realm = util.BrowserLabAdminRealm
			_, err := configuration.CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)

			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.Zones = []ufspb.Zone{ufspb.Zone_ZONE_CHROMEOS2}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateVlan(ctx, vlan2, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update vlan(different Zone with permission): permission denied", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-10", "192.168.100.0/28")
			vlan1.Zones = []ufspb.Zone{ufspb.Zone_ZONE_MTV97, ufspb.Zone_ZONE_MTV96}
			vlan1.Realm = util.BrowserLabAdminRealm
			_, err := configuration.CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)

			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.Zones = []ufspb.Zone{ufspb.Zone_ZONE_CHROMEOS2}
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.NetworksUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.NetworksUpdate),
				),
			})
			resp, err := UpdateVlan(ctx, vlan2, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetZones(), should.Match(vlan2.Zones))
		})

		t.Run("Partial Update vlan(No/empty Zone): permission denied", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-11", "192.168.100.0/28")
			vlan1.Zones = []ufspb.Zone{ufspb.Zone_ZONE_MTV97, ufspb.Zone_ZONE_MTV96}
			vlan1.Realm = util.BrowserLabAdminRealm
			_, err := configuration.CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)

			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.Zones = []ufspb.Zone{}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateVlan(ctx, vlan2, &field_mask.FieldMask{Paths: []string{"zones"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetZones(), should.Match([]ufspb.Zone(nil)))
		})

		t.Run("Update vlan - partial update tags", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-12", "7.7.7.0/27")
			vlan1.Tags = []string{"tag-1"}
			vlan1.Description = "before update"
			configuration.BatchUpdateVlans(ctx, []*ufspb.Vlan{vlan1})

			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.Tags = []string{"tag-2"}
			resp, err := UpdateVlan(ctx, vlan2, &field_mask.FieldMask{Paths: []string{"tags"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetTags(), should.Match([]string{"tag-1", "tag-2"}))
		})

		t.Run("Update vlan - invalid IP Range", func(t *ftt.Test) {
			vlan1 := mockVlan("update-vlan-14", "9.9.9.0/27")
			vlan1.Description = "before update"
			resp, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetFreeStartIpv4Str(), should.Equal("9.9.9.11"))
			assert.Loosely(t, resp.GetFreeEndIpv4Str(), should.Equal("9.9.9.30"))
			startIPInt, err := util.IPv4StrToInt("9.9.9.11")
			assert.Loosely(t, err, should.BeNil)
			endIPInt, err := util.IPv4StrToInt("9.9.9.30")
			assert.Loosely(t, err, should.BeNil)
			ips, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"vlan": "update-vlan-14"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(32))
			for _, ip := range ips {
				if ip.GetIpv4() < startIPInt || ip.GetIpv4() > endIPInt {
					assert.Loosely(t, ip.GetReserve(), should.BeTrue)
				} else {
					assert.Loosely(t, ip.GetReserve(), should.BeFalse)
				}
			}

			vlan2 := proto.Clone(vlan1).(*ufspb.Vlan)
			vlan2.FreeStartIpv4Str = "7.7.7.30"
			vlan2.FreeEndIpv4Str = "7.7.7.29"
			resp, err = UpdateVlan(ctx, vlan2, &field_mask.FieldMask{Paths: []string{"free_start_ip", "free_end_ip"}})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("invalid IP Range"))
		})
	})
}

func TestListVlans(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	vlans := make([]*ufspb.Vlan, 0, 4)
	for i := range 4 {
		vlan1 := mockVlan("", "")
		vlan1.Name = fmt.Sprintf("vlan-%d", i)
		vlan1.Zones = []ufspb.Zone{ufspb.Zone_ZONE_MTV96}
		if i == 0 {
			vlan1.Zones = append(vlan1.Zones, ufspb.Zone_ZONE_IAD97)
		}
		resp, _ := configuration.CreateVlan(ctx, vlan1)
		vlans = append(vlans, resp)
	}
	ftt.Run("ListVlans", t, func(t *ftt.Test) {
		t.Run("List Vlans - filter invalid", func(t *ftt.Test) {
			_, _, err := ListVlans(ctx, 5, "", "machine=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Failed to read filter for listing vlans"))
		})

		t.Run("ListVlans - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListVlans(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(vlans))
		})

		t.Run("ListVlans - list by zones - happy path", func(t *ftt.Test) {
			resp, _, err := ListVlans(ctx, 5, "", "zone = iad97", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0].GetName(), should.Equal("vlan-0"))

			resp, _, err = ListVlans(ctx, 5, "", "zone = mtv96", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
		})
	})
}

func TestListIPs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("ListIPs", t, func(t *ftt.Test) {
		t.Run("ListIPs - filter invalid", func(t *ftt.Test) {
			_, _, err := ListIPs(ctx, 5, "", "machine=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name"))
		})

		t.Run("ListIPs - list by vlan - happy path", func(t *ftt.Test) {
			vlan1 := mockVlan("listip-vlan-2", "192.168.100.0/27")
			expectedIPLengh := 32 // the subnet mask is 27, the number of available IPs is 2 ^ (32-27)
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksCreate, util.BrowserLabAdminRealm)
			_, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)

			resp, _, err := ListIPs(ctx, 100, "", "vlan = listip-vlan-2", false)
			fmt.Println("listips222")
			fmt.Println(resp)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(expectedIPLengh))
		})
	})
}

func TestDeleteVlan(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	vlan1 := mockVlan("vlan-1", "")
	vlan2 := mockVlan("vlan-2", "")
	ftt.Run("DeleteVlan", t, func(t *ftt.Test) {
		t.Run("Delete vlan by existing ID with machinelse reference", func(t *ftt.Test) {
			resp, cerr := configuration.CreateVlan(ctx, vlan1)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan1))
			machineLSE1 := &ufspb.MachineLSE{
				Name: "machineLSE-1",
				Vlan: "vlan-1",
			}
			mresp, merr := inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, merr, should.BeNil)
			assert.Loosely(t, mresp, should.Match(machineLSE1))

			err := DeleteVlan(ctx, "vlan-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("vlan vlan-1 is occupied by"))

			resp, cerr = configuration.GetVlan(ctx, "vlan-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan1))
		})
		t.Run("Delete vlan successfully with large numbers of IPs", func(t *ftt.Test) {
			vlan2.VlanAddress = "192.168.16.0/24"
			vlan1.Zones = []ufspb.Zone{ufspb.Zone_ZONE_MTV97, ufspb.Zone_ZONE_MTV96}
			resp, err := CreateVlan(ctx, vlan2)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksGet, util.BrowserLabAdminRealm)
			err = DeleteVlan(ctx, "vlan-2")
			assert.Loosely(t, err, should.BeNil)

			resp, err = configuration.GetVlan(ctx, "vlan-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			respIps, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.16.1"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respIps, should.BeNil)
		})
		t.Run("Delete vlan: permission denied", func(t *ftt.Test) {
			vlan1 := mockVlan("delete-vlan-3", "192.168.100.0/28")
			vlan1.Zones = []ufspb.Zone{ufspb.Zone_ZONE_MTV97, ufspb.Zone_ZONE_MTV96}
			vlan1.Realm = util.BrowserLabAdminRealm
			_, err := configuration.CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)

			//same realm and no delete permission
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksGet, util.BrowserLabAdminRealm)
			err = DeleteVlan(ctx, "delete-vlan-3")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))

			//different realm
			ctx = initializeFakeAuthDB(ctx, "user:user@example.com", util.NetworksDelete, util.AtlLabAdminRealm)
			err = DeleteVlan(ctx, "delete-vlan-3")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestBatchGetVlans(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetVlans", t, func(t *ftt.Test) {
		t.Run("Batch get vlans - happy path", func(t *ftt.Test) {
			entities := make([]*ufspb.Vlan, 4)
			for i := range 4 {
				entities[i] = &ufspb.Vlan{
					Name: fmt.Sprintf("vlan-batchGet-%d", i),
				}
			}
			_, err := configuration.BatchUpdateVlans(ctx, entities)
			assert.Loosely(t, err, should.BeNil)
			resp, err := configuration.BatchGetVlans(ctx, []string{"vlan-batchGet-0", "vlan-batchGet-1", "vlan-batchGet-2", "vlan-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(entities))
		})
		t.Run("Batch get vlans  - missing id", func(t *ftt.Test) {
			resp, err := configuration.BatchGetVlans(ctx, []string{"vlan-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("vlan-batchGet-non-existing"))
		})
		t.Run("Batch get vlans  - empty input", func(t *ftt.Test) {
			resp, err := configuration.BatchGetVlans(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = configuration.BatchGetVlans(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}
