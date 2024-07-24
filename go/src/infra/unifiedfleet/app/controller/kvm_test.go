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
	"infra/unifiedfleet/app/model/configuration"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/history"
	"infra/unifiedfleet/app/model/registration"
	"infra/unifiedfleet/app/model/state"
	"infra/unifiedfleet/app/util"
)

func mockKVM(id string) *ufspb.KVM {
	return &ufspb.KVM{
		Name: id,
	}
}

func TestCreateKVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	rack1 := &ufspb.Rack{
		Name: "rack-1",
		Rack: &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
		},
	}
	registration.CreateRack(ctx, rack1)
	ftt.Run("CreateKVM", t, func(t *ftt.Test) {
		t.Run("Create new kvm with already existing kvm - error", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-5",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			registration.CreateRack(ctx, rack1)

			kvm1 := &ufspb.KVM{
				Name: "kvm-1",
				Rack: "rack-5",
			}
			_, err := registration.CreateKVM(ctx, kvm1)
			assert.Loosely(t, err, should.BeNil)

			resp, err := CreateKVM(ctx, kvm1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("KVM kvm-1 already exists in the system"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create new kvm with non existing chromePlatform", func(t *ftt.Test) {
			kvm2 := &ufspb.KVM{
				Name:           "kvm-2",
				ChromePlatform: "chromePlatform-1",
				Rack:           "rack-1",
			}
			resp, err := CreateKVM(ctx, kvm2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no ChromePlatform with ChromePlatformID chromePlatform-1 in the system"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create new kvm with existing resources", func(t *ftt.Test) {
			chromePlatform2 := &ufspb.ChromePlatform{
				Name: "chromePlatform-2",
			}
			_, err := configuration.CreateChromePlatform(ctx, chromePlatform2)
			assert.Loosely(t, err, should.BeNil)

			kvm2 := &ufspb.KVM{
				Name:           "kvm-2",
				ChromePlatform: "chromePlatform-2",
				Rack:           "rack-1",
			}
			resp, err := CreateKVM(ctx, kvm2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(kvm2))
			s, err := state.GetStateRecord(ctx, "kvms/kvm-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("kvm"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "kvms/kvm-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Create KVM - duplicated mac address", func(t *ftt.Test) {
			kvm := &ufspb.KVM{
				Name:       "kvm-2-mac",
				MacAddress: "kvm-2-address",
			}
			_, err := registration.CreateKVM(ctx, kvm)
			assert.Loosely(t, err, should.BeNil)
			kvm2 := &ufspb.KVM{
				Name:       "kvm-2-mac2",
				MacAddress: "kvm-2-address",
				Rack:       "rack-1",
			}
			_, err = CreateKVM(ctx, kvm2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("mac_address kvm-2-address is already occupied"))
		})

		t.Run("Create new kvm with existing rack", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-10",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			kvm1 := &ufspb.KVM{
				Name: "kvm-20",
				Rack: "rack-10",
			}
			resp, err := CreateKVM(ctx, kvm1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(kvm1))

			s, err := state.GetStateRecord(ctx, "kvms/kvm-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("kvm"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "kvms/kvm-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Create new kvm - Permission denied: same realm and no create permission", func(t *ftt.Test) {
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

			kvm1 := &ufspb.KVM{
				Name: "kvm-20",
				Rack: "rack-20",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = CreateKVM(ctx, kvm1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Create new kvm - Permission denied: different realm", func(t *ftt.Test) {
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

			kvm1 := &ufspb.KVM{
				Name: "kvm-21",
				Rack: "rack-21",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.AtlLabAdminRealm)
			_, err = CreateKVM(ctx, kvm1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestUpdateKVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateKVM", t, func(t *ftt.Test) {
		t.Run("Update kvm with non-existing kvm", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-1",
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			kvm1 := &ufspb.KVM{
				Name: "kvm-1",
				Rack: "rack-1",
			}
			resp, err := UpdateKVM(ctx, kvm1, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update kvm with new rack(same realm) - pass", func(t *ftt.Test) {
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

			kvm3 := &ufspb.KVM{
				Name: "kvm-3",
				Rack: "rack-3",
				// Needs to be manually set since we directly call
				// registration.CreateKVM which doesn't pull the zone from
				// the rack.
				Zone: "ZONE_SFO36_BROWSER",
			}
			_, err = registration.CreateKVM(ctx, kvm3)
			assert.Loosely(t, err, should.BeNil)

			kvm3.Rack = "rack-4"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateKVM(ctx, kvm3, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(kvm3))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("kvm.rack"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("rack-3"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("rack-4"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "kvms/kvm-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Update kvm with same rack(same realm) - pass", func(t *ftt.Test) {
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

			kvm1 := &ufspb.KVM{
				Name:       "kvm-5",
				Rack:       "rack-5",
				MacAddress: "kvm-10-address",
			}
			_, err = registration.CreateKVM(ctx, kvm1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateKVM(ctx, kvm1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(kvm1))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-5")
			assert.Loosely(t, err, should.BeNil)
			// Nothing is changed for kvm-5
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "kvms/kvm-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Update kvm with non existing rack", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-6.1",
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			kvm1 := &ufspb.KVM{
				Name: "kvm-6",
				Rack: "rack-6.1",
			}
			_, err = registration.CreateKVM(ctx, kvm1)
			assert.Loosely(t, err, should.BeNil)

			kvm1.Rack = "rack-6"
			resp, err := UpdateKVM(ctx, kvm1, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no Rack with RackID rack-6 in the system"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Partial Update kvm", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-7",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_BROWSER,
				},
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			kvm := &ufspb.KVM{
				Name:           "kvm-7",
				Rack:           "rack-7",
				ChromePlatform: "chromePlatform-7",
				Tags:           []string{"testkvm"},
			}
			_, err = registration.CreateKVM(ctx, kvm)
			assert.Loosely(t, err, should.BeNil)

			chromePlatform := &ufspb.ChromePlatform{
				Name: "chromePlatform-8",
			}
			_, err = configuration.CreateChromePlatform(ctx, chromePlatform)
			assert.Loosely(t, err, should.BeNil)

			kvm1 := &ufspb.KVM{
				Name:           "kvm-7",
				MacAddress:     "efgh",
				ChromePlatform: "chromePlatform-8",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateKVM(ctx, kvm1, &field_mask.FieldMask{Paths: []string{"platform", "macAddress"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetChromePlatform(), should.Match("chromePlatform-8"))
			assert.Loosely(t, resp.GetMacAddress(), should.Match("efgh"))
			assert.Loosely(t, resp.GetTags(), should.Resemble([]string{"testkvm"}))
		})

		t.Run("Partial Update kvm mac address - duplicated mac address", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-8",
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			kvm := &ufspb.KVM{
				Name:       "kvm-8",
				Rack:       "rack-8",
				MacAddress: "kvm-8-address",
			}
			_, err = registration.CreateKVM(ctx, kvm)
			assert.Loosely(t, err, should.BeNil)

			kvm2 := &ufspb.KVM{
				Name:       "kvm-8.2",
				Rack:       "rack-8",
				MacAddress: "kvm-8.2-address",
			}
			_, err = registration.CreateKVM(ctx, kvm2)
			assert.Loosely(t, err, should.BeNil)

			kvm1 := &ufspb.KVM{
				Name:       "kvm-8",
				MacAddress: "kvm-8.2-address",
			}
			_, err = UpdateKVM(ctx, kvm1, &field_mask.FieldMask{Paths: []string{"macAddress"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("mac_address kvm-8.2-address is already occupied"))
		})

		t.Run("Update kvm mac address - duplicated mac address", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-9",
			}
			_, err := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)

			kvm := &ufspb.KVM{
				Name:       "kvm-9",
				Rack:       "rack-9",
				MacAddress: "kvm-9-address",
			}
			_, err = registration.CreateKVM(ctx, kvm)
			assert.Loosely(t, err, should.BeNil)
			kvm2 := &ufspb.KVM{
				Name:       "kvm-9.2",
				Rack:       "rack-9",
				MacAddress: "kvm-9.2-address",
			}
			_, err = registration.CreateKVM(ctx, kvm2)
			assert.Loosely(t, err, should.BeNil)

			kvm1 := &ufspb.KVM{
				Name:       "kvm-9",
				MacAddress: "kvm-9.2-address",
			}
			_, err = UpdateKVM(ctx, kvm1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("mac_address kvm-9.2-address is already occupied"))
		})

		t.Run("Update kvm - Permission denied: same realm and no update permission", func(t *ftt.Test) {
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

			kvm1 := &ufspb.KVM{
				Name: "kvm-51",
				Rack: "rack-51",
			}
			_, err = registration.CreateKVM(ctx, kvm1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = UpdateKVM(ctx, kvm1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update kvm - Permission denied: different realm", func(t *ftt.Test) {
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

			kvm1 := &ufspb.KVM{
				Name: "kvm-52",
				Rack: "rack-52",
			}
			_, err = registration.CreateKVM(ctx, kvm1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			_, err = UpdateKVM(ctx, kvm1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update kvm with new rack(different realm with no permission)- fail", func(t *ftt.Test) {
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
				}}
			_, err = registration.CreateRack(ctx, rack4)
			assert.Loosely(t, err, should.BeNil)

			kvm3 := &ufspb.KVM{
				Name: "kvm-53",
				Rack: "rack-53",
			}
			_, err = registration.CreateKVM(ctx, kvm3)
			assert.Loosely(t, err, should.BeNil)

			kvm3.Rack = "rack-54"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateKVM(ctx, kvm3, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update kvm with new rack(different realm with permission)- pass", func(t *ftt.Test) {
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

			kvm3 := &ufspb.KVM{
				Name: "kvm-55",
				Rack: "rack-55",
				// Needs to be manually set since we directly call
				// registration.CreateRPM which doesn't pull the zone from
				// the rack.
				Zone: "ZONE_SFO36_BROWSER",
			}
			_, err = registration.CreateKVM(ctx, kvm3)
			assert.Loosely(t, err, should.BeNil)

			kvm3.Rack = "rack-56"
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.RegistrationsUpdate),
				),
			})
			resp, err := UpdateKVM(ctx, kvm3, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(kvm3))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-55")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("kvm.zone"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("ZONE_SFO36_BROWSER"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("ZONE_CHROMEOS4"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("kvm.rack"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("rack-55"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("rack-56"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "kvms/kvm-55")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Partial Update kvm with new rack(same realm) - pass", func(t *ftt.Test) {
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

			s := &ufspb.KVM{
				Name: "kvm-57",
				Rack: "rack-57",
			}
			_, err = registration.CreateKVM(ctx, s)
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
			resp, err := UpdateKVM(ctx, s, &field_mask.FieldMask{Paths: []string{"rack"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetRack(), should.Match("rack-58"))
		})

		t.Run("Partial Update kvm with new rack(different realm with permission) - pass", func(t *ftt.Test) {
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

			s := &ufspb.KVM{
				Name: "kvm-59",
				Rack: "rack-59",
			}
			_, err = registration.CreateKVM(ctx, s)
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
			resp, err := UpdateKVM(ctx, s, &field_mask.FieldMask{Paths: []string{"rack"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetRack(), should.Match("rack-60"))
		})

		t.Run("Partial Update kvm with new rack(different realm without permission) - fail", func(t *ftt.Test) {
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

			s := &ufspb.KVM{
				Name: "kvm-61",
				Rack: "rack-61",
			}
			_, err = registration.CreateKVM(ctx, s)
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
			_, err = UpdateKVM(ctx, s, &field_mask.FieldMask{Paths: []string{"rack"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

	})
}

func TestDeleteKVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteKVM", t, func(t *ftt.Test) {
		t.Run("Delete kvm by non-existing ID - error", func(t *ftt.Test) {
			err := DeleteKVM(ctx, "kvm-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Delete KVM by existing ID with machine reference", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-1",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			registration.CreateRack(ctx, rack1)

			KVM1 := &ufspb.KVM{
				Name: "KVM-1",
				Rack: "rack-1",
			}
			_, err := registration.CreateKVM(ctx, KVM1)
			assert.Loosely(t, err, should.BeNil)

			chromeBrowserMachine1 := &ufspb.Machine{
				Name: "machine-1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						KvmInterface: &ufspb.KVMInterface{
							Kvm: "KVM-1",
						},
					},
				},
			}
			_, err = registration.CreateMachine(ctx, chromeBrowserMachine1)
			assert.Loosely(t, err, should.BeNil)

			err = DeleteKVM(ctx, "KVM-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Machines referring the KVM:"))

			resp, err := registration.GetKVM(ctx, "KVM-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(KVM1))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/KVM-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Delete KVM successfully", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-2",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			registration.CreateRack(ctx, rack1)

			kvm2 := mockKVM("kvm-2")
			kvm2.Rack = "rack-2"
			_, err := registration.CreateKVM(ctx, kvm2)
			assert.Loosely(t, err, should.BeNil)
			_, err = state.BatchUpdateStates(ctx, []*ufspb.StateRecord{
				{
					ResourceName: "kvms/kvm-2",
					State:        ufspb.State_STATE_SERVING,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteKVM(ctx, "kvm-2")
			assert.Loosely(t, err, should.BeNil)

			resp, err := registration.GetKVM(ctx, "kvm-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = state.GetStateRecord(ctx, "kvms/kvm-2")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("kvm"))
		})

		t.Run("Delete KVM successfully together with deleting ip", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-ip2",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			registration.CreateRack(ctx, rack1)

			kvm2 := mockKVM("kvm-ip2")
			kvm2.Rack = "rack-ip2"
			_, err := registration.CreateKVM(ctx, kvm2)
			assert.Loosely(t, err, should.BeNil)
			_, err = state.BatchUpdateStates(ctx, []*ufspb.StateRecord{
				{
					ResourceName: "kvms/kvm-ip2",
					State:        ufspb.State_STATE_SERVING,
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateDHCPs(ctx, []*ufspb.DHCPConfig{
				{
					Hostname: "kvm-ip2",
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

			err = DeleteKVM(ctx, "kvm-ip2")
			assert.Loosely(t, err, should.BeNil)
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "1.2.3.4"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeFalse)
			_, err = configuration.GetDHCPConfig(ctx, "kvm-ip2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			resp, err := registration.GetKVM(ctx, "kvm-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = state.GetStateRecord(ctx, "kvms/kvm-2")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "kvms/kvm-ip2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("kvm"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "kvms/kvm-ip2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/kvm-ip2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/kvms/kvm-ip2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
		})

		t.Run("Delete kvm - Permission denied: same realm and no delete permission", func(t *ftt.Test) {
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

			kvm2 := mockKVM("kvm-53")
			kvm2.Rack = "rack-53"
			_, err = registration.CreateKVM(ctx, kvm2)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			err = DeleteKVM(ctx, "kvm-53")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Delete kvm - Permission denied: different realm", func(t *ftt.Test) {
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

			kvm2 := mockKVM("kvm-54")
			kvm2.Rack = "rack-54"
			_, err = registration.CreateKVM(ctx, kvm2)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.AtlLabAdminRealm)
			err = DeleteKVM(ctx, "kvm-54")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestListKVMs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	kvmsWithChromeplatform := make([]*ufspb.KVM, 0, 2)
	kvms := make([]*ufspb.KVM, 0, 4)
	for i := 0; i < 4; i++ {
		kvm := mockKVM(fmt.Sprintf("kvm-%d", i))
		if i%2 == 0 {
			kvm.ChromePlatform = "chromeplatform-12"
		}
		resp, _ := registration.CreateKVM(ctx, kvm)
		if i%2 == 0 {
			kvmsWithChromeplatform = append(kvmsWithChromeplatform, resp)
		}
		kvms = append(kvms, resp)
	}
	ftt.Run("ListKVMs", t, func(t *ftt.Test) {
		t.Run("List KVMs - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListKVMs(ctx, 5, "", "invalid=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name invalid"))
		})

		t.Run("List KVMs - filter chromeplatform - happy path", func(t *ftt.Test) {
			resp, _, _ := ListKVMs(ctx, 5, "", "platform=chromeplatform-12", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(kvmsWithChromeplatform))
		})

		t.Run("ListKVMs - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListKVMs(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(kvms))
		})
	})
}

func TestBatchGetKVMs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetKVMs", t, func(t *ftt.Test) {
		t.Run("Batch get kvms - happy path", func(t *ftt.Test) {
			kvms := make([]*ufspb.KVM, 0, 4)
			for i := 0; i < 4; i++ {
				kvm := mockKVM(fmt.Sprintf("kvm-batchGet-%d", i))
				resp, err := registration.CreateKVM(ctx, kvm)
				assert.Loosely(t, err, should.BeNil)
				kvms = append(kvms, resp)
			}
			resp, err := registration.BatchGetKVM(ctx, []string{"kvm-batchGet-0", "kvm-batchGet-1", "kvm-batchGet-2", "kvm-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Resemble(kvms))
		})
		t.Run("Batch get kvms - missing id", func(t *ftt.Test) {
			resp, err := registration.BatchGetKVM(ctx, []string{"kvm-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("kvm-batchGet-non-existing"))
		})
		t.Run("Batch get kvms - empty input", func(t *ftt.Test) {
			resp, err := registration.BatchGetKVM(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = registration.BatchGetKVM(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}

func TestUpdateIndexInKVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	registration.CreateKVM(ctx, &ufspb.KVM{
		Name:       "kvm-update-index",
		MacAddress: "kvm-update-index-address",
		Rack:       "index-rack-old",
	})
	ftt.Run("Testing updateIndexInKVM", t, func(t *ftt.Test) {
		t.Run("updateIndexInKVM - update index rack", func(t *ftt.Test) {
			err := updateIndexInKVM(ctx, "rack", "index-rack-old", "index-rack-new", &HistoryClient{})
			assert.Loosely(t, err, should.BeNil)
			kvm, err := registration.GetKVM(ctx, "kvm-update-index")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, kvm.GetRack(), should.Equal("index-rack-new"))
		})
	})
}
