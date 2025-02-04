// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	labApi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	ufspb "infra/unifiedfleet/api/v1/models"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/external"
	"infra/unifiedfleet/app/model/configuration"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/history"
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/model/registration"
	"infra/unifiedfleet/app/model/state"
	"infra/unifiedfleet/app/util"
)

func mockDutMachineLSE(name string) *ufspb.MachineLSE {
	dut := &chromeosLab.DeviceUnderTest{
		Hostname: name,
	}
	device := &ufspb.ChromeOSDeviceLSE_Dut{
		Dut: dut,
	}
	deviceLse := &ufspb.ChromeOSDeviceLSE{
		Device: device,
	}
	chromeosLse := &ufspb.ChromeOSMachineLSE_DeviceLse{
		DeviceLse: deviceLse,
	}
	chromeOSMachineLse := &ufspb.ChromeOSMachineLSE{
		ChromeosLse: chromeosLse,
	}
	lse := &ufspb.MachineLSE_ChromeosMachineLse{
		ChromeosMachineLse: chromeOSMachineLse,
	}
	return &ufspb.MachineLSE{
		Name:        name,
		Hostname:    name,
		Lse:         lse,
		LogicalZone: ufspb.LogicalZone_LOGICAL_ZONE_UNSPECIFIED,
	}
}

func mockLabstationMachineLSE(name string) *ufspb.MachineLSE {
	labstation := &chromeosLab.Labstation{
		Hostname: name,
	}
	device := &ufspb.ChromeOSDeviceLSE_Labstation{
		Labstation: labstation,
	}
	deviceLse := &ufspb.ChromeOSDeviceLSE{
		Device: device,
	}
	chromeosLse := &ufspb.ChromeOSMachineLSE_DeviceLse{
		DeviceLse: deviceLse,
	}
	chromeOSMachineLse := &ufspb.ChromeOSMachineLSE{
		ChromeosLse: chromeosLse,
	}
	lse := &ufspb.MachineLSE_ChromeosMachineLse{
		ChromeosMachineLse: chromeOSMachineLse,
	}
	return &ufspb.MachineLSE{
		Name:     name,
		Hostname: name,
		Lse:      lse,
	}
}

func mockAttachedDeviceMachineLSE(name string) *ufspb.MachineLSE {
	return &ufspb.MachineLSE{
		Name:     name,
		Hostname: name,
		Lse: &ufspb.MachineLSE_AttachedDeviceLse{
			AttachedDeviceLse: &ufspb.AttachedDeviceLSE{
				OsVersion: &ufspb.OSVersion{
					Value: "test",
				},
			},
		},
	}
}

func mockDevboardMachineLSE(name string) *ufspb.MachineLSE {
	device := &ufspb.ChromeOSDeviceLSE_Devboard{
		Devboard: &chromeosLab.Devboard{
			Servo: &chromeosLab.Servo{},
		},
	}
	deviceLse := &ufspb.ChromeOSDeviceLSE{
		Device: device,
	}
	chromeosLse := &ufspb.ChromeOSMachineLSE_DeviceLse{
		DeviceLse: deviceLse,
	}
	chromeOSMachineLse := &ufspb.ChromeOSMachineLSE{
		ChromeosLse: chromeosLse,
	}
	lse := &ufspb.MachineLSE_ChromeosMachineLse{
		ChromeosMachineLse: chromeOSMachineLse,
	}
	return &ufspb.MachineLSE{
		Name:     name,
		Hostname: name,
		Lse:      lse,
	}
}

func TestCreateMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ftt.Run("CreateMachineLSE", t, func(t *ftt.Test) {
		t.Run("Create new machineLSE with already existing machinelse", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-10",
			}
			_, merr := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, merr, should.BeNil)

			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "machinelse-1",
			})
			assert.Loosely(t, err, should.BeNil)

			machineLSE1 := &ufspb.MachineLSE{
				Hostname: "machinelse-1",
				Machines: []string{"machine-10"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			resp, err := CreateMachineLSE(ctx, machineLSE1, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("MachineLSE machinelse-1 already exists in the system."))

			// No changes are recorded as the creation fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/machinelse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create new machineLSE with non existing machines", func(t *ftt.Test) {
			machineLSE1 := &ufspb.MachineLSE{
				Hostname: "machinelse-2",
				Machines: []string{"machine-1", "machine-2"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			resp, err := CreateMachineLSE(ctx, machineLSE1, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			// No changes are recorded as the creation fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/machinelse-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create new machineLSE Chromium DUT with wrong zone", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "chromium-asset",
			}
			_, merr := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, merr, should.BeNil)

			dut := mockDutMachineLSE("chromium-dut")
			dut.Machines = []string{"chromium-asset"}
			dut.GetChromeosMachineLse().GetDeviceLse().GetDut().Pools = []string{"chromium"}
			resp, err := CreateMachineLSE(ctx, dut, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("has to be in zone"))
		})

		t.Run("Create new machineLSE Chromium DUT with mismatched pools", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "chromium-asset2",
			}
			_, merr := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, merr, should.BeNil)

			dut := mockDutMachineLSE("chromium-dut2")
			dut.Machines = []string{"chromium-asset2"}
			dut.GetChromeosMachineLse().GetDeviceLse().GetDut().Pools = []string{"chrome"}
			resp, err := CreateMachineLSE(ctx, dut, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("chromium DUTs has to have prefix"))
		})

		t.Run("Create new machineLSE chrome DUT with mismatched pools", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "chrome-asset",
			}
			_, merr := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, merr, should.BeNil)

			dut := mockDutMachineLSE("chrome-dut")
			dut.Machines = []string{"chrome-asset"}
			dut.GetChromeosMachineLse().GetDeviceLse().GetDut().Pools = []string{"other_pool"}
			resp, err := CreateMachineLSE(ctx, dut, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("chrome DUTs has to have prefix"))
		})

		t.Run("Create new machineLSE ChromePerf DUT with mismatched pools", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "chrome-perf-asset3",
			}
			_, merr := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, merr, should.BeNil)

			dut := mockDutMachineLSE("chrome-perf-dut3")
			dut.Machines = []string{"chrome-perf-asset3"}
			dut.GetChromeosMachineLse().GetDeviceLse().GetDut().Pools = []string{"chrome"}
			resp, err := CreateMachineLSE(ctx, dut, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("chrome perf DUTs has to have prefix"))
		})

		t.Run("Create new machineLSE ChromePerf DUT with matched pools", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "chrome-perf-asset4",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, merr := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, merr, should.BeNil)

			dut := mockDutMachineLSE("chrome-perf-dut4")
			dut.Machines = []string{"chrome-perf-asset4"}
			dut.GetChromeosMachineLse().GetDeviceLse().GetDut().Pools = []string{"chrome.tests.perf"}
			_, err := CreateMachineLSE(ctx, dut, nil)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Create new machineLSE with existing machines, specify ip with wrong vlan name", func(t *ftt.Test) {
			_, err := registration.CreateNic(ctx, &ufspb.Nic{
				Name:    "eth1",
				Machine: "machine-wrong-nic",
			})
			assert.Loosely(t, err, should.BeNil)
			machine1 := &ufspb.Machine{
				Name: "machine-wrong-nic",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err = registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)

			machineLSE2 := &ufspb.MachineLSE{
				Hostname: "machinelse-wrong-nic",
				Machines: []string{"machine-wrong-nic"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			resp, err := CreateMachineLSE(ctx, machineLSE2, &ufsAPI.NetworkOption{
				Vlan: "vlan-1",
				Nic:  "eth1",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no Vlan with VlanID vlan-1"))

			// No changes are recorded as the creation fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-wrong-nic")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/machinelse-wrong-nic")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create new machineLSE with existing machines and specify ip", func(t *ftt.Test) {
			_, err := registration.CreateNic(ctx, &ufspb.Nic{
				Name:    "eth0",
				Machine: "machine-with-ip",
			})
			assert.Loosely(t, err, should.BeNil)
			machine1 := &ufspb.Machine{
				Name: "machine-with-ip",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err = registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			vlan := &ufspb.Vlan{
				Name:        "vlan-1",
				VlanAddress: "192.168.40.0/22",
			}
			configuration.CreateVlan(ctx, vlan)
			ips, _, _, _, _, err := util.ParseVlan(vlan.GetName(), vlan.GetVlanAddress(), "", "")
			assert.Loosely(t, err, should.BeNil)
			// Only import the first 20 as one single transaction cannot import all.
			_, err = configuration.ImportIPs(ctx, ips[0:20])
			assert.Loosely(t, err, should.BeNil)

			vm1 := &ufspb.VM{
				Name:       "vm1-ip",
				MacAddress: "old_mac_address",
			}
			machineLSE2 := &ufspb.MachineLSE{
				Name:     "machinelse-with-ip",
				Hostname: "machinelse-with-ip",
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{
						Vms: []*ufspb.VM{vm1},
					},
				},
				Machines: []string{"machine-with-ip"},
			}
			resp, err := CreateMachineLSE(ctx, machineLSE2, &ufsAPI.NetworkOption{
				Vlan: "vlan-1",
				Nic:  "eth0",
			})
			assert.Loosely(t, err, should.BeNil)
			machineLSE2.Nic = "eth0"
			assert.Loosely(t, resp, should.Match(machineLSE2))
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.11"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeTrue)
			dhcp, err := configuration.GetDHCPConfig(ctx, "machinelse-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dhcp.GetIp(), should.Equal("192.168.40.11"))
			s, err := state.GetStateRecord(ctx, "hosts/machinelse-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_DEPLOYING))
			s, err = state.GetStateRecord(ctx, "machines/machine-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_SERVING))
			vm, err := inventory.GetVM(ctx, "vm1-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, vm, should.Match(vm1))

			// verify changes
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "vms/vm1-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("vm"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/vms/vm1-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/hosts/machinelse-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_DEPLOYING.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/machines/machine-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_SERVING.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/machinelse-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(dhcp.GetIp()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/machinelse-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/machinelse-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "vms/vm1-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/vms/vm1-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/hosts/machinelse-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/machines/machine-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/machinelse-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/machinelse-with-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Create new machineLSE with existing machines", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name:         "machine-3",
				SerialNumber: "machine-3-serial",
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)

			machineLSE2 := &ufspb.MachineLSE{
				Hostname: "machinelse-3",
				Machines: []string{"machine-3"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			resp, err := CreateMachineLSE(ctx, machineLSE2, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE2))
			s, err := state.GetStateRecord(ctx, "hosts/machinelse-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
			s, err = state.GetStateRecord(ctx, "machines/machine-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_SERVING))
			dr, err := inventory.GetMachineLSEDeployment(ctx, "machine-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dr.GetHostname(), should.Equal("machinelse-3"))

			// Verify the change events
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machineLSEDeployments/machine-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse_deployment"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/machinelse-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machineLSEDeployments/machine-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/machinelse-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Create new machineLSE with existing machines & existing deployment record", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name:         "machine-dr-3",
				SerialNumber: "machine-dr-3-serial",
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)

			machineLSE2 := &ufspb.MachineLSE{
				Hostname: "machinelse-dr-3",
				Machines: []string{"machine-dr-3"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}

			_, err = inventory.UpdateMachineLSEDeployments(ctx, []*ufspb.MachineLSEDeployment{util.FormatDeploymentRecord("", machine1.GetSerialNumber())})
			assert.Loosely(t, err, should.BeNil)
			dr, err := inventory.GetMachineLSEDeployment(ctx, "machine-dr-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dr.GetHostname(), should.Equal("no-host-yet-machine-dr-3-serial"))

			_, err = CreateMachineLSE(ctx, machineLSE2, nil)
			assert.Loosely(t, err, should.BeNil)
			dr, err = inventory.GetMachineLSEDeployment(ctx, "machine-dr-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dr.GetHostname(), should.Equal("machinelse-dr-3"))

			// Verify the change events
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machineLSEDeployments/machine-dr-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("no-host-yet-machine-dr-3-serial"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("machinelse-dr-3"))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse_deployment.hostname"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/machinelse-dr-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machineLSEDeployments/machine-dr-3-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/machinelse-dr-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Create new machineLSE with a machine already attached to a different machinelse - error", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-4",
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-4",
				Machines: []string{"machine-4"},
			})
			assert.Loosely(t, err, should.BeNil)

			machineLSE := &ufspb.MachineLSE{
				Hostname: "machinelse-5",
				Machines: []string{"machine-4"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			resp, err := CreateMachineLSE(ctx, machineLSE, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Hosts referring the machine machine-4:\nmachinelse-4"))

			// No changes are recorded as the creation fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/machine-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/machinelse-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create new devboard machineLSE", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-5",
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)

			devboard := &ufspb.ChromeOSDeviceLSE_Devboard{
				Devboard: &chromeosLab.Devboard{},
			}
			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-5",
				Machines: []string{"machine-5"},
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: devboard,
							},
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestCreateMachineLSELabstation(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	machine := &ufspb.Machine{
		Name: "machine-4",
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				BuildTarget: "test",
				Model:       "test",
			},
		},
	}
	registration.CreateMachine(ctx, machine)
	ftt.Run("CreateMachineLSE for a Labstation", t, func(t *ftt.Test) {
		t.Run("Create machineLSE Labstation with Servo Info", func(t *ftt.Test) {
			labstationMachinelse1 := mockLabstationMachineLSE("RedLabstation-0")
			labstationMachinelse1.Machines = []string{"machine-4"}
			servo := &chromeosLab.Servo{
				ServoHostname: "RedLabstation-0",
				ServoPort:     22,
			}
			labstationMachinelse1.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Servos = []*chromeosLab.Servo{servo}
			resp, err := CreateMachineLSE(ctx, labstationMachinelse1, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Servos are not allowed to be added"))
			_, err = state.GetStateRecord(ctx, "hosts/RedLabstation-0")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			// No changes are recorded as the creation fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/RedLabstation-0")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})
		t.Run("Create machineLSE Labstation without Servo Info", func(t *ftt.Test) {
			labstationMachinelse1 := mockLabstationMachineLSE("RedLabstation-1")
			labstationMachinelse1.Machines = []string{"machine-4"}
			resp, err := CreateMachineLSE(ctx, labstationMachinelse1, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(labstationMachinelse1))
			s, err := state.GetStateRecord(ctx, "hosts/RedLabstation-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
			s, err = state.GetStateRecord(ctx, "machines/machine-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_SERVING))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/RedLabstation-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse"))
		})
	})
}

func TestUpdateMachineLSEDUT(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = withAuthorizedAtlUser(ctx)
	machine10 := &ufspb.Machine{
		Name: "machine-10",
	}
	registration.CreateMachine(ctx, machine10)

	machine21 := &ufspb.Machine{
		Name: "machine-21",
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_UNSPECIFIED,
		},
	}
	registration.CreateMachine(ctx, machine21)

	machine22 := &ufspb.Machine{
		Name: "machine-22",
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_SFO36_OS,
		},
	}
	registration.CreateMachine(ctx, machine22)

	servo1 := &chromeosLab.Servo{
		ServoHostname: "BlueLabstation-10",
		ServoPort:     9921,
		ServoSerial:   "Serial-1",
	}
	servo2 := &chromeosLab.Servo{
		ServoHostname: "BlueLabstation-10",
		ServoPort:     9922,
		ServoSerial:   "Serial-2",
	}

	labstationMachinelse := mockLabstationMachineLSE("BlueLabstation-10")
	labstationMachinelse.Machines = []string{"machine-10"}
	labstationMachinelse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Servos = []*chromeosLab.Servo{servo1, servo2}
	inventory.CreateMachineLSE(ctx, labstationMachinelse)

	peripherals1 := &chromeosLab.Peripherals{
		Servo: servo1,
	}
	dutMachinelse1 := mockDutMachineLSE("DUTMachineLSE-21")
	dutMachinelse1.Machines = []string{"machine-21"}
	dutMachinelse1.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = peripherals1
	inventory.CreateMachineLSE(ctx, dutMachinelse1)

	peripherals2 := &chromeosLab.Peripherals{
		Servo: servo2,
	}
	dutMachinelse2 := mockDutMachineLSE("DUTMachineLSE-22")
	dutMachinelse2.Machines = []string{"machine-22"}
	dutMachinelse2.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = peripherals2
	inventory.CreateMachineLSE(ctx, dutMachinelse2)

	inventory.GetMachineLSE(ctx, "BlueLabstation-10")
	ftt.Run("UpdateMachineLSE for a DUT", t, func(t *ftt.Test) {
		t.Run("Update non-existing machineLSE DUT", func(t *ftt.Test) {
			dutMachinelse := mockDutMachineLSE("DUTMachineLSE-23")
			resp, err := UpdateMachineLSE(ctx, dutMachinelse, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Update machineLSE DUT with same ServoPort and same ServoHostname", func(t *ftt.Test) {
			servo3 := &chromeosLab.Servo{
				ServoHostname: "BlueLabstation-10",
				ServoPort:     9921,
				ServoSerial:   "Serial-1",
			}
			peripherals3 := &chromeosLab.Peripherals{
				Servo: servo3,
			}
			dutMachinelse3 := mockDutMachineLSE("DUTMachineLSE-21")
			dutMachinelse3.Machines = []string{"machine-21"}
			dutMachinelse3.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = peripherals3
			resp, err := UpdateMachineLSE(ctx, dutMachinelse3, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dutMachinelse3))

			resp, _ = inventory.GetMachineLSE(ctx, "BlueLabstation-10")
			assert.Loosely(t, resp.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Servos, should.HaveLength(2))
			// Labstation host is not updated
			_, err = state.GetStateRecord(ctx, "hosts/BlueLabstation-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Update machineLSE DUT with different ServerPort and same ServoHostname", func(t *ftt.Test) {
			servo3 := &chromeosLab.Servo{
				ServoHostname: "BlueLabstation-10",
				ServoPort:     9958,
				ServoSerial:   "Serial-2",
			}
			peripherals3 := &chromeosLab.Peripherals{
				Servo: servo3,
			}
			dutMachinelse3 := mockDutMachineLSE("DUTMachineLSE-22")
			dutMachinelse3.Machines = []string{"machine-22"}
			dutMachinelse3.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = peripherals3
			dutMachinelse3.ResourceState = ufspb.State_STATE_SERVING
			resp, err := UpdateMachineLSE(ctx, dutMachinelse3, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(dutMachinelse3))

			dummyServos := []*chromeosLab.Servo{servo1, servo3}

			resp, _ = inventory.GetMachineLSE(ctx, "BlueLabstation-10")
			assert.Loosely(t, resp.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Servos, should.Match(dummyServos))

			// Labstation host is not updated
			_, err = state.GetStateRecord(ctx, "hosts/BlueLabstation-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Update machineLSE DUT with different ServoHostname", func(t *ftt.Test) {
			servo1 := &chromeosLab.Servo{
				ServoHostname: "BlueLabstation-17",
				ServoPort:     9917,
				ServoSerial:   "Serial-17",
			}
			labstationMachinelse1 := mockLabstationMachineLSE("BlueLabstation-17")
			labstationMachinelse1.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Servos = []*chromeosLab.Servo{servo1}
			_, err := inventory.CreateMachineLSE(ctx, labstationMachinelse1)
			assert.Loosely(t, err, should.BeNil)

			servo2 := &chromeosLab.Servo{
				ServoHostname: "BlueLabstation-18",
				ServoPort:     9918,
				ServoSerial:   "Serial-18",
			}
			labstationMachinelse2 := mockLabstationMachineLSE("BlueLabstation-18")
			labstationMachinelse2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Servos = []*chromeosLab.Servo{}
			_, err = inventory.CreateMachineLSE(ctx, labstationMachinelse2)
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-17",
			})
			assert.Loosely(t, err, should.BeNil)
			peripherals1 := &chromeosLab.Peripherals{
				Servo: servo1,
			}
			dutMachinelse1 := mockDutMachineLSE("DUTMachineLSE-17")
			dutMachinelse1.Machines = []string{"machine-17"}
			dutMachinelse1.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = peripherals1
			_, err = inventory.CreateMachineLSE(ctx, dutMachinelse1)
			assert.Loosely(t, err, should.BeNil)

			peripherals2 := &chromeosLab.Peripherals{
				Servo: servo2,
			}
			dutMachinelse2 := mockDutMachineLSE("DUTMachineLSE-17")
			dutMachinelse2.Machines = []string{"machine-17"}
			dutMachinelse2.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = peripherals2
			resp, err := UpdateMachineLSE(ctx, dutMachinelse2, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dutMachinelse2))

			resp, _ = inventory.GetMachineLSE(ctx, "BlueLabstation-17")
			assert.Loosely(t, resp, should.NotBeNil)
			servos := resp.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos()
			assert.Loosely(t, servos, should.BeEmpty)

			resp, _ = inventory.GetMachineLSE(ctx, "BlueLabstation-18")
			assert.Loosely(t, resp, should.NotBeNil)
			servos = resp.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos()
			assert.Loosely(t, servo2, should.Match(servos[0]))

			// Labstation host is not updated
			_, err = state.GetStateRecord(ctx, "hosts/BlueLabstation-17")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			_, err = state.GetStateRecord(ctx, "hosts/BlueLabstation-18")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Update machineLSE DUT with Pool Names", func(t *ftt.Test) {
			dutMachinelse3 := mockDutMachineLSE("DUTMachineLSE-22")
			dutMachinelse3.Machines = []string{"machine-22"}
			dutMachinelse3.GetChromeosMachineLse().GetDeviceLse().GetDut().Pools = []string{"pool1", "pool2"}
			dutMachinelse3.ResourceState = ufspb.State_STATE_SERVING
			resp, err := UpdateMachineLSE(ctx, dutMachinelse3, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(dutMachinelse3))
		})

		t.Run("Update machineLSE DUT with Invalid Pool Names", func(t *ftt.Test) {
			dutMachinelse3 := mockDutMachineLSE("DUTMachineLSE-22")
			dutMachinelse3.Machines = []string{"machine-22"}
			dutMachinelse3.GetChromeosMachineLse().GetDeviceLse().GetDut().Pools = []string{"\"pool1", "pool2"}
			dutMachinelse3.ResourceState = ufspb.State_STATE_SERVING
			resp, err := UpdateMachineLSE(ctx, dutMachinelse3, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid Pool Name"))
		})
	})

	ftt.Run("UpdateMachineLSE for a DUT - LogicalZone", t, func(t *ftt.Test) {
		t.Run("Update machineLSE DUT LogicalZone - DRILLZONE_SFO36 - Success", func(t *ftt.Test) {
			dutMachinelse3 := mockDutMachineLSE("DUTMachineLSE-22")
			dutMachinelse3.Machines = []string{"machine-22"}
			dutMachinelse3.LogicalZone = ufspb.LogicalZone_LOGICAL_ZONE_DRILLZONE_SFO36
			resp, err := UpdateMachineLSE(ctx, dutMachinelse3, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(dutMachinelse3))
		})
		t.Run("Update machineLSE DUT LogicalZone - DRILLZONE_SFO36 - Failure", func(t *ftt.Test) {
			dutMachinelse3 := mockDutMachineLSE("DUTMachineLSE-21")
			dutMachinelse3.Machines = []string{"machine-21"}
			dutMachinelse3.LogicalZone = ufspb.LogicalZone_LOGICAL_ZONE_DRILLZONE_SFO36
			resp, err := UpdateMachineLSE(ctx, dutMachinelse3, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})

	ftt.Run("UpdateMachineLSE for a DUT - Hive - Success", t, func(t *ftt.Test) {
		dutMachinelse3 := mockDutMachineLSE("DUTMachineLSE-22")
		dutMachinelse3.Machines = []string{"machine-22"}
		dutMachinelse3.GetChromeosMachineLse().GetDeviceLse().GetDut().Hive = "hive-1"
		resp, err := UpdateMachineLSE(ctx, dutMachinelse3, nil)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
		assert.Loosely(t, resp, should.Match(dutMachinelse3))
	})
}

func TestUpdateMachineLSELabstation(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateMachineLSE for a Labstation", t, func(t *ftt.Test) {
		t.Run("Update machineLSE Labstation with Servo Info", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-10",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			labstationMachinelse1 := mockLabstationMachineLSE("RedLabstation-10")
			labstationMachinelse1.Machines = []string{"machine-10"}
			inventory.CreateMachineLSE(ctx, labstationMachinelse1)

			labstationMachinelse2 := mockLabstationMachineLSE("RedLabstation-10")
			servo := &chromeosLab.Servo{
				ServoHostname: "RedLabstation-10",
				ServoPort:     22,
			}
			labstationMachinelse2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Servos = []*chromeosLab.Servo{servo}
			resp, err := UpdateMachineLSE(ctx, labstationMachinelse2, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Servos are not allowed to be updated"))
			_, err = state.GetStateRecord(ctx, "hosts/RedLabstation-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			// No changes are recorded as the updating fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/RedLabstation-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update machineLSE Labstation without Servo Info", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-4x",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			labstationMachinelse1 := mockLabstationMachineLSE("RedLabstation-11")
			labstationMachinelse1.Machines = []string{"machine-4x"}
			inventory.CreateMachineLSE(ctx, labstationMachinelse1)

			labstationMachinelse2 := mockLabstationMachineLSE("RedLabstation-11")
			labstationMachinelse2.Machines = []string{"machine-4x"}
			resp, err := UpdateMachineLSE(ctx, labstationMachinelse2, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(labstationMachinelse2))

			// No changes happened in this update
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/RedLabstation-11")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update machineLSE Labstation with Pool Names", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-12",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			labstationMachinelse1 := mockLabstationMachineLSE("RedLabstation-12")
			labstationMachinelse1.Machines = []string{"machine-12"}
			inventory.CreateMachineLSE(ctx, labstationMachinelse1)

			labstationMachinelse2 := mockLabstationMachineLSE("RedLabstation-12")
			labstationMachinelse2.Machines = []string{"machine-12"}
			labstationMachinelse2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Pools = []string{"pool1", "pool2"}
			resp, err := UpdateMachineLSE(ctx, labstationMachinelse2, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(labstationMachinelse2))
		})

		t.Run("Update machineLSE Labstation with invalid Pool Names", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-13",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			labstationMachinelse1 := mockLabstationMachineLSE("RedLabstation-13")
			labstationMachinelse1.Machines = []string{"machine-13"}
			inventory.CreateMachineLSE(ctx, labstationMachinelse1)

			labstationMachinelse2 := mockLabstationMachineLSE("RedLabstation-13")
			labstationMachinelse2.Machines = []string{"machine-13"}
			labstationMachinelse2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Pools = []string{"-pool1", "pool2"}
			resp, err := UpdateMachineLSE(ctx, labstationMachinelse2, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid Pool Name"))
		})
	})

	ftt.Run("UpdateMachineLSE for a Labstation - LogicalZone", t, func(t *ftt.Test) {
		t.Run("Update machineLSE Labstation LogicalZone - UNSPECIFIED - Success", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-101",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			labstationMachinelse1 := mockLabstationMachineLSE("RedLabstation-101")
			labstationMachinelse1.Machines = []string{"machine-101"}
			inventory.CreateMachineLSE(ctx, labstationMachinelse1)

			labstationMachinelse2 := mockLabstationMachineLSE("RedLabstation-101")
			labstationMachinelse2.Machines = []string{"machine-101"}
			labstationMachinelse2.LogicalZone = ufspb.LogicalZone_LOGICAL_ZONE_UNSPECIFIED
			resp, err := UpdateMachineLSE(ctx, labstationMachinelse2, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(labstationMachinelse2))
		})
		t.Run("Update machineLSE Labstation LogicalZone - DRILLZONE_SFO36 - Failure", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-102",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			labstationMachinelse1 := mockLabstationMachineLSE("RedLabstation-102")
			labstationMachinelse1.Machines = []string{"machine-102"}
			inventory.CreateMachineLSE(ctx, labstationMachinelse1)

			labstationMachinelse2 := mockLabstationMachineLSE("RedLabstation-102")
			labstationMachinelse2.Machines = []string{"machine-102"}
			labstationMachinelse2.LogicalZone = ufspb.LogicalZone_LOGICAL_ZONE_DRILLZONE_SFO36
			resp, err := UpdateMachineLSE(ctx, labstationMachinelse2, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}

func TestUpdateMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateMachineLSE for a browser machine", t, func(t *ftt.Test) {
		t.Run("Update machineLSE with setting state", func(t *ftt.Test) {
			machineLSE1 := &ufspb.MachineLSE{
				Name:     "machinelse-update1",
				Hostname: "machinelse-update1",
				Machines: []string{"machine-update1"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-update1",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.BatchUpdateVMs(ctx, []*ufspb.VM{
				{
					Name:         "vm1",
					MacAddress:   "old_mac_address",
					MachineLseId: "machinelse-update1",
				},
				{
					Name:         "vm-update",
					MacAddress:   "old_mac_address",
					MachineLseId: "machinelse-update1",
				},
			})
			assert.Loosely(t, err, should.BeNil)
			machineLSE1.GetChromeBrowserMachineLse().OsVersion = &ufspb.OSVersion{
				Value: "new_os",
			}
			machineLSE1.ResourceState = ufspb.State_STATE_DEPLOYED_TESTING
			m, err := UpdateMachineLSE(ctx, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m.GetChromeBrowserMachineLse().GetVms(), should.HaveLength(2))
			// State remains unchanged as vm1 is not updated
			s, err := state.GetStateRecord(ctx, "hosts/machinelse-update1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_DEPLOYED_TESTING))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-update1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.chrome_browser_machine_lse.os_version"))
			assert.Loosely(t, changes[0].OldValue, should.Equal("<nil>"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("value:\"new_os\""))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/hosts/machinelse-update1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_DEPLOYED_TESTING.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/machines/machine-update1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/machinelse-update1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/machinelse-update1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/hosts/machinelse-update1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/machines/machine-update1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/machinelse-update1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})

		t.Run("Update machineLSE by setting ip by vlan for host", func(t *ftt.Test) {
			_, err := registration.CreateNic(ctx, &ufspb.Nic{
				Name:    "eth0",
				Machine: "machine-update-host",
			})
			assert.Loosely(t, err, should.BeNil)
			machine1 := &ufspb.Machine{
				Name: "machine-update-host",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			machineLSE1 := &ufspb.MachineLSE{
				Name:     "machinelse-update-host",
				Hostname: "machinelse-update-host",
				Machines: []string{"machine-update-host"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{
						Vms: []*ufspb.VM{
							{
								Name:       "vm1",
								MacAddress: "old_mac_address",
							},
						},
					},
				},
			}
			_, err = registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			vlan := &ufspb.Vlan{
				Name:        "vlan-1",
				VlanAddress: "192.168.40.0/22",
			}
			configuration.CreateVlan(ctx, vlan)
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

			_, err = UpdateMachineLSEHost(ctx, machineLSE1.Name, &ufsAPI.NetworkOption{
				Vlan: "vlan-1",
				Nic:  "eth0",
			})
			assert.Loosely(t, err, should.BeNil)
			dhcp, err := configuration.GetDHCPConfig(ctx, "machinelse-update-host")
			assert.Loosely(t, err, should.BeNil)
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": dhcp.GetIp()})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeTrue)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-update-host")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// nic & vlan & ip info are changed
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.nic"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("eth0"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine_lse.vlan"))
			assert.Loosely(t, changes[1].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("vlan-1"))
			assert.Loosely(t, changes[2].GetEventLabel(), should.Equal("machine_lse.ip"))
			assert.Loosely(t, changes[2].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[2].GetNewValue(), should.Equal("192.168.40.11"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/hosts/machinelse-update-host")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_DEPLOYING.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/machinelse-update-host")
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
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/machinelse-update-host")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/hosts/machinelse-update-host")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/machinelse-update-host")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})

		t.Run("Update machineLSE by deleting ip for host", func(t *ftt.Test) {
			_, err := registration.CreateNic(ctx, &ufspb.Nic{
				Name:    "eth0-delete",
				Machine: "machine-update-host-delete-ip",
			})
			assert.Loosely(t, err, should.BeNil)
			machine1 := &ufspb.Machine{
				Name: "machine-update-host-delete-ip",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			machineLSE1 := &ufspb.MachineLSE{
				Name:     "machinelse-update-host-delete-ip",
				Hostname: "machinelse-update-host-delete-ip",
				Machines: []string{"machine-update-host-delete-ip"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			_, err = registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			vlan := &ufspb.Vlan{
				Name:        "vlan-1",
				VlanAddress: "192.168.40.0/22",
			}
			configuration.CreateVlan(ctx, vlan)
			ips, _, _, _, _, err := util.ParseVlan(vlan.GetName(), vlan.GetVlanAddress(), vlan.GetFreeStartIpv4Str(), vlan.GetFreeEndIpv4Str())
			assert.Loosely(t, err, should.BeNil)
			// Only import the first 20 as one single transaction cannot import all.
			_, err = configuration.ImportIPs(ctx, ips[0:20])
			assert.Loosely(t, err, should.BeNil)
			_, err = UpdateMachineLSEHost(ctx, machineLSE1.Name, &ufsAPI.NetworkOption{
				Vlan: "vlan-1",
				Ip:   "192.168.40.12",
				Nic:  "eth0-delete",
			})
			assert.Loosely(t, err, should.BeNil)
			oldIPs, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.12"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, oldIPs, should.HaveLength(1))
			assert.Loosely(t, oldIPs[0].GetOccupied(), should.BeTrue)

			err = DeleteMachineLSEHost(ctx, machineLSE1.Name)
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.GetDHCPConfig(ctx, "machinelse-update-host-delete-ip")
			// Not found error
			assert.Loosely(t, err, should.NotBeNil)
			ip2, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.12"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip2, should.HaveLength(1))
			assert.Loosely(t, ip2[0].GetOccupied(), should.BeFalse)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-update-host-delete-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(6))
			// nic & vlan & ip info are changed
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.nic"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("eth0-delete"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine_lse.vlan"))
			assert.Loosely(t, changes[1].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("vlan-1"))
			assert.Loosely(t, changes[2].GetEventLabel(), should.Equal("machine_lse.ip"))
			assert.Loosely(t, changes[2].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[2].GetNewValue(), should.Equal("192.168.40.12"))
			// From deleting host
			assert.Loosely(t, changes[3].GetEventLabel(), should.Equal("machine_lse.nic"))
			assert.Loosely(t, changes[3].GetOldValue(), should.Equal("eth0-delete"))
			assert.Loosely(t, changes[3].GetNewValue(), should.BeEmpty)
			assert.Loosely(t, changes[4].GetEventLabel(), should.Equal("machine_lse.vlan"))
			assert.Loosely(t, changes[4].GetOldValue(), should.Equal("vlan-1"))
			assert.Loosely(t, changes[4].GetNewValue(), should.BeEmpty)
			assert.Loosely(t, changes[5].GetEventLabel(), should.Equal("machine_lse.ip"))
			assert.Loosely(t, changes[5].GetOldValue(), should.Equal("192.168.40.12"))
			assert.Loosely(t, changes[5].GetNewValue(), should.BeEmpty)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/hosts/machinelse-update-host-delete-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_DEPLOYING.String()))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(ufspb.State_STATE_DEPLOYING.String()))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/machinelse-update-host-delete-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("192.168.40.12"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("dhcp_config.ip"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("192.168.40.12"))
			assert.Loosely(t, changes[1].GetNewValue(), should.BeEmpty)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", fmt.Sprintf("ips/%s", oldIPs[0].GetId()))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("ip.occupied"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("false"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("true"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("ip.occupied"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("true"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("false"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/machinelse-update-host-delete-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/hosts/machinelse-update-host-delete-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/machinelse-update-host-delete-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)
		})

		t.Run("Update machineLSE by setting ip by user for host", func(t *ftt.Test) {
			_, err := registration.CreateNic(ctx, &ufspb.Nic{
				Name:    "eth0-user",
				Machine: "machine-update-host-user",
			})
			assert.Loosely(t, err, should.BeNil)
			machine1 := &ufspb.Machine{
				Name: "machine-update-host-user",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			machineLSE1 := &ufspb.MachineLSE{
				Name:     "machinelse-update-host-user",
				Hostname: "machinelse-update-host-user",
				Machines: []string{"machine-update-host-user"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{
						Vms: []*ufspb.VM{
							{
								Name:       "vm1",
								MacAddress: "old_mac_address",
							},
						},
					},
				},
			}
			_, err = registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			vlan := &ufspb.Vlan{
				Name:        "vlan-1",
				VlanAddress: "192.168.40.0/22",
			}
			configuration.CreateVlan(ctx, vlan)
			ips, _, _, _, _, err := util.ParseVlan(vlan.GetName(), vlan.GetVlanAddress(), vlan.GetFreeStartIpv4Str(), vlan.GetFreeEndIpv4Str())
			assert.Loosely(t, err, should.BeNil)
			// Only import the first 20 as one single transaction cannot import all.
			_, err = configuration.ImportIPs(ctx, ips[0:20])
			assert.Loosely(t, err, should.BeNil)

			_, err = UpdateMachineLSEHost(ctx, machineLSE1.Name, &ufsAPI.NetworkOption{
				Vlan: "vlan-1",
				Ip:   "192.168.40.19",
				Nic:  "eth0-user",
			})
			assert.Loosely(t, err, should.BeNil)
			dhcp, err := configuration.GetDHCPConfig(ctx, "machinelse-update-host-user")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dhcp.GetIp(), should.Equal("192.168.40.19"))
			ips, err = configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.19"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(1))
			assert.Loosely(t, ips[0].GetOccupied(), should.BeTrue)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-update-host-user")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// nic & vlan & ip info are changed
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.nic"))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("eth0-user"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine_lse.vlan"))
			assert.Loosely(t, changes[1].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("vlan-1"))
			assert.Loosely(t, changes[2].GetEventLabel(), should.Equal("machine_lse.ip"))
			assert.Loosely(t, changes[2].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[2].GetNewValue(), should.Equal("192.168.40.19"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/hosts/machinelse-update-host-user")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("STATE_UNSPECIFIED"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("STATE_DEPLOYING"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/machinelse-update-host-user")
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
		})

		t.Run("Update machineLSE nic for host with existing dhcp record", func(t *ftt.Test) {
			nic0, err := registration.CreateNic(ctx, &ufspb.Nic{
				Name:       "eth0-nic-user",
				Machine:    "machine-update-host-nic-user",
				MacAddress: "eth0-nic-macaddress",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateNic(ctx, &ufspb.Nic{
				Name:       "eth1-nic-user",
				Machine:    "machine-update-host-nic-user",
				MacAddress: "eth1-nic-macaddress",
			})
			assert.Loosely(t, err, should.BeNil)
			machine1 := &ufspb.Machine{
				Name: "machine-update-host-nic-user",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			machineLSE1 := &ufspb.MachineLSE{
				Name:     "machinelse-update-host-nic-user",
				Hostname: "machinelse-update-host-nic-user",
				Machines: []string{"machine-update-host-nic-user"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
				Nic: "eth0-nic-user",
			}
			_, err = registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			dhcp := &ufspb.DHCPConfig{
				Hostname:   machineLSE1.GetName(),
				Ip:         "fake_ip",
				Vlan:       "fake_vlan",
				MacAddress: nic0.GetMacAddress(),
			}
			_, err = configuration.BatchUpdateDHCPs(ctx, []*ufspb.DHCPConfig{dhcp})
			assert.Loosely(t, err, should.BeNil)
			ip := &ufspb.IP{
				Id:       "test_ip_id",
				Ipv4:     12345,
				Ipv4Str:  dhcp.GetIp(),
				Vlan:     dhcp.GetVlan(),
				Occupied: true,
			}
			_, err = configuration.BatchUpdateIPs(ctx, []*ufspb.IP{ip})
			assert.Loosely(t, err, should.BeNil)

			_, err = UpdateMachineLSEHost(ctx, machineLSE1.Name, &ufsAPI.NetworkOption{
				Nic: "eth1-nic-user",
			})
			assert.Loosely(t, err, should.BeNil)

			// Verify nic change & dhcp change
			dhcp, err = configuration.GetDHCPConfig(ctx, "machinelse-update-host-nic-user")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dhcp.GetMacAddress(), should.Equal("eth1-nic-macaddress"))
			lse, _ := inventory.GetMachineLSE(ctx, "machinelse-update-host-nic-user")
			assert.Loosely(t, lse.GetNic(), should.Equal("eth1-nic-user"))
			ips, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "fake_ip"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(1))
			assert.Loosely(t, ips[0].GetOccupied(), should.BeTrue)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-update-host-nic-user")
			assert.Loosely(t, err, should.BeNil)
			// nic is changed
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.nic"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("eth0-nic-user"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("eth1-nic-user"))
		})

		t.Run("Update machineLSE Labstation without Servo Info", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-4x",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			labstationMachinelse1 := mockLabstationMachineLSE("RedLabstation-11")
			labstationMachinelse1.Machines = []string{"machine-4x"}
			_, err = inventory.CreateMachineLSE(ctx, labstationMachinelse1)
			assert.Loosely(t, err, should.BeNil)

			labstationMachinelse2 := mockLabstationMachineLSE("RedLabstation-11")
			labstationMachinelse2.Machines = []string{"machine-4x"}
			resp, err := UpdateMachineLSE(ctx, labstationMachinelse2, nil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(labstationMachinelse2))

			// No changes happened in this update
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/RedLabstation-11")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update a machineLSE with a machine already attached to a different machinelse - error", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-4",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-4",
				Machines: []string{"machine-4"},
			})
			assert.Loosely(t, err, should.BeNil)

			machine = &ufspb.Machine{
				Name: "machine-5",
			}
			_, err = registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-5",
				Machines: []string{"machine-5"},
			})
			assert.Loosely(t, err, should.BeNil)

			machineLSE := &ufspb.MachineLSE{
				Name:     "machinelse-4",
				Hostname: "machinelse-4",
				Machines: []string{"machine-5"},
			}
			resp, err := UpdateMachineLSE(ctx, machineLSE, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("there is another host machinelse-5 which is referring this machine machine-5"))

			_, err = state.GetStateRecord(ctx, "hosts/machinelse-4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Partial update machinelse", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-7",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			lse := &ufspb.MachineLSE{
				Name:     "lse-7",
				Tags:     []string{"tag-1"},
				Machines: []string{"machine-7"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			_, err = inventory.CreateMachineLSE(ctx, lse)
			assert.Loosely(t, err, should.BeNil)

			lse1 := &ufspb.MachineLSE{
				Name: "lse-7",
				Tags: []string{"tag-2"},
			}
			resp, err := UpdateMachineLSE(ctx, lse1, &field_mask.FieldMask{Paths: []string{"tags"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetTags(), should.Match([]string{"tag-1", "tag-2"}))
			assert.Loosely(t, resp.GetMachines(), should.Match([]string{"machine-7"}))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/lse-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})

		t.Run("Partially update machinelse virtual datacenter", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-vdc",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			lse := &ufspb.MachineLSE{
				Name:     "lse-vdc",
				Machines: []string{"machine-vdc"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{
						VirtualDatacenter: "oldvdc",
					},
				},
			}
			_, err = inventory.CreateMachineLSE(ctx, lse)
			assert.Loosely(t, err, should.BeNil)

			lse1 := &ufspb.MachineLSE{
				Name: "lse-vdc",
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{
						VirtualDatacenter: "newvdc",
					},
				},
			}
			resp, err := UpdateMachineLSE(ctx, lse1, &field_mask.FieldMask{Paths: []string{"virtualDatacenter"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetChromeBrowserMachineLse().GetVirtualDatacenter(), should.Equal("newvdc"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/lse-vdc")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.chrome_browser_machine_lse.virtual_datacenter"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("oldvdc"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("newvdc"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/lse-vdc")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})

		t.Run("Partially update machinelse without an lse", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-nolse",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			lse := &ufspb.MachineLSE{
				Name:     "no-lse",
				Machines: []string{"machine-nolse"},
			}
			_, err = inventory.CreateMachineLSE(ctx, lse)
			assert.Loosely(t, err, should.BeNil)

			lse1 := &ufspb.MachineLSE{
				Name: "no-lse",
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{
						OsVersion: &ufspb.OSVersion{
							Value: "windows-98",
							Image: "win98-floppy-img",
						},
					},
				},
			}
			resp, err := UpdateMachineLSE(ctx, lse1, &field_mask.FieldMask{Paths: []string{"osVersion", "osImage"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetChromeBrowserMachineLse().GetOsVersion().GetValue(), should.Equal("windows-98"))
			assert.Loosely(t, resp.GetChromeBrowserMachineLse().GetOsVersion().GetImage(), should.Equal("win98-floppy-img"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/no-lse")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.chrome_browser_machine_lse.os_version"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("<nil>"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(fmt.Sprintf("%v", &ufspb.OSVersion{
				Value: "windows-98",
				Image: "win98-floppy-img",
			})))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/no-lse")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})
	})

	ftt.Run("UpdateMachineLSE for an attached device machine", t, func(t *ftt.Test) {
		t.Run("Partial Update attached device host", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "adm-1",
				Device: &ufspb.Machine_AttachedDevice{
					AttachedDevice: &ufspb.AttachedDevice{
						DeviceType:   ufspb.AttachedDeviceType_ATTACHED_DEVICE_TYPE_APPLE_PHONE,
						Manufacturer: "test-man",
						BuildTarget:  "test-target",
						Model:        "test-model",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			lse := &ufspb.MachineLSE{
				Name:     "adh-lse-1",
				Machines: []string{"adm-1"},
				Lse: &ufspb.MachineLSE_AttachedDeviceLse{
					AttachedDeviceLse: &ufspb.AttachedDeviceLSE{
						OsVersion: &ufspb.OSVersion{
							Value: "test-os",
						},
						AssociatedHostname: "adm-1",
						AssociatedHostPort: "test-port-1",
					},
				},
				Schedulable: false,
			}
			_, err = inventory.CreateMachineLSE(ctx, lse)
			assert.Loosely(t, err, should.BeNil)

			lse1 := &ufspb.MachineLSE{
				Name:     "adh-lse-1",
				Machines: []string{"adm-1"},
				Lse: &ufspb.MachineLSE_AttachedDeviceLse{
					AttachedDeviceLse: &ufspb.AttachedDeviceLSE{
						OsVersion: &ufspb.OSVersion{
							Value: "test-os-2",
						},
						AssociatedHostname: "adm-1",
						AssociatedHostPort: "test-port-2",
					},
				},
				Schedulable: true,
			}
			resp, err := UpdateMachineLSE(ctx, lse1, &field_mask.FieldMask{Paths: []string{
				"osVersion",
				"assocHostname",
				"assocHostPort",
				"schedulable",
			}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetMachines(), should.Match([]string{"adm-1"}))
			assert.Loosely(t, resp.GetAttachedDeviceLse().GetOsVersion().GetValue(), should.Equal("test-os-2"))
			assert.Loosely(t, resp.GetAttachedDeviceLse().GetAssociatedHostname(), should.Equal("adm-1"))
			assert.Loosely(t, resp.GetAttachedDeviceLse().GetAssociatedHostPort(), should.Equal("test-port-2"))
			assert.Loosely(t, resp.GetSchedulable(), should.BeTrue)
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/adh-lse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})

		t.Run("Update attached device host with device labels", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "adm-2",
				Device: &ufspb.Machine_AttachedDevice{
					AttachedDevice: &ufspb.AttachedDevice{
						DeviceType:   ufspb.AttachedDeviceType_ATTACHED_DEVICE_TYPE_APPLE_PHONE,
						Manufacturer: "test-man",
						BuildTarget:  "test-target",
						Model:        "test-model",
					},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CROS_GOOGLER_DESK,
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.NoErr(t, err)

			lse := &ufspb.MachineLSE{
				Name:     "adh-lse-2",
				Machines: []string{"adm-2"},
				Lse: &ufspb.MachineLSE_AttachedDeviceLse{
					AttachedDeviceLse: &ufspb.AttachedDeviceLSE{
						OsVersion: &ufspb.OSVersion{
							Value: "test-os",
						},
						AssociatedHostname: "adm-2",
						AssociatedHostPort: "test-port-2",
					},
				},
				Schedulable: false,
			}
			_, err = inventory.CreateMachineLSE(ctx, lse)
			assert.NoErr(t, err)

			su1 := &ufspb.SchedulingUnit{
				Name:        "adh-su-2",
				MachineLSEs: []string{"adh-lse-2"},
				ExposeType:  ufspb.SchedulingUnit_DEFAULT,
			}
			_, err = inventory.CreateSchedulingUnit(ctx, su1)
			assert.NoErr(t, err)

			resp, err := inventory.GetDeviceLabels(ctx, util.AddPrefix(util.SchedulingUnitCollection, "adh-su-2"))
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)

			lse1 := &ufspb.MachineLSE{
				Name:     "adh-lse-2",
				Machines: []string{"adm-2"},
				Lse: &ufspb.MachineLSE_AttachedDeviceLse{
					AttachedDeviceLse: &ufspb.AttachedDeviceLSE{
						OsVersion: &ufspb.OSVersion{
							Value: "test-os-2",
						},
						AssociatedHostname: "adm-3",
						AssociatedHostPort: "test-port-3",
					},
				},
				Schedulable: true,
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
			_, err = UpdateMachineLSE(ctx, lse1, &field_mask.FieldMask{Paths: []string{
				"osVersion",
				"assocHostname",
				"assocHostPort",
				"schedulable",
			}})
			assert.NoErr(t, err)
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/adh-lse-2")
			assert.NoErr(t, err)
			assert.Loosely(t, msgs, should.HaveLength(1))
			// Scheduling Unit labels
			resp, err = inventory.GetDeviceLabels(ctx, util.AddPrefix(util.SchedulingUnitCollection, "adh-su-2"))
			assert.NoErr(t, err)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.That(t, resp.GetName(), should.Equal("schedulingunits/adh-su-2"))
			assert.That(t, resp.GetResourceType(), should.Equal(ufspb.ResourceType_RESOURCE_TYPE_SCHEDULING_UNIT))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "devicelabels/schedulingunits/adh-su-2")
			assert.NoErr(t, err)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.That(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.That(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.That(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
		})
	})
}

func TestUpdateLabMeta(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateLabMeta for an OS machine lse", t, func(t *ftt.Test) {
		t.Run("Update a non-OS machine lse", func(t *ftt.Test) {
			machineLSE1 := &ufspb.MachineLSE{
				Name:     "machinelse-labmeta-1",
				Hostname: "machinelse-labmeta-1",
				Machines: []string{"machine-labmeta1"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-labmeta1",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)

			err = UpdateLabMeta(ctx, &ufspb.LabMeta{
				ChromeosDeviceId: "machine-labmeta1",
				Hostname:         "machinelse-labmeta-1",
				SmartUsbhub:      true,
			})
			// Update is skipped without error
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Update a OS machine lse - happy path", func(t *ftt.Test) {
			machineLSE1 := mockDutMachineLSE("machinelse-labmeta-2")
			machineLSE1.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)

			topology := &chromeosLab.ServoTopology{
				Main: &chromeosLab.ServoTopologyItem{Type: "v3"},
			}
			err = UpdateLabMeta(ctx, &ufspb.LabMeta{
				ChromeosDeviceId: "machine-labmeta1",
				Hostname:         "machinelse-labmeta-2",
				SmartUsbhub:      true,
				ServoType:        "fake-type",
				ServoTopology:    topology,
			})
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, "machinelse-labmeta-2")
			assert.Loosely(t, err, should.BeNil)
			peri := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.Equal("fake-type"))
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology))
			assert.Loosely(t, peri.Servo.GetServoComponent(), should.Match([]string{"v3"}))
		})

		t.Run("Update a OS machine lse - empty servo topology", func(t *ftt.Test) {
			machineLSE2 := mockDutMachineLSE("machinelse-labmeta-3")
			machineLSE2.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)

			topology := &chromeosLab.ServoTopology{
				Main: &chromeosLab.ServoTopologyItem{Type: "v3"},
			}
			err = UpdateLabMeta(ctx, &ufspb.LabMeta{
				ChromeosDeviceId: "machine-labmeta1",
				Hostname:         "machinelse-labmeta-3",
				SmartUsbhub:      true,
				ServoType:        "fake-type",
				ServoTopology:    topology,
			})
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, "machinelse-labmeta-3")
			assert.Loosely(t, err, should.BeNil)
			peri := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.Equal("fake-type"))
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology))
			assert.Loosely(t, peri.Servo.GetServoComponent(), should.Match([]string{"v3"}))
		})

		t.Run("Update a OS machine lse - two servo components", func(t *ftt.Test) {
			machineLSE := mockDutMachineLSE("machinelse-labmeta-4")
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)

			topology := &chromeosLab.ServoTopology{
				Main: &chromeosLab.ServoTopologyItem{Type: "v34"},
			}
			err = UpdateLabMeta(ctx, &ufspb.LabMeta{
				ChromeosDeviceId: "machine-labmeta1",
				Hostname:         "machinelse-labmeta-4",
				SmartUsbhub:      true,
				ServoType:        "fake-type_with_foo",
				ServoTopology:    topology,
			})
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, "machinelse-labmeta-4")
			assert.Loosely(t, err, should.BeNil)
			peri := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.Equal("fake-type_with_foo"))
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology))
			assert.Loosely(t, peri.Servo.GetServoComponent(), should.Match([]string{"v34"}))
		})

		t.Run("Update a OS machine lse - with three servo componments", func(t *ftt.Test) {
			machineLSE := mockDutMachineLSE("machinelse-labmeta-5")
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)

			topology := &chromeosLab.ServoTopology{
				Main:     &chromeosLab.ServoTopologyItem{Type: "foo"},
				Children: []*chromeosLab.ServoTopologyItem{{Type: "fake"}},
			}
			err = UpdateLabMeta(ctx, &ufspb.LabMeta{
				ChromeosDeviceId: "machine-labmeta1",
				Hostname:         "machinelse-labmeta-5",
				SmartUsbhub:      true,
				ServoType:        "fake-type_with_foo_and_bar",
				ServoTopology:    topology,
			})
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, "machinelse-labmeta-5")
			assert.Loosely(t, err, should.BeNil)
			peri := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.Equal("fake-type_with_foo_and_bar"))
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology))
			assert.Loosely(t, peri.Servo.GetServoComponent(), should.Match([]string{"foo", "fake"}))
		})

		t.Run("Update a OS machine lse - with no servo_type", func(t *ftt.Test) {
			machineLSE := mockDutMachineLSE("machinelse-labmeta-6")
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)

			topology := &chromeosLab.ServoTopology{}
			err = UpdateLabMeta(ctx, &ufspb.LabMeta{
				ChromeosDeviceId: "machine-labmeta1",
				Hostname:         "machinelse-labmeta-6",
				SmartUsbhub:      true,
				ServoTopology:    topology,
			})
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, "machinelse-labmeta-6")
			assert.Loosely(t, err, should.BeNil)
			peri := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.BeEmpty)
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology))
			assert.Loosely(t, len(peri.Servo.GetServoComponent()), should.BeZero)
		})
	})
}

