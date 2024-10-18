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

	ufspb "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/model/configuration"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/history"
	"infra/unifiedfleet/app/model/registration"
	"infra/unifiedfleet/app/model/state"
	"infra/unifiedfleet/app/util"
)

func mockRPM(id string) *ufspb.RPM {
	return &ufspb.RPM{
		Name: id,
	}
}

func TestCreateRPM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	rack1 := &ufspb.Rack{
		Name: "rack-1",
		Rack: &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
		},
	}
	registration.CreateRack(ctx, rack1)
	ftt.Run("CreateRPM", t, func(t *ftt.Test) {
		t.Run("Create new rpm with already existing rpm - error", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-5",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			registration.CreateRack(ctx, rack1)

			rpm1 := &ufspb.RPM{
				Name: "rpm-1",
				Rack: "rack-5",
			}
			_, err := registration.CreateRPM(ctx, rpm1)
			assert.Loosely(t, err, should.BeNil)

			resp, err := CreateRPM(ctx, rpm1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("RPM rpm-1 already exists in the system"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "rpms/rpm-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create RPM - duplicated mac address", func(t *ftt.Test) {
			rpm := &ufspb.RPM{
				Name:       "rpm-2-mac",
				MacAddress: "rpm-2-address",
			}
			_, err := registration.CreateRPM(ctx, rpm)
			assert.Loosely(t, err, should.BeNil)
			rpm2 := &ufspb.RPM{
				Name:       "rpm-2-mac2",
				MacAddress: "rpm-2-address",
				Rack:       "rack-1",
			}
			_, err = CreateRPM(ctx, rpm2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("mac_address rpm-2-address is already occupied"))
		})

		t.Run("Create new rpm with existing rack", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-10",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rpm1 := &ufspb.RPM{
				Name: "rpm-20",
				Rack: "rack-10",
			}
			resp, err := CreateRPM(ctx, rpm1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rpm1))

			s, err := state.GetStateRecord(ctx, "rpms/rpm-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "rpms/rpm-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rpm"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "rpms/rpm-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Create new rpm - Permission denied: same realm and no create permission", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-20",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rpm1 := &ufspb.RPM{
				Name: "rpm-20",
				Rack: "rack-20",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = CreateRPM(ctx, rpm1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Create new rpm - Permission denied: different realm", func(t *ftt.Test) {
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

			rpm1 := &ufspb.RPM{
				Name: "rpm-21",
				Rack: "rack-21",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.AtlLabAdminRealm)
			_, err = CreateRPM(ctx, rpm1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestUpdateRPM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateRPM", t, func(t *ftt.Test) {
		t.Run("Update rpm with non-existing rpm", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-1",
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			rpm1 := &ufspb.RPM{
				Name: "rpm-1",
				Rack: "rack-1",
			}
			resp, err := UpdateRPM(ctx, rpm1, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "rpms/rpm-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update rpm with new rack(same realm) - pass", func(t *ftt.Test) {
			rack3 := &ufspb.Rack{
				Name: "rack-3",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack3)
			assert.Loosely(t, err, should.BeNil)

			rack4 := &ufspb.Rack{
				Name: "rack-4",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err = registration.CreateRack(ctx, rack4)
			assert.Loosely(t, err, should.BeNil)

			rpm3 := &ufspb.RPM{
				Name: "rpm-3",
				Rack: "rack-3",
				// Needs to be manually set since we directly call
				// registration.CreateRPM which doesn't pull the zone from
				// the rack.
				Zone: "ZONE_SFO36_BROWSER",
			}
			_, err = registration.CreateRPM(ctx, rpm3)
			assert.Loosely(t, err, should.BeNil)

			rpm3.Rack = "rack-4"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateRPM(ctx, rpm3, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(rpm3))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "rpms/rpm-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rpm.rack"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("rack-3"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("rack-4"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "rpms/rpm-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Update rpm with same rack(same realm) - pass", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-5",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rpm1 := &ufspb.RPM{
				Name:       "rpm-5",
				Rack:       "rack-5",
				MacAddress: "rpm-10-address",
			}
			_, err = registration.CreateRPM(ctx, rpm1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateRPM(ctx, rpm1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(rpm1))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "rpms/rpm-5")
			assert.Loosely(t, err, should.BeNil)
			// Nothing is changed for rpm-5
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "rpms/rpm-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Update rpm with non existing rack", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-6.1",
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			rpm1 := &ufspb.RPM{
				Name: "rpm-6",
				Rack: "rack-6.1",
			}
			_, err = registration.CreateRPM(ctx, rpm1)
			assert.Loosely(t, err, should.BeNil)

			rpm1.Rack = "rack-6"
			resp, err := UpdateRPM(ctx, rpm1, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no Rack with RackID rack-6 in the system"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "rpms/rpm-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Partial Update rpm mac address - duplicated mac address", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-8",
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			rpm := &ufspb.RPM{
				Name:       "rpm-8",
				Rack:       "rack-8",
				MacAddress: "rpm-8-address",
			}
			_, err = registration.CreateRPM(ctx, rpm)
			assert.Loosely(t, err, should.BeNil)

			rpm2 := &ufspb.RPM{
				Name:       "rpm-8.2",
				Rack:       "rack-8",
				MacAddress: "rpm-8.2-address",
			}
			_, err = registration.CreateRPM(ctx, rpm2)
			assert.Loosely(t, err, should.BeNil)

			rpm1 := &ufspb.RPM{
				Name:       "rpm-8",
				MacAddress: "rpm-8.2-address",
			}
			_, err = UpdateRPM(ctx, rpm1, &field_mask.FieldMask{Paths: []string{"macAddress"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("mac_address rpm-8.2-address is already occupied"))
		})

		t.Run("Update rpm mac address - duplicated mac address", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-9",
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			rpm := &ufspb.RPM{
				Name:       "rpm-9",
				Rack:       "rack-9",
				MacAddress: "rpm-9-address",
			}
			_, err = registration.CreateRPM(ctx, rpm)
			assert.Loosely(t, err, should.BeNil)
			rpm2 := &ufspb.RPM{
				Name:       "rpm-9.2",
				Rack:       "rack-9",
				MacAddress: "rpm-9.2-address",
			}
			_, err = registration.CreateRPM(ctx, rpm2)
			assert.Loosely(t, err, should.BeNil)

			rpm1 := &ufspb.RPM{
				Name:       "rpm-9",
				MacAddress: "rpm-9.2-address",
			}
			_, err = UpdateRPM(ctx, rpm1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("mac_address rpm-9.2-address is already occupied"))
		})

		t.Run("Update rpm - Permission denied: same realm and no update permission", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-51",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rpm1 := &ufspb.RPM{
				Name: "rpm-51",
				Rack: "rack-51",
			}
			_, err = registration.CreateRPM(ctx, rpm1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = UpdateRPM(ctx, rpm1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update rpm - Permission denied: different realm", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-52",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rpm1 := &ufspb.RPM{
				Name: "rpm-52",
				Rack: "rack-52",
			}
			_, err = registration.CreateRPM(ctx, rpm1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			_, err = UpdateRPM(ctx, rpm1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update rpm with new rack(different realm with no permission)- fail", func(t *ftt.Test) {
			rack3 := &ufspb.Rack{
				Name: "rack-53",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack3)
			assert.Loosely(t, err, should.BeNil)

			rack4 := &ufspb.Rack{
				Name: "rack-54",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS4,
				},
			}
			_, err = registration.CreateRack(ctx, rack4)
			assert.Loosely(t, err, should.BeNil)

			rpm3 := &ufspb.RPM{
				Name: "rpm-53",
				Rack: "rack-53",
			}
			_, err = registration.CreateRPM(ctx, rpm3)
			assert.Loosely(t, err, should.BeNil)

			rpm3.Rack = "rack-54"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateRPM(ctx, rpm3, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update rpm with new rack(different realm with permission)- pass", func(t *ftt.Test) {
			rack3 := &ufspb.Rack{
				Name: "rack-55",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack3)
			assert.Loosely(t, err, should.BeNil)

			rack4 := &ufspb.Rack{
				Name: "rack-56",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS4,
				},
			}
			_, err = registration.CreateRack(ctx, rack4)
			assert.Loosely(t, err, should.BeNil)

			rpm3 := &ufspb.RPM{
				Name: "rpm-55",
				Rack: "rack-55",
				// Needs to be manually set since we directly call
				// registration.CreateRPM which doesn't pull the zone from
				// the rack.
				Zone: "ZONE_SFO36_BROWSER",
			}
			_, err = registration.CreateRPM(ctx, rpm3)
			assert.Loosely(t, err, should.BeNil)

			rpm3.Rack = "rack-56"
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.RegistrationsUpdate),
				),
			})
			resp, err := UpdateRPM(ctx, rpm3, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(rpm3))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "rpms/rpm-55")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rpm.zone"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("ZONE_SFO36_BROWSER"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("ZONE_CHROMEOS4"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("rpm.rack"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("rack-55"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("rack-56"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "rpms/rpm-55")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Partial Update rpm with new rack(same realm) - pass", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-57",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			s := &ufspb.RPM{
				Name: "rpm-57",
				Rack: "rack-57",
			}
			_, err = registration.CreateRPM(ctx, s)
			assert.Loosely(t, err, should.BeNil)

			rack = &ufspb.Rack{
				Name: "rack-58",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err = registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			s.Rack = "rack-58"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateRPM(ctx, s, &field_mask.FieldMask{Paths: []string{"rack"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetRack(), should.Match("rack-58"))
		})

		t.Run("Partial Update rpm with new rack(different realm with permission) - pass", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-59",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			s := &ufspb.RPM{
				Name: "rpm-59",
				Rack: "rack-59",
			}
			_, err = registration.CreateRPM(ctx, s)
			assert.Loosely(t, err, should.BeNil)

			rack = &ufspb.Rack{
				Name: "rack-60",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS4,
				},
			}
			_, err = registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			s.Rack = "rack-60"
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.RegistrationsUpdate),
				),
			})
			resp, err := UpdateRPM(ctx, s, &field_mask.FieldMask{Paths: []string{"rack"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetRack(), should.Match("rack-60"))
		})

		t.Run("Partial Update rpm with new rack(different realm without permission) - fail", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-61",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			s := &ufspb.RPM{
				Name: "rpm-61",
				Rack: "rack-61",
			}
			_, err = registration.CreateRPM(ctx, s)
			assert.Loosely(t, err, should.BeNil)

			rack = &ufspb.Rack{
				Name: "rack-62",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS4,
				},
			}
			_, err = registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			s.Rack = "rack-62"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateRPM(ctx, s, &field_mask.FieldMask{Paths: []string{"rack"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

	})
}

func TestDeleteRPM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteRPM", t, func(t *ftt.Test) {
		t.Run("Delete rpm by non-existing ID - error", func(t *ftt.Test) {
			err := DeleteRPM(ctx, "rpm-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "rpms/rpm-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Delete RPM by existing ID with machine reference", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-1",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			registration.CreateRack(ctx, rack1)

			RPM1 := &ufspb.RPM{
				Name: "RPM-1",
				Rack: "rack-1",
			}
			_, err := registration.CreateRPM(ctx, RPM1)
			assert.Loosely(t, err, should.BeNil)

			chromeBrowserMachine1 := &ufspb.Machine{
				Name: "machine-1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						RpmInterface: &ufspb.RPMInterface{
							Rpm: "RPM-1",
						},
					},
				},
			}
			_, err = registration.CreateMachine(ctx, chromeBrowserMachine1)
			assert.Loosely(t, err, should.BeNil)

			err = DeleteRPM(ctx, "RPM-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Machines referring the RPM:"))

			resp, err := registration.GetRPM(ctx, "RPM-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(RPM1))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "rpms/RPM-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Delete RPM successfully", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-2",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			registration.CreateRack(ctx, rack1)

			rpm2 := mockRPM("rpm-2")
			rpm2.Rack = "rack-2"
			_, err := registration.CreateRPM(ctx, rpm2)
			assert.Loosely(t, err, should.BeNil)
			_, err = state.BatchUpdateStates(ctx, []*ufspb.StateRecord{
				{
					ResourceName: "rpms/rpm-2",
					State:        ufspb.State_STATE_SERVING,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteRPM(ctx, "rpm-2")
			assert.Loosely(t, err, should.BeNil)

			resp, err := registration.GetRPM(ctx, "rpm-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = state.GetStateRecord(ctx, "rpms/rpm-2")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "rpms/rpm-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rpm"))
		})

		t.Run("Delete RPM successfully together with deleting ip", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-ip2",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			registration.CreateRack(ctx, rack1)

			rpm2 := mockRPM("rpm-ip2")
			rpm2.Rack = "rack-ip2"
			_, err := registration.CreateRPM(ctx, rpm2)
			assert.Loosely(t, err, should.BeNil)
			_, err = state.BatchUpdateStates(ctx, []*ufspb.StateRecord{
				{
					ResourceName: "rpms/rpm-ip2",
					State:        ufspb.State_STATE_SERVING,
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateDHCPs(ctx, []*ufspb.DHCPConfig{
				{
					Hostname: "rpm-ip2",
					Ip:       "1.2.3.4",
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.ImportIPs(ctx, []*ufspb.IP{
				{
					Id:       "vlan-1:123",
					Ipv4Str:  "1.2.3.4",
					Vlan:     "vlan-1",
					Occupied: true,
					Ipv4:     123,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteRPM(ctx, "rpm-ip2")
			assert.Loosely(t, err, should.BeNil)
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "1.2.3.4"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeFalse)
			_, err = configuration.GetDHCPConfig(ctx, "rpm-ip2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			resp, err := registration.GetRPM(ctx, "rpm-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = state.GetStateRecord(ctx, "rpms/rpm-2")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "rpms/rpm-ip2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("rpm"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "rpms/rpm-ip2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/rpm-ip2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/rpms/rpm-ip2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
		})

		t.Run("Delete rpm - Permission denied: same realm and no delete permission", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-53",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rpm2 := mockRPM("rpm-53")
			rpm2.Rack = "rack-53"
			_, err = registration.CreateRPM(ctx, rpm2)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			err = DeleteRPM(ctx, "rpm-53")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Delete rpm - Permission denied: different realm", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-54",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			rpm2 := mockRPM("rpm-54")
			rpm2.Rack = "rack-54"
			_, err = registration.CreateRPM(ctx, rpm2)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.AtlLabAdminRealm)
			err = DeleteRPM(ctx, "rpm-54")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestListRPMs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	rpms := make([]*ufspb.RPM, 0, 2)
	for i := 0; i < 4; i++ {
		rpm := mockRPM(fmt.Sprintf("rpm-%d", i))
		resp, _ := registration.CreateRPM(ctx, rpm)
		rpms = append(rpms, resp)
	}
	ftt.Run("ListRPMs", t, func(t *ftt.Test) {
		t.Run("List RPMs - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListRPMs(ctx, 5, "", "invalid=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name invalid"))
		})

		t.Run("ListRPMs - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListRPMs(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(rpms))
		})
	})
}

func TestBatchGetRPMs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetRPMs", t, func(t *ftt.Test) {
		t.Run("Batch get rpms - happy path", func(t *ftt.Test) {
			entities := make([]*ufspb.RPM, 4)
			for i := 0; i < 4; i++ {
				entities[i] = &ufspb.RPM{
					Name: fmt.Sprintf("rpm-batchGet-%d", i),
				}
			}
			_, err := registration.BatchUpdateRPMs(ctx, entities)
			assert.Loosely(t, err, should.BeNil)
			resp, err := registration.BatchGetRPMs(ctx, []string{"rpm-batchGet-0", "rpm-batchGet-1", "rpm-batchGet-2", "rpm-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(entities))
		})
		t.Run("Batch get rpms  - missing id", func(t *ftt.Test) {
			resp, err := registration.BatchGetRPMs(ctx, []string{"rpm-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("rpm-batchGet-non-existing"))
		})
		t.Run("Batch get rpms  - empty input", func(t *ftt.Test) {
			resp, err := registration.BatchGetRPMs(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = registration.BatchGetRPMs(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}

func TestUpdateRPMIP(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	rack := &ufspb.Rack{
		Name: "rack-1-ip",
		Rack: &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
		},
	}
	registration.CreateRack(ctx, rack)
	ftt.Run("TestUpdateRPMIP", t, func(t *ftt.Test) {
		t.Run("Update rpm by setting ip by vlan for rpm", func(t *ftt.Test) {
			rpm := &ufspb.RPM{
				Name:       "rpm-1",
				Rack:       "rack-1-ip",
				MacAddress: "old_mac_address",
			}
			_, err := registration.CreateRPM(ctx, rpm)
			assert.Loosely(t, err, should.BeNil)

			assert.Loosely(t, err, should.BeNil)
			vlan := &ufspb.Vlan{
				Name:        "vlan-1",
				VlanAddress: "192.168.40.0/22",
			}
			_, err = configuration.CreateVlan(ctx, vlan)
			assert.Loosely(t, err, should.BeNil)
			ips, _, startFreeIP, _, _, err := util.ParseVlan(vlan.GetName(), vlan.GetVlanAddress(), vlan.GetFreeStartIpv4Str(), vlan.GetFreeEndIpv4Str())
			var assignedIP *ufspb.IP
			for _, ip := range ips {
				if ip.GetIpv4Str() == startFreeIP {
					assignedIP = ip
				}
			}
			assert.Loosely(t, err, should.BeNil)
			// Only import the first 20 as one single transaction cannot import all.
			_, err = configuration.ImportIPs(ctx, ips[0:20])
			assert.Loosely(t, err, should.BeNil)

			err = UpdateRPMHost(ctx, rpm, &ufsAPI.NetworkOption{
				Vlan: "vlan-1",
			})
			assert.Loosely(t, err, should.BeNil)
			dhcp, err := configuration.GetDHCPConfig(ctx, "rpm-1")
			assert.Loosely(t, err, should.BeNil)
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": dhcp.GetIp()})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeTrue)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "states/rpms/rpm-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_DEPLOYING.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/rpm-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(dhcp.GetIp()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", fmt.Sprintf("ips/%s", assignedIP.GetId()))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("ip.occupied"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("false"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/rpms/rpm-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/rpm-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})

		t.Run("Update rpm by deleting ip for rpm host", func(t *ftt.Test) {
			rpm := &ufspb.RPM{
				Name:       "rpm-2",
				Rack:       "rack-1-ip",
				MacAddress: "old_mac_address",
			}
			_, err := registration.CreateRPM(ctx, rpm)
			assert.Loosely(t, err, should.BeNil)

			assert.Loosely(t, err, should.BeNil)
			vlan := &ufspb.Vlan{
				Name:        "vlan-1",
				VlanAddress: "192.168.40.0/22",
			}
			_, err = configuration.CreateVlan(ctx, vlan)
			// TODO(gregorynisbet): Fix this test.
			assert.Loosely(t, err, should.NotBeNil)
			ips, _, _, _, _, err := util.ParseVlan(vlan.GetName(), vlan.GetVlanAddress(), vlan.GetFreeStartIpv4Str(), vlan.GetFreeEndIpv4Str())
			assert.Loosely(t, err, should.BeNil)
			// Only import the first 20 as one single transaction cannot import all.
			_, err = configuration.ImportIPs(ctx, ips[0:20])
			assert.Loosely(t, err, should.BeNil)

			err = UpdateRPMHost(ctx, rpm, &ufsAPI.NetworkOption{
				Vlan: "vlan-1",
				Ip:   "192.168.40.12",
			})
			assert.Loosely(t, err, should.BeNil)
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.12"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeTrue)

			err = DeleteRPMHost(ctx, rpm.Name)
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.GetDHCPConfig(ctx, "rpm-2")
			// Not found error
			assert.Loosely(t, err, should.NotBeNil)
			ip2, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.12"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip2, should.HaveLength(1))
			assert.Loosely(t, ip2[0].GetOccupied(), should.BeFalse)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "states/rpms/rpm-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_DEPLOYING.String()))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(ufspb.State_STATE_DEPLOYING.String()))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/rpm-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("192.168.40.12"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("192.168.40.12"))
			assert.Loosely(t, changes[1].GetNewValue(), should.BeEmpty)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", fmt.Sprintf("ips/%s", ip[0].GetId()))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("ip.occupied"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("false"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("true"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("ip.occupied"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("true"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("false"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/rpms/rpm-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/rpm-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)
		})

		t.Run("Update rpm by setting ip by user for host", func(t *ftt.Test) {
			rpm := &ufspb.RPM{
				Name:       "rpm-3",
				Rack:       "rack-1-ip",
				MacAddress: "old_mac_address",
			}
			_, err := registration.CreateRPM(ctx, rpm)
			assert.Loosely(t, err, should.BeNil)

			assert.Loosely(t, err, should.BeNil)
			vlan := &ufspb.Vlan{
				Name:        "vlan-1",
				VlanAddress: "192.168.40.0/22",
			}
			_, err = configuration.CreateVlan(ctx, vlan)
			// TODO(gregorynisbet): Fix this test.
			assert.Loosely(t, err, should.NotBeNil)
			ips, _, _, _, _, err := util.ParseVlan(vlan.GetName(), vlan.GetVlanAddress(), vlan.GetFreeStartIpv4Str(), vlan.GetFreeEndIpv4Str())
			assert.Loosely(t, err, should.BeNil)
			// Only import the first 20 as one single transaction cannot import all.
			_, err = configuration.ImportIPs(ctx, ips[0:20])
			assert.Loosely(t, err, should.BeNil)

			err = UpdateRPMHost(ctx, rpm, &ufsAPI.NetworkOption{
				Vlan: "vlan-1",
				Ip:   "192.168.40.19",
			})
			assert.Loosely(t, err, should.BeNil)
			dhcp, err := configuration.GetDHCPConfig(ctx, "rpm-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dhcp.GetIp(), should.Equal("192.168.40.19"))
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.19"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeTrue)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "states/rpms/rpm-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("STATE_UNSPECIFIED"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("STATE_DEPLOYING"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/rpm-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(dhcp.GetIp()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", fmt.Sprintf("ips/%s", ip[0].GetId()))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("ip.occupied"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("false"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("true"))
		})

	})
}
