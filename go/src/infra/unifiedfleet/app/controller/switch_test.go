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
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/history"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func mockSwitch(id string) *ufspb.Switch {
	return &ufspb.Switch{
		Name: id,
	}
}

func TestCreateSwitch(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	rack1 := &ufspb.Rack{
		Name: "rack-1",
		Rack: &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
		},
	}
	registration.CreateRack(ctx, rack1)
	ftt.Run("CreateSwitch", t, func(t *ftt.Test) {
		t.Run("Create new switch with already existing switch - error", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-11",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			switch1 := &ufspb.Switch{
				Name: "switch-1",
				Rack: "rack-11",
			}
			_, err = registration.CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)

			resp, err := CreateSwitch(ctx, switch1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Switch switch-1 already exists in the system"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create new switch with non existing rack", func(t *ftt.Test) {
			switch2 := &ufspb.Switch{
				Name: "switch-2",
				Rack: "rack-5",
			}
			resp, err := CreateSwitch(ctx, switch2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create new switch with existing rack", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-15",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			switch1 := &ufspb.Switch{
				Name: "switch-25",
				Rack: "rack-15",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.BrowserLabAdminRealm)
			resp, err := CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(switch1))

			s, err := state.GetStateRecord(ctx, "switches/switch-25")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_SERVING))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-25")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("switch"))
		})

		t.Run("Create new switch - Permission denied: same realm and no create permission", func(t *ftt.Test) {
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

			switch1 := &ufspb.Switch{
				Name: "switch-20",
				Rack: "rack-20",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Create new switch - Permission denied: different realm", func(t *ftt.Test) {
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

			switch1 := &ufspb.Switch{
				Name: "switch-21",
				Rack: "rack-21",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.AtlLabAdminRealm)
			_, err = CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestUpdateSwitch(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateSwitch", t, func(t *ftt.Test) {
		t.Run("Update switch with non-existing switch", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-1",
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			switch1 := &ufspb.Switch{
				Name: "switch-1",
				Rack: "rack-1",
			}
			resp, err := UpdateSwitch(ctx, switch1, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update switch with new rack(same realm) - pass", func(t *ftt.Test) {
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

			switch3 := &ufspb.Switch{
				Name: "switch-3",
				Rack: "rack-3",
				// Needs to be manually set since we directly call
				// registration.CreateSwitch which doesn't pull the zone from
				// the rack.
				Zone: "ZONE_SFO36_BROWSER",
			}
			_, err = registration.CreateSwitch(ctx, switch3)
			assert.Loosely(t, err, should.BeNil)

			switch3.Rack = "rack-4"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateSwitch(ctx, switch3, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(switch3))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("switch.rack"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("rack-3"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("rack-4"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "switches/switch-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Update switch with same rack(same realm) - pass", func(t *ftt.Test) {
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

			switch1 := &ufspb.Switch{
				Name: "switch-5",
				Rack: "rack-5",
			}
			_, err = registration.CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateSwitch(ctx, switch1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(switch1))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-5")
			assert.Loosely(t, err, should.BeNil)
			// Nothing is changed for switch-5
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "switches/switch-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Update switch with non existing rack", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-6",
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			switch1 := &ufspb.Switch{
				Name: "switch-6",
				Rack: "rack-6",
			}
			_, err = registration.CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)

			switch1.Rack = "rack-61"
			resp, err := UpdateSwitch(ctx, switch1, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no Rack with RackID rack-61 in the system"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Partial Update switch", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-7",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			s := &ufspb.Switch{
				Name:         "switch-7",
				Rack:         "rack-7",
				CapacityPort: 10,
				Description:  "Hello Switch",
			}
			_, err = registration.CreateSwitch(ctx, s)
			assert.Loosely(t, err, should.BeNil)

			switch1 := &ufspb.Switch{
				Name:         "switch-7",
				CapacityPort: 44,
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateSwitch(ctx, switch1, &field_mask.FieldMask{Paths: []string{"capacity"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetDescription(), should.Match("Hello Switch"))
			assert.Loosely(t, resp.GetCapacityPort(), should.Equal(44))
		})

		t.Run("Update switch - Permission denied: same realm and no update permission", func(t *ftt.Test) {
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

			switch1 := &ufspb.Switch{
				Name: "switch-51",
				Rack: "rack-51",
			}
			_, err = registration.CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = UpdateSwitch(ctx, switch1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update switch - Permission denied: different realm", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-52",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				}}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			switch1 := &ufspb.Switch{
				Name: "switch-52",
				Rack: "rack-52",
			}
			_, err = registration.CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			_, err = UpdateSwitch(ctx, switch1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update switch with new rack(different realm with no permission)- fail", func(t *ftt.Test) {
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

			switch3 := &ufspb.Switch{
				Name: "switch-53",
				Rack: "rack-53",
			}
			_, err = registration.CreateSwitch(ctx, switch3)
			assert.Loosely(t, err, should.BeNil)

			switch3.Rack = "rack-54"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateSwitch(ctx, switch3, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update switch with new rack(different realm with permission)- pass", func(t *ftt.Test) {
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
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err = registration.CreateRack(ctx, rack4)
			assert.Loosely(t, err, should.BeNil)

			switch3 := &ufspb.Switch{
				Name: "switch-55",
				Rack: "rack-55",
				// Needs to be manually set since we directly call
				// registration.CreateSwitch which doesn't pull the zone from
				// the rack.
				Zone: "ZONE_SFO36_BROWSER",
			}
			_, err = registration.CreateSwitch(ctx, switch3)
			assert.Loosely(t, err, should.BeNil)

			switch3.Rack = "rack-56"
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.RegistrationsUpdate),
				),
			})
			resp, err := UpdateSwitch(ctx, switch3, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(switch3))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-55")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("switch.rack"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("rack-55"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("rack-56"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "switches/switch-55")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Partial Update switch with new rack(same realm) - pass", func(t *ftt.Test) {
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

			s := &ufspb.Switch{
				Name: "switch-57",
				Rack: "rack-57",
			}
			_, err = registration.CreateSwitch(ctx, s)
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
			resp, err := UpdateSwitch(ctx, s, &field_mask.FieldMask{Paths: []string{"rack"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetRack(), should.Match("rack-58"))
		})

		t.Run("Partial Update switch with new rack(different realm with permission) - pass", func(t *ftt.Test) {
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

			s := &ufspb.Switch{
				Name: "switch-59",
				Rack: "rack-59",
			}
			_, err = registration.CreateSwitch(ctx, s)
			assert.Loosely(t, err, should.BeNil)

			rack = &ufspb.Rack{
				Name: "rack-60",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
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
			resp, err := UpdateSwitch(ctx, s, &field_mask.FieldMask{Paths: []string{"rack"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetRack(), should.Match("rack-60"))
		})

		t.Run("Partial Update switch with new rack(different realm without permission) - fail", func(t *ftt.Test) {
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

			s := &ufspb.Switch{
				Name: "switch-61",
				Rack: "rack-61",
			}
			_, err = registration.CreateSwitch(ctx, s)
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
			_, err = UpdateSwitch(ctx, s, &field_mask.FieldMask{Paths: []string{"rack"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestDeleteSwitch(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteSwitch", t, func(t *ftt.Test) {
		t.Run("Delete switch by non-existing ID - error", func(t *ftt.Test) {
			err := DeleteSwitch(ctx, "switch-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Delete switch by existing ID with nic reference", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-5",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			switch1 := mockSwitch("switch-1")
			switch1.Rack = "rack-5"
			_, err = registration.CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)

			nic := &ufspb.Nic{
				Name: "machine1-eth0",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch: "switch-1",
				},
			}
			_, err = registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			err = DeleteSwitch(ctx, "switch-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Nics referring to the Switch:"))

			resp, err := registration.GetSwitch(ctx, "switch-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(switch1))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Delete switch successfully", func(t *ftt.Test) {
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

			switch2 := mockSwitch("switch-2")
			switch2.Rack = "rack-52"
			_, err = registration.CreateSwitch(ctx, switch2)
			assert.Loosely(t, err, should.BeNil)
			_, err = state.BatchUpdateStates(ctx, []*ufspb.StateRecord{
				{
					ResourceName: "switches/switch-2",
					State:        ufspb.State_STATE_SERVING,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.BrowserLabAdminRealm)
			err = DeleteSwitch(ctx, "switch-2")
			assert.Loosely(t, err, should.BeNil)

			resp, err := registration.GetSwitch(ctx, "switch-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = state.GetStateRecord(ctx, "switches/switch-2")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("switch"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "switches/switch-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
		})

		t.Run("Delete switch - Permission denied: same realm and no delete permission", func(t *ftt.Test) {
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

			switch2 := mockSwitch("switch-53")
			switch2.Rack = "rack-53"
			_, err = registration.CreateSwitch(ctx, switch2)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			err = DeleteSwitch(ctx, "switch-53")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Delete switch - Permission denied: different realm", func(t *ftt.Test) {
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

			switch2 := mockSwitch("switch-54")
			switch2.Rack = "rack-54"
			_, err = registration.CreateSwitch(ctx, switch2)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.AtlLabAdminRealm)
			err = DeleteSwitch(ctx, "switch-54")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestListSwitches(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	switches := make([]*ufspb.Switch, 0, 2)
	for i := 0; i < 4; i++ {
		Switch := mockSwitch(fmt.Sprintf("Switch-%d", i))
		resp, _ := registration.CreateSwitch(ctx, Switch)
		switches = append(switches, resp)
	}
	ftt.Run("ListSwitches", t, func(t *ftt.Test) {
		t.Run("List Switches - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListSwitches(ctx, 5, "", "invalid=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name invalid"))
		})

		t.Run("ListSwitches - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListSwitches(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(switches))
		})
	})
}

func TestBatchGetSwitches(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetSwitches", t, func(t *ftt.Test) {
		t.Run("Batch get switches - happy path", func(t *ftt.Test) {
			entities := make([]*ufspb.Switch, 4)
			for i := 0; i < 4; i++ {
				entities[i] = &ufspb.Switch{
					Name: fmt.Sprintf("switch-batchGet-%d", i),
				}
			}
			_, err := registration.BatchUpdateSwitches(ctx, entities)
			assert.Loosely(t, err, should.BeNil)
			resp, err := registration.BatchGetSwitches(ctx, []string{"switch-batchGet-0", "switch-batchGet-1", "switch-batchGet-2", "switch-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(entities))
		})
		t.Run("Batch get switches  - missing id", func(t *ftt.Test) {
			resp, err := registration.BatchGetSwitches(ctx, []string{"switch-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("switch-batchGet-non-existing"))
		})
		t.Run("Batch get switches  - empty input", func(t *ftt.Test) {
			resp, err := registration.BatchGetSwitches(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = registration.BatchGetSwitches(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}

func TestRenameSwitch(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	registration.CreateRack(ctx, &ufspb.Rack{
		Name: "rack-1",
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
		},
	})
	ftt.Run("RenameSwitch", t, func(t *ftt.Test) {
		t.Run("Rename a Switch with new switch name", func(t *ftt.Test) {
			_, err := registration.CreateNic(ctx, &ufspb.Nic{
				Name: "nic-1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch: "switch-1",
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateDrac(ctx, &ufspb.Drac{
				Name: "drac-1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch: "switch-1",
				},
			})
			assert.Loosely(t, err, should.BeNil)
			host := mockDutMachineLSE("machinelse-1")
			host.GetChromeosMachineLse().GetDeviceLse().NetworkDeviceInterface = &ufspb.SwitchInterface{
				Switch: "switch-1",
			}
			_, err = inventory.CreateMachineLSE(ctx, host)
			assert.Loosely(t, err, should.BeNil)

			switch2 := mockSwitch("switch-1")
			switch2.Rack = "rack-1"
			_, err = registration.CreateSwitch(ctx, switch2)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			res, err := RenameSwitch(ctx, "switch-1", "switch-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.Name, should.Equal("switch-2"))

			_, err = registration.GetSwitch(ctx, "switch-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			nic, err := registration.GetNic(ctx, "nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nic, should.NotBeNil)
			assert.Loosely(t, nic.GetSwitchInterface().GetSwitch(), should.Equal("switch-2"))
			drac, err := registration.GetDrac(ctx, "drac-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, drac, should.NotBeNil)
			assert.Loosely(t, drac.GetSwitchInterface().GetSwitch(), should.Equal("switch-2"))
			lse, err := inventory.GetMachineLSE(ctx, "machinelse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse, should.NotBeNil)
			assert.Loosely(t, lse.GetChromeosMachineLse().GetDeviceLse().GetNetworkDeviceInterface().GetSwitch(), should.Equal("switch-2"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "switches/switch-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("switch"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("switch-1"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("switch-2"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("switch.name"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "switches/switch-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("switch"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("switch-1"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("switch-2"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("switch.name"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "nics/nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("switch-1"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("switch-2"))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("switch_interface.switch"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("switch-1"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("switch-2"))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("switch_interface.switch"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("switch-1"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("switch-2"))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("switch_interface.switch"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "switches/switch-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "switches/switch-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "nics/nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dracs/drac-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/machinelse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
		t.Run("Rename a non-existing Switch", func(t *ftt.Test) {
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err := RenameSwitch(ctx, "switch-3", "switch-4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Rename a Switch to an already existing switch name", func(t *ftt.Test) {
			_, err := registration.CreateSwitch(ctx, &ufspb.Switch{
				Name: "switch-5",
				Rack: "rack-1",
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateSwitch(ctx, &ufspb.Switch{
				Name: "switch-6",
				Rack: "rack-1",
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = RenameSwitch(ctx, "switch-5", "switch-6")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Switch switch-6 already exists in the system"))
		})
		t.Run("Rename a Machine - permission denied: same realm and no update permission", func(t *ftt.Test) {
			_, err := registration.CreateSwitch(ctx, &ufspb.Switch{
				Name: "switch-7",
				Rack: "rack-1",
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = RenameSwitch(ctx, "switch-7", "switch-8")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
		t.Run("Rename a Switch - permission denied: different realm", func(t *ftt.Test) {
			_, err := registration.CreateSwitch(ctx, &ufspb.Switch{
				Name: "switch-9",
				Rack: "rack-1",
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			_, err = RenameSwitch(ctx, "switch-9", "switch-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}
