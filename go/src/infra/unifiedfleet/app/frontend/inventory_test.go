// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func mockMachineLSE(id string) *ufspb.MachineLSE {
	return &ufspb.MachineLSE{
		Name:     util.AddPrefix(util.MachineLSECollection, id),
		Hostname: id,
	}
}

func mockDutMachineLSE(id string) *ufspb.MachineLSE {
	dut := &chromeosLab.DeviceUnderTest{
		Hostname: id,
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
		Name:     util.AddPrefix(util.MachineLSECollection, id),
		Hostname: id,
		Lse:      lse,
	}
}

func mockRackLSE(id string) *ufspb.RackLSE {
	return &ufspb.RackLSE{
		Name: util.AddPrefix(util.RackLSECollection, id),
	}
}

func mockSchedulingUnit(name string) *ufspb.SchedulingUnit {
	return &ufspb.SchedulingUnit{
		Name: util.AddPrefix(util.SchedulingUnitCollection, name),
	}
}

func mockAttachedDeviceMachineLSE(name string) *ufspb.MachineLSE {
	return &ufspb.MachineLSE{
		Name:     util.AddPrefix(util.MachineLSECollection, name),
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

func TestUpdateNetworkOpt(t *testing.T) {
	input := &ufsAPI.NetworkOption{
		Vlan: "vlan1",
		Ip:   "",
		Nic:  "eth0",
	}
	ftt.Run("No vlan & ip, empty nwOpt", t, func(t *ftt.Test) {
		nwOpt := updateNetworkOpt("", "", nil)
		assert.Loosely(t, nwOpt, should.BeNil)
	})
	ftt.Run("No vlan & ip, non-empty nwOpt", t, func(t *ftt.Test) {
		nwOpt := updateNetworkOpt("", "", input)
		assert.Loosely(t, nwOpt, should.Match(input))
	})
	ftt.Run("Have vlan, no ip, empty nwOpt", t, func(t *ftt.Test) {
		nwOpt := updateNetworkOpt("vlan1", "", nil)
		assert.Loosely(t, nwOpt, should.Match(&ufsAPI.NetworkOption{
			Vlan: "vlan1",
		}))
	})
	ftt.Run("Have vlan, no ip, non-empty nwOpt", t, func(t *ftt.Test) {
		nwOpt := updateNetworkOpt("vlan2", "", input)
		assert.Loosely(t, nwOpt, should.Match(&ufsAPI.NetworkOption{
			Vlan: "vlan2",
			Nic:  "eth0",
		}))
	})
	ftt.Run("no vlan, have ip, empty nwOpt", t, func(t *ftt.Test) {
		nwOpt := updateNetworkOpt("", "0.0.0.0", nil)
		assert.Loosely(t, nwOpt, should.Match(&ufsAPI.NetworkOption{
			Ip: "0.0.0.0",
		}))
	})
	ftt.Run("no vlan, have ip, non-empty nwOpt", t, func(t *ftt.Test) {
		nwOpt := updateNetworkOpt("", "0.0.0.0", input)
		assert.Loosely(t, nwOpt, should.Match(&ufsAPI.NetworkOption{
			Ip:  "0.0.0.0",
			Nic: "eth0",
		}))
	})
	ftt.Run("have vlan, have ip, empty nwOpt", t, func(t *ftt.Test) {
		nwOpt := updateNetworkOpt("vlan1", "0.0.0.0", nil)
		assert.Loosely(t, nwOpt, should.Match(&ufsAPI.NetworkOption{
			Ip:   "0.0.0.0",
			Vlan: "vlan1",
		}))
	})
	ftt.Run("have vlan, have ip, non-empty nwOpt", t, func(t *ftt.Test) {
		nwOpt := updateNetworkOpt("vlan2", "0.0.0.0", input)
		assert.Loosely(t, nwOpt, should.Match(&ufsAPI.NetworkOption{
			Ip:   "0.0.0.0",
			Vlan: "vlan2",
			Nic:  "eth0",
		}))
	})
}

func TestCreateMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("CreateMachineLSE", t, func(t *ftt.Test) {
		t.Run("Create new machineLSE with machineLSE_id", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-1",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			mlsePrototype := &ufspb.MachineLSEPrototype{
				Name: "browser:no-vm",
			}
			_, err = configuration.CreateMachineLSEPrototype(ctx, mlsePrototype)
			assert.Loosely(t, err, should.BeNil)

			machineLSE1 := mockMachineLSE("machineLSE-1")
			machineLSE1.MachineLsePrototype = "browser:no-vm"
			machineLSE1.Lse = &ufspb.MachineLSE_ChromeBrowserMachineLse{
				ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
			}
			machineLSE1.Machines = []string{"machine-1"}
			req := &ufsAPI.CreateMachineLSERequest{
				MachineLSE:   machineLSE1,
				MachineLSEId: "machinelse-1",
			}
			resp, err := tf.Fleet.CreateMachineLSE(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE1))
		})

		t.Run("Create new machineLSE - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.CreateMachineLSERequest{
				MachineLSE: nil,
			}
			resp, err := tf.Fleet.CreateMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new machineLSE - Invalid input empty ID", func(t *ftt.Test) {
			req := &ufsAPI.CreateMachineLSERequest{
				MachineLSE:   mockMachineLSE("machineLSE-3"),
				MachineLSEId: "",
			}
			resp, err := tf.Fleet.CreateMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new machineLSE - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.CreateMachineLSERequest{
				MachineLSE:   mockMachineLSE("machineLSE-4"),
				MachineLSEId: "a.b)7&",
			}
			resp, err := tf.Fleet.CreateMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidHostname))
		})

		t.Run("Create new machineLSE - Invalid input nil machines", func(t *ftt.Test) {
			req := &ufsAPI.CreateMachineLSERequest{
				MachineLSE:   mockMachineLSE("machineLSE-4"),
				MachineLSEId: "machineLSE-4",
			}
			resp, err := tf.Fleet.CreateMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyMachineName))
		})

		t.Run("Create new machineLSE - Invalid input empty machines", func(t *ftt.Test) {
			mlse := mockMachineLSE("machineLSE-4")
			mlse.Machines = []string{""}
			req := &ufsAPI.CreateMachineLSERequest{
				MachineLSE:   mlse,
				MachineLSEId: "machineLSE-4",
			}
			resp, err := tf.Fleet.CreateMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyMachineName))
		})
	})
}

func TestUpdateMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("UpdateMachineLSE", t, func(t *ftt.Test) {
		t.Run("Update existing machineLSEs", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-0",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-1",
				Machines: []string{"machine-0"},
			})
			assert.Loosely(t, err, should.BeNil)

			machineLSE := mockMachineLSE("machineLSE-1")
			machineLSE.Machines = []string{"machine-0"}
			req := &ufsAPI.UpdateMachineLSERequest{
				MachineLSE: machineLSE,
			}
			resp, err := tf.Fleet.UpdateMachineLSE(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE))
		})

		t.Run("Update existing machineLSEs with states", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-1",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-state",
				Machines: []string{"machine-1"},
			})
			assert.Loosely(t, err, should.BeNil)

			machineLSE := mockMachineLSE("machineLSE-state")
			machineLSE.Machines = []string{"machine-1"}
			machineLSE.ResourceState = ufspb.State_STATE_DEPLOYED_TESTING
			req := &ufsAPI.UpdateMachineLSERequest{
				MachineLSE: machineLSE,
			}
			resp, err := tf.Fleet.UpdateMachineLSE(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE))
			s, err := state.GetStateRecord(ctx, "hosts/machinelse-state")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_DEPLOYED_TESTING))
		})

		t.Run("Update machineLSE - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateMachineLSERequest{
				MachineLSE: nil,
			}
			resp, err := tf.Fleet.UpdateMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update machineLSE - Invalid input empty name", func(t *ftt.Test) {
			machineLSE4 := mockMachineLSE("")
			machineLSE4.Name = ""
			req := &ufsAPI.UpdateMachineLSERequest{
				MachineLSE: machineLSE4,
			}
			resp, err := tf.Fleet.UpdateMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update machineLSE - Invalid input invalid characters", func(t *ftt.Test) {
			machineLSE5 := mockMachineLSE("a.b)7&")
			req := &ufsAPI.UpdateMachineLSERequest{
				MachineLSE: machineLSE5,
			}
			resp, err := tf.Fleet.UpdateMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("GetMachineLSE", t, func(t *ftt.Test) {
		t.Run("Get machineLSE by existing ID", func(t *ftt.Test) {
			machineLSE1, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "machinelse-1",
			})
			assert.Loosely(t, err, should.BeNil)
			machineLSE1.Name = util.AddPrefix(util.MachineLSECollection, machineLSE1.Name)

			req := &ufsAPI.GetMachineLSERequest{
				Name: util.AddPrefix(util.MachineLSECollection, "machinelse-1"),
			}
			resp, err := tf.Fleet.GetMachineLSE(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE1))
		})
		t.Run("Get machineLSE by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineLSERequest{
				Name: util.AddPrefix(util.MachineLSECollection, "machineLSE-2"),
			}
			resp, err := tf.Fleet.GetMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get machineLSE - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineLSERequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Get machineLSE - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineLSERequest{
				Name: util.AddPrefix(util.MachineLSECollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestCreateVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("CreateVM", t, func(t *ftt.Test) {
		registration.CreateMachine(ctx, &ufspb.Machine{
			Name: "inventory-create-machine",
			Location: &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CHROMEOS3,
			},
		})
		inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
			Name:     "inventory-create-host",
			Zone:     ufspb.Zone_ZONE_CHROMEOS3.String(),
			Machines: []string{"inventory-create-machine"},
		})
		t.Run("Create new VM - happy path", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "inventory-create-vm1",
				MachineLseId: "inventory-create-host",
			}
			_, err := tf.Fleet.CreateVM(ctx, &ufsAPI.CreateVMRequest{
				Vm: vm1,
			})
			assert.Loosely(t, err, should.BeNil)

			resp, err := tf.Fleet.GetVM(ctx, &ufsAPI.GetVMRequest{
				Name: util.AddPrefix(util.VMCollection, "inventory-create-vm1"),
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetName(), should.Equal("vms/inventory-create-vm1"))
			assert.Loosely(t, resp.GetZone(), should.Equal(ufspb.Zone_ZONE_CHROMEOS3.String()))
			assert.Loosely(t, resp.GetMachineLseId(), should.Equal("inventory-create-host"))
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_REGISTERED))

			s, err := state.GetStateRecord(ctx, "vms/inventory-create-vm1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})

		t.Run("Create new VM - missing host ", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name: "missing",
			}
			_, err := tf.Fleet.CreateVM(ctx, &ufsAPI.CreateVMRequest{
				Vm: vm1,
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyHostName))
		})

		t.Run("Create vm - Invalid mac", func(t *ftt.Test) {
			resp, err := tf.Fleet.CreateVM(ctx, &ufsAPI.CreateVMRequest{
				Vm: &ufspb.VM{
					Name:         "createvm-0",
					MacAddress:   "123",
					MachineLseId: "inventory-create-host",
				},
			})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidMac))
		})

		t.Run("Create new VM - assign ip", func(t *ftt.Test) {
			setupTestVlan(ctx)
			vm1 := &ufspb.VM{
				Name:         "inventory-create-vm2",
				MachineLseId: "inventory-create-host",
			}
			_, err := tf.Fleet.CreateVM(ctx, &ufsAPI.CreateVMRequest{
				Vm: vm1,
				NetworkOption: &ufsAPI.NetworkOption{
					Vlan: "vlan-1",
				},
			})
			assert.Loosely(t, err, should.BeNil)

			resp, err := tf.Fleet.GetVM(ctx, &ufsAPI.GetVMRequest{
				Name: util.AddPrefix(util.VMCollection, "inventory-create-vm2"),
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetVlan(), should.Equal("vlan-1"))
			dhcp, err := configuration.GetDHCPConfig(ctx, "inventory-create-vm2")
			assert.Loosely(t, err, should.BeNil)
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": dhcp.GetIp()})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeTrue)
		})
	})
}

func TestUpdateVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("UpdateVM", t, func(t *ftt.Test) {
		registration.CreateMachine(ctx, &ufspb.Machine{
			Name: "inventory-update-machine",
			Location: &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CHROMEOS3,
			},
		})
		inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
			Name:     "inventory-update-host",
			Zone:     ufspb.Zone_ZONE_CHROMEOS3.String(),
			Machines: []string{"inventory-update-machine"},
		})
		t.Run("Update existing VM", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "inventory-update-vm1",
				MachineLseId: "inventory-update-host",
			}
			_, err := tf.Fleet.CreateVM(ctx, &ufsAPI.CreateVMRequest{
				Vm: vm1,
			})
			assert.Loosely(t, err, should.BeNil)
			vm, err := tf.Fleet.GetVM(ctx, &ufsAPI.GetVMRequest{
				Name: util.AddPrefix(util.VMCollection, "inventory-update-vm1"),
			})
			assert.Loosely(t, err, should.BeNil)
			vm.UpdateTime = nil

			req := &ufsAPI.UpdateVMRequest{
				Vm: vm1,
			}
			resp, err := tf.Fleet.UpdateVM(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			resp.UpdateTime = nil
			assert.Loosely(t, resp, should.Match(vm))
		})

		t.Run("Update existing VMs with states", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "inventory-update-vm2",
				MachineLseId: "inventory-update-host",
			}
			_, err := tf.Fleet.CreateVM(ctx, &ufsAPI.CreateVMRequest{
				Vm: vm1,
			})
			assert.Loosely(t, err, should.BeNil)

			vm1.ResourceState = ufspb.State_STATE_DEPLOYED_TESTING
			req := &ufsAPI.UpdateVMRequest{
				Vm: vm1,
			}
			resp, err := tf.Fleet.UpdateVM(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_DEPLOYED_TESTING))
			s, err := state.GetStateRecord(ctx, "vms/inventory-update-vm2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_DEPLOYED_TESTING))
		})

		t.Run("Update VM - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateVMRequest{
				Vm: nil,
			}
			resp, err := tf.Fleet.UpdateVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update vm - Invalid mac", func(t *ftt.Test) {
			resp, err := tf.Fleet.UpdateVM(ctx, &ufsAPI.UpdateVMRequest{
				Vm: &ufspb.VM{
					Name:       "updatevm-0",
					MacAddress: "123",
				},
			})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidMac))
		})

		t.Run("Update VM - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.UpdateVMRequest{
				Vm: &ufspb.VM{
					Name:         "",
					MachineLseId: "inventory-update-host",
				},
			}
			resp, err := tf.Fleet.UpdateVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update VM - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.UpdateVMRequest{
				Vm: &ufspb.VM{
					Name:         util.AddPrefix(util.VMCollection, "a.b)7&"),
					MachineLseId: "inventory-update-host",
				},
			}
			resp, err := tf.Fleet.UpdateVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestDeleteVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("DeleteVM", t, func(t *ftt.Test) {
		registration.CreateMachine(ctx, &ufspb.Machine{
			Name: "inventory-delete-machine",
			Location: &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CHROMEOS3,
			},
		})
		inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
			Name:     "inventory-delete-host",
			Zone:     ufspb.Zone_ZONE_CHROMEOS3.String(),
			Machines: []string{"inventory-delete-machine"},
		})
		t.Run("Delete vm by existing ID", func(t *ftt.Test) {
			vm1 := &ufspb.VM{
				Name:         "inventory-delete-vm1",
				MachineLseId: "inventory-delete-host",
			}
			_, err := tf.Fleet.CreateVM(ctx, &ufsAPI.CreateVMRequest{
				Vm: vm1,
			})
			assert.Loosely(t, err, should.BeNil)

			req := &ufsAPI.DeleteVMRequest{
				Name: util.AddPrefix(util.VMCollection, "inventory-delete-vm1"),
			}
			_, err = tf.Fleet.DeleteVM(tf.C, req)
			assert.Loosely(t, err, should.BeNil)

			res, err := inventory.GetVM(tf.C, "inventory-delete-vm1")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			s, err := state.GetStateRecord(ctx, "vms/inventory-delete-vm1")
			assert.Loosely(t, s, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete vm by existing ID with assigned ip", func(t *ftt.Test) {
			setupTestVlan(ctx)
			vm1 := &ufspb.VM{
				Name:         "inventory-delete-vm2",
				MachineLseId: "inventory-delete-host",
			}
			_, err := tf.Fleet.CreateVM(ctx, &ufsAPI.CreateVMRequest{
				Vm: vm1,
				NetworkOption: &ufsAPI.NetworkOption{
					Ip: "192.168.40.18",
				},
			})
			assert.Loosely(t, err, should.BeNil)

			req := &ufsAPI.DeleteVMRequest{
				Name: util.AddPrefix(util.VMCollection, "inventory-delete-vm2"),
			}
			_, err = tf.Fleet.DeleteVM(tf.C, req)
			assert.Loosely(t, err, should.BeNil)

			res, err := inventory.GetVM(tf.C, "inventory-delete-vm2")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			dhcp, err := configuration.GetDHCPConfig(ctx, "inventory-delete-vm2")
			assert.Loosely(t, dhcp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			ips, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.18"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(1))
			assert.Loosely(t, ips[0].GetOccupied(), should.BeFalse)
		})
		t.Run("Delete vm by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.DeleteVMRequest{
				Name: util.AddPrefix(util.VMCollection, "inventory-delete-vm3"),
			}
			_, err := tf.Fleet.DeleteVM(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete vm - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteVMRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Delete vm - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteVMRequest{
				Name: util.AddPrefix(util.VMCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListVMs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
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
			Zone:          ufspb.Zone_ZONE_CHROMEOS3.String(),
			Vlan:          "vlan-2",
			ResourceState: ufspb.State_STATE_DEPLOYED_TESTING,
		},
	}
	ftt.Run("ListVMs", t, func(t *ftt.Test) {
		_, err := inventory.BatchUpdateVMs(ctx, vms)
		assert.Loosely(t, err, should.BeNil)
		t.Run("ListVMs - page_size negative - error", func(t *ftt.Test) {
			resp, err := tf.Fleet.ListVMs(tf.C, &ufsAPI.ListVMsRequest{
				PageSize: -5,
			})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListVMs - invalid filter - error", func(t *ftt.Test) {
			resp, err := tf.Fleet.ListVMs(tf.C, &ufsAPI.ListVMsRequest{
				Filter: "os=os-1 | state=serving",
			})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})

		t.Run("List VMs - happy path", func(t *ftt.Test) {
			resp, err := tf.Fleet.ListVMs(tf.C, &ufsAPI.ListVMsRequest{
				Filter:   "os=os-1 & state=serving",
				PageSize: 5,
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetVms(), should.HaveLength(2))
			assert.Loosely(t, ufsAPI.ParseResources(resp.GetVms(), "Name"), should.Match([]string{"vm-list-1", "vm-list-2"}))
		})
	})
}

func TestListMachineLSEs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	machineLSEs := make([]*ufspb.MachineLSE, 0, 4)
	for i := 0; i < 4; i++ {
		resp, _ := inventory.CreateMachineLSE(tf.C, &ufspb.MachineLSE{
			Name:     fmt.Sprintf("machineLSEFilter-%d", i),
			Machines: []string{"mac-1"},
		})
		resp.Name = util.AddPrefix(util.MachineLSECollection, resp.Name)
		machineLSEs = append(machineLSEs, resp)
	}
	ftt.Run("ListMachineLSEs", t, func(t *ftt.Test) {
		t.Run("ListMachineLSEs - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListMachineLSEsRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListMachineLSEs(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListMachineLSEs - Full listing - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListMachineLSEsRequest{}
			resp, err := tf.Fleet.ListMachineLSEs(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.MachineLSEs, should.Match(machineLSEs))
		})

		t.Run("ListMachineLSEs - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListMachineLSEsRequest{
				Filter: "machine=mac-1|rpm=rpm-2",
			}
			_, err := tf.Fleet.ListMachineLSEs(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})

		t.Run("ListMachineLSEs - filter format valid AND", func(t *ftt.Test) {
			req := &ufsAPI.ListMachineLSEsRequest{
				Filter: "machine=mac-1 & machineprototype=mlsep-1",
			}
			resp, err := tf.Fleet.ListMachineLSEs(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.MachineLSEs, should.BeNil)
		})

		t.Run("ListMachineLSEs - filter format valid", func(t *ftt.Test) {
			req := &ufsAPI.ListMachineLSEsRequest{
				Filter: "machine=mac-1",
			}
			resp, err := tf.Fleet.ListMachineLSEs(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.MachineLSEs, should.Match(machineLSEs))
		})
	})
}

func TestDeleteMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("DeleteMachineLSE", t, func(t *ftt.Test) {
		t.Run("Delete machineLSE by existing ID", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-1",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-1",
				Hostname: "machinelse-1",
				Machines: []string{"machine-1"},
			})
			assert.Loosely(t, err, should.BeNil)

			req := &ufsAPI.DeleteMachineLSERequest{
				Name: util.AddPrefix(util.MachineLSECollection, "machineLSE-1"),
			}
			_, err = tf.Fleet.DeleteMachineLSE(tf.C, req)
			assert.Loosely(t, err, should.BeNil)

			res, err := inventory.GetMachineLSE(tf.C, "machineLSE-1")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete machineLSE by existing ID with assigned ip", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-with-ip",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			_, err = inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "machinelse-with-ip",
				Hostname: "machinelse-with-ip",
				Nic:      "eth0",
				Machines: []string{"machine-1"},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateDHCPs(ctx, []*ufspb.DHCPConfig{
				{
					Hostname: "machinelse-with-ip",
					Ip:       "1.2.3.4",
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateIPs(ctx, []*ufspb.IP{
				{
					Id:       "vlan:1234",
					Ipv4:     1234,
					Ipv4Str:  "1.2.3.4",
					Vlan:     "vlan",
					Occupied: true,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			req := &ufsAPI.DeleteMachineLSERequest{
				Name: util.AddPrefix(util.MachineLSECollection, "machineLSE-with-ip"),
			}
			_, err = tf.Fleet.DeleteMachineLSE(tf.C, req)
			assert.Loosely(t, err, should.BeNil)

			res, err := inventory.GetMachineLSE(tf.C, "machineLSE-1")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			dhcp, err := configuration.GetDHCPConfig(ctx, "machineLSE-with-ip")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, dhcp, should.BeNil)
			s, _ := status.FromError(err)
			assert.Loosely(t, s.Code(), should.Equal(codes.NotFound))
			ips, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "1.2.3.4"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ips, should.HaveLength(1))
			assert.Loosely(t, ips[0].GetOccupied(), should.BeFalse)
		})
		t.Run("Delete machineLSE by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.DeleteMachineLSERequest{
				Name: util.AddPrefix(util.MachineLSECollection, "machineLSE-2"),
			}
			_, err := tf.Fleet.DeleteMachineLSE(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete machineLSE - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteMachineLSERequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Delete machineLSE - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteMachineLSERequest{
				Name: util.AddPrefix(util.MachineLSECollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteMachineLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestRenameMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("RenameMachineLSE", t, func(t *ftt.Test) {
		t.Run("Rename an empty machineLSE name - returns error", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameMachineLSE(tf.C, &ufsAPI.RenameMachineLSERequest{
				Name: "",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Rename a machineLSE to an empty name - returns error", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameMachineLSE(tf.C, &ufsAPI.RenameMachineLSERequest{
				Name:    "oldMachineLSE",
				NewName: "",
			})
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}

func TestCreateRackLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	rackLSE1 := mockRackLSE("rackLSE-1")
	rackLSE2 := mockRackLSE("rackLSE-2")
	ftt.Run("CreateRackLSEs", t, func(t *ftt.Test) {
		t.Run("Create new rackLSE with rackLSE_id", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSERequest{
				RackLSE:   rackLSE1,
				RackLSEId: "rackLSE-1",
			}
			resp, err := tf.Fleet.CreateRackLSE(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSE1))
		})

		t.Run("Create existing rackLSEs", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSERequest{
				RackLSE:   rackLSE1,
				RackLSEId: "rackLSE-1",
			}
			resp, err := tf.Fleet.CreateRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})

		t.Run("Create new rackLSE - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSERequest{
				RackLSE: nil,
			}
			resp, err := tf.Fleet.CreateRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new rackLSE - Invalid input empty ID", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSERequest{
				RackLSE:   rackLSE2,
				RackLSEId: "",
			}
			resp, err := tf.Fleet.CreateRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new rackLSE - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSERequest{
				RackLSE:   rackLSE2,
				RackLSEId: "a.b)7&",
			}
			resp, err := tf.Fleet.CreateRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestUpdateRackLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	rackLSE1 := mockRackLSE("rackLSE-1")
	rackLSE2 := mockRackLSE("rackLSE-1")
	rackLSE3 := mockRackLSE("rackLSE-3")
	rackLSE4 := mockRackLSE("")
	rackLSE5 := mockRackLSE("a.b)7&")
	ftt.Run("UpdateRackLSEs", t, func(t *ftt.Test) {
		t.Run("Update existing rackLSEs", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSERequest{
				RackLSE:   rackLSE1,
				RackLSEId: "rackLSE-1",
			}
			resp, err := tf.Fleet.CreateRackLSE(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSE1))
			ureq := &ufsAPI.UpdateRackLSERequest{
				RackLSE: rackLSE2,
			}
			resp, err = tf.Fleet.UpdateRackLSE(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSE2))
		})

		t.Run("Update non-existing rackLSEs", func(t *ftt.Test) {
			ureq := &ufsAPI.UpdateRackLSERequest{
				RackLSE: rackLSE3,
			}
			resp, err := tf.Fleet.UpdateRackLSE(tf.C, ureq)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Update rackLSE - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateRackLSERequest{
				RackLSE: nil,
			}
			resp, err := tf.Fleet.UpdateRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update rackLSE - Invalid input empty name", func(t *ftt.Test) {
			rackLSE4.Name = ""
			req := &ufsAPI.UpdateRackLSERequest{
				RackLSE: rackLSE4,
			}
			resp, err := tf.Fleet.UpdateRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update rackLSE - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.UpdateRackLSERequest{
				RackLSE: rackLSE5,
			}
			resp, err := tf.Fleet.UpdateRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetRackLSE(t *testing.T) {
	t.Parallel()
	ftt.Run("GetRackLSE", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		rackLSE1 := mockRackLSE("rackLSE-1")
		req := &ufsAPI.CreateRackLSERequest{
			RackLSE:   rackLSE1,
			RackLSEId: "rackLSE-1",
		}
		resp, err := tf.Fleet.CreateRackLSE(tf.C, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(rackLSE1))
		t.Run("Get rackLSE by existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetRackLSERequest{
				Name: util.AddPrefix(util.RackLSECollection, "rackLSE-1"),
			}
			resp, err := tf.Fleet.GetRackLSE(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSE1))
		})
		t.Run("Get rackLSE by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetRackLSERequest{
				Name: util.AddPrefix(util.RackLSECollection, "rackLSE-2"),
			}
			resp, err := tf.Fleet.GetRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get rackLSE - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetRackLSERequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Get rackLSE - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetRackLSERequest{
				Name: util.AddPrefix(util.RackLSECollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListRackLSEs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	rackLSEs := make([]*ufspb.RackLSE, 0, 4)
	for i := 0; i < 4; i++ {
		resp, _ := inventory.CreateRackLSE(tf.C, &ufspb.RackLSE{
			Name:  fmt.Sprintf("rackLSE-%d", i),
			Racks: []string{"rack-1"},
		})
		resp.Name = util.AddPrefix(util.RackLSECollection, resp.Name)
		rackLSEs = append(rackLSEs, resp)
	}
	ftt.Run("ListRackLSEs", t, func(t *ftt.Test) {
		t.Run("ListRackLSEs - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListRackLSEsRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListRackLSEs(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListRackLSEs - Full listing - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListRackLSEsRequest{}
			resp, err := tf.Fleet.ListRackLSEs(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.RackLSEs, should.Match(rackLSEs))
		})

		t.Run("ListRackLSEs - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListRackLSEsRequest{
				Filter: "rack=mac-1|rpm=rpm-2",
			}
			_, err := tf.Fleet.ListRackLSEs(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})

		t.Run("ListRackLSEs - filter format valid AND", func(t *ftt.Test) {
			req := &ufsAPI.ListRackLSEsRequest{
				Filter: "rack=rack-1 & rackprototype=mlsep-1",
			}
			resp, err := tf.Fleet.ListRackLSEs(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.RackLSEs, should.BeNil)
		})

		t.Run("ListRackLSEs - filter format valid", func(t *ftt.Test) {
			req := &ufsAPI.ListRackLSEsRequest{
				Filter: "rack=rack-1",
			}
			resp, err := tf.Fleet.ListRackLSEs(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.RackLSEs, should.Match(rackLSEs))
		})
	})
}

func TestDeleteRackLSE(t *testing.T) {
	t.Parallel()
	ftt.Run("DeleteRackLSE", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		rackLSE1 := mockRackLSE("")
		req := &ufsAPI.CreateRackLSERequest{
			RackLSE:   rackLSE1,
			RackLSEId: "rackLSE-1",
		}
		resp, err := tf.Fleet.CreateRackLSE(tf.C, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(rackLSE1))
		t.Run("Delete rackLSE by existing ID", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRackLSERequest{
				Name: util.AddPrefix(util.RackLSECollection, "rackLSE-1"),
			}
			_, err := tf.Fleet.DeleteRackLSE(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			greq := &ufsAPI.GetRackLSERequest{
				Name: util.AddPrefix(util.RackLSECollection, "rackLSE-1"),
			}
			res, err := tf.Fleet.GetRackLSE(tf.C, greq)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete rackLSE by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRackLSERequest{
				Name: util.AddPrefix(util.RackLSECollection, "rackLSE-2"),
			}
			_, err := tf.Fleet.DeleteRackLSE(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete rackLSE - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRackLSERequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Delete rackLSE - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRackLSERequest{
				Name: util.AddPrefix(util.RackLSECollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteRackLSE(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetMachineLSEDeployment(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("GetMachineLSEDeployment", t, func(t *ftt.Test) {
		t.Run("Get machine lse deployment record by existing ID", func(t *ftt.Test) {
			dr1, err := inventory.UpdateMachineLSEDeployments(ctx, []*ufspb.MachineLSEDeployment{
				{
					SerialNumber: "dr-get-1",
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dr1, should.HaveLength(1))
			dr1[0].SerialNumber = util.AddPrefix(util.MachineLSEDeploymentCollection, dr1[0].SerialNumber)

			req := &ufsAPI.GetMachineLSEDeploymentRequest{
				Name: util.AddPrefix(util.MachineLSEDeploymentCollection, "dr-get-1"),
			}
			resp, err := tf.Fleet.GetMachineLSEDeployment(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dr1[0]))
		})
		t.Run("Get machine lse deployment record by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineLSEDeploymentRequest{
				Name: util.AddPrefix(util.MachineLSEDeploymentCollection, "dr-get-2"),
			}
			resp, err := tf.Fleet.GetMachineLSEDeployment(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get machine lse deployment record - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineLSEDeploymentRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetMachineLSEDeployment(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Get machine lse deployment record - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineLSEDeploymentRequest{
				Name: util.AddPrefix(util.MachineLSEDeploymentCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetMachineLSEDeployment(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestCreateSchedulingUnit(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("CreateSchedulingUnit", t, func(t *ftt.Test) {
		t.Run("Create new SchedulingUnit with schedulingUnitId - happy path", func(t *ftt.Test) {
			su := mockSchedulingUnit("")
			req := &ufsAPI.CreateSchedulingUnitRequest{
				SchedulingUnit:   su,
				SchedulingUnitId: "su-1",
			}
			resp, err := tf.Fleet.CreateSchedulingUnit(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(su))
		})

		t.Run("Create new SchedulingUnit with nil entity", func(t *ftt.Test) {
			req := &ufsAPI.CreateSchedulingUnitRequest{
				SchedulingUnit:   nil,
				SchedulingUnitId: "su-2",
			}
			_, err := tf.Fleet.CreateSchedulingUnit(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new SchedulingUnit without schedulingUnitId", func(t *ftt.Test) {
			su := mockSchedulingUnit("")
			req := &ufsAPI.CreateSchedulingUnitRequest{
				SchedulingUnit: su,
			}
			_, err := tf.Fleet.CreateSchedulingUnit(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("b/388896025 -- empty string as machine LSE should be rejected informmatively", func(t *ftt.Test) {
			req := &ufsAPI.CreateSchedulingUnitRequest{
				SchedulingUnitId: "aaa",
				SchedulingUnit: &ufspb.SchedulingUnit{
					Name:        "aaa",
					MachineLSEs: []string{""},
				},
			}
			_, err := tf.Fleet.CreateSchedulingUnit(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			status := status.Code(err).String()
			assert.That(t, status, should.Equal(codes.InvalidArgument.String()))
		})
	})
}

func TestUpdateSchedulingUnit(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("UpdateSchedulingUnit", t, func(t *ftt.Test) {
		t.Run("Update existing SchedulingUnit - happy path", func(t *ftt.Test) {
			inventory.CreateSchedulingUnit(ctx, &ufspb.SchedulingUnit{
				Name: "su-1",
			})

			su1 := mockSchedulingUnit("su-1")
			su1.Description = "Updatedesc"
			ureq := &ufsAPI.UpdateSchedulingUnitRequest{
				SchedulingUnit: su1,
			}
			resp, err := tf.Fleet.UpdateSchedulingUnit(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(su1))
		})

		t.Run("Update SchedulingUnit - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateSchedulingUnitRequest{
				SchedulingUnit: nil,
			}
			resp, err := tf.Fleet.UpdateSchedulingUnit(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update SchedulingUnit - Invalid input empty name", func(t *ftt.Test) {
			su := mockSchedulingUnit("")
			su.Name = ""
			req := &ufsAPI.UpdateSchedulingUnitRequest{
				SchedulingUnit: su,
			}
			resp, err := tf.Fleet.UpdateSchedulingUnit(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update SchedulingUnit - Invalid input invalid name", func(t *ftt.Test) {
			su := mockSchedulingUnit("a.b)7&")
			req := &ufsAPI.UpdateSchedulingUnitRequest{
				SchedulingUnit: su,
			}
			resp, err := tf.Fleet.UpdateSchedulingUnit(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetSchedulingUnit(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	su, _ := inventory.CreateSchedulingUnit(ctx, &ufspb.SchedulingUnit{
		Name: "su-1",
	})
	ftt.Run("GetSchedulingUnit", t, func(t *ftt.Test) {
		t.Run("Get SchedulingUnit by existing ID - happy path", func(t *ftt.Test) {
			req := &ufsAPI.GetSchedulingUnitRequest{
				Name: util.AddPrefix(util.SchedulingUnitCollection, "su-1"),
			}
			resp, _ := tf.Fleet.GetSchedulingUnit(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			resp.Name = util.RemovePrefix(resp.Name)
			assert.Loosely(t, resp, should.Match(su))
		})

		t.Run("Get SchedulingUnit - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetSchedulingUnitRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetSchedulingUnit(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Get SchedulingUnit - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetSchedulingUnitRequest{
				Name: util.AddPrefix(util.SchedulingUnitCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetSchedulingUnit(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestDeleteSchedulingUnit(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	inventory.CreateSchedulingUnit(ctx, &ufspb.SchedulingUnit{
		Name: "su-1",
	})
	ftt.Run("DeleteSchedulingUnit", t, func(t *ftt.Test) {
		t.Run("Delete SchedulingUnit by existing ID - happy path", func(t *ftt.Test) {
			req := &ufsAPI.DeleteSchedulingUnitRequest{
				Name: util.AddPrefix(util.SchedulingUnitCollection, "su-1"),
			}
			_, err := tf.Fleet.DeleteSchedulingUnit(tf.C, req)
			assert.Loosely(t, err, should.BeNil)

			res, err := inventory.GetSchedulingUnit(tf.C, "su-1")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Delete SchedulingUnit - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteSchedulingUnitRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteSchedulingUnit(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete SchedulingUnit - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteSchedulingUnitRequest{
				Name: util.AddPrefix(util.SchedulingUnitCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteSchedulingUnit(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListSchedulingUnits(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	schedulingUnits := make([]*ufspb.SchedulingUnit, 0, 4)
	for i := 0; i < 4; i++ {
		su := mockSchedulingUnit("")
		su.Name = fmt.Sprintf("su-%d", i)
		resp, _ := inventory.CreateSchedulingUnit(tf.C, su)
		resp.Name = util.AddPrefix(util.SchedulingUnitCollection, resp.Name)
		schedulingUnits = append(schedulingUnits, resp)
	}
	ftt.Run("ListSchedulingUnits", t, func(t *ftt.Test) {
		t.Run("ListSchedulingUnits - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListSchedulingUnitsRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListSchedulingUnits(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListSchedulingUnits - Full listing with no pagination - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListSchedulingUnitsRequest{}
			resp, err := tf.Fleet.ListSchedulingUnits(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.SchedulingUnits, should.Match(schedulingUnits))
		})

		t.Run("ListSchedulingUnits - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListSchedulingUnitsRequest{
				Filter: "state=x|state=y",
			}
			_, err := tf.Fleet.ListSchedulingUnits(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})
	})
}

func TestGetDeviceData(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = gologger.StdConfig.Use(ctx)
	ctx = logging.SetLevel(ctx, logging.Debug)
	osCtx, _ := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()

	su, _ := inventory.CreateSchedulingUnit(osCtx, &ufspb.SchedulingUnit{
		Name: "su-1",
	})

	machine := &ufspb.Machine{
		Name: "machine-1",
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				ReferenceBoard: "test",
				BuildTarget:    "test",
				Model:          "test",
				Hwid:           "test",
			},
		},
	}
	registration.CreateMachine(osCtx, machine)

	machinelse := mockDutMachineLSE("lse-1")
	// In datastore, the name doesn't contain prefix
	machinelse.Name = "lse-1"
	machinelse.Machines = []string{"machine-1"}
	inventory.CreateMachineLSE(osCtx, machinelse)

	adm := &ufspb.Machine{
		Name: "adm-1",
		Device: &ufspb.Machine_AttachedDevice{
			AttachedDevice: &ufspb.AttachedDevice{
				Manufacturer: "Apple",
				DeviceType:   ufspb.AttachedDeviceType_ATTACHED_DEVICE_TYPE_APPLE_PHONE,
				BuildTarget:  "test",
				Model:        "test",
			},
		},
	}
	registration.CreateMachine(osCtx, adm)

	admlse := mockAttachedDeviceMachineLSE("admlse-1")
	admlse.Name = "admlse-1"
	admlse.Machines = []string{"adm-1"}
	inventory.CreateMachineLSE(osCtx, admlse)

	// Add Browser host/vm
	inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
		Name:     "browser-host1",
		Hostname: "browser-host1",
		Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
			ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
		},
	})
	vms := []*ufspb.VM{
		{
			Name: "browser-vm1",
			OsVersion: &ufspb.OSVersion{
				Value: "os-1",
			},
			ResourceState: ufspb.State_STATE_SERVING,
		},
	}
	inventory.BatchUpdateVMs(ctx, vms)

	ftt.Run("GetDeviceData", t, func(t *ftt.Test) {
		t.Run("Get Browser Host by hostname - happy path", func(t *ftt.Test) {
			req := &ufsAPI.GetDeviceDataRequest{
				Hostname: "browser-host1",
			}
			resp, err := tf.Fleet.GetDeviceData(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_BROWSER_DEVICE))
			assert.Loosely(t, resp.GetBrowserDeviceData().GetHost().GetHostname(), should.Equal("browser-host1"))
		})

		t.Run("Get Browser VM by hostname - happy path", func(t *ftt.Test) {
			req := &ufsAPI.GetDeviceDataRequest{
				Hostname: "browser-vm1",
			}
			resp, err := tf.Fleet.GetDeviceData(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_BROWSER_DEVICE))
			assert.Loosely(t, resp.GetBrowserDeviceData().GetVm().GetName(), should.Equal("browser-vm1"))
		})

		t.Run("Get SchedulingUnit by existing ID - happy path", func(t *ftt.Test) {
			req := &ufsAPI.GetDeviceDataRequest{
				Hostname: util.AddPrefix(util.SchedulingUnitCollection, "su-1"),
			}
			resp, err := tf.Fleet.GetDeviceData(osCtx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			resp.GetSchedulingUnit().Name = util.RemovePrefix(resp.GetSchedulingUnit().Name)
			assert.Loosely(t, resp.GetSchedulingUnit(), should.Match(su))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT))
		})

		t.Run("Get ChromeOSDeviceData by existing hostname - happy path", func(t *ftt.Test) {
			req := &ufsAPI.GetDeviceDataRequest{
				Hostname: util.AddPrefix(util.MachineLSECollection, "lse-1"),
			}
			resp, err := tf.Fleet.GetDeviceData(osCtx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetChromeOsDeviceData().GetLabConfig(), should.Match(machinelse))
			assert.Loosely(t, resp.GetChromeOsDeviceData().GetMachine(), should.Match(machine))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE))
		})

		t.Run("Get ChromeOSDeviceData by existing hostname in os namespace", func(t *ftt.Test) {
			machineOs := &ufspb.Machine{
				Name: "machine-os-1",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						ReferenceBoard: "test",
						BuildTarget:    "test",
						Model:          "test",
						Hwid:           "test",
					},
				},
			}
			registration.CreateMachine(osCtx, machineOs)

			machineOsLse := mockDutMachineLSE("lse-os-1")
			machineOsLse.Name = "lse-os-1"
			machineOsLse.Machines = []string{"machine-os-1"}
			inventory.CreateMachineLSE(osCtx, machineOsLse)

			req := &ufsAPI.GetDeviceDataRequest{
				Hostname: util.AddPrefix(util.MachineLSECollection, "lse-os-1"),
			}
			resp, err := tf.Fleet.GetDeviceData(osCtx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetChromeOsDeviceData().GetLabConfig(), should.Match(machineOsLse))
			assert.Loosely(t, resp.GetChromeOsDeviceData().GetMachine(), should.Match(machineOs))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE))

			// Should not exist in Browser namespace
			respBrowser, err := tf.Fleet.GetDeviceData(tf.C, req)
			assert.Loosely(t, respBrowser, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("no valid device found"))
		})

		t.Run("Get ChromeOSDeviceData by existing asset tag - happy path", func(t *ftt.Test) {
			req := &ufsAPI.GetDeviceDataRequest{
				DeviceId: "machine-1",
			}
			resp, err := tf.Fleet.GetDeviceData(osCtx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetChromeOsDeviceData().GetLabConfig(), should.Match(machinelse))
			assert.Loosely(t, resp.GetChromeOsDeviceData().GetMachine(), should.Match(machine))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE))
		})

		t.Run("Get AttachedDeviceData by existing hostname - happy path", func(t *ftt.Test) {
			req := &ufsAPI.GetDeviceDataRequest{
				Hostname: util.AddPrefix(util.MachineLSECollection, "admlse-1"),
			}
			resp, err := tf.Fleet.GetDeviceData(osCtx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetAttachedDeviceData().GetLabConfig(), should.Match(admlse))
			assert.Loosely(t, resp.GetAttachedDeviceData().GetMachine(), should.Match(adm))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE))
		})

		t.Run("Get AttachedDeviceData by existing hostname in os namespace", func(t *ftt.Test) {
			adm := &ufspb.Machine{
				Name: "adm-os-1",
				Device: &ufspb.Machine_AttachedDevice{
					AttachedDevice: &ufspb.AttachedDevice{
						Manufacturer: "Apple",
						DeviceType:   ufspb.AttachedDeviceType_ATTACHED_DEVICE_TYPE_APPLE_PHONE,
						BuildTarget:  "test",
						Model:        "test",
					},
				},
			}
			registration.CreateMachine(osCtx, adm)

			admlse := mockAttachedDeviceMachineLSE("admlse-os-1")
			admlse.Name = "admlse-os-1"
			admlse.Machines = []string{"adm-os-1"}
			inventory.CreateMachineLSE(osCtx, admlse)

			req := &ufsAPI.GetDeviceDataRequest{
				Hostname: util.AddPrefix(util.MachineLSECollection, "admlse-os-1"),
			}
			resp, err := tf.Fleet.GetDeviceData(osCtx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetAttachedDeviceData().GetLabConfig(), should.Match(admlse))
			assert.Loosely(t, resp.GetAttachedDeviceData().GetMachine(), should.Match(adm))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE))

			// Should not exist in Browser namespace
			respBrowser, err := tf.Fleet.GetDeviceData(tf.C, req)
			assert.Loosely(t, respBrowser, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("no valid device found"))
		})

		t.Run("Get AttachedDeviceData by existing asset tag - happy path", func(t *ftt.Test) {
			req := &ufsAPI.GetDeviceDataRequest{
				DeviceId: "adm-1",
			}
			resp, err := tf.Fleet.GetDeviceData(osCtx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetAttachedDeviceData().GetLabConfig(), should.Match(admlse))
			assert.Loosely(t, resp.GetAttachedDeviceData().GetMachine(), should.Match(adm))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE))
		})

		t.Run("Get device data - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetDeviceDataRequest{
				Hostname: "",
			}
			resp, err := tf.Fleet.GetDeviceData(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Both Id and hostname are empty"))
		})

		t.Run("Get device data - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetDeviceDataRequest{
				Hostname: util.AddPrefix(util.SchedulingUnitCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetDeviceData(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("no valid device found"))
		})
	})
}

func mustParseMachineLSE(input string) *ufspb.MachineLSE {
	out := &ufspb.MachineLSE{}
	err := protojson.Unmarshal([]byte(input), out)
	if err != nil {
		panic(err.Error())
	}
	return out
}

// TestGetDUTsForLabstation tests extracting DUT names from servo names on a labstation object.
func TestGetDUTsForLabstation(t *testing.T) {
	t.Parallel()
	ctx := memory.Use(context.Background())
	datastore.GetTestable(ctx).Consistent(true)
	ctx = logging.SetLevel(ctx, logging.Debug)
	ctx, _ = util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()

	const labstation = `
{
        "name": "fake-labstation",
        "hostname": "fake-labstation",
        "chromeosMachineLse": {
                "deviceLse": {
                        "networkDeviceInterface": null,
                        "labstation": {
                                "hostname": "fake-labstation",
                                "servos": [
                                        {
                                                "servoHostname": "fake-labstation",
                                                "servoPort": 9999,
                                                "servoSerial": "XXXXXXXXXXX"
                                        }
                                ],
                                "rpm": {
                                        "powerunitName": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
                                        "powerunitOutlet": "XXX",
                                        "powerunitType": "TYPE_UNKNOWN"
                                },
                                "pools": [
                                        "labstation_main"
                                ]
                        }
                }
        },
        "machines": [
                "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
        ]
}
`

	const dut = `
{
        "name":  "fake-dut",
        "machineLsePrototype":  "atl:standard",
        "hostname":  "fake-dut",
        "chromeosMachineLse":  {
                "deviceLse":  {
                        "dut":  {
                                "hostname":  "fake-dut",
                                "peripherals":  {
                                        "servo":  {
                                                "servoHostname":  "fake-labstation",
                                                "servoPort":  9999
                                        }
                                }
                        }
                }
        },
        "machines":  [
                "XXXXXXX"
        ]
}
`

	ftt.Run("smoke test", t, func(t *ftt.Test) {
		_, err := inventory.CreateMachineLSE(ctx, mustParseMachineLSE(labstation))
		if err != nil {
			panic(err.Error())
		}

		req := &ufsAPI.GetDUTsForLabstationRequest{
			Hostname: []string{"fake-labstation"},
		}

		resp, err := tf.Fleet.GetDUTsForLabstation(tf.C, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(&ufsAPI.GetDUTsForLabstationResponse{
			Items: []*ufsAPI.GetDUTsForLabstationResponse_LabstationMapping{
				{
					Hostname: "fake-labstation",
					DutName:  []string{},
				},
			},
		}))

		_, err = inventory.CreateMachineLSE(ctx, mustParseMachineLSE(dut))
		if err != nil {
			panic(err.Error())
		}

		resp, err = tf.Fleet.GetDUTsForLabstation(tf.C, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(&ufsAPI.GetDUTsForLabstationResponse{
			Items: []*ufsAPI.GetDUTsForLabstationResponse_LabstationMapping{
				{
					Hostname: "fake-labstation",
					DutName:  []string{"fake-dut"},
				},
			},
		}))

	})
}