func TestUpdateRecoveryLabData(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateRecoveryLabData for an OS machine lse", t, func(t *ftt.Test) {
		t.Run("Update a non-OS machine LSE", func(t *ftt.Test) {
			const machineName = "machine-labdata-1"
			machineLSE1 := &ufspb.MachineLSE{
				Name:     machineName,
				Hostname: machineName,
				Machines: []string{"machine-labdata1"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: machineName,
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)

			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_UNSPECIFIED, &ufsAPI.ChromeOsRecoveryData_LabData{
				SmartUsbhub: true,
			})
			// Update is skipped without error
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Update a OS machine LSE - successful path", func(t *ftt.Test) {
			const machineName = "machine-labdata-2"
			topology := &chromeosLab.ServoTopology{
				Main: &chromeosLab.ServoTopologyItem{
					Type: "v3",
				},
			}
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				SmartUsbhub:   true,
				ServoType:     "fake-type",
				ServoTopology: topology,
				WifiRouters: []*ufsAPI.ChromeOsRecoveryData_WifiRouter{
					{
						Hostname: machineName + "-router",
						State:    chromeosLab.PeripheralState_WORKING,
					},
					{
						Hostname: machineName + "-pcap",
						State:    chromeosLab.PeripheralState_WORKING,
					},
				},
				ModemInfo: &ufsAPI.ChromeOsRecoveryData_ModemInfo{
					ModelVariant: "some_cellular_variant",
					Imei:         "654321",
					Type:         chromeosLab.ModemType_MODEM_TYPE_NL668,
				},
				SupportedCarriers: []string{
					"ATT", "VERIZON", "TMOBILE",
				},
				SimInfos: []*chromeosLab.SIMInfo{
					{
						SlotId: 1,
						Type:   chromeosLab.SIMType_SIM_PHYSICAL,
						Eid:    "0000",
						ProfileInfo: []*chromeosLab.SIMProfileInfo{
							{
								Iccid:       "1111",
								SimPin:      "2222",
								SimPuk:      "3333",
								CarrierName: chromeosLab.NetworkProvider_NETWORK_TMOBILE,
								OwnNumber:   "12345",
							},
						},
					},
				},
				ServoUsbDrive: &labApi.UsbDrive{
					Serial:        "usb-drive serial",
					Manufacturer:  "usb-drive-make",
					FirstSeenTime: timestamppb.Now(),
				},
				AudioboxJackpluggerState: chromeosLab.Chameleon_AUDIOBOX_JACKPLUGGER_WORKING,
			}
			machineLSE1 := mockDutMachineLSE(machineName)
			machineLSE1.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
				Wifi: &chromeosLab.Wifi{
					WifiRouters: []*chromeosLab.WifiRouter{
						{
							Hostname: machineName + "-pcap",
							State:    chromeosLab.PeripheralState_BROKEN,
						},
						{
							Hostname: machineName + "-router",
							State:    chromeosLab.PeripheralState_BROKEN,
						},
					},
				},
				Chameleon: &chromeosLab.Chameleon{
					Hostname:            machineName + "-chameleon",
					AudioboxJackplugger: chromeosLab.Chameleon_AUDIOBOX_JACKPLUGGER_BROKEN,
				},
			}
			machineLSE1.GetChromeosMachineLse().GetDeviceLse().GetDut().Modeminfo = &chromeosLab.ModemInfo{
				Type:           chromeosLab.ModemType_MODEM_TYPE_QUALCOMM_SC7180,
				Imei:           "123456",
				SupportedBands: "1,2,3,4",
				SimCount:       3,
				ModelVariant:   "some_other_cellular_variant",
			}

			req, err := inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetWifi().GetWifiRouters()[0].GetState(), should.Equal(chromeosLab.PeripheralState_BROKEN))
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_UNSPECIFIED))

			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_REPAIR_FAILED, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_REPAIR_FAILED))
			peri := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.Equal("fake-type"))
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology))
			assert.Loosely(t, peri.Servo.GetServoComponent(), should.Match([]string{"v3"}))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[0].GetHostname(), should.Equal(machineName+"-pcap"))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[0].GetState(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[1].GetHostname(), should.Equal(machineName+"-router"))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[1].GetState(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, peri.GetSupportedCarriers()[0], should.Equal("ATT"))
			assert.Loosely(t, peri.GetSupportedCarriers()[1], should.Equal("VERIZON"))
			assert.Loosely(t, peri.GetSupportedCarriers()[2], should.Equal("TMOBILE"))
			assert.Loosely(t, peri.Servo.GetUsbDrive().GetSerial(), should.Equal("usb-drive serial"))
			assert.Loosely(t, peri.Servo.GetUsbDrive().GetManufacturer(), should.Equal("usb-drive-make"))
			assert.Loosely(t, peri.Chameleon.GetHostname(), should.Equal(machineName+"-chameleon"))
			assert.Loosely(t, peri.Chameleon.GetAudioboxJackplugger(), should.Equal(chromeosLab.Chameleon_AUDIOBOX_JACKPLUGGER_WORKING))

			modem := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetModeminfo()
			assert.Loosely(t, modem.GetType(), should.Equal(chromeosLab.ModemType_MODEM_TYPE_NL668))
			assert.Loosely(t, modem.GetImei(), should.Equal("654321"))
			assert.Loosely(t, modem.GetSupportedBands(), should.Equal("1,2,3,4"))
			assert.Loosely(t, modem.GetSimCount(), should.Equal(3))
			assert.Loosely(t, modem.GetModelVariant(), should.Equal("some_cellular_variant"))
		})

		t.Run("Update a OS machine LSE - empty servo topology and multiple wifi routers", func(t *ftt.Test) {
			const machineName = "machine-labdata-3"
			topology := &chromeosLab.ServoTopology{
				Main: &chromeosLab.ServoTopologyItem{
					Type: "v4",
				},
				Children: []*chromeosLab.ServoTopologyItem{
					{
						Type: "c2d2",
					},
				},
			}
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				SmartUsbhub:   true,
				ServoType:     "fake-type",
				ServoTopology: topology,
				WifiRouters: []*ufsAPI.ChromeOsRecoveryData_WifiRouter{
					{
						Hostname: machineName + "-router",
						State:    chromeosLab.PeripheralState_WORKING,
					},
					{
						Hostname: machineName + "-pcap",
						State:    chromeosLab.PeripheralState_WORKING,
					},
				},
			}
			machineLSE2 := mockDutMachineLSE(machineName)
			machineLSE2.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
				Wifi: &chromeosLab.Wifi{
					WifiRouters: []*chromeosLab.WifiRouter{
						{
							Hostname: machineName + "-router",
							State:    chromeosLab.PeripheralState_BROKEN,
						},
						{
							Hostname: machineName + "-pcap",
							State:    chromeosLab.PeripheralState_BROKEN,
						},
					},
				},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)

			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_READY, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			peri := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.Equal("fake-type"))
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology))
			assert.Loosely(t, peri.Servo.GetServoComponent(), should.Match([]string{"v4", "c2d2"}))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[0].GetHostname(), should.Equal(machineName+"-router"))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[0].GetState(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[1].GetHostname(), should.Equal(machineName+"-pcap"))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[1].GetState(), should.Equal(chromeosLab.PeripheralState_WORKING))
		})
		t.Run("Update a OS machine LSE - resource state", func(t *ftt.Test) {
			const machineName = "machine-labdata-4"
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{}
			machineLSE2 := mockDutMachineLSE(machineName)
			machineLSE2.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
				Wifi: &chromeosLab.Wifi{
					WifiRouters: []*chromeosLab.WifiRouter{
						{
							Hostname: machineName + "-router",
							State:    chromeosLab.PeripheralState_BROKEN,
						},
						{
							Hostname: machineName + "-pcap",
							State:    chromeosLab.PeripheralState_BROKEN,
						},
					},
				},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)

			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_READY, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_READY))
		})
		t.Run("Update a OS machine LSE - two servo components", func(t *ftt.Test) {
			const machineName = "machine-labdata-5"
			topology := &chromeosLab.ServoTopology{
				Main:     &chromeosLab.ServoTopologyItem{Type: "v4"},
				Children: []*chromeosLab.ServoTopologyItem{{Type: "v4"}},
			}
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				SmartUsbhub:   true,
				ServoType:     "fake-type_with_foo",
				ServoTopology: topology,
			}
			machineLSE := mockDutMachineLSE(machineName)
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)

			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_UNSPECIFIED, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			peri := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.Equal("fake-type_with_foo"))
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology))
			assert.Loosely(t, peri.Servo.GetServoComponent(), should.Match([]string{"v4"}))
		})
		t.Run("Update a OS machine LSE - three servo components", func(t *ftt.Test) {
			const machineName = "machine-labdata-6"
			topology := &chromeosLab.ServoTopology{
				Main:     &chromeosLab.ServoTopologyItem{Type: "v4"},
				Children: []*chromeosLab.ServoTopologyItem{{Type: "v5"}},
			}
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				SmartUsbhub:   true,
				ServoType:     "fake-type_with_foo_and_bar",
				ServoTopology: topology,
			}
			machineLSE := mockDutMachineLSE(machineName)
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)

			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_UNSPECIFIED, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			peri := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.Equal("fake-type_with_foo_and_bar"))
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology))
			assert.Loosely(t, peri.Servo.GetServoComponent(), should.Match([]string{"v4", "v5"}))
		})
		t.Run("Update a OS machine LSE with no servo_type", func(t *ftt.Test) {
			const machineName = "machine-labdata-7"
			topology := &chromeosLab.ServoTopology{}
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				SmartUsbhub:   true,
				ServoTopology: topology,
				ServoType:     "fake-type_with_foo_and_bar",
			}
			machineLSE := mockDutMachineLSE(machineName)
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)

			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_UNSPECIFIED, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			peri := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.Equal("fake-type_with_foo_and_bar"))
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology))
			assert.Loosely(t, len(peri.Servo.GetServoComponent()), should.BeZero)
		})
		t.Run("Update a OS machine LSE - wifi router state update, ResourceState update", func(t *ftt.Test) {
			const machineName = "machine-labdata-8"
			topology := &chromeosLab.ServoTopology{}
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				SmartUsbhub:   true,
				ServoType:     "fake-type",
				ServoTopology: topology,
				WifiRouters: []*ufsAPI.ChromeOsRecoveryData_WifiRouter{
					{
						Hostname: machineName + "-router",
						State:    chromeosLab.PeripheralState_WORKING,
					},
					{
						Hostname: machineName + "-pcap",
						State:    chromeosLab.PeripheralState_WORKING,
					},
				},
			}
			machineLSE2 := mockDutMachineLSE(machineName)
			machineLSE2.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = &chromeosLab.Peripherals{
				Servo: &chromeosLab.Servo{},
				Wifi: &chromeosLab.Wifi{
					WifiRouters: []*chromeosLab.WifiRouter{
						{
							Hostname: machineName + "-pcap",
							State:    chromeosLab.PeripheralState_BROKEN,
						},
						{
							Hostname: machineName + "-router",
							State:    chromeosLab.PeripheralState_BROKEN,
						},
					},
				},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_UNSPECIFIED))
			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_READY, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_READY))

			peri := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.Equal("fake-type"))
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology))
			assert.Loosely(t, len(peri.Servo.GetServoComponent()), should.BeZero)
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[0].GetHostname(), should.Equal(machineName+"-pcap"))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[0].GetState(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[1].GetHostname(), should.Equal(machineName+"-router"))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[1].GetState(), should.Equal(chromeosLab.PeripheralState_WORKING))
			topology2 := &chromeosLab.ServoTopology{
				Main: &chromeosLab.ServoTopologyItem{Type: "hi"},
			}
			labData = &ufsAPI.ChromeOsRecoveryData_LabData{
				SmartUsbhub:   true,
				ServoType:     "fake-type",
				ServoTopology: topology2,
				WifiRouters: []*ufsAPI.ChromeOsRecoveryData_WifiRouter{
					{
						Hostname: machineName + "-router",
						State:    chromeosLab.PeripheralState_WORKING,
					},
					{
						Hostname: machineName + "-pcap-new",
						State:    chromeosLab.PeripheralState_WORKING,
					},
				},
			}
			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_READY, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			peri = req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
			assert.Loosely(t, peri.GetSmartUsbhub(), should.BeTrue)
			assert.Loosely(t, peri.Servo.GetServoType(), should.Equal("fake-type"))
			assert.Loosely(t, peri.Servo.GetServoTopology(), should.Match(topology2))
			assert.Loosely(t, peri.Servo.GetServoComponent(), should.Match([]string{"hi"}))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[0].GetHostname(), should.Equal(machineName+"-router"))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[0].GetState(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[1].GetHostname(), should.Equal(machineName+"-pcap-new"))
			assert.Loosely(t, peri.GetWifi().GetWifiRouters()[1].GetState(), should.Equal(chromeosLab.PeripheralState_WORKING))
		})
		t.Run("Update a OS machine LSE - labstation with no labdata", func(t *ftt.Test) {
			const machineName = "machine-labdata-9"
			machineLSE := mockLabstationMachineLSE(machineName)
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut(), should.BeNil)
			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_SERVING, &ufsAPI.ChromeOsRecoveryData_LabData{})
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_SERVING))
		})
		t.Run("Update a OS machine LSE - empty modem recovery data", func(t *ftt.Test) {
			const machineName = "machine-labdata-10"
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				ModemInfo: &ufsAPI.ChromeOsRecoveryData_ModemInfo{
					ModelVariant: "",
					Imei:         "",
					Type:         chromeosLab.ModemType_MODEM_TYPE_UNSPECIFIED,
				},
			}
			machineLSE := mockDutMachineLSE(machineName)
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Modeminfo = &chromeosLab.ModemInfo{
				Type:           chromeosLab.ModemType_MODEM_TYPE_QUALCOMM_SC7180,
				Imei:           "123456",
				SupportedBands: "1,2,3,4",
				SimCount:       3,
				ModelVariant:   "some_cellular_variant",
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_UNSPECIFIED))
			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_READY, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_READY))

			modem := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetModeminfo()
			assert.Loosely(t, modem.GetType(), should.Equal(chromeosLab.ModemType_MODEM_TYPE_QUALCOMM_SC7180))
			assert.Loosely(t, modem.GetImei(), should.Equal("123456"))
			assert.Loosely(t, modem.GetSupportedBands(), should.Equal("1,2,3,4"))
			assert.Loosely(t, modem.GetSimCount(), should.Equal(3))
			assert.Loosely(t, modem.GetModelVariant(), should.Equal("some_cellular_variant"))
		})
		t.Run("Update a OS machine LSE - missing modem recovery data", func(t *ftt.Test) {
			const machineName = "machine-labdata-11"
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				ModemInfo: nil,
			}
			machineLSE := mockDutMachineLSE(machineName)
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Modeminfo = &chromeosLab.ModemInfo{
				Type:           chromeosLab.ModemType_MODEM_TYPE_QUALCOMM_SC7180,
				Imei:           "123456",
				SupportedBands: "1,2,3,4",
				SimCount:       3,
				ModelVariant:   "some_cellular_variant",
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_UNSPECIFIED))
			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_READY, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_READY))

			modem := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetModeminfo()
			assert.Loosely(t, modem.GetType(), should.Equal(chromeosLab.ModemType_MODEM_TYPE_QUALCOMM_SC7180))
			assert.Loosely(t, modem.GetImei(), should.Equal("123456"))
			assert.Loosely(t, modem.GetSupportedBands(), should.Equal("1,2,3,4"))
			assert.Loosely(t, modem.GetSimCount(), should.Equal(3))
			assert.Loosely(t, modem.GetModelVariant(), should.Equal("some_cellular_variant"))
		})
		t.Run("Update a OS machine LSE - missing SIM info in lab data", func(t *ftt.Test) {
			const machineName = "machine-labdata-12"
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				SimInfos: nil,
			}
			machineLSE := mockDutMachineLSE(machineName)
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Siminfo = []*chromeosLab.SIMInfo{
				{
					SlotId:   1,
					Type:     chromeosLab.SIMType_SIM_DIGITAL,
					Eid:      "123456",
					TestEsim: true,
					ProfileInfo: []*chromeosLab.SIMProfileInfo{
						{
							Iccid:       "654321",
							SimPin:      "1111",
							SimPuk:      "2222",
							OwnNumber:   "3333",
							CarrierName: chromeosLab.NetworkProvider_NETWORK_ATT,
							State:       chromeosLab.SIMProfileInfo_WORKING,
							Features: []chromeosLab.SIMProfileInfo_Feature{
								chromeosLab.SIMProfileInfo_FEATURE_UNSPECIFIED,
								chromeosLab.SIMProfileInfo_FEATURE_LIVE_NETWORK,
								chromeosLab.SIMProfileInfo_FEATURE_SMS,
							},
						},
					},
				},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_UNSPECIFIED))
			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_READY, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_READY))

			si := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetSiminfo()
			assert.Loosely(t, len(si), should.Equal(1))
			assert.Loosely(t, si[0].GetSlotId(), should.Equal(1))
			assert.Loosely(t, si[0].GetType(), should.Equal(chromeosLab.SIMType_SIM_DIGITAL))
			assert.Loosely(t, si[0].GetEid(), should.Equal("123456"))
			assert.Loosely(t, si[0].GetTestEsim(), should.Equal(true))

			pi := si[0].GetProfileInfo()
			assert.Loosely(t, len(pi), should.Equal(1))
			assert.Loosely(t, pi[0].GetIccid(), should.Equal("654321"))
			assert.Loosely(t, pi[0].GetSimPin(), should.Equal("1111"))
			assert.Loosely(t, pi[0].GetSimPuk(), should.Equal("2222"))
			assert.Loosely(t, pi[0].GetOwnNumber(), should.Equal("3333"))
			assert.Loosely(t, pi[0].GetCarrierName(), should.Equal(chromeosLab.NetworkProvider_NETWORK_ATT))
			assert.Loosely(t, pi[0].GetState(), should.Equal(chromeosLab.SIMProfileInfo_WORKING))

			features := pi[0].GetFeatures()
			assert.Loosely(t, features[0], should.Equal(chromeosLab.SIMProfileInfo_FEATURE_UNSPECIFIED))
			assert.Loosely(t, features[1], should.Equal(chromeosLab.SIMProfileInfo_FEATURE_LIVE_NETWORK))
			assert.Loosely(t, features[2], should.Equal(chromeosLab.SIMProfileInfo_FEATURE_SMS))
		})
		t.Run("Update a OS machine LSE - missing SIM info in machine lse", func(t *ftt.Test) {
			const machineName = "machine-labdata-13"
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				SimInfos: []*chromeosLab.SIMInfo{
					{
						SlotId:   1,
						Type:     chromeosLab.SIMType_SIM_DIGITAL,
						Eid:      "123456",
						TestEsim: true,
						ProfileInfo: []*chromeosLab.SIMProfileInfo{
							{
								Iccid:       "654321",
								SimPin:      "1111",
								SimPuk:      "2222",
								OwnNumber:   "3333",
								CarrierName: chromeosLab.NetworkProvider_NETWORK_ATT,
								State:       chromeosLab.SIMProfileInfo_BROKEN,
								Features: []chromeosLab.SIMProfileInfo_Feature{
									chromeosLab.SIMProfileInfo_FEATURE_LIVE_NETWORK,
								},
							},
						},
					},
				},
			}
			machineLSE := mockDutMachineLSE(machineName)
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Siminfo = nil
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_UNSPECIFIED))
			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_READY, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_READY))

			si := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetSiminfo()
			assert.Loosely(t, len(si), should.Equal(1))
			assert.Loosely(t, si[0].GetSlotId(), should.Equal(1))
			assert.Loosely(t, si[0].GetType(), should.Equal(chromeosLab.SIMType_SIM_DIGITAL))
			assert.Loosely(t, si[0].GetEid(), should.Equal("123456"))
			assert.Loosely(t, si[0].GetTestEsim(), should.Equal(true))

			pi := si[0].GetProfileInfo()
			assert.Loosely(t, len(pi), should.Equal(1))
			assert.Loosely(t, pi[0].GetIccid(), should.Equal("654321"))
			assert.Loosely(t, pi[0].GetSimPin(), should.Equal("1111"))
			assert.Loosely(t, pi[0].GetSimPuk(), should.Equal("2222"))
			assert.Loosely(t, pi[0].GetOwnNumber(), should.Equal("3333"))
			assert.Loosely(t, pi[0].GetCarrierName(), should.Equal(chromeosLab.NetworkProvider_NETWORK_ATT))
			assert.Loosely(t, pi[0].GetState(), should.Equal(chromeosLab.SIMProfileInfo_BROKEN))

			features := pi[0].GetFeatures()
			assert.Loosely(t, features[0], should.Equal(chromeosLab.SIMProfileInfo_FEATURE_LIVE_NETWORK))
		})
		t.Run("Update a OS machine LSE - missing SIM slot", func(t *ftt.Test) {
			const machineName = "machine-labdata-14"
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				SimInfos: []*chromeosLab.SIMInfo{
					{
						SlotId: 1,
						Type:   chromeosLab.SIMType_SIM_PHYSICAL,
						Eid:    "0000",
						ProfileInfo: []*chromeosLab.SIMProfileInfo{
							{
								Iccid:       "4444",
								SimPin:      "3333",
								SimPuk:      "2222",
								OwnNumber:   "1111",
								CarrierName: chromeosLab.NetworkProvider_NETWORK_TMOBILE,
								State:       chromeosLab.SIMProfileInfo_NO_NETWORK,
								Features: []chromeosLab.SIMProfileInfo_Feature{
									chromeosLab.SIMProfileInfo_FEATURE_LIVE_NETWORK,
								},
							},
						},
					},
				},
			}
			machineLSE := mockDutMachineLSE(machineName)
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Siminfo = []*chromeosLab.SIMInfo{
				{
					SlotId:   1,
					Type:     chromeosLab.SIMType_SIM_DIGITAL,
					Eid:      "123456",
					TestEsim: true,
					ProfileInfo: []*chromeosLab.SIMProfileInfo{
						{
							Iccid:       "654321",
							SimPin:      "1111",
							SimPuk:      "2222",
							OwnNumber:   "3333",
							CarrierName: chromeosLab.NetworkProvider_NETWORK_ATT,
							State:       chromeosLab.SIMProfileInfo_BROKEN,
							Features: []chromeosLab.SIMProfileInfo_Feature{
								chromeosLab.SIMProfileInfo_FEATURE_UNSPECIFIED,
							},
						},
						{
							Iccid:       "123456",
							SimPin:      "1111",
							SimPuk:      "2222",
							OwnNumber:   "3333",
							CarrierName: chromeosLab.NetworkProvider_NETWORK_ATT,
							State:       chromeosLab.SIMProfileInfo_BROKEN,
							Features: []chromeosLab.SIMProfileInfo_Feature{
								chromeosLab.SIMProfileInfo_FEATURE_SMS,
							},
						},
					},
				},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_UNSPECIFIED))
			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_READY, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_READY))

			si := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetSiminfo()
			assert.Loosely(t, len(si), should.Equal(1))
			assert.Loosely(t, si[0].GetSlotId(), should.Equal(1))
			assert.Loosely(t, si[0].GetType(), should.Equal(chromeosLab.SIMType_SIM_PHYSICAL))
			assert.Loosely(t, si[0].GetEid(), should.Equal("0000"))
			assert.Loosely(t, si[0].GetTestEsim(), should.Equal(false))

			pi := si[0].GetProfileInfo()
			assert.Loosely(t, len(pi), should.Equal(1))
			assert.Loosely(t, pi[0].GetIccid(), should.Equal("4444"))
			assert.Loosely(t, pi[0].GetSimPin(), should.Equal("3333"))
			assert.Loosely(t, pi[0].GetSimPuk(), should.Equal("2222"))
			assert.Loosely(t, pi[0].GetOwnNumber(), should.Equal("1111"))
			assert.Loosely(t, pi[0].GetCarrierName(), should.Equal(chromeosLab.NetworkProvider_NETWORK_TMOBILE))
			assert.Loosely(t, pi[0].GetState(), should.Equal(chromeosLab.SIMProfileInfo_NO_NETWORK))

			features := pi[0].GetFeatures()
			assert.Loosely(t, features[0], should.Equal(chromeosLab.SIMProfileInfo_FEATURE_LIVE_NETWORK))
		})
		t.Run("Update a OS machine LSE - Add one profile and skip one", func(t *ftt.Test) {
			const machineName = "machine-labdata-15"
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				SimInfos: []*chromeosLab.SIMInfo{
					{
						SlotId: 2,
						Type:   chromeosLab.SIMType_SIM_PHYSICAL,
						Eid:    "0000",
						ProfileInfo: []*chromeosLab.SIMProfileInfo{
							{
								Iccid:       "1111",
								SimPin:      "2222",
								SimPuk:      "3333",
								OwnNumber:   "4444",
								CarrierName: chromeosLab.NetworkProvider_NETWORK_TMOBILE,
								State:       chromeosLab.SIMProfileInfo_BROKEN,
								Features: []chromeosLab.SIMProfileInfo_Feature{
									chromeosLab.SIMProfileInfo_FEATURE_SMS,
								},
							},
						},
					},
				},
			}
			machineLSE := mockDutMachineLSE(machineName)
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Siminfo = []*chromeosLab.SIMInfo{
				{
					SlotId:   1,
					Type:     chromeosLab.SIMType_SIM_DIGITAL,
					Eid:      "123456",
					TestEsim: true,
					ProfileInfo: []*chromeosLab.SIMProfileInfo{
						{
							Iccid:       "654321",
							SimPin:      "1111",
							SimPuk:      "2222",
							OwnNumber:   "3333",
							CarrierName: chromeosLab.NetworkProvider_NETWORK_ATT,
							State:       chromeosLab.SIMProfileInfo_WORKING,
							Features: []chromeosLab.SIMProfileInfo_Feature{
								chromeosLab.SIMProfileInfo_FEATURE_UNSPECIFIED,
							},
						},
						{
							Iccid:       "123456",
							SimPin:      "2222",
							SimPuk:      "1111",
							OwnNumber:   "0000",
							CarrierName: chromeosLab.NetworkProvider_NETWORK_VERIZON,
							State:       chromeosLab.SIMProfileInfo_UNSPECIFIED,
							Features: []chromeosLab.SIMProfileInfo_Feature{
								chromeosLab.SIMProfileInfo_FEATURE_LIVE_NETWORK,
							},
						},
					},
				},
			}
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_UNSPECIFIED))
			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_READY, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_READY))

			si := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetSiminfo()
			assert.Loosely(t, len(si), should.Equal(2))
			assert.Loosely(t, si[0].GetSlotId(), should.Equal(1))
			assert.Loosely(t, si[0].GetType(), should.Equal(chromeosLab.SIMType_SIM_DIGITAL))
			assert.Loosely(t, si[0].GetEid(), should.Equal("123456"))
			assert.Loosely(t, si[0].GetTestEsim(), should.Equal(true))

			// Check profiles on SIM slot 1
			pi1 := si[0].GetProfileInfo()
			assert.Loosely(t, len(pi1), should.Equal(2))
			assert.Loosely(t, pi1[0].GetIccid(), should.Equal("654321"))
			assert.Loosely(t, pi1[0].GetSimPin(), should.Equal("1111"))
			assert.Loosely(t, pi1[0].GetSimPuk(), should.Equal("2222"))
			assert.Loosely(t, pi1[0].GetOwnNumber(), should.Equal("3333"))
			assert.Loosely(t, pi1[0].GetCarrierName(), should.Equal(chromeosLab.NetworkProvider_NETWORK_ATT))
			assert.Loosely(t, pi1[0].GetState(), should.Equal(chromeosLab.SIMProfileInfo_WORKING))
			assert.Loosely(t, pi1[0].GetFeatures()[0], should.Equal(chromeosLab.SIMProfileInfo_FEATURE_UNSPECIFIED))
			assert.Loosely(t, pi1[1].GetIccid(), should.Equal("123456"))
			assert.Loosely(t, pi1[1].GetSimPin(), should.Equal("2222"))
			assert.Loosely(t, pi1[1].GetSimPuk(), should.Equal("1111"))
			assert.Loosely(t, pi1[1].GetOwnNumber(), should.Equal("0000"))
			assert.Loosely(t, pi1[1].GetCarrierName(), should.Equal(chromeosLab.NetworkProvider_NETWORK_VERIZON))
			assert.Loosely(t, pi1[1].GetState(), should.Equal(chromeosLab.SIMProfileInfo_UNSPECIFIED))
			assert.Loosely(t, pi1[1].GetFeatures()[0], should.Equal(chromeosLab.SIMProfileInfo_FEATURE_LIVE_NETWORK))

			// Check profiles on SIM slot 2
			assert.Loosely(t, si[1].GetSlotId(), should.Equal(2))
			assert.Loosely(t, si[1].GetType(), should.Equal(chromeosLab.SIMType_SIM_PHYSICAL))
			assert.Loosely(t, si[1].GetEid(), should.Equal("0000"))
			assert.Loosely(t, si[1].GetTestEsim(), should.Equal(false))

			pi2 := si[1].GetProfileInfo()
			assert.Loosely(t, len(pi2), should.Equal(1))
			assert.Loosely(t, pi2[0].GetIccid(), should.Equal("1111"))
			assert.Loosely(t, pi2[0].GetSimPin(), should.Equal("2222"))
			assert.Loosely(t, pi2[0].GetSimPuk(), should.Equal("3333"))
			assert.Loosely(t, pi2[0].GetOwnNumber(), should.Equal("4444"))
			assert.Loosely(t, pi2[0].GetCarrierName(), should.Equal(chromeosLab.NetworkProvider_NETWORK_TMOBILE))
			assert.Loosely(t, pi2[0].GetState(), should.Equal(chromeosLab.SIMProfileInfo_BROKEN))
			assert.Loosely(t, pi2[0].GetFeatures()[0], should.Equal(chromeosLab.SIMProfileInfo_FEATURE_SMS))
		})
		t.Run("Update a OS machine LSE - missing modem lab data", func(t *ftt.Test) {
			const machineName = "machine-labdata-16"
			labData := &ufsAPI.ChromeOsRecoveryData_LabData{
				ModemInfo: &ufsAPI.ChromeOsRecoveryData_ModemInfo{
					ModelVariant: "some_cellular_variant",
					Imei:         "123456",
					Type:         chromeosLab.ModemType_MODEM_TYPE_QUALCOMM_SC7180,
				},
			}
			machineLSE := mockDutMachineLSE(machineName)
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Modeminfo = nil
			req, err := inventory.CreateMachineLSE(ctx, machineLSE)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetSmartUsbhub(), should.BeFalse)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_UNSPECIFIED))
			err = updateRecoveryLabData(ctx, machineName, ufspb.State_STATE_READY, labData)
			assert.Loosely(t, err, should.BeNil)
			req, err = inventory.GetMachineLSE(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetResourceState(), should.Equal(ufspb.State_STATE_READY))

			modem := req.GetChromeosMachineLse().GetDeviceLse().GetDut().GetModeminfo()
			assert.Loosely(t, modem.GetType(), should.Equal(chromeosLab.ModemType_MODEM_TYPE_QUALCOMM_SC7180))
			assert.Loosely(t, modem.GetImei(), should.Equal("123456"))
			assert.Loosely(t, modem.GetModelVariant(), should.Equal("some_cellular_variant"))
		})
	})
}

func TestDeleteMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteMachineLSE", t, func(t *ftt.Test) {
		t.Run("Delete machineLSE - happy path", func(t *ftt.Test) {
			machineLSE1 := &ufspb.MachineLSE{
				Name:     "machinelse-delete-1",
				Hostname: "machinelse-delete-1",
				Machines: []string{"machine-delete-1"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{
						Vms: []*ufspb.VM{
							{
								Name:       "vm-delete-1",
								MacAddress: "old_mac_address",
							},
						},
					},
				},
			}
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name:         "machine-delete-1",
				SerialNumber: "machine-delete-1-serial",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = CreateMachineLSE(ctx, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)

			err = DeleteMachineLSE(ctx, "machinelse-delete-1")
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.GetMachineLSEDeployment(ctx, "machine-delete-1-serial")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			// verify changes
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine_lse"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "vms/vm-delete-1")
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
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/hosts/machinelse-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/machines/machine-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(ufspb.State_STATE_SERVING.String()))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(ufspb.State_STATE_REGISTERED.String()))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dhcps/machinelse-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/machinelse-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("device_labels"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/machinelse-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "vms/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/vms/vm-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/hosts/machinelse-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "states/machines/machine-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/machinelse-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)

			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machineLSEDeployments/machine-delete-1-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse_deployment"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine_lse_deployment"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machineLSEDeployments/machine-delete-1-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)
		})

		t.Run("Delete machineLSE with existing deployment record - happy path", func(t *ftt.Test) {
			machineLSE1 := &ufspb.MachineLSE{
				Name:     "machinelse-delete-2",
				Hostname: "machinelse-delete-2",
				Machines: []string{"machine-delete-2"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name:         "machine-delete-2",
				SerialNumber: "machine-delete-2-serial",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = CreateMachineLSE(ctx, machineLSE1, nil)
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.GetMachineLSEDeployment(ctx, "machine-delete-2-serial")
			assert.Loosely(t, err, should.BeNil)

			err = DeleteMachineLSE(ctx, "machinelse-delete-2")
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.GetMachineLSEDeployment(ctx, "machine-delete-2-serial")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			// Verify change events
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machineLSEDeployments/machine-delete-2-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse_deployment"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine_lse_deployment"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machineLSEDeployments/machine-delete-2-serial")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)
		})
	})
}

