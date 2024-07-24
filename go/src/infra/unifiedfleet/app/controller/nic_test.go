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
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/model/registration"
	"infra/unifiedfleet/app/util"
)

func mockNic(id string) *ufspb.Nic {
	return &ufspb.Nic{
		Name: id,
	}
}

func TestCreateNic(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	machine1 := &ufspb.Machine{
		Name: "machine-1",
		Device: &ufspb.Machine_ChromeBrowserMachine{
			ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
		},
	}
	registration.CreateMachine(ctx, machine1)
	ftt.Run("CreateNics", t, func(t *ftt.Test) {
		t.Run("Create new nic with non existing machine", func(t *ftt.Test) {
			nic1 := &ufspb.Nic{
				Name:    "nic-1",
				Machine: "machine-5",
			}
			resp, err := CreateNic(ctx, nic1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create nic - duplicated switch ports", func(t *ftt.Test) {
			nic := &ufspb.Nic{
				Name: "nic-create-1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "nic-create-switch-1",
					PortName: "25",
				},
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)
			switch1 := &ufspb.Switch{
				Name: "nic-create-switch-1",
			}
			_, err = registration.CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)

			nic2 := &ufspb.Nic{
				Name:    "nic-create-2",
				Machine: "machine-1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "nic-create-switch-1",
					PortName: "25",
				},
			}
			_, err = CreateNic(ctx, nic2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("switch port 25 of nic-create-switch-1 is already occupied"))
		})

		t.Run("Create new nic with existing machine with nics", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-10",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			nic := &ufspb.Nic{
				Name:    "nic-20",
				Machine: "machine-10",
			}
			resp, err := CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(nic))

			mresp, err := GetMachine(ctx, "machine-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, mresp.GetChromeBrowserMachine().GetNicObjects()[0].GetName(), should.Match("nic-20"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/nic-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("nic"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/machine-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "nics/nic-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machines/machine-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("Create new nic with existing machine without nics", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-15",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			nic := &ufspb.Nic{
				Name:    "nic-25",
				Machine: "machine-15",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.BrowserLabAdminRealm)
			resp, err := CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(nic))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/nic-25")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("nic"))
		})

		t.Run("Create new nic with non existing switch", func(t *ftt.Test) {
			nic1 := &ufspb.Nic{
				Name:    "nic-1",
				Machine: "machine-1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch: "switch-1",
				},
			}
			resp, err := CreateNic(ctx, nic1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no Switch with SwitchID switch-1"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "nics/nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("Create new nic with existing switch", func(t *ftt.Test) {
			switch2 := &ufspb.Switch{
				Name: "switch-2",
			}
			_, err := registration.CreateSwitch(ctx, switch2)
			assert.Loosely(t, err, should.BeNil)

			nic2 := &ufspb.Nic{
				Name:    "nic-2",
				Machine: "machine-1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch: "switch-2",
				},
			}
			resp, err := CreateNic(ctx, nic2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(nic2))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/nic-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("nic"))
		})

		t.Run("Create new nic - permission denied: same realm and no create permission", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-16",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			nic := &ufspb.Nic{
				Name:    "nic-26",
				Machine: "machine-16",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = CreateNic(ctx, nic)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Create new nic - permission denied: different realm", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-17",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			nic := &ufspb.Nic{
				Name:    "nic-27",
				Machine: "machine-17",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.AtlLabAdminRealm)
			_, err = CreateNic(ctx, nic)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestUpdateNic(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateNics", t, func(t *ftt.Test) {
		t.Run("Update nic with non-existing nic", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-1",
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:    "nic-1",
				Machine: "machine-1",
			}
			resp, err := UpdateNic(ctx, nic, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update nic with non existing switch", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-2",
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:    "nic-2",
				Machine: "machine-2",
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			nic2 := &ufspb.Nic{
				Name:    "nic-2",
				Machine: "machine-2",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch: "switch-1",
				},
			}
			resp, err := UpdateNic(ctx, nic2, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("here is no Switch with SwitchID switch-1"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/nic-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update nic with new machine(same realm) - success", func(t *ftt.Test) {
			machine3 := &ufspb.Machine{
				Name: "machine-3",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err := registration.CreateMachine(ctx, machine3)
			assert.Loosely(t, err, should.BeNil)

			machine4 := &ufspb.Machine{
				Name: "machine-4",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err = registration.CreateMachine(ctx, machine4)
			assert.Loosely(t, err, should.BeNil)

			nic := &ufspb.Nic{
				Name:    "nic-3",
				Machine: "machine-3",
			}
			_, err = registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			nic = &ufspb.Nic{
				Name:    "nic-3",
				Machine: "machine-4",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateNic(ctx, nic, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(nic))

			// Verify the changes
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/nic-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
		})

		t.Run("Update nic with non existing machine", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-5",
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:    "nic-6",
				Machine: "machine-5",
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			nic = &ufspb.Nic{
				Name:    "nic-6",
				Machine: "machine-6",
			}
			resp, err := UpdateNic(ctx, nic, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no Machine with MachineID machine-6 in the system"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/nic-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Partial Update nic", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-7.1",
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:    "nic-7",
				Machine: "machine-7.1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-7",
					PortName: "25",
				},
			}
			resp, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Resemble(ufspb.State_STATE_UNSPECIFIED))

			nic1 := &ufspb.Nic{
				Name:       "nic-7",
				MacAddress: "efgh",
				SwitchInterface: &ufspb.SwitchInterface{
					PortName: "75",
				},
				ResourceState: ufspb.State_STATE_NEEDS_REPAIR,
			}
			resp, err = UpdateNic(ctx, nic1, &field_mask.FieldMask{Paths: []string{"portName", "macAddress", "resourceState"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetMachine(), should.Match("machine-7.1"))
			assert.Loosely(t, resp.GetSwitchInterface().GetSwitch(), should.Match("switch-7"))
			assert.Loosely(t, resp.GetSwitchInterface().GetPortName(), should.Equal("75"))
			assert.Loosely(t, resp.GetMacAddress(), should.Match("efgh"))
			assert.Loosely(t, resp.GetResourceState(), should.Resemble(ufspb.State_STATE_NEEDS_REPAIR))
		})

		t.Run("Partial update - nic state", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-7.2",
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:    "nic-7.2",
				Machine: "machine-7.2",
				SwitchInterface: &ufspb.SwitchInterface{
					PortName: "25",
				},
			}
			resp, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Resemble(ufspb.State_STATE_UNSPECIFIED))

			nic1 := &ufspb.Nic{
				Name:          "nic-7.2",
				ResourceState: ufspb.State_STATE_NEEDS_REPAIR,
				SwitchInterface: &ufspb.SwitchInterface{
					PortName: "25",
				},
			}
			resp, err = UpdateNic(ctx, nic1, &field_mask.FieldMask{Paths: []string{"portName", "macAddress", "resourceState"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetMachine(), should.Match("machine-7.2"))
			assert.Loosely(t, resp.GetResourceState(), should.Resemble(ufspb.State_STATE_NEEDS_REPAIR))
		})

		t.Run("Partial Update nic mac address and machine(same realm) - succeed", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-8",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:       "nic-8",
				Machine:    "machine-8",
				MacAddress: "abcd",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-8",
					PortName: "25",
				},
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			lse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "lse-partial-update-mac",
				Hostname: "lse-partial-update-mac",
				Machines: []string{"machine-8"},
				Nic:      "nic-8",
			})
			assert.Loosely(t, lse, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "lse-partial-update-mac2",
				Hostname: "lse-partial-update-mac2",
				Machines: []string{"machine-8-8"},
			})
			assert.Loosely(t, err, should.BeNil)
			dhcp := &ufspb.DHCPConfig{
				Hostname:   lse.GetName(),
				Ip:         "fake_ip",
				Vlan:       "fake_vlan",
				MacAddress: nic.GetMacAddress(),
			}
			_, err = configuration.BatchUpdateDHCPs(ctx, []*ufspb.DHCPConfig{dhcp})
			assert.Loosely(t, err, should.BeNil)

			machine1 = &ufspb.Machine{
				Name: "machine-8-8",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			registration.CreateMachine(ctx, machine1)

			nic1 := &ufspb.Nic{
				Name:       "nic-8",
				Machine:    "machine-8-8",
				MacAddress: "nic-8-address",
			}
			ctx := initializeMockAuthDB(ctx, "user:user@example.com", util.BrowserLabAdminRealm, util.RegistrationsUpdate, util.InventoriesUpdate)
			nic, err = UpdateNic(ctx, nic1, &field_mask.FieldMask{Paths: []string{"macAddress", "machine"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nic.GetMacAddress(), should.Equal("nic-8-address"))
			// new machine's corresponding host has new dhcp record, new nic
			lse, err = inventory.GetMachineLSE(ctx, "lse-partial-update-mac2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse.GetNic(), should.Equal("nic-8"))
			dhcp, err = configuration.GetDHCPConfig(ctx, "lse-partial-update-mac2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dhcp.GetMacAddress(), should.Equal("nic-8-address"))
			// old machine's corresponding host has empty dhcp record, empty nic
			dhcp, err = configuration.GetDHCPConfig(ctx, "lse-partial-update-mac")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, dhcp, should.BeNil)
			lse, err = inventory.GetMachineLSE(ctx, "lse-partial-update-mac")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse.GetNic(), should.BeEmpty)

			// verify change events
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/lse-partial-update-mac")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.nic"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("nic-8"))
			assert.Loosely(t, changes[0].GetNewValue(), should.BeEmpty)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/lse-partial-update-mac2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.nic"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("nic-8"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/lse-partial-update-mac")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("fake_ip"))
			assert.Loosely(t, changes[0].GetNewValue(), should.BeEmpty)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/lse-partial-update-mac2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("fake_ip"))
		})

		t.Run("Partial Update nic mac address - duplicated mac address", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-8.1",
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:       "nic-8.1",
				Machine:    "machine-8.1",
				MacAddress: "nic-8.1-address",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-nic-8.1",
					PortName: "25",
				},
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)
			nic2 := &ufspb.Nic{
				Name:       "nic-8.2",
				MacAddress: "nic-8.2-address",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-nic-8.1",
					PortName: "26",
				},
			}
			_, err = registration.CreateNic(ctx, nic2)
			assert.Loosely(t, err, should.BeNil)

			nic1 := &ufspb.Nic{
				Name:       "nic-8.1",
				MacAddress: "nic-8.2-address",
			}
			_, err = UpdateNic(ctx, nic1, &field_mask.FieldMask{Paths: []string{"macAddress"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("mac_address nic-8.2-address is already occupied"))
		})

		t.Run("Partial Update nic mac address - no update at all", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-9",
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:       "nic-9",
				Machine:    "machine-9",
				MacAddress: "nic-9-address",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "nic-switch-9",
					PortName: "25",
				},
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			nic1 := &ufspb.Nic{
				Name:       "nic-9",
				MacAddress: "nic-9-address",
			}
			_, err = UpdateNic(ctx, nic1, &field_mask.FieldMask{Paths: []string{"macAddress"}})
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Update nic mac address - duplicated mac address", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-10",
			}
			registration.CreateMachine(ctx, machine1)
			nic := &ufspb.Nic{
				Name:       "nic-full-update",
				Machine:    "machine-10",
				MacAddress: "nic-full-update-address",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-nic-full",
					PortName: "25",
				},
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			machine2 := &ufspb.Machine{
				Name: "machine-11",
			}
			registration.CreateMachine(ctx, machine2)
			nic2 := &ufspb.Nic{
				Name:       "nic-full-update2",
				Machine:    "machine-11",
				MacAddress: "nic-full-update-address2",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-nic-full",
					PortName: "26",
				},
			}
			_, err = registration.CreateNic(ctx, nic2)
			assert.Loosely(t, err, should.BeNil)

			nic1 := &ufspb.Nic{
				Name:       "nic-full-update",
				MacAddress: "nic-full-update-address2",
			}
			_, err = UpdateNic(ctx, nic1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("mac_address nic-full-update-address2 is already occupied"))
		})

		t.Run("Update nic mac address - happy path", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-7",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			nic := &ufspb.Nic{
				Name:       "nic-10",
				Machine:    "machine-7",
				MacAddress: "nic-10-address-old",
			}
			_, err = registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			lse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "lse-update-mac",
				Hostname: "lse-update-mac",
				Machines: []string{"machine-7"},
				Nic:      "nic-10",
			})
			assert.Loosely(t, err, should.BeNil)
			dhcp := &ufspb.DHCPConfig{
				Hostname:   lse.GetName(),
				Ip:         "fake_ip",
				Vlan:       "fake_vlan",
				MacAddress: nic.GetMacAddress(),
			}
			_, err = configuration.BatchUpdateDHCPs(ctx, []*ufspb.DHCPConfig{dhcp})
			assert.Loosely(t, err, should.BeNil)

			nic1 := &ufspb.Nic{
				Name:       "nic-10",
				Machine:    "machine-7",
				MacAddress: "nic-10-address",
			}
			res, _ := UpdateNic(ctx, nic1, nil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Resemble(nic1))

			// new machine's corresponding host has new dhcp record
			dhcp, err = configuration.GetDHCPConfig(ctx, "lse-update-mac")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dhcp.GetMacAddress(), should.Equal("nic-10-address"))
			// no change event for lse as nic name is not changed
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/lse-update-mac")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			// verify dhcp change events
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/lse-update-mac")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dhcp_config.mac_address"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("nic-10-address-old"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("nic-10-address"))
		})

		t.Run("Update nic - permission denied: same realm and no update permission", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-12",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:    "nic-12",
				Machine: "machine-12",
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			nic2 := &ufspb.Nic{
				Name:       "nic-12",
				Machine:    "machine-12",
				MacAddress: "nic-12-address",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.AtlLabAdminRealm)
			_, err = UpdateNic(ctx, nic2, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update nic - permission denied: different realm", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-13",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_OS_CHROMIUM,
				},
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:    "nic-13",
				Machine: "machine-13",
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			nic2 := &ufspb.Nic{
				Name:       "nic-13",
				Machine:    "machine-13",
				MacAddress: "nic-13-address",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateNic(ctx, nic2, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update nic with new machine(different realm with no permission) - Permission denied", func(t *ftt.Test) {
			machine3 := &ufspb.Machine{
				Name: "machine-14",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err := registration.CreateMachine(ctx, machine3)
			assert.Loosely(t, err, should.BeNil)

			machine4 := &ufspb.Machine{
				Name: "machine-15",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_OS_CHROMIUM,
				},
			}
			_, err = registration.CreateMachine(ctx, machine4)
			assert.Loosely(t, err, should.BeNil)

			nic := &ufspb.Nic{
				Name:    "nic-14",
				Machine: "machine-14",
			}
			_, err = registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			nic = &ufspb.Nic{
				Name:    "nic-14",
				Machine: "machine-15",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateNic(ctx, nic, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Partial Update nic with new machine(different realm with no permission) - Permission denied", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-16",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:    "nic-16",
				Machine: "machine-16",
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			machine2 := &ufspb.Machine{
				Name: "machine-17",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_SFO36_OS_CHROMIUM,
				},
			}
			registration.CreateMachine(ctx, machine2)

			nic1 := &ufspb.Nic{
				Name:    "nic-16",
				Machine: "machine-17",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateNic(ctx, nic1, &field_mask.FieldMask{Paths: []string{"machine"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update nic with new machine(different realm with permission) - Pass", func(t *ftt.Test) {
			machine3 := &ufspb.Machine{
				Name: "machine-18",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err := registration.CreateMachine(ctx, machine3)
			assert.Loosely(t, err, should.BeNil)

			machine4 := &ufspb.Machine{
				Name: "machine-19",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err = registration.CreateMachine(ctx, machine4)
			assert.Loosely(t, err, should.BeNil)

			nic := &ufspb.Nic{
				Name:    "nic-18",
				Machine: "machine-18",
			}
			_, err = registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			nic.Machine = "machine-19"
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.RegistrationsUpdate),
				),
			})
			resp, err := UpdateNic(ctx, nic, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(nic))
		})

		t.Run("Partial Update nic with new machine(different realm with permission) - Pass", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-20",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			registration.CreateMachine(ctx, machine1)

			nic := &ufspb.Nic{
				Name:    "nic-20",
				Machine: "machine-20",
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			machine2 := &ufspb.Machine{
				Name: "machine-21",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			registration.CreateMachine(ctx, machine2)

			nic.Machine = "machine-21"
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.RegistrationsUpdate),
				),
			})
			resp, err := UpdateNic(ctx, nic, &field_mask.FieldMask{Paths: []string{"machine"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetMachine(), should.Equal("machine-21"))
		})
	})
}

