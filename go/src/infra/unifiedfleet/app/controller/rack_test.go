// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"google.golang.org/genproto/protobuf/field_mask"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/history"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func TestRackRegistration(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("RackRegistration", t, func(t *ftt.Test) {
		t.Run("Create new rack", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-1",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.BrowserLabAdminRealm)
			resp, err := RackRegistration(ctx, rack)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetChromeBrowserRack().GetKvmObjects(), should.BeNil)
			assert.Loosely(t, resp.GetChromeBrowserRack().GetRpmObjects(), should.BeNil)
			assert.Loosely(t, resp.GetChromeBrowserRack().GetSwitchObjects(), should.BeNil)
			s, err := state.GetStateRecord(ctx, "racks/rack-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "racks/rack-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rack"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "racks/rack-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)

			rack.Name = "RACK-1"
			resp, err = RackRegistration(ctx, rack)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("already exists in the system"))
			assert.Loosely(t, resp, should.BeNil)
		})

		t.Run("Create new rack with nil browser/chromeos rack", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-2",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATLANTA,
				},
			}
			resp, err := RackRegistration(ctx, rack)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetChromeBrowserRack(), should.NotBeNil)

			resp, _ = registration.GetRack(ctx, "rack-2")
			assert.Loosely(t, resp.GetChromeBrowserRack(), should.NotBeNil)
			s, err := state.GetStateRecord(ctx, "racks/rack-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "racks/rack-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rack"))
		})

		t.Run("Create new rack with nil browser/chromeos rack and no lcoation info", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-3",
			}
			_, err := RackRegistration(ctx, rack)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("zone information in the location object cannot be empty/unspecified for a rack"))

			// No changes are recorded as the creation fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "racks/rack-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create new rack - permission denied: same realm and no create permission", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-12",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err := RackRegistration(ctx, rack)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Create new rack - permission denied: different realm", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-13",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.AtlLabAdminRealm)
			_, err := RackRegistration(ctx, rack)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Create new rack - duplicated bbnum", func(t *ftt.Test) {
			fmt.Println("duplicated bbnum")
			rack := &ufspb.Rack{
				Name:  "rack-14",
				Bbnum: 14,
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.BrowserLabAdminRealm)
			_, err := RackRegistration(ctx, rack)
			assert.Loosely(t, err, should.BeNil)
			rack2 := &ufspb.Rack{
				Name:  "rack-14-duplicated",
				Bbnum: 14,
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			_, err = RackRegistration(ctx, rack2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("already has bbnum 14"))
		})
	})
}