func TestDeleteMachineLSEDUT(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteMachineLSE for a DUT", t, func(t *ftt.Test) {
		t.Run("Delete machineLSE DUT with Servo Info", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-1",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			servo := &chromeosLab.Servo{
				ServoHostname: "RedLabstation-92",
				ServoPort:     92,
				ServoSerial:   "RedSerial-92",
			}

			labstationMachinelse := mockLabstationMachineLSE("RedLabstation-92")
			labstationMachinelse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Servos = []*chromeosLab.Servo{servo}
			_, err = inventory.CreateMachineLSE(ctx, labstationMachinelse)
			assert.Loosely(t, err, should.BeNil)

			peripherals := &chromeosLab.Peripherals{
				Servo: servo,
			}
			dutMachinelse := mockDutMachineLSE("DUTMachineLse-92")
			dutMachinelse.Machines = []string{"machine-1"}
			dutMachinelse.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = peripherals
			_, err = inventory.CreateMachineLSE(ctx, dutMachinelse)
			assert.Loosely(t, err, should.BeNil)
			_, err = state.UpdateStateRecord(ctx, &ufspb.StateRecord{
				State:        ufspb.State_STATE_SERVING,
				ResourceName: "hosts/DUTMachineLse-92",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateNic(ctx, &ufspb.Nic{
				Name:       "DUTMachineLse-92:eth0",
				MacAddress: "DUTMachineLse-92-macaddress",
				Machine:    "machine-1",
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteMachineLSE(ctx, "DUTMachineLse-92")
			assert.Loosely(t, err, should.BeNil)

			_, err = state.GetStateRecord(ctx, "hosts/DUTMachineLse-92")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			resp, _ := inventory.GetMachineLSE(ctx, "RedLabstation-92")
			assert.Loosely(t, resp.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos(), should.BeEmpty)

			_, err = registration.GetNic(ctx, "DUTMachineLse-92:eth0")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			// verify changes
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/RedLabstation-92")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.chromeos_machine_lse.labstation.servos"))
			assert.Loosely(t, changes[0].GetOldValue(), should.ContainSubstring(servo.ServoHostname))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("[]"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/RedLabstation-92")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
		})

		t.Run("Delete machineLSE DUT with non existing Servo Info", func(t *ftt.Test) {
			// Attempt to delete a misconfigured DUT with servo host that doesn't exist. Controller should log an error
			// as this means that somewhere DUT/Labstation was misconfigured. But delete should not fail
			machine := &ufspb.Machine{
				Name: "machine-2",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			servo := &chromeosLab.Servo{
				ServoHostname: "RedLabstation-93",
				ServoPort:     9996,
				ServoSerial:   "RedSerial-93",
			}

			peripherals := &chromeosLab.Peripherals{
				Servo: servo,
			}
			dutMachinelse := mockDutMachineLSE("DUTMachineLse-93")
			dutMachinelse.Machines = []string{"machine-2"}
			dutMachinelse.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = peripherals
			_, err = inventory.CreateMachineLSE(ctx, dutMachinelse)
			assert.Loosely(t, err, should.BeNil)
			_, err = state.UpdateStateRecord(ctx, &ufspb.StateRecord{
				State:        ufspb.State_STATE_SERVING,
				ResourceName: "hosts/DUTMachineLse-93",
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteMachineLSE(ctx, "DUTMachineLse-93")
			assert.Loosely(t, err, should.BeNil)

			_, err = state.GetStateRecord(ctx, "hosts/DUTMachineLse-93")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			// Ensure that the labstation doesn't exist
			_, err = inventory.GetMachineLSE(ctx, "RedLabstation-93")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			// verify that no changes were recorded for labstation
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/RedLabstation-93")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/RedLabstation-93")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("Delete machineLSE DUT associated with a SchedulingUnit", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "m-1",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			dutMachinelse := mockDutMachineLSE("dut-1")
			dutMachinelse.Machines = []string{"m-1"}
			_, err = inventory.CreateMachineLSE(ctx, dutMachinelse)
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateSchedulingUnit(ctx, &ufspb.SchedulingUnit{
				Name:        "su-1",
				MachineLSEs: []string{"dut-1"},
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteMachineLSE(ctx, "dut-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("DUT is associated with SchedulingUnit."))
		})

		t.Run("Delete machineLSE DUT with servod on docker", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-8",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			servo := &chromeosLab.Servo{
				ServoHostname:       "LocalServodHost",
				ServoPort:           9996,
				ServoSerial:         "Servo-serial",
				DockerContainerName: "docker-2",
			}

			peripherals := &chromeosLab.Peripherals{
				Servo: servo,
			}
			dutMachinelse := mockDutMachineLSE("DUTMachineLse-98")
			dutMachinelse.Machines = []string{"machine-8"}
			dutMachinelse.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = peripherals
			_, err = inventory.CreateMachineLSE(ctx, dutMachinelse)
			assert.Loosely(t, err, should.BeNil)
			// Delete should not throw any error wven though the labstation doesn't exist
			err = DeleteMachineLSE(ctx, "DUTMachineLse-98")
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestDeleteMachineLSELabstation(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteMachineLSE for a Labstation", t, func(t *ftt.Test) {
		t.Run("Delete machineLSE Labstation with Servo Info", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-90",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			servo := &chromeosLab.Servo{
				ServoHostname: "RedLabstation-90",
				ServoPort:     90,
			}
			labstationMachinelse := mockLabstationMachineLSE("RedLabstation-90")
			labstationMachinelse.Machines = []string{"machine-90"}
			labstationMachinelse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Servos = []*chromeosLab.Servo{servo}
			_, err = inventory.CreateMachineLSE(ctx, labstationMachinelse)
			assert.Loosely(t, err, should.BeNil)

			peripherals := &chromeosLab.Peripherals{
				Servo: servo,
			}
			dutMachinelse := mockDutMachineLSE("DUTMachineLSE-90")
			dutMachinelse.GetChromeosMachineLse().GetDeviceLse().GetDut().Peripherals = peripherals
			_, err = inventory.CreateMachineLSE(ctx, dutMachinelse)
			assert.Loosely(t, err, should.BeNil)
			_, err = state.UpdateStateRecord(ctx, &ufspb.StateRecord{
				State:        ufspb.State_STATE_SERVING,
				ResourceName: "hosts/RedLabstation-90",
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteMachineLSE(ctx, "RedLabstation-90")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("cannot be deleted"))
			s, err := state.GetStateRecord(ctx, "hosts/RedLabstation-90")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_SERVING))

			// No changes are recorded as the deletion fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/RedLabstation-90")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})
		t.Run("Delete machineLSE Labstation without Servo Info", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-100",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			labstationMachinelse := mockLabstationMachineLSE("RedLabstation-100")
			labstationMachinelse.Machines = []string{"machine-100"}
			inventory.CreateMachineLSE(ctx, labstationMachinelse)
			_, err = state.UpdateStateRecord(ctx, &ufspb.StateRecord{
				State:        ufspb.State_STATE_SERVING,
				ResourceName: "hosts/RedLabstation-100",
			})
			assert.Loosely(t, err, should.BeNil)

			err = DeleteMachineLSE(ctx, "RedLabstation-100")
			assert.Loosely(t, err, should.BeNil)
			_, err = state.GetStateRecord(ctx, "hosts/RedLabstation-100")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/RedLabstation-100")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse"))
			// Both states for old & new are unspecified.
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "states/hosts/RedLabstation-100")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("state_record.state"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(ufspb.State_STATE_SERVING.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(ufspb.State_STATE_UNSPECIFIED.String()))
		})
	})
}

func TestListMachineLSEs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	machineLSEsWithProperties := make([]*ufspb.MachineLSE, 0, 2)
	machineLSEs := make([]*ufspb.MachineLSE, 0, 4)
	for i := 0; i < 4; i++ {
		machineLSE := mockDutMachineLSE(fmt.Sprintf("machineLSE-%d", i))
		if i%2 == 0 {
			machineLSE.GetChromeosMachineLse().GetDeviceLse().NetworkDeviceInterface = &ufspb.SwitchInterface{Switch: "switch-1"}
			machineLSE.GetChromeosMachineLse().GetDeviceLse().GetDut().Hive = "hive-1"
		}
		resp, _ := inventory.CreateMachineLSE(ctx, machineLSE)
		if i%2 == 0 {
			machineLSEsWithProperties = append(machineLSEsWithProperties, resp)
		}
		machineLSEs = append(machineLSEs, resp)
	}
	ftt.Run("ListMachineLSEs", t, func(t *ftt.Test) {
		t.Run("List MachineLSEs - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListMachineLSEs(ctx, 5, "", "invalid=mx-1", false, false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name invalid"))
		})

		t.Run("List MachineLSEs - filter switch - happy path with filter", func(t *ftt.Test) {
			resp, _, _ := ListMachineLSEs(ctx, 5, "", "switch=switch-1", false, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(machineLSEsWithProperties))
		})

		t.Run("List MachineLSEs - filter hive - happy path with filter", func(t *ftt.Test) {
			resp, _, _ := ListMachineLSEs(ctx, 5, "", "hive=hive-1", false, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(machineLSEsWithProperties))
		})

		t.Run("ListMachineLSEs - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListMachineLSEs(ctx, 5, "", "", false, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs))
		})

		t.Run("List machineLSEs - list machine lses with free slots", func(t *ftt.Test) {
			for i := 0; i < 8; i++ {
				machineLSE1 := &ufspb.MachineLSE{
					Name: fmt.Sprintf("machineLSE-free-%d", i),
				}
				machineLSE1.Manufacturer = "apple"
				var vmCapacity int32
				if i > 4 && i <= 6 {
					vmCapacity = int32(i - 4)
				}
				machineLSE1.Lse = &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{
						VmCapacity: vmCapacity,
					},
				}
				inventory.CreateMachineLSE(ctx, machineLSE1)
			}
			fields := make([]interface{}, 1)
			fields[0] = "apple"
			resp, nextPageToken, err := ListMachineLSEs(ctx, 4, "", "man=apple & free=true", false, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			for _, r := range resp {
				assert.Loosely(t, r.GetName(), should.BeIn([]string{"machineLSE-free-5", "machineLSE-free-6"}...))
			}
		})
	})
}

