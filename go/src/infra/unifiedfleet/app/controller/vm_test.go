// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/genproto/protobuf/field_mask"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/history"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func TestCreateVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("CreateVM", t, func(t *ftt.Test) {
		ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.AcsLabAdminRealm)
		registration.CreateMachine(ctx, &ufspb.Machine{
			Name: "update-machine",
			Location: &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CHROMEOS3,
			},
		})
		ctx = initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesCreate, util.AcsLabAdminRealm)
		inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
			Name:     "create-host",
			Zone:     ufspb.Zone_ZONE_CHROMEOS3.String(),
			Machines: []string{"update-machine"},
		})
		t.Run("Create new VM", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-create-1",
				MachineLseId: "create-host",
			}
			resp, err := CreateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_REGISTERED))
			assert.Loosely(t, resp.GetMachineLseId(), should.Equal("create-host"))
			assert.Loosely(t, resp.GetZone(), should.Equal(ufspb.Zone_ZONE_CHROMEOS3.String()))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "vms/vm-create-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("vm"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/vms/vm-create-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/vms/vm-create-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
		})
		t.Run("Create new VM with specified State", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:          "vm-create-4",
				MachineLseId:  "create-host",
				ResourceState: ufspb.State_STATE_SERVING,
			}
			resp, err := CreateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_SERVING))
			assert.Loosely(t, resp.GetMachineLseId(), should.Equal("create-host"))
			assert.Loosely(t, resp.GetZone(), should.Equal(ufspb.Zone_ZONE_CHROMEOS3.String()))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "vms/vm-create-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("vm"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/vms/vm-create-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_SERVING.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/vms/vm-create-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
		})

		t.Run("Create new VM with specifying vlan", func(t *ftt.Test) {
			setupTestVlan(ctx)

			vm1 := &ufspb.VM{
				Name:         "vm-create-2",
				MachineLseId: "create-host",
			}
			resp, err := CreateVM(ctx, vm1, &ufsAPI.NetworkOption{
				Vlan: "vlan-1",
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetMachineLseId(), should.Equal("create-host"))
			dhcp, err := configuration.GetDHCPConfig(ctx, "vm-create-2")
			assert.Loosely(t, err, should.BeNil)
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": dhcp.GetIp()})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeTrue)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "vms/vm-create-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("vm"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/vms/vm-create-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/vms/vm-create-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/vm-create-2")
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
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "vms/vm-create-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/vms/vm-create-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/vm-create-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})

		t.Run("Create new VM with specifying ip", func(t *ftt.Test) {
			setupTestVlan(ctx)

			vm1 := &ufspb.VM{
				Name:         "vm-create-3",
				MachineLseId: "create-host",
			}
			resp, err := CreateVM(ctx, vm1, &ufsAPI.NetworkOption{
				Ip: "192.168.40.19",
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetMachineLseId(), should.Equal("create-host"))
			dhcp, err := configuration.GetDHCPConfig(ctx, "vm-create-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dhcp.GetIp(), should.Equal("192.168.40.19"))
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.19"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeTrue)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "vms/vm-create-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("vm"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/vms/vm-create-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/vms/vm-create-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/vm-create-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("192.168.40.19"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", fmt.Sprintf("ips/%s", ip[0].GetId()))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("ip.occupied"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("false"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "vms/vm-create-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/vms/vm-create-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/vm-create-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})
	})
}

func TestUpdateVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateVM", t, func(t *ftt.Test) {
		registration.CreateMachine(ctx, &ufspb.Machine{
			Name: "update-machine",
		})
		inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
			Name:     "update-host",
			Zone:     "fake_zone",
			Machines: []string{"update-machine"},
		})
		t.Run("Update non-existing VM", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-update-1",
				MachineLseId: "create-host",
			}
			resp, err := UpdateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "vms/vm-update-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update VM - happy path with vlan", func(t *ftt.Test) {
			setupTestVlan(ctx)

			vm1 := &ufspb.VM{
				Name:         "vm-update-2",
				MachineLseId: "update-host",
			}
			_, err := CreateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.BeNil)
			resp, err := UpdateVMHost(ctx, vm1.Name, &ufsAPI.NetworkOption{
				Vlan: "vlan-1",
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_DEPLOYING))
			s, err := state.GetStateRecord(ctx, "vms/vm-update-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_DEPLOYING))
			dhcp, err := configuration.GetDHCPConfig(ctx, "vm-update-2")
			assert.Loosely(t, err, should.BeNil)
			ips, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": dhcp.GetIp()})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(1))
			assert.Loosely(t, ips[0].GetOccupied(), should.BeTrue)

			// Come from CreateVM+UpdateVMHost
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "vms/vm-update-2")
			assert.Loosely(t, err, should.BeNil)
			// VM created & vlan, ip changes
			assert.Loosely(t, changes, should.HaveLength(4))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("vm"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("vm.vlan"))
			assert.Loosely(t, changes[1].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("vlan-1"))
			assert.Loosely(t, changes[2].GetEventLabel(), should.Equal("vm.ip"))
			assert.Loosely(t, changes[2].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[2].GetNewValue(), should.Equal("192.168.40.11"))
			assert.Loosely(t, changes[3].GetEventLabel(), should.Equal("vm.resource_state"))
			assert.Loosely(t, changes[3].GetOldValue(), should.Equal("STATE_REGISTERED"))
			assert.Loosely(t, changes[3].GetNewValue(), should.Equal("STATE_DEPLOYING"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/vms/vm-update-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(ufspb.State_STATE_DEPLOYING.String()))
			// Come from UpdateVM
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/vm-update-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(dhcp.GetIp()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", fmt.Sprintf("ips/%s", ips[0].GetId()))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("ip.occupied"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("false"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("true"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "vms/vm-update-2")
			assert.Loosely(t, err, should.BeNil)
			// 1 come from CreateVM
			assert.Loosely(t, msgs, should.HaveLength(2))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/vms/vm-update-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/vm-update-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})

		t.Run("Update VM - happy path with ip specification & deletion", func(t *ftt.Test) {
			setupTestVlan(ctx)
			vm1 := &ufspb.VM{
				Name:         "vm-update-3",
				MachineLseId: "update-host",
			}
			_, err := CreateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.BeNil)

			_, err = UpdateVMHost(ctx, vm1.Name, &ufsAPI.NetworkOption{
				Ip: "192.168.40.19",
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteVMHost(ctx, vm1.Name)
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.GetDHCPConfig(ctx, "vm-update-3")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			ips, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.19"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(1))
			assert.Loosely(t, ips[0].GetOccupied(), should.BeFalse)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "vms/vm-update-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(7))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("vm"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			// vlan & ip info are changed
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("vm.vlan"))
			assert.Loosely(t, changes[1].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("vlan-1"))
			assert.Loosely(t, changes[2].GetEventLabel(), should.Equal("vm.ip"))
			assert.Loosely(t, changes[2].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[2].GetNewValue(), should.Equal("192.168.40.19"))
			assert.Loosely(t, changes[3].GetEventLabel(), should.Equal("vm.resource_state"))
			assert.Loosely(t, changes[3].GetOldValue(), should.Equal("STATE_REGISTERED"))
			assert.Loosely(t, changes[3].GetNewValue(), should.Equal("STATE_DEPLOYING"))
			// From deleting vm's ip
			assert.Loosely(t, changes[4].GetEventLabel(), should.Equal("vm.vlan"))
			assert.Loosely(t, changes[4].GetOldValue(), should.Equal("vlan-1"))
			assert.Loosely(t, changes[4].GetNewValue(), should.BeEmpty)
			assert.Loosely(t, changes[5].GetEventLabel(), should.Equal("vm.ip"))
			assert.Loosely(t, changes[5].GetOldValue(), should.Equal("192.168.40.19"))
			assert.Loosely(t, changes[5].GetNewValue(), should.BeEmpty)
			assert.Loosely(t, changes[6].GetEventLabel(), should.Equal("vm.resource_state"))
			assert.Loosely(t, changes[6].GetOldValue(), should.Equal("STATE_DEPLOYING"))
			assert.Loosely(t, changes[6].GetNewValue(), should.Equal("STATE_REGISTERED"))
			// log dhcp changes
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/vm-update-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("192.168.40.19"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("192.168.40.19"))
			assert.Loosely(t, changes[1].GetNewValue(), should.BeEmpty)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", fmt.Sprintf("ips/%s", ips[0].GetId()))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("ip.occupied"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("false"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("true"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("ip.occupied"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("true"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("false"))
			// snapshots
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "vms/vm-update-3")
			assert.Loosely(t, err, should.BeNil)
			// 1 create, 1 UpdateVMHost, 1 DeleteVMHost
			assert.Loosely(t, msgs, should.HaveLength(3))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/vms/vm-update-3")
			assert.Loosely(t, err, should.BeNil)
			// 1 create, 1 UpdateVMHost, 1 DeleteVMHost
			assert.Loosely(t, msgs, should.HaveLength(3))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/vm-update-3")
			assert.Loosely(t, err, should.BeNil)
			// 2 host update
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)
		})

		t.Run("Update VM - happy path with state updating", func(t *ftt.Test) {
			setupTestVlan(ctx)

			vm1 := &ufspb.VM{
				Name:         "vm-update-4",
				MachineLseId: "update-host",
			}
			_, err := CreateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.BeNil)
			vm1.ResourceState = ufspb.State_STATE_NEEDS_REPAIR
			resp, err := UpdateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_NEEDS_REPAIR))
			assert.Loosely(t, resp.GetMachineLseId(), should.Equal("update-host"))
			s, err := state.GetStateRecord(ctx, "vms/vm-update-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_NEEDS_REPAIR))

			// Come from CreateVM
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "states/vms/vm-update-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			// Come from UpdateVM
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(ufspb.State_STATE_NEEDS_REPAIR.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/vm-update-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			// snapshots

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "vms/vm-update-4")
			assert.Loosely(t, err, should.BeNil)
			// 1 create, 1 update
			assert.Loosely(t, msgs, should.HaveLength(2))

			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/vms/vm-update-4")
			assert.Loosely(t, err, should.BeNil)
			// 1 create, 1 update
			assert.Loosely(t, msgs, should.HaveLength(2))

			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/vms/vm-update-4")
			assert.Loosely(t, err, should.BeNil)
			// 1 create, 1 update
			assert.Loosely(t, msgs, should.HaveLength(2))

			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/vm-update-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("Partial Update vm", func(t *ftt.Test) {
			vm := &ufspb.VM{
				Name: "vm-7",
				OsVersion: &ufspb.OSVersion{
					Value: "windows",
				},
				Tags:         []string{"tag-1"},
				MachineLseId: "update-host",
				CpuCores:     16,
				Vmid:         111,
			}
			_, err := CreateVM(ctx, vm, nil)
			assert.Loosely(t, err, should.BeNil)

			vm1 := &ufspb.VM{
				Name:   "vm-7",
				Tags:   []string{"tag-2"},
				Memory: 1000,
				Vmid:   222,
			}
			resp, err := UpdateVM(ctx, vm1, &field_mask.FieldMask{Paths: []string{"tags", "memory", "vmid"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetTags(), should.Match([]string{"tag-1", "tag-2"}))
			assert.Loosely(t, resp.GetOsVersion().GetValue(), should.Equal("windows"))
			assert.Loosely(t, resp.GetCpuCores(), should.Equal(16))
			assert.Loosely(t, resp.GetMemory(), should.Equal(1000))
			assert.Loosely(t, resp.GetVmid(), should.Equal(222))
			// DeviceLabels
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/vms/vm-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
		})
	})
}

func TestDeleteVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteVM", t, func(t *ftt.Test) {
		registration.CreateMachine(ctx, &ufspb.Machine{
			Name: "delete-machine",
		})
		inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
			Name:     "delete-host",
			Zone:     "fake_zone",
			Machines: []string{"delete-machine"},
		})
		t.Run("Delete non-existing VM", func(t *ftt.Test) {
			err := DeleteVM(ctx, "vm-delete-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "vms/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})
		t.Run("Delete VM - happy path", func(t *ftt.Test) {
			setupTestVlan(ctx)
			vm1 := &ufspb.VM{
				Name:         "vm-delete-1",
				MachineLseId: "delete-host",
			}
			_, err := CreateVM(ctx, vm1, &ufsAPI.NetworkOption{
				Ip: "192.168.40.17",
			})
			assert.Loosely(t, err, should.BeNil)

			// Before
			_, err = state.GetStateRecord(ctx, "vms/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			dhcp, err := configuration.GetDHCPConfig(ctx, "vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dhcp.GetIp(), should.Equal("192.168.40.17"))
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.17"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeTrue)

			// After
			err = DeleteVM(ctx, "vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			_, err = state.GetStateRecord(ctx, "vms/vm-delete-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			_, err = configuration.GetDHCPConfig(ctx, "vm-delete-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			ips, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.17"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(1))
			assert.Loosely(t, ips[0].GetOccupied(), should.BeFalse)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "vms/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("vm"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/vms/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/vms/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(LifeCycleRetire))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("192.168.40.17"))
			assert.Loosely(t, changes[1].GetNewValue(), should.BeEmpty)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", fmt.Sprintf("ips/%s", ips[0].GetId()))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("ip.occupied"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("true"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("false"))

			// snapshots
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "vms/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			// 1 create, 1 deletion
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)

			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/vms/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			// 1 create, 1 deletion
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)

			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			// 1 create, 1 deletion
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)

			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/vms/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			// 1 create, 1 deletion
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)
		})
	})
}