func TestUpdateRack(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateRack", t, func(t *ftt.Test) {
		t.Run("Update non-existing rack - error", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-1",
			}
			_, err := UpdateRack(ctx, rack, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Update existing rack", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name:  "rack-2",
				Bbnum: 100,
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rack = &ufspb.Rack{
				Name:  "rack-2",
				Bbnum: 200,
				Tags:  []string{"tag-1"},
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateRack(ctx, rack, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(rack))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "racks/rack-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Update existing rack - valid/invalid bbnum", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name:  "rack-bbnum1",
				Bbnum: 101,
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)
			rack2 := &ufspb.Rack{
				Name:  "rack-bbnum2",
				Bbnum: 102,
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err = registration.CreateRack(ctx, rack2)
			assert.Loosely(t, err, should.BeNil)

			rackToUpdate := &ufspb.Rack{
				Name:  "rack-bbnum1",
				Bbnum: 101,
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err = UpdateRack(ctx, rackToUpdate, nil)
			assert.Loosely(t, err, should.BeNil)

			rackToUpdate.Bbnum = 102
			_, err = UpdateRack(ctx, rackToUpdate, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("already has bbnum 102"))
		})

		t.Run("Update existing rack with nil rack/browser machine", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-3",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rack = &ufspb.Rack{
				Name: "rack-3",
			}
			resp, err := UpdateRack(ctx, rack, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
		})

		t.Run("Partial Update rack", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name:       "rack-4",
				CapacityRu: 55,
				Bbnum:      155,
				Tags:       []string{"atl", "megarack"},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rack1 := &ufspb.Rack{
				Name:       "rack-4",
				CapacityRu: 100,
				Bbnum:      255,
			}
			resp, err := UpdateRack(ctx, rack1, &field_mask.FieldMask{Paths: []string{"capacity", "bbnum"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetCapacityRu(), should.Equal(100))
			assert.Loosely(t, resp.GetBbnum(), should.Equal(255))
			assert.Loosely(t, resp.GetTags(), should.Match([]string{"atl", "megarack"}))
		})

		t.Run("Partial Update rack - invalid bbnum", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name:  "rack-4-bbnum1",
				Bbnum: 155,
				Tags:  []string{"atl", "megarack"},
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)
			rack2 := &ufspb.Rack{
				Name:  "rack-4-bbnum2",
				Bbnum: 156,
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err = registration.CreateRack(ctx, rack2)
			assert.Loosely(t, err, should.BeNil)

			rackToUpdate := &ufspb.Rack{
				Name:  "rack-4-bbnum1",
				Bbnum: 156,
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}

			_, err = UpdateRack(ctx, rackToUpdate, &field_mask.FieldMask{Paths: []string{"bbnum"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("already has bbnum 156"))
		})

		t.Run("Update rack - permission denied: same realm and no update permission", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-21",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rack.Tags = []string{"tag-21"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = UpdateRack(ctx, rack, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update rack - permission denied: different realm", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-22",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rack.Tags = []string{"tag-22"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			_, err = UpdateRack(ctx, rack, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update rack(realm name) - different realm with permission success", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-23",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rack.Realm = util.AtlLabAdminRealm
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.RegistrationsUpdate),
				),
			})
			resp, err := UpdateRack(ctx, rack, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rack))
		})

		t.Run("Update rack(realm name) - permission denied: different realm without permission", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-24",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rack.Realm = util.AtlLabAdminRealm
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateRack(ctx, rack, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestDeleteRack(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteRack", t, func(t *ftt.Test) {
		t.Run("Delete rack by existing ID with rackLSE reference", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-3",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			rackLSE1 := &ufspb.RackLSE{
				Name:  "racklse-1",
				Racks: []string{"rack-3"},
			}
			_, err = inventory.CreateRackLSE(ctx, rackLSE1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.BrowserLabAdminRealm)
			err = DeleteRack(ctx, "rack-3")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(CannotDelete))

			resp, err := registration.GetRack(ctx, "rack-3")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rack1))

			// No changes are recorded as the deletion fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "racks/rack-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Delete rack by existing ID without references", func(t *ftt.Test) {
			rack2 := &ufspb.Rack{
				Name: "rack-4",
			}
			_, err := registration.CreateRack(ctx, rack2)
			assert.Loosely(t, err, should.BeNil)

			err = DeleteRack(ctx, "rack-4")
			assert.Loosely(t, err, should.BeNil)

			resp, err := registration.GetRack(ctx, "rack-4")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "racks/rack-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rack"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "racks/rack-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
		})

		t.Run("Delete rack with rpms - happy path", func(t *ftt.Test) {
			rpm := &ufspb.RPM{
				Name: "rpm-6",
				Rack: "rack-6",
			}
			_, err := registration.CreateRPM(ctx, rpm)
			assert.Loosely(t, err, should.BeNil)

			rack := &ufspb.Rack{
				Name: "rack-6",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err = registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			_, err = state.BatchUpdateStates(ctx, []*ufspb.StateRecord{
				{
					ResourceName: "rpms/rpm-6",
					State:        ufspb.State_STATE_SERVING,
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateDHCPs(ctx, []*ufspb.DHCPConfig{
				{
					Hostname: "rpm-6",
					Ip:       "1.2.3.6",
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateIPs(ctx, []*ufspb.IP{
				{
					Id:       "ip2",
					Occupied: true,
					Ipv4Str:  "1.2.3.6",
					Vlan:     "fake_vlan",
					Ipv4:     uint32(100),
				},
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteRack(ctx, "rack-6")
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.GetRack(ctx, "rack-6")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = registration.GetRPM(ctx, "rpm-6")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			_, err = configuration.GetDHCPConfig(ctx, "kvm-6")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			resIPs, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "1.2.3.6"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resIPs, should.HaveLength(1))
			assert.Loosely(t, resIPs[0].Occupied, should.BeFalse)

			_, err = state.GetStateRecord(ctx, "racks/rack-6")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			_, err = state.GetStateRecord(ctx, "rpms/rpm-6")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "racks/rack-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rack"))
		})

		t.Run("Delete rack with switches, kvms - happy path", func(t *ftt.Test) {
			kvm := &ufspb.KVM{
				Name: "kvm-5",
				Rack: "rack-5",
			}
			_, err := registration.CreateKVM(ctx, kvm)
			assert.Loosely(t, err, should.BeNil)

			switch5 := &ufspb.Switch{
				Name: "switch-5",
				Rack: "rack-5",
			}
			_, err = registration.CreateSwitch(ctx, switch5)
			assert.Loosely(t, err, should.BeNil)

			rack := &ufspb.Rack{
				Name: "rack-5",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err = registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			_, err = state.BatchUpdateStates(ctx, []*ufspb.StateRecord{
				{
					ResourceName: "racks/rack-5",
					State:        ufspb.State_STATE_SERVING,
				},
				{
					ResourceName: "switches/switch-5",
					State:        ufspb.State_STATE_SERVING,
				},
				{
					ResourceName: "kvms/kvm-5",
					State:        ufspb.State_STATE_SERVING,
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateDHCPs(ctx, []*ufspb.DHCPConfig{
				{
					Hostname: "kvm-5",
					Ip:       "1.2.3.4",
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateIPs(ctx, []*ufspb.IP{
				{
					Id:       "ip1",
					Occupied: true,
					Ipv4Str:  "1.2.3.4",
					Vlan:     "fake_vlan",
					Ipv4:     uint32(100),
				},
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteRack(ctx, "rack-5")
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.GetRack(ctx, "rack-5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = registration.GetKVM(ctx, "kvm-5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			_, err = configuration.GetDHCPConfig(ctx, "kvm-5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			resIPs, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "1.2.3.4"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resIPs, should.HaveLength(1))
			assert.Loosely(t, resIPs[0].Occupied, should.BeFalse)

			_, err = registration.GetSwitch(ctx, "switch-5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = state.GetStateRecord(ctx, "racks/rack-5")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			_, err = state.GetStateRecord(ctx, "switches/switch-5")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			_, err = state.GetStateRecord(ctx, "kvms/kvm-5")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "racks/rack-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rack"))
		})

		t.Run("Delete rack - Permission denied: same realm and no delete permission", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-31",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			err = DeleteRack(ctx, "rack-31")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Delete rack - Permission denied: different realm", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-32",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.AtlLabAdminRealm)
			err = DeleteRack(ctx, "rack-32")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestReplaceRack(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("ReplaceRacks", t, func(t *ftt.Test) {
		t.Run("Repalce an old Rack with new rack with RackLSE reference", func(t *ftt.Test) {
			oldRack1 := &ufspb.Rack{
				Name: "rack-4",
			}
			_, err := registration.CreateRack(ctx, oldRack1)
			assert.Loosely(t, err, should.BeNil)

			rackLSE1 := &ufspb.RackLSE{
				Name:  "racklse-1",
				Racks: []string{"rack-0", "rack-50", "rack-4", "rack-7"},
			}
			_, err = inventory.CreateRackLSE(ctx, rackLSE1)
			assert.Loosely(t, err, should.BeNil)

			newRack2 := &ufspb.Rack{
				Name: "rack-100",
			}
			resp, err := ReplaceRack(ctx, oldRack1, newRack2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(newRack2))

			rlse, err := inventory.GetRackLSE(ctx, "racklse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rlse, should.NotBeNil)
			assert.Loosely(t, rlse.GetRacks(), should.Match([]string{"rack-0", "rack-50", "rack-100", "rack-7"}))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "racks/rack-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rack"))

			changes, err = history.QueryChangesByPropertyName(ctx, "name", "racks/rack-100")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rack"))
		})

		t.Run("Repalce an old Rack with already existing rack", func(t *ftt.Test) {
			existingRack1 := &ufspb.Rack{
				Name: "rack-105",
			}
			_, err := registration.CreateRack(ctx, existingRack1)
			assert.Loosely(t, err, should.BeNil)

			oldRack1 := &ufspb.Rack{
				Name: "rack-5",
			}
			_, err = registration.CreateRack(ctx, oldRack1)
			assert.Loosely(t, err, should.BeNil)

			newRack2 := &ufspb.Rack{
				Name: "rack-105",
			}
			rresp, rerr := ReplaceRack(ctx, oldRack1, newRack2)
			assert.Loosely(t, rerr, should.NotBeNil)
			assert.Loosely(t, rresp, should.BeNil)
			assert.Loosely(t, rerr.Error(), should.ContainSubstring(AlreadyExists))

			// No change are recorded as the replacement fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "racks/rack-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "racks/rack-105")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})
	})
}

func TestListRacks(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	racksWithSwitch := make([]*ufspb.Rack, 0, 2)
	racks := make([]*ufspb.Rack, 0, 4)
	for i := range 4 {
		rack := &ufspb.Rack{
			Name: fmt.Sprintf("rack-%d", i),
			Rack: &ufspb.Rack_ChromeBrowserRack{
				ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
			},
		}
		if i%2 == 0 {
			rack.Tags = []string{"tag-12"}
		}
		resp, _ := registration.CreateRack(ctx, rack)
		if i%2 == 0 {
			racksWithSwitch = append(racksWithSwitch, resp)
		}
		racks = append(racks, resp)
	}
	ftt.Run("ListRacks", t, func(t *ftt.Test) {
		t.Run("List Racks - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListRacks(ctx, 5, "", "invalid=mx-1", false, false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name invalid"))
		})

		t.Run("List Racks - filter switch - happy path", func(t *ftt.Test) {
			resp, _, _ := ListRacks(ctx, 5, "", "tag=tag-12", false, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(racksWithSwitch))
		})

		t.Run("ListRacks - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListRacks(ctx, 5, "", "", false, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(racks))
		})
	})
}

func TestBatchGetRacks(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetRacks", t, func(t *ftt.Test) {
		t.Run("Batch get racks - happy path", func(t *ftt.Test) {
			entities := make([]*ufspb.Rack, 4)
			for i := range 4 {
				entities[i] = &ufspb.Rack{
					Name: fmt.Sprintf("rack-batchGet-%d", i),
				}
			}
			_, err := registration.BatchUpdateRacks(ctx, entities)
			assert.Loosely(t, err, should.BeNil)
			resp, err := registration.BatchGetRacks(ctx, []string{"rack-batchGet-0", "rack-batchGet-1", "rack-batchGet-2", "rack-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(entities))
		})
		t.Run("Batch get racks  - missing id", func(t *ftt.Test) {
			resp, err := registration.BatchGetRacks(ctx, []string{"rack-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("rack-batchGet-non-existing"))
		})
		t.Run("Batch get racks  - empty input", func(t *ftt.Test) {
			resp, err := registration.BatchGetRacks(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = registration.BatchGetRacks(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}

func TestRenameRack(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	_, err := registration.CreateRack(ctx, &ufspb.Rack{
		Name: "rename-rack",
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_CHROMEOS4,
		},
	})
	ftt.Run("RenameRack", t, func(t *ftt.Test) {
		t.Run("RenameRack - happy path", func(t *ftt.Test) {
			ctx := initializeMockAuthDB(ctx, "user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate, util.RegistrationsCreate, util.InventoriesCreate)
			_, err := registration.CreateRack(ctx, &ufspb.Rack{
				Name: "rename-rack-0-old",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS4,
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = AssetRegistration(ctx, &ufspb.Asset{
				Name: "rename-asset-0",
				Location: &ufspb.Location{
					Rack: "rename-rack-0-old",
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
				Realm: util.AtlLabAdminRealm,
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = MachineRegistration(ctx, &ufspb.Machine{
				Name: "rename-machine-0",
				Location: &ufspb.Location{
					Rack: "rename-rack-0-old",
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
				Realm: util.AtlLabAdminRealm,
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Hostname: "rename-host-0",
				Machines: []string{"rename-machine-0"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}, nil)
			assert.Loosely(t, err, should.BeNil)

			// Test begins
			_, err = RenameRack(ctx, "rename-rack-0-old", "rename-rack-0-new")
			assert.Loosely(t, err, should.BeNil)

			asset, err := GetAsset(ctx, "rename-asset-0")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, asset.GetLocation().GetRack(), should.Equal("rename-rack-0-new"))
			machine, err := GetMachine(ctx, "rename-machine-0")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, machine.GetLocation().GetRack(), should.Equal("rename-rack-0-new"))
			lse, err := GetMachineLSE(ctx, "rename-host-0")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse.GetRack(), should.Equal("rename-rack-0-new"))
			// Verify change events
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "racks/rename-rack-0-new")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rack"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("rack.name"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("rename-rack-0-old"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("rename-rack-0-new"))

			changes, err = history.QueryChangesByPropertyName(ctx, "name", "assets/rename-asset-0")
			assert.Loosely(t, err, should.BeNil)
			// The first is the registration of the asset, the second is the rack change.
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("asset"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("asset.location"))
			assert.Loosely(t, changes[1].GetOldValue(), should.ContainSubstring("rename-rack-0-old"))
			assert.Loosely(t, changes[1].GetNewValue(), should.ContainSubstring("rename-rack-0-new"))

			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/rename-machine-0")
			assert.Loosely(t, err, should.BeNil)
			// The first is the registration of the machine, the second is the rack change.
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			// Please note that adding a machine will set the machine state to registered.
			// Adding a machine lse for a machine will change the machine's state to SERVING automatically.
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine.resource_state"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("STATE_REGISTERED"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("STATE_SERVING"))
			assert.Loosely(t, changes[2].GetEventLabel(), should.Equal("machine.location"))
			assert.Loosely(t, changes[2].GetOldValue(), should.ContainSubstring("rename-rack-0-old"))
			assert.Loosely(t, changes[2].GetNewValue(), should.ContainSubstring("rename-rack-0-new"))

			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/rename-host-0")
			assert.Loosely(t, err, should.BeNil)
			// The first is the registration of the machineLSE, the second is the rack change.
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine_lse.rack"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("rename-rack-0-old"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("rename-rack-0-new"))

		})
		t.Run("RenameRack - permission denied", func(t *ftt.Test) {
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = RenameRack(ctx, "rename-rack", "rename-rack-new")
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
		t.Run("RenameRack - old rack doesn't exist", func(t *ftt.Test) {
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			_, err = RenameRack(ctx, "rename-rack-non-existing", "rename-rack-0-new")
			assert.Loosely(t, util.IsNotFoundError(err), should.BeTrue)
		})
		t.Run("RenameRack - it's ok that the new rack already exists", func(t *ftt.Test) {
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			_, err = RenameRack(ctx, "rename-rack", "rename-rack")
			assert.Loosely(t, err, should.BeNil)
		})
	})
}