func TestDeleteNic(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteNic", t, func(t *ftt.Test) {
		t.Run("Delete nic error by non-existing ID", func(t *ftt.Test) {
			err := DeleteNic(ctx, "nic-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/nic-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Delete nic successfully by existing ID", func(t *ftt.Test) {
			nic := mockNic("nic-1")
			machine1 := &ufspb.Machine{
				Name: "machine-1",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			registration.CreateMachine(ctx, machine1)
			nic.Machine = "machine-1"
			resp, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(nic))

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.AtlLabAdminRealm)
			err = DeleteNic(ctx, "nic-1")
			assert.Loosely(t, err, should.BeNil)

			resp, err = registration.GetNic(ctx, "nic-1")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("nic"))
		})

		t.Run("Delete nic error as it's used by a host", func(t *ftt.Test) {
			nic := mockNic("nic-ip")
			nic.Machine = "machine-ip"
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			chromeBrowserMachine1 := &ufspb.Machine{
				Name: "machine-ip",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err = registration.CreateMachine(ctx, chromeBrowserMachine1)
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "lse-ip",
				Hostname: "lse-ip",
				Machines: []string{"machine-ip"},
				Nic:      "nic-ip",
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteNic(ctx, "nic-ip")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("nic nic-ip is used by host lse-ip"))

			resp, err := registration.GetNic(ctx, "nic-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(nic))
		})

		t.Run("Delete nic - permission denied: same realm and no delete permission", func(t *ftt.Test) {
			nic := mockNic("nic-3")
			machine1 := &ufspb.Machine{
				Name: "machine-3",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			registration.CreateMachine(ctx, machine1)
			nic.Machine = "machine-3"
			resp, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(nic))

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.AtlLabAdminRealm)
			err = DeleteNic(ctx, "nic-3")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Delete nic - permission denied: different realm", func(t *ftt.Test) {
			nic := mockNic("nic-4")
			machine1 := &ufspb.Machine{
				Name: "machine-4",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			registration.CreateMachine(ctx, machine1)
			nic.Machine = "machine-4"
			resp, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(nic))

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.BrowserLabAdminRealm)
			err = DeleteNic(ctx, "nic-4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestListNics(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	nicsWithSwitch := make([]*ufspb.Nic, 0, 2)
	nics := make([]*ufspb.Nic, 0, 4)
	for i := 0; i < 4; i++ {
		nic := mockNic(fmt.Sprintf("nic-%d", i))
		if i%2 == 0 {
			nic.SwitchInterface = &ufspb.SwitchInterface{Switch: "switch-12"}
		}
		resp, _ := registration.CreateNic(ctx, nic)
		if i%2 == 0 {
			nicsWithSwitch = append(nicsWithSwitch, resp)
		}
		nics = append(nics, resp)
	}
	ftt.Run("ListNics", t, func(t *ftt.Test) {
		t.Run("List Nics - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListNics(ctx, 5, "", "invalid=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name invalid"))
		})

		t.Run("List Nics - filter switch - happy path", func(t *ftt.Test) {
			resp, _, _ := ListNics(ctx, 5, "", "switch=switch-12", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(nicsWithSwitch))
		})

		t.Run("ListNics - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListNics(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(nics))
		})
	})
}