func TestBatchGetMachineLSEs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetMachineLSEs", t, func(t *ftt.Test) {
		t.Run("Batch get machine lses - happy path", func(t *ftt.Test) {
			lses := make([]*ufspb.MachineLSE, 4)
			for i := 0; i < 4; i++ {
				lse := &ufspb.MachineLSE{
					Name: fmt.Sprintf("lse-batchGet-%d", i),
				}
				lses[i] = lse
			}
			_, err := inventory.BatchUpdateMachineLSEs(ctx, lses)
			assert.Loosely(t, err, should.BeNil)
			resp, err := inventory.BatchGetMachineLSEs(ctx, []string{"lse-batchGet-0", "lse-batchGet-1", "lse-batchGet-2", "lse-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(lses))
		})
		t.Run("Batch get machine lses - missing id", func(t *ftt.Test) {
			resp, err := inventory.BatchGetMachineLSEs(ctx, []string{"lse-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("lse-batchGet-non-existing"))
		})
		t.Run("Batch get machine lses - empty input", func(t *ftt.Test) {
			resp, err := inventory.BatchGetMachineLSEs(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = inventory.BatchGetMachineLSEs(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}

func TestRealmPermissionForMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("TestRealmPermissionForMachineLSE", t, func(t *ftt.Test) {
		t.Run("CreateMachineLSE with permission - pass", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-1",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse := &ufspb.MachineLSE{
				Name:     "machinelse-1",
				Machines: []string{"machine-1"},
				Hostname: "machinelse-1",
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesCreate, util.BrowserLabAdminRealm)
			resp, err := CreateMachineLSE(ctx, mlse, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(mlse))
		})

		t.Run("CreateMachineLSE without permission - fail", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-2",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse := &ufspb.MachineLSE{
				Name:     "machinelse-2",
				Machines: []string{"machine-2"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
			_, err = CreateMachineLSE(ctx, mlse, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("DeleteMachineLSE with permission - pass", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-3",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-3",
				Machines: []string{"machine-3"},
				Hostname: "machinelse-3",
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesDelete, util.BrowserLabAdminRealm)
			err = DeleteMachineLSE(ctx, "machinelse-3")
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("DeleteMachineLSE without permission - fail", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-4",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-4",
				Machines: []string{"machine-4"},
				Hostname: "machinelse-4",
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesDelete, util.AtlLabAdminRealm)
			err = DeleteMachineLSE(ctx, "machinelse-4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("UpdateMachineLSE with permission - pass", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-5",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-5",
				Machines: []string{"machine-5"},
				Hostname: "machinelse-5",
			})
			assert.Loosely(t, err, should.BeNil)

			mlse.Tags = []string{"Dell"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateMachineLSE(ctx, mlse, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.Tags, should.Match([]string{"Dell"}))
		})

		t.Run("UpdateMachineLSE without permission - fail", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-6",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-6",
				Machines: []string{"machine-6"},
				Hostname: "machinelse-6",
			})
			assert.Loosely(t, err, should.BeNil)

			mlse.Tags = []string{"Dell"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
			_, err = UpdateMachineLSE(ctx, mlse, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("UpdateMachineLSE(new machine and same realm) with permission - pass", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-7",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-7",
				Machines: []string{"machine-7"},
				Hostname: "machinelse-7",
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-7.1",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse.Machines = []string{"machine-7.1"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateMachineLSE(ctx, mlse, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.Machines, should.Match([]string{"machine-7.1"}))
		})

		t.Run("UpdateMachineLSE(new machine and different realm) without permission - fail", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-8",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-8",
				Machines: []string{"machine-8"},
				Hostname: "machinelse-8",
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-8.1",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse.Machines = []string{"machine-8.1"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateMachineLSE(ctx, mlse, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("UpdateMachineLSE(new machine and different realm) with permission - pass", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-9",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-9",
				Machines: []string{"machine-9"},
				Hostname: "machinelse-9",
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-9.1",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse.Machines = []string{"machine-9.1"}
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.InventoriesUpdate),
				),
			})
			resp, err := UpdateMachineLSE(ctx, mlse, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.Machines, should.Match([]string{"machine-9.1"}))
		})

		t.Run("Partial UpdateMachineLSE with permission - pass", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-10",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-10",
				Machines: []string{"machine-10"},
				Hostname: "machinelse-10",
			})
			assert.Loosely(t, err, should.BeNil)

			mlse.Tags = []string{"Dell"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateMachineLSE(ctx, mlse, &field_mask.FieldMask{Paths: []string{"tags"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.Tags, should.Match([]string{"Dell"}))
		})

		t.Run("Partial UpdateMachineLSE without permission - fail", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-11",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-11",
				Machines: []string{"machine-11"},
				Hostname: "machinelse-11",
			})
			assert.Loosely(t, err, should.BeNil)

			mlse.Tags = []string{"Dell"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.AtlLabAdminRealm)
			_, err = UpdateMachineLSE(ctx, mlse, &field_mask.FieldMask{Paths: []string{"tags"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Partial UpdateMachineLSE(new machine and same realm) with permission - pass", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-12",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-12",
				Machines: []string{"machine-12"},
				Hostname: "machinelse-12",
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-12.1",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse.Machines = []string{"machine-12.1"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateMachineLSE(ctx, mlse, &field_mask.FieldMask{Paths: []string{"machines"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.Machines, should.Match([]string{"machine-12.1"}))
		})

		t.Run("Partial UpdateMachineLSE(new machine and different realm) without permission - fail", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-13",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-13",
				Machines: []string{"machine-13"},
				Hostname: "machinelse-13",
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-13.1",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse.Machines = []string{"machine-13.1"}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateMachineLSE(ctx, mlse, &field_mask.FieldMask{Paths: []string{"machines"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Partial UpdateMachineLSE(new machine and different realm) with permission - pass", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-14",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-14",
				Machines: []string{"machine-14"},
				Hostname: "machinelse-14",
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-14.1",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			mlse.Machines = []string{"machine-14.1"}
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.InventoriesUpdate),
				),
			})
			resp, err := UpdateMachineLSE(ctx, mlse, &field_mask.FieldMask{Paths: []string{"machines"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.Machines, should.Match([]string{"machine-14.1"}))
		})

	})
}

func TestRenameMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("RenameMachineLSE", t, func(t *ftt.Test) {
		t.Run("Rename non-existent machineLSE", func(t *ftt.Test) {
			_, err := RenameMachineLSE(ctx, "existingMLSE", "non-existentMLSE")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Rename to existing machineLSE", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-15",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-16",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-15",
				Machines: []string{"machine-15"},
				Hostname: "machinelse-15",
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hostname: "machinelse-15",
									},
								},
							},
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-16",
				Machines: []string{"machine-16"},
				Hostname: "machinelse-16",
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hostname: "machinelse-16",
									},
								},
							},
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			// Should not be possible to assign machinelse-15 to machinelse-16
			_, err = RenameMachineLSE(ctx, "machinelse-15", "machinelse-16")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(codes.FailedPrecondition.String()))
		})
		t.Run("Rename non os machineLSE", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-21",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-21",
				Machines: []string{"machine-21"},
				Hostname: "machinelse-21",
				// Not an os machine as there is not ChromeosMachineLSE
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesDelete),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesCreate),
				),
			})

			_, err = RenameMachineLSE(ctx, "machinelse-21", "machinelse-22")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(codes.Unimplemented.String()))
		})
		t.Run("Rename machineLSE with out create permission", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-17",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-17",
				Machines: []string{"machine-17"},
				Hostname: "machinelse-17",
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hostname: "machinelse-17",
									},
								},
							},
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesUpdate),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesDelete),
					// Missing create permission
				),
			})

			_, err = RenameMachineLSE(ctx, "machinelse-17", "machinelse-18")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(codes.PermissionDenied.String()))
		})
		t.Run("Rename machineLSE with out delete permission", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-18",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-18",
				Machines: []string{"machine-18"},
				Hostname: "machinelse-18",
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hostname: "machinelse-18",
									},
								},
							},
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesUpdate),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesCreate),
				),
			})

			_, err = RenameMachineLSE(ctx, "machinelse-18", "machinelse-19")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(codes.PermissionDenied.String()))
		})
		t.Run("Rename machineLSE happy path", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-19",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-19",
				Machines: []string{"machine-19"},
				Hostname: "machinelse-19",
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hostname: "machinelse-19",
									},
								},
							},
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesUpdate),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesCreate),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesDelete),
				),
			})

			_, err = RenameMachineLSE(ctx, "machinelse-19", "machinelse-20")
			assert.Loosely(t, err, should.BeNil)
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/machinelse-19")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/machinelse-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-19")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("machinelse-19"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("machinelse-20"))
			assert.Loosely(t, changes[0].OldValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("RENAME"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[1].OldValue, should.Equal("machinelse-19"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("machinelse-20"))
			assert.Loosely(t, changes[0].OldValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("RENAME"))
		})
	})
}