func TestListVMs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	vms := []*ufspb.VM{
		{
			Name: "vm-list-1",
			OsVersion: &ufspb.OSVersion{
				Value: "os-1",
			},
			Vlan:          "vlan-1",
			ResourceState: ufspb.State_STATE_SERVING,
		},
		{
			Name: "vm-list-2",
			OsVersion: &ufspb.OSVersion{
				Value: "os-1",
			},
			Vlan:          "vlan-2",
			ResourceState: ufspb.State_STATE_SERVING,
		},
		{
			Name: "vm-list-3",
			OsVersion: &ufspb.OSVersion{
				Value: "os-2",
			},
			Vlan:          "vlan-1",
			ResourceState: ufspb.State_STATE_SERVING,
		},
		{
			Name: "vm-list-4",
			OsVersion: &ufspb.OSVersion{
				Value: "os-2",
			},
			Zone:          ufspb.Zone_ZONE_ATLANTA.String(),
			Vlan:          "vlan-2",
			ResourceState: ufspb.State_STATE_DEPLOYED_TESTING,
		},
		{
			Name:     "vm-list-5",
			Vlan:     "vlan-3",
			CpuCores: 8,
			Memory:   1234,
			Storage:  9876,
		},
	}
	ftt.Run("ListVMs", t, func(t *ftt.Test) {
		_, err := inventory.BatchUpdateVMs(ctx, vms)
		assert.Loosely(t, err, should.BeNil)
		t.Run("List VMs - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListVMs(ctx, 5, "", "invalid=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name invalid"))
		})

		t.Run("List VMs - filter vlan - happy path with filter", func(t *ftt.Test) {
			resp, _, _ := ListVMs(ctx, 5, "", "vlan=vlan-1", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, ufsAPI.ParseResources(resp, "Name"), should.Match([]string{"vm-list-1", "vm-list-3"}))
		})

		t.Run("List VMs - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListVMs(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(vms))
		})
		t.Run("List VMs - multiple filters", func(t *ftt.Test) {
			resp, _, err := ListVMs(ctx, 5, "", "vlan=vlan-2 & state=deployed_testing & zone=atlanta", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0].GetName(), should.Equal("vm-list-4"))
		})
		t.Run("List VMs - resource filters", func(t *ftt.Test) {
			resp, _, err := ListVMs(ctx, 5, "", "cpucores=8 & memory=1234 & storage=9876", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0].GetName(), should.Equal("vm-list-5"))
		})
	})
}
func TestBatchGetVMs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetVMs", t, func(t *ftt.Test) {
		t.Run("Batch get vms - happy path", func(t *ftt.Test) {
			entities := make([]*ufspb.VM, 4)
			for i := range 4 {
				entities[i] = &ufspb.VM{
					Name: fmt.Sprintf("vm-batchGet-%d", i),
				}
			}
			_, err := inventory.BatchUpdateVMs(ctx, entities)
			assert.Loosely(t, err, should.BeNil)
			resp, err := inventory.BatchGetVMs(ctx, []string{"vm-batchGet-0", "vm-batchGet-1", "vm-batchGet-2", "vm-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(entities))
		})
		t.Run("Batch get vms  - missing id", func(t *ftt.Test) {
			resp, err := inventory.BatchGetVMs(ctx, []string{"vm-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("vm-batchGet-non-existing"))
		})
		t.Run("Batch get vms  - empty input", func(t *ftt.Test) {
			resp, err := inventory.BatchGetVMs(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = inventory.BatchGetVMs(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}

func TestRealmPermissionForVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	registration.CreateMachine(ctx, &ufspb.Machine{
		Name: "machine-browser-1",
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_ATL97,
		},
	})
	inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
		Name:     "lse-browser-1",
		Machines: []string{"machine-browser-1"},
		Hostname: "lse-browser-1",
	})
	inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
		Name:     "lse-browser-1.1",
		Machines: []string{"machine-browser-1"},
		Hostname: "lse-browser-1.1",
	})
	registration.CreateMachine(ctx, &ufspb.Machine{
		Name: "machine-osatl-2",
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_CHROMEOS1,
		},
	})
	inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
		Name:     "lse-browser-2",
		Machines: []string{"machine-osatl-2"},
		Hostname: "lse-browser-2",
	})
	ftt.Run("TestRealmPermissionForVM", t, func(t *ftt.Test) {

		t.Run("CreateVM with permission - pass", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-1",
				MachineLseId: "lse-browser-1",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesCreate, util.BrowserLabAdminRealm)
			resp, _ := CreateVM(ctx, vm1, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(vm1))
		})

		t.Run("CreateVM without permission - fail", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-2",
				MachineLseId: "lse-browser-1",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesCreate, util.AtlLabAdminRealm)
			_, err := CreateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("DeleteVM with permission - pass", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-3",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesDelete, util.BrowserLabAdminRealm)
			err = DeleteVM(ctx, "vm-3")
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("DeleteVM without permission - fail", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-4",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesDelete, util.AtlLabAdminRealm)
			err = DeleteVM(ctx, "vm-4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("UpdateVM with permission - pass", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-5",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			vm1.Tags = []string{"Dell"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.Tags, should.Match([]string{"Dell"}))
		})

		t.Run("UpdateVM without permission - fail", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-6",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			vm1.Tags = []string{"Dell"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
			_, err = UpdateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("UpdateVM(new machinelse and same realm) with permission - pass", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-7",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			vm1.MachineLseId = "lse-browser-1.1"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.MachineLseId, should.Equal("lse-browser-1.1"))
		})

		t.Run("UpdateVM(new machinelse and different realm) without permission - fail", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-8",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			vm1.MachineLseId = "lse-browser-2"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("UpdateVM(new machinelse and different realm) with permission - pass", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-9",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			vm1.MachineLseId = "lse-browser-2"
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.InventoriesUpdate),
				),
			})
			resp, err := UpdateVM(ctx, vm1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.MachineLseId, should.Equal("lse-browser-2"))
		})

		t.Run("Partial UpdateVM with permission - pass", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-10",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			vm1.Tags = []string{"Dell"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateVM(ctx, vm1, &field_mask.FieldMask{Paths: []string{"tags"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.Tags, should.Match([]string{"Dell"}))
		})

		t.Run("Partial UpdateVM without permission - fail", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-11",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			vm1.Tags = []string{"Dell"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
			_, err = UpdateVM(ctx, vm1, &field_mask.FieldMask{Paths: []string{"tags"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Partial UpdateVM(new machinelse and same realm) with permission - pass", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-12",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			vm1.MachineLseId = "lse-browser-1.1"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateVM(ctx, vm1, &field_mask.FieldMask{Paths: []string{"machineLseId"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.MachineLseId, should.Match("lse-browser-1.1"))
		})

		t.Run("Partial UpdateVM(new machinelse and different realm) without permission - fail", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-13",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			vm1.MachineLseId = "lse-browser-2"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateVM(ctx, vm1, &field_mask.FieldMask{Paths: []string{"machineLseId"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Partial UpdateVM(new machinelse and different realm) with permission - pass", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "vm-14",
				MachineLseId: "lse-browser-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			vm1.MachineLseId = "lse-browser-2"
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.InventoriesUpdate),
				),
			})
			resp, err := UpdateVM(ctx, vm1, &field_mask.FieldMask{Paths: []string{"machineLseId"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.MachineLseId, should.Match("lse-browser-2"))
		})

	})
}