func TestBatchGetNics(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetNics", t, func(t *ftt.Test) {
		t.Run("Batch get nics - happy path", func(t *ftt.Test) {
			entities := make([]*ufspb.Nic, 4)
			for i := 0; i < 4; i++ {
				entities[i] = &ufspb.Nic{
					Name: fmt.Sprintf("nic-batchGet-%d", i),
				}
			}
			_, err := registration.BatchUpdateNics(ctx, entities)
			assert.Loosely(t, err, should.BeNil)
			resp, err := registration.BatchGetNics(ctx, []string{"nic-batchGet-0", "nic-batchGet-1", "nic-batchGet-2", "nic-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Resemble(entities))
		})
		t.Run("Batch get nics  - missing id", func(t *ftt.Test) {
			resp, err := registration.BatchGetNics(ctx, []string{"nic-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("nic-batchGet-non-existing"))
		})
		t.Run("Batch get nics  - empty input", func(t *ftt.Test) {
			resp, err := registration.BatchGetNics(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = registration.BatchGetNics(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}

func TestRenameNic(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	registration.CreateMachine(ctx, &ufspb.Machine{
		Name: "machine-ren-1",
		Device: &ufspb.Machine_ChromeBrowserMachine{
			ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
		},
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_ATL97,
		},
	})
	ftt.Run("RenameNic", t, func(t *ftt.Test) {

		t.Run("Rename a Nic with new nic name", func(t *ftt.Test) {
			_, err := registration.CreateNic(ctx, &ufspb.Nic{
				Name:    "machine-ren-1:nic-1",
				Machine: "machine-ren-1",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-1",
				Machines: []string{"machine-ren-1"},
				Nic:      "machine-ren-1:nic-1",
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			res, err := RenameNic(ctx, "machine-ren-1:nic-1", "machine-ren-1:nic-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.Name, should.Equal("machine-ren-1:nic-2"))
			machine, err := GetMachine(ctx, "machine-ren-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, machine, should.NotBeNil)
			assert.Loosely(t, machine.GetChromeBrowserMachine().GetNicObjects()[0].GetName(), should.Equal("machine-ren-1:nic-2"))
			_, err = registration.GetNic(ctx, "machine-ren-1:nic-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			nic, err := registration.GetNic(ctx, "machine-ren-1:nic-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nic, should.NotBeNil)
			assert.Loosely(t, nic.GetName(), should.Equal("machine-ren-1:nic-2"))
			assert.Loosely(t, nic.GetMachine(), should.Equal("machine-ren-1"))
			host, err := inventory.GetMachineLSE(ctx, "machinelse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, host, should.NotBeNil)
			assert.Loosely(t, host.GetNic(), should.Match("machine-ren-1:nic-2"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "nics/machine-ren-1:nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("nic"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("machine-ren-1:nic-1"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("machine-ren-1:nic-2"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("nic.name"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "nics/machine-ren-1:nic-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("nic"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("machine-ren-1:nic-1"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("machine-ren-1:nic-2"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("nic.name"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("machine-ren-1:nic-1"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("machine-ren-1:nic-2"))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.nic"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "nics/machine-ren-1:nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "nics/machine-ren-1:nic-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/machinelse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
		t.Run("Rename a non-existing Nic", func(t *ftt.Test) {
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err := RenameNic(ctx, "machine-ren-1:nic-3", "machine-ren-1:nic-4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Rename a Nic to an already existing nic name", func(t *ftt.Test) {
			_, err := registration.CreateNic(ctx, &ufspb.Nic{
				Name:    "machine-ren-1:nic-5",
				Machine: "machine-ren-1",
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateNic(ctx, &ufspb.Nic{
				Name:    "machine-ren-1:nic-6",
				Machine: "machine-ren-1",
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = RenameNic(ctx, "machine-ren-1:nic-5", "machine-ren-1:nic-6")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Nic machine-ren-1:nic-6 already exists in the system"))
		})
		t.Run("Rename a Machine - permission denied: same realm and no update permission", func(t *ftt.Test) {
			_, err := registration.CreateNic(ctx, &ufspb.Nic{
				Name:    "machine-ren-1:nic-7",
				Machine: "machine-ren-1",
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = RenameNic(ctx, "machine-ren-1:nic-7", "machine-ren-1:nic-8")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
		t.Run("Rename a Nic - permission denied: different realm", func(t *ftt.Test) {
			_, err := registration.CreateNic(ctx, &ufspb.Nic{
				Name:    "machine-ren-1:nic-9",
				Machine: "machine-ren-1",
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			_, err = RenameNic(ctx, "machine-ren-1:nic-9", "machine-ren-1:nic-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}