func TestGetAttachedDeviceData(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ctx = useTestingCfg(ctx)

	machine := &ufspb.Machine{
		Name: "machine-1",
		Device: &ufspb.Machine_AttachedDevice{
			AttachedDevice: &ufspb.AttachedDevice{
				Manufacturer: "Apple",
				DeviceType:   ufspb.AttachedDeviceType_ATTACHED_DEVICE_TYPE_APPLE_PHONE,
				BuildTarget:  "test",
				Model:        "test",
			},
		},
	}
	registration.CreateMachine(ctx, machine)

	// var dummylse *ufspb.MachineLSE
	admlse := mockAttachedDeviceMachineLSE("lse-1")
	admlse.Machines = []string{"machine-1"}
	inventory.CreateMachineLSE(ctx, admlse)

	dutState := mockDutState("machine-1", "lse-1")
	UpdateDutState(ctx, dutState)

	ftt.Run("TestGetAttachedDeviceData", t, func(t *ftt.Test) {
		t.Run("GetAttachedDeviceData - happy path", func(t *ftt.Test) {
			resp, err := GetAttachedDeviceData(ctx, admlse)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Match(admlse))
			assert.Loosely(t, resp.GetMachine(), should.Match(machine))
			assert.Loosely(t, resp.GetDutState(), should.Match(dutState))
		})

		t.Run("GetAttachedDeviceData - machine not found by hostname", func(t *ftt.Test) {
			admlse2 := mockAttachedDeviceMachineLSE("lse-2")
			admlse2.Machines = []string{"machine-2"}
			inventory.CreateMachineLSE(ctx, admlse2)

			resp, err := GetAttachedDeviceData(ctx, admlse2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Match(admlse2))
			assert.Loosely(t, resp.GetMachine(), should.BeNil)
			assert.Loosely(t, resp.GetDutState(), should.BeNil)
		})

		t.Run("GetAttachedDeviceData - machine not found by id", func(t *ftt.Test) {
			admlse3 := mockAttachedDeviceMachineLSE("lse-3")
			admlse3.Machines = []string{"machine-3"}
			inventory.CreateMachineLSE(ctx, admlse3)

			resp, err := GetAttachedDeviceData(ctx, admlse3)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetLabConfig(), should.Match(admlse3))
			assert.Loosely(t, resp.GetMachine(), should.BeNil)
			assert.Loosely(t, resp.GetDutState(), should.BeNil)
		})

		t.Run("GetAttachedDeviceData - no machine specified", func(t *ftt.Test) {
			admlse4 := mockAttachedDeviceMachineLSE("lse-4")
			admlse4.Machines = []string{}
			inventory.CreateMachineLSE(ctx, admlse4)

			resp, err := GetAttachedDeviceData(ctx, admlse4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("host does not have machines registered to it"))
		})

		t.Run("GetAttachedDeviceData - nil machinelse", func(t *ftt.Test) {
			resp, err := GetAttachedDeviceData(ctx, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("host cannot be empty"))
		})
	})
}

func TestUpdateBluetoothPeerStates(t *testing.T) {
	btpsData := &ufsAPI.ChromeOsRecoveryData_BluetoothPeer{
		Hostname: "h1",
		State:    chromeosLab.PeripheralState_WORKING,
	}
	ftt.Run("Update bluetooth peers states", t, func(t *ftt.Test) {
		t.Run("BluetoothPeer data is nil", func(t *ftt.Test) {
			err := updateBluetoothPeerStates(nil, nil)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Bluetooth peer is not present, returns error", func(t *ftt.Test) {
			var p chromeosLab.Peripherals
			err := updateBluetoothPeerStates(&p, []*ufsAPI.ChromeOsRecoveryData_BluetoothPeer{btpsData})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.Equal("unknown BTP with hostname \"h1\" received from lab"))
		})
		t.Run("Bluetooth peer is present, successful path", func(t *ftt.Test) {
			p := &chromeosLab.Peripherals{
				BluetoothPeers: []*chromeosLab.BluetoothPeer{
					{
						Device: &chromeosLab.BluetoothPeer_RaspberryPi{
							RaspberryPi: &chromeosLab.RaspberryPi{
								Hostname: "h1",
								State:    chromeosLab.PeripheralState_BROKEN,
							},
						},
					},
					{
						Device: &chromeosLab.BluetoothPeer_RaspberryPi{
							RaspberryPi: &chromeosLab.RaspberryPi{
								Hostname: "h2",
								State:    chromeosLab.PeripheralState_BROKEN,
							},
						},
					},
				},
			}
			err := updateBluetoothPeerStates(p, []*ufsAPI.ChromeOsRecoveryData_BluetoothPeer{btpsData})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, p.GetBluetoothPeers()[0].GetRaspberryPi().GetHostname(), should.Equal("h1"))
			assert.Loosely(t, p.GetBluetoothPeers()[0].GetRaspberryPi().GetState(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, p.GetBluetoothPeers()[1].GetRaspberryPi().GetHostname(), should.Equal("h2"))
			assert.Loosely(t, p.GetBluetoothPeers()[1].GetRaspberryPi().GetState(), should.Equal(chromeosLab.PeripheralState_BROKEN))
		})
	})
}

func TestUpdateAudioboxJackpluggerStates(t *testing.T) {
	ctx := testingContext()
	ftt.Run("Update audiobox jackplugger states", t, func(t *ftt.Test) {
		t.Run("audiobox jackplugger is unspecified, successful path", func(t *ftt.Test) {
			p := &chromeosLab.Peripherals{
				Chameleon: &chromeosLab.Chameleon{
					AudioboxJackplugger: chromeosLab.Chameleon_AUDIOBOX_JACKPLUGGER_WORKING,
				},
			}
			updateRecoveryPeripheralAudioboxJackplugger(ctx, p, &ufsAPI.ChromeOsRecoveryData_LabData{AudioboxJackpluggerState: chromeosLab.Chameleon_AUDIOBOX_JACKPLUGGER_UNSPECIFIED})
			assert.Loosely(t, p.GetChameleon().GetAudioboxJackplugger(), should.Equal(chromeosLab.Chameleon_AUDIOBOX_JACKPLUGGER_WORKING))
		})
		t.Run("audiobox jackplugger is specified", func(t *ftt.Test) {
			t.Run("Chameleon is present in peripherals, successful path", func(t *ftt.Test) {
				p := &chromeosLab.Peripherals{
					Chameleon: &chromeosLab.Chameleon{
						AudioboxJackplugger: chromeosLab.Chameleon_AUDIOBOX_JACKPLUGGER_BROKEN,
					},
				}

				assert.Loosely(t, p.GetChameleon().GetAudioboxJackplugger(), should.Equal(chromeosLab.Chameleon_AUDIOBOX_JACKPLUGGER_BROKEN))
				updateRecoveryPeripheralAudioboxJackplugger(ctx, p, &ufsAPI.ChromeOsRecoveryData_LabData{AudioboxJackpluggerState: chromeosLab.Chameleon_AUDIOBOX_JACKPLUGGER_WORKING})
				assert.Loosely(t, p.GetChameleon().GetAudioboxJackplugger(), should.Equal(chromeosLab.Chameleon_AUDIOBOX_JACKPLUGGER_WORKING))
			})
		})
	})
}

func TestUpdateDolosInfoFromRecovery(t *testing.T) {
	ctx := testingContext()
	ftt.Run("Update dolos info from recovery", t, func(t *ftt.Test) {
		t.Run("update info successful path", func(t *ftt.Test) {
			p := &chromeosLab.Peripherals{
				Dolos: &chromeosLab.Dolos{
					Hostname:        "fake-dolos-host",
					SerialCable:     "fake-dolos-cable-serial",
					HwMajorRevision: chromeosLab.DolosHWMajorRevision_DOLOS_V1,
				},
			}
			updateRecoveryPeripheralDolos(ctx, p, &ufsAPI.ChromeOsRecoveryData_LabData{
				Dolos: &ufsAPI.ChromeOsRecoveryData_Dolos{
					SerialUsb: "fake-dolos-usb-serial",
					FwVersion: "fake-dolos-fw-version",
				},
			})
			assert.Loosely(t, p.GetDolos().GetHostname(), should.Equal("fake-dolos-host"))
			assert.Loosely(t, p.GetDolos().GetSerialCable(), should.Equal("fake-dolos-cable-serial"))
			assert.Loosely(t, p.GetDolos().GetSerialUsb(), should.Equal("fake-dolos-usb-serial"))
			assert.Loosely(t, p.GetDolos().GetFwVersion(), should.Equal("fake-dolos-fw-version"))
			assert.Loosely(t, p.GetDolos().GetHwMajorRevision(), should.Equal(chromeosLab.DolosHWMajorRevision_DOLOS_V1))
		})
		t.Run("update info when dolos doesn't exists", func(t *ftt.Test) {
			p := &chromeosLab.Peripherals{
				Dolos: &chromeosLab.Dolos{},
			}
			updateRecoveryPeripheralDolos(ctx, p, &ufsAPI.ChromeOsRecoveryData_LabData{
				Dolos: &ufsAPI.ChromeOsRecoveryData_Dolos{
					SerialUsb: "fake-dolos-usb-serial",
					FwVersion: "fake-dolos-fw-version",
				},
			})
			assert.Loosely(t, p.GetDolos().GetHostname(), should.BeEmpty)
			assert.Loosely(t, p.GetDolos().GetSerialCable(), should.BeEmpty)
			assert.Loosely(t, p.GetDolos().GetSerialUsb(), should.BeEmpty)
			assert.Loosely(t, p.GetDolos().GetFwVersion(), should.BeEmpty)
			assert.Loosely(t, p.GetDolos().GetHwMajorRevision(), should.Equal(chromeosLab.DolosHWMajorRevision_DOLOS_UNSPECIFIED))
		})
	})
}

func TestUpdateMachineLSEDevboard(t *testing.T) {
	t.Parallel()
	ctx := testingContext()

	devboardMachinelse1 := mockDevboardMachineLSE("fake-devboard")
	inventory.CreateMachineLSE(ctx, devboardMachinelse1)
	ftt.Run("UpdateMachineLSE for a Devboard", t, func(t *ftt.Test) {
		t.Run("Update non-existing machineLSE Devboard", func(t *ftt.Test) {
			devboardMachinelse := mockDevboardMachineLSE("fake-devboard-non-existing")
			resp, err := UpdateMachineLSE(ctx, devboardMachinelse, nil)

			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update machineLSE Devboard pools", func(t *ftt.Test) {
			devboardMachinelse := mockDevboardMachineLSE("fake-devboard")
			devboardMachinelse.GetChromeosMachineLse().GetDeviceLse().GetDevboard().Pools = []string{"new-pool"}

			resp, err := UpdateMachineLSE(ctx, devboardMachinelse, &field_mask.FieldMask{Paths: []string{"pools-devboard"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetChromeosMachineLse().GetDeviceLse().GetDevboard().GetPools(), should.Contain("new-pool"))

			resp, err = UpdateMachineLSE(ctx, devboardMachinelse, &field_mask.FieldMask{Paths: []string{"pools-devboard-remove"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetChromeosMachineLse().GetDeviceLse().GetDevboard().GetPools(), should.NotContain("new-pool"))
		})
	})
}

func TestGetMachineLSEBySerial(t *testing.T) {
	//t.Parallel()
	ctx := testingContext()
	ftt.Run("GetMachineLSEBySerial", t, func(t *ftt.Test) {
		t.Run("GetMachineLSEBySerial - Missing machine", func(t *ftt.Test) {
			_, err := GetMachineLSEBySerial(ctx, "e34a2b3c8c8e9f9acc", true)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("GetMachineLSEBySerial - Missing machine lse", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name:         "machine-100",
				SerialNumber: "e34a2b3c8c8e9f9acc",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = GetMachineLSEBySerial(ctx, "e34a2b3c8c8e9f9acc", true)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("GetMachineLSEBySerial - Multiple machines with serial", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name:         "machine-102",
				SerialNumber: "e34a2b3c8c8e9f7acc",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateMachine(ctx, &ufspb.Machine{
				Name:         "machine-103",
				SerialNumber: "e34a2b3c8c8e9f7acc",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-102",
				Machines: []string{"machine-102"},
				Hostname: "machinelse-102",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = GetMachineLSEBySerial(ctx, "e34a2b3c8c8e9f7acc", true)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("GetMachineLSEBySerial - Multiple lses with machine", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name:         "machine-104",
				SerialNumber: "e34a2b3c8c8e9f2acc",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-104",
				Machines: []string{"machine-104"},
				Hostname: "machinelse-104",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-105",
				Machines: []string{"machine-104"},
				Hostname: "machinelse-105",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = GetMachineLSEBySerial(ctx, "e34a2b3c8c8e9f2acc", true)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("GetMachineLSEBySerial - Happy path", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name:         "machine-101",
				SerialNumber: "e34a2b3c8c8e9f8acc",
			})
			assert.Loosely(t, err, should.BeNil)
			host := &ufspb.MachineLSE{
				Name:     "machinelse-101",
				Machines: []string{"machine-101"},
				Hostname: "machinelse-101",
			}
			resp, err := inventory.CreateMachineLSE(ctx, host)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(host))
			resp, err = GetMachineLSEBySerial(ctx, "e34a2b3c8c8e9f8acc", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(host))
		})
	})
}

func TestGetHostData(t *testing.T) {
	//t.Parallel()
	ctx := testingContext()
	ftt.Run("GetHostData", t, func(t *ftt.Test) {
		t.Run("GetHostData - Missing machine", func(t *ftt.Test) {
			_, _, err := GetHostData(ctx, "e34a2b3c8c8e9f9acc", true)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("GetHostData - Missing machine lse", func(t *ftt.Test) {
			creMachine := &ufspb.Machine{
				Name:         "machine-100",
				SerialNumber: "e34a2b3c8c8e9f9acc",
			}
			_, err := registration.CreateMachine(ctx, creMachine)
			assert.Loosely(t, err, should.BeNil)
			_, retMachine, err := GetHostData(ctx, "e34a2b3c8c8e9f9acc", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, retMachine, should.Match(creMachine))
		})
		t.Run("GetHostData - Multiple machines with serial", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name:         "machine-102",
				SerialNumber: "e34a2b3c8c8e9f7acc",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateMachine(ctx, &ufspb.Machine{
				Name:         "machine-103",
				SerialNumber: "e34a2b3c8c8e9f7acc",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-102",
				Machines: []string{"machine-102"},
				Hostname: "machinelse-102",
			})
			assert.Loosely(t, err, should.BeNil)
			_, _, err = GetHostData(ctx, "e34a2b3c8c8e9f7acc", true)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("GetHostData - Multiple lses with machine", func(t *ftt.Test) {
			creMachine := &ufspb.Machine{
				Name:         "machine-104",
				SerialNumber: "e34a2b3c8c8e9f2acc",
			}
			_, err := registration.CreateMachine(ctx, creMachine)
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-104",
				Machines: []string{"machine-104"},
				Hostname: "machinelse-104",
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-105",
				Machines: []string{"machine-104"},
				Hostname: "machinelse-105",
			})
			assert.Loosely(t, err, should.BeNil)
			_, retMachine, err := GetHostData(ctx, "e34a2b3c8c8e9f2acc", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, retMachine, should.Match(creMachine))
		})
		t.Run("GetHostData - Happy path", func(t *ftt.Test) {
			creMachine := &ufspb.Machine{
				Name:         "machine-101",
				SerialNumber: "e34a2b3c8c8e9f8acc",
			}
			_, err := registration.CreateMachine(ctx, creMachine)
			assert.Loosely(t, err, should.BeNil)
			host := &ufspb.MachineLSE{
				Name:     "machinelse-101",
				Machines: []string{"machine-101"},
				Hostname: "machinelse-101",
			}
			resp, err := inventory.CreateMachineLSE(ctx, host)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(host))
			resp, retMachine, err := GetHostData(ctx, "e34a2b3c8c8e9f8acc", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(host))
			assert.Loosely(t, retMachine, should.Match(creMachine))
		})
	})
}