func TestGenNewMacAddress(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("genNewMacAddress", t, func(t *ftt.Test) {
		entities := make([]*ufspb.VM, 2)
		entities[0] = &ufspb.VM{
			Name:       "vm-genNewMac-0",
			MacAddress: "00:50:56:3f:ff:fd",
		}
		entities[1] = &ufspb.VM{
			Name:       "vm-genNewMac-1",
			MacAddress: "00:50:56:3f:ff:ff",
		}
		_, err := inventory.BatchUpdateVMs(ctx, entities)
		assert.Loosely(t, err, should.BeNil)
		t.Run("genNewMacAddress - happy path", func(t *ftt.Test) {
			mac, err := genNewMacAddress(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, mac, should.Equal("00:50:56:00:00:01"))

			sc, err := configuration.GetServiceConfig(ctx)
			assert.Loosely(t, err, should.BeNil)
			sc.LastCheckedVMMacAddress = "3ffffc"
			err = configuration.UpdateServiceConfig(ctx, sc)
			assert.Loosely(t, err, should.BeNil)
			mac, err = genNewMacAddress(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, mac, should.Equal("00:50:56:3f:ff:fe"))

			sc.LastCheckedVMMacAddress = "3ffffe"
			err = configuration.UpdateServiceConfig(ctx, sc)
			assert.Loosely(t, err, should.BeNil)
			mac, err = genNewMacAddress(ctx)
			assert.Loosely(t, mac, should.Equal(""))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("4 million"))
		})
	})
}

func setupTestVlan(ctx context.Context) {
	vlan := &ufspb.Vlan{
		Name:        "vlan-1",
		VlanAddress: "192.168.40.0/22",
	}
	configuration.CreateVlan(ctx, vlan)
	ips, _, _, _, _, _ := util.ParseVlan(vlan.GetName(), vlan.GetVlanAddress(), vlan.GetFreeStartIpv4Str(), vlan.GetFreeEndIpv4Str())
	// Only import the first 20 as one single transaction cannot import all.
	configuration.ImportIPs(ctx, ips[0:20])
}
