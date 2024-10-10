// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/model/configuration"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/registration"
	"infra/unifiedfleet/app/util"
)

func mockChromeOSMachine(id, lab, board string) *ufspb.Machine {
	return &ufspb.Machine{
		Name: util.AddPrefix(util.MachineCollection, id),
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				ReferenceBoard: board,
			},
		},
	}
}

func mockChromeBrowserMachine(id, lab, name string) *ufspb.Machine {
	return &ufspb.Machine{
		Name: util.AddPrefix(util.MachineCollection, id),
		Device: &ufspb.Machine_ChromeBrowserMachine{
			ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
				Description: name,
			},
		},
	}
}

func assertMachineEqual(t *ftt.Test, a *ufspb.Machine, b *ufspb.Machine) {
	assert.Loosely(t, a.GetName(), should.Equal(b.GetName()))
	assert.Loosely(t, a.GetChromeBrowserMachine().GetDescription(), should.Equal(
		b.GetChromeBrowserMachine().GetDescription()))
	assert.Loosely(t, a.GetChromeosMachine().GetReferenceBoard(), should.Equal(
		b.GetChromeosMachine().GetReferenceBoard()))
}

func mockRack(id string, rackCapactiy int32) *ufspb.Rack {
	return &ufspb.Rack{
		Name:       util.AddPrefix(util.RackCollection, id),
		CapacityRu: rackCapactiy,
	}
}

func mockNic(id string) *ufspb.Nic {
	return &ufspb.Nic{
		Name:       util.AddPrefix(util.NicCollection, id),
		MacAddress: "12:ab:34:56:78:90",
		SwitchInterface: &ufspb.SwitchInterface{
			Switch:   "test-switch",
			PortName: "1",
		},
	}
}

func mockKVM(id string) *ufspb.KVM {
	return &ufspb.KVM{
		Name: util.AddPrefix(util.KVMCollection, id),
	}
}

func mockRPM(id string) *ufspb.RPM {
	return &ufspb.RPM{
		Name: util.AddPrefix(util.RPMCollection, id),
	}
}

func mockDrac(id string) *ufspb.Drac {
	return &ufspb.Drac{
		Name:       util.AddPrefix(util.DracCollection, id),
		MacAddress: "12:ab:34:56:78:90",
		SwitchInterface: &ufspb.SwitchInterface{
			Switch:   "test-switch",
			PortName: "1",
		},
	}
}

func mockSwitch(id string) *ufspb.Switch {
	return &ufspb.Switch{
		Name: util.AddPrefix(util.SwitchCollection, id),
	}
}

func TestMachineRegistration(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("Machine Registration", t, func(t *ftt.Test) {
		t.Run("Register machine - happy path", func(t *ftt.Test) {
			s := mockSwitch("")
			s.Name = "test-switch"
			_, err := registration.CreateSwitch(ctx, s)
			assert.Loosely(t, err, should.BeNil)

			nic := mockNic("")
			nic.Name = "nic-X"
			nics := []*ufspb.Nic{nic}
			drac := mockDrac("")
			drac.Name = "drac-X"
			machine := &ufspb.Machine{
				Name: "machine-X",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						NicObjects: nics,
						DracObject: drac,
					},
				},
			}
			req := &ufsAPI.MachineRegistrationRequest{
				Machine: machine,
			}
			resp, err := tf.Fleet.MachineRegistration(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(machine))
		})

		t.Run("Register machine with nil machine", func(t *ftt.Test) {
			req := &ufsAPI.MachineRegistrationRequest{}
			_, err := tf.Fleet.MachineRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Machine "+ufsAPI.NilEntity))
		})

		t.Run("Register machine - Invalid input empty machine name", func(t *ftt.Test) {
			req := &ufsAPI.MachineRegistrationRequest{
				Machine: &ufspb.Machine{
					Name: "",
				},
			}
			_, err := tf.Fleet.MachineRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Machine "+ufsAPI.EmptyName))
		})

		t.Run("Create new machine - Invalid input invalid characters in machine name", func(t *ftt.Test) {
			req := &ufsAPI.MachineRegistrationRequest{
				Machine: &ufspb.Machine{
					Name: "a.b)7&",
				},
			}
			_, err := tf.Fleet.MachineRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})

		t.Run("Register machine - Invalid input empty nic name", func(t *ftt.Test) {
			req := &ufsAPI.MachineRegistrationRequest{
				Machine: &ufspb.Machine{
					Name: "machine-1",
					Device: &ufspb.Machine_ChromeBrowserMachine{
						ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
							NicObjects: []*ufspb.Nic{{
								Name: "",
							}},
						},
					},
				},
			}
			_, err := tf.Fleet.MachineRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Nic "+ufsAPI.EmptyName))
		})

		t.Run("Register machine - Invalid input invalid characters in nic name", func(t *ftt.Test) {
			req := &ufsAPI.MachineRegistrationRequest{
				Machine: &ufspb.Machine{
					Name: "machine-1",
					Device: &ufspb.Machine_ChromeBrowserMachine{
						ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
							NicObjects: []*ufspb.Nic{{
								Name: "a.b)7&",
							}},
						},
					},
				},
			}
			_, err := tf.Fleet.MachineRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})

		t.Run("Register machine - Invalid input empty drac name", func(t *ftt.Test) {
			req := &ufsAPI.MachineRegistrationRequest{
				Machine: &ufspb.Machine{
					Name: "machine-1",
					Device: &ufspb.Machine_ChromeBrowserMachine{
						ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
							DracObject: &ufspb.Drac{
								Name: "",
							},
						},
					},
				},
			}
			_, err := tf.Fleet.MachineRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Drac "+ufsAPI.EmptyName))
		})

		t.Run("Register machine - Invalid input invalid characters in drac name", func(t *ftt.Test) {
			req := &ufsAPI.MachineRegistrationRequest{
				Machine: &ufspb.Machine{
					Name: "machine-1",
					Device: &ufspb.Machine_ChromeBrowserMachine{
						ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
							DracObject: &ufspb.Drac{
								Name: "a.b)7&",
							},
						},
					},
				},
			}
			_, err := tf.Fleet.MachineRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestRackRegistration(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("Rack Registration", t, func(t *ftt.Test) {
		t.Run("Register Rack - happy path", func(t *ftt.Test) {
			switches := []*ufspb.Switch{{
				Name: "switch-X",
			}}
			kvms := []*ufspb.KVM{{
				Name: "kvm-X",
			}}
			rack := &ufspb.Rack{
				Name: "rack-X",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{
						SwitchObjects: switches,
						KvmObjects:    kvms,
					},
				},
			}
			req := &ufsAPI.RackRegistrationRequest{
				Rack: rack,
			}
			resp, _ := tf.Fleet.RackRegistration(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(rack))
		})

		t.Run("Register rack with nil rack", func(t *ftt.Test) {
			req := &ufsAPI.RackRegistrationRequest{}
			_, err := tf.Fleet.RackRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Rack "+ufsAPI.NilEntity))
		})

		t.Run("Register rack - Invalid input empty rack name", func(t *ftt.Test) {
			req := &ufsAPI.RackRegistrationRequest{
				Rack: &ufspb.Rack{
					Name: "",
				},
			}
			_, err := tf.Fleet.RackRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Rack "+ufsAPI.EmptyName))
		})

		t.Run("Create new rack - Invalid input invalid characters in rack name", func(t *ftt.Test) {
			req := &ufsAPI.RackRegistrationRequest{
				Rack: &ufspb.Rack{
					Name: "a.b)7&",
				},
			}
			_, err := tf.Fleet.RackRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})

		t.Run("Register rack - Invalid input empty switch name", func(t *ftt.Test) {
			req := &ufsAPI.RackRegistrationRequest{
				Rack: &ufspb.Rack{
					Name: "rack-1",
					Rack: &ufspb.Rack_ChromeBrowserRack{
						ChromeBrowserRack: &ufspb.ChromeBrowserRack{
							SwitchObjects: []*ufspb.Switch{{
								Name: "",
							}},
						},
					},
				},
			}
			_, err := tf.Fleet.RackRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Switch "+ufsAPI.EmptyName))
		})

		t.Run("Register rack - Invalid input invalid characters in switch name", func(t *ftt.Test) {
			req := &ufsAPI.RackRegistrationRequest{
				Rack: &ufspb.Rack{
					Name: "rack-1",
					Rack: &ufspb.Rack_ChromeBrowserRack{
						ChromeBrowserRack: &ufspb.ChromeBrowserRack{
							SwitchObjects: []*ufspb.Switch{{
								Name: "a.b)7&",
							}},
						},
					},
				},
			}
			_, err := tf.Fleet.RackRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})

		t.Run("Register rack - Invalid input empty kvm name", func(t *ftt.Test) {
			req := &ufsAPI.RackRegistrationRequest{
				Rack: &ufspb.Rack{
					Name: "rack-1",
					Rack: &ufspb.Rack_ChromeBrowserRack{
						ChromeBrowserRack: &ufspb.ChromeBrowserRack{
							KvmObjects: []*ufspb.KVM{{
								Name: "",
							}},
						},
					},
				},
			}
			_, err := tf.Fleet.RackRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("KVM "+ufsAPI.EmptyName))
		})

		t.Run("Register rack - Invalid input invalid characters in kvm name", func(t *ftt.Test) {
			req := &ufsAPI.RackRegistrationRequest{
				Rack: &ufspb.Rack{
					Name: "rack-1",
					Rack: &ufspb.Rack_ChromeBrowserRack{
						ChromeBrowserRack: &ufspb.ChromeBrowserRack{
							KvmObjects: []*ufspb.KVM{{
								Name: "a.b)7&",
							}},
						},
					},
				},
			}
			_, err := tf.Fleet.RackRegistration(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestUpdateMachine(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	chromeOSMachine2 := mockChromeOSMachine("chromeos-asset-1", "chromeoslab", "veyron")
	chromeOSMachine3 := mockChromeOSMachine("", "chromeoslab", "samus")
	chromeOSMachine4 := mockChromeOSMachine("a.b)7&", "chromeoslab", "samus")
	ftt.Run("UpdateMachines", t, func(t *ftt.Test) {
		t.Run("Update existing machines - happy path", func(t *ftt.Test) {
			_, err := registration.CreateMachine(tf.C, &ufspb.Machine{
				Name: "chromeos-asset-1",
			})
			assert.Loosely(t, err, should.BeNil)

			ureq := &ufsAPI.UpdateMachineRequest{
				Machine: chromeOSMachine2,
			}
			resp, err := tf.Fleet.UpdateMachine(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assertMachineEqual(t, resp, chromeOSMachine2)
		})

		t.Run("Update machine - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateMachineRequest{
				Machine: nil,
			}
			resp, err := tf.Fleet.UpdateMachine(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update machine - Invalid input empty name", func(t *ftt.Test) {
			chromeOSMachine3.Name = ""
			req := &ufsAPI.UpdateMachineRequest{
				Machine: chromeOSMachine3,
			}
			resp, err := tf.Fleet.UpdateMachine(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update machine - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.UpdateMachineRequest{
				Machine: chromeOSMachine4,
			}
			resp, err := tf.Fleet.UpdateMachine(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetMachine(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	chromeOSMachine1, _ := registration.CreateMachine(tf.C, &ufspb.Machine{
		Name: "chromeos-asset-1",
	})
	ftt.Run("GetMachine", t, func(t *ftt.Test) {
		t.Run("Get machine by existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineRequest{
				Name: util.AddPrefix(util.MachineCollection, "chromeos-asset-1"),
			}
			resp, err := tf.Fleet.GetMachine(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			resp.Name = util.RemovePrefix(resp.Name)
			assert.Loosely(t, resp, should.Resemble(chromeOSMachine1))
		})

		t.Run("Get machine - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetMachine(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Get machine - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineRequest{
				Name: util.AddPrefix(util.MachineCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetMachine(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListMachines(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	machines := make([]*ufspb.Machine, 0, 4)
	for i := 0; i < 4; i++ {
		chromeOSMachine1 := mockChromeOSMachine("", "chromeoslab", "samus")
		chromeOSMachine1.Name = fmt.Sprintf("chromeos-asset-%d", i)
		resp, _ := registration.CreateMachine(tf.C, chromeOSMachine1)
		chromeOSMachine1.Name = util.AddPrefix(util.MachineCollection, chromeOSMachine1.Name)
		machines = append(machines, resp)
	}
	ftt.Run("ListMachines", t, func(t *ftt.Test) {
		t.Run("ListMachines - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListMachinesRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListMachines(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListMachines - Full listing - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListMachinesRequest{}
			resp, err := tf.Fleet.ListMachines(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Machines, should.Resemble(machines))
		})

		t.Run("ListMachines - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListMachinesRequest{
				Filter: "machine=mac-1 | rpm=rpm-2",
			}
			_, err := tf.Fleet.ListMachines(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})

		t.Run("ListMachines - filter format valid AND", func(t *ftt.Test) {
			req := &ufsAPI.ListMachinesRequest{
				Filter: "kvm=kvm-1 & rpm=rpm-1",
			}
			resp, err := tf.Fleet.ListMachines(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Machines, should.BeNil)
		})

		t.Run("ListMachines - filter format valid", func(t *ftt.Test) {
			req := &ufsAPI.ListMachinesRequest{
				Filter: "rack=rack-1",
			}
			resp, err := tf.Fleet.ListMachines(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Machines, should.BeNil)
		})
	})
}

func TestDeleteMachine(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	registration.CreateMachine(tf.C, &ufspb.Machine{
		Name: "chromeos-asset-1",
	})
	ftt.Run("DeleteMachine", t, func(t *ftt.Test) {
		t.Run("Delete machine by existing ID", func(t *ftt.Test) {
			req := &ufsAPI.DeleteMachineRequest{
				Name: util.AddPrefix(util.MachineCollection, "chromeos-asset-1"),
			}
			_, err := tf.Fleet.DeleteMachine(tf.C, req)
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.GetMachine(tf.C, "chromeos-asset-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Delete machine - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteMachineRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteMachine(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete machine - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteMachineRequest{
				Name: util.AddPrefix(util.MachineCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteMachine(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestRenameMachine(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("RenameMachine", t, func(t *ftt.Test) {
		t.Run("Rename an empty machine name", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameMachine(tf.C, &ufsAPI.RenameMachineRequest{
				Name: "",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Rename a machine to an empty name", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameMachine(tf.C, &ufsAPI.RenameMachineRequest{
				Name:    "oldMachine",
				NewName: "",
			})
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("The new machine name after renaming doesn't follow machine name format", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameMachine(tf.C, &ufsAPI.RenameMachineRequest{
				Name:    "machines/oldMachine",
				NewName: "newMachine",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.MachineNameFormat))
		})
	})
}

func TestUpdateRack(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	rack3 := mockRack("rack-3", 6)
	rack4 := mockRack("a.b)7&", 6)
	ftt.Run("UpdateRack", t, func(t *ftt.Test) {
		t.Run("Update rack - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateRackRequest{
				Rack: nil,
			}
			resp, err := tf.Fleet.UpdateRack(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update rack - Invalid input empty name", func(t *ftt.Test) {
			rack3.Name = ""
			req := &ufsAPI.UpdateRackRequest{
				Rack: rack3,
			}
			resp, err := tf.Fleet.UpdateRack(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update rack - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.UpdateRackRequest{
				Rack: rack4,
			}
			resp, err := tf.Fleet.UpdateRack(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetRack(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("GetRack", t, func(t *ftt.Test) {
		t.Run("Get rack by existing ID", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-1",
			}
			_, err := registration.CreateRack(tf.C, rack1)
			assert.Loosely(t, err, should.BeNil)
			rack1.Name = util.AddPrefix(util.RackCollection, "rack-1")

			resp, err := tf.Fleet.GetRack(tf.C, &ufsAPI.GetRackRequest{
				Name: util.AddPrefix(util.RackCollection, "rack-1"),
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(rack1))

			resp, err = tf.Fleet.GetRack(tf.C, &ufsAPI.GetRackRequest{
				Name: util.AddPrefix(util.RackCollection, "RACK-1"),
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(rack1))
		})
		t.Run("Get rack by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetRackRequest{
				Name: util.AddPrefix(util.RackCollection, "rack-2"),
			}
			resp, err := tf.Fleet.GetRack(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get rack - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetRackRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetRack(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Get rack - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetRackRequest{
				Name: util.AddPrefix(util.RackCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetRack(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListRacks(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	racks := make([]*ufspb.Rack, 0, 4)
	for i := 0; i < 4; i++ {
		rack1 := &ufspb.Rack{
			Name: fmt.Sprintf("rack-%d", i),
		}
		resp, _ := registration.CreateRack(tf.C, rack1)
		rack1.Name = util.AddPrefix(util.RackCollection, rack1.Name)
		racks = append(racks, resp)
	}
	ftt.Run("ListRacks", t, func(t *ftt.Test) {
		t.Run("ListRacks - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListRacksRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListRacks(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListRacks - Full listing - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListRacksRequest{}
			resp, err := tf.Fleet.ListRacks(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Racks, should.Resemble(racks))
		})

		t.Run("ListRacks - filter format valid", func(t *ftt.Test) {
			req := &ufsAPI.ListRacksRequest{
				Filter: "tag=tag-1",
			}
			resp, err := tf.Fleet.ListRacks(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Racks, should.BeNil)
		})
	})
}

func TestDeleteRack(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("DeleteRack", t, func(t *ftt.Test) {
		t.Run("Delete rack by existing ID", func(t *ftt.Test) {
			_, err := registration.CreateRack(tf.C, &ufspb.Rack{
				Name: "rack-1",
			})
			assert.Loosely(t, err, should.BeNil)

			req := &ufsAPI.DeleteRackRequest{
				Name: util.AddPrefix(util.RackCollection, "rack-1"),
			}
			_, err = tf.Fleet.DeleteRack(tf.C, req)
			assert.Loosely(t, err, should.BeNil)

			res, err := registration.GetRack(tf.C, "rack-1")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete rack by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRackRequest{
				Name: util.AddPrefix(util.RackCollection, "rack-2"),
			}
			_, err := tf.Fleet.DeleteRack(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete rack - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRackRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteRack(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Delete rack - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRackRequest{
				Name: util.AddPrefix(util.RackCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteRack(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestRenameRack(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("RenameRack", t, func(t *ftt.Test) {
		t.Run("Rename a empty rack name", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameRack(tf.C, &ufsAPI.RenameRackRequest{
				Name: "",
			})
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Rename a rack to an empty name", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameRack(tf.C, &ufsAPI.RenameRackRequest{
				Name:    "oldRack",
				NewName: "",
			})
			assert.Loosely(t, err.Error(), should.ContainSubstring("Missing new rack name"))
		})
		t.Run("The new rack name after renaming doesn't follow rackname format", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameRack(tf.C, &ufsAPI.RenameRackRequest{
				Name:    "racks/oldRack",
				NewName: "newrack",
			})
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.RackNameFormat))
		})
		t.Run("The new rack name is not all lowercase", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameRack(tf.C, &ufsAPI.RenameRackRequest{
				Name:    "racks/oldRack",
				NewName: "newRack",
			})
			assert.Loosely(t, err.Error(), should.ContainSubstring("should be in all lowercase"))
		})
	})
}

func TestCreateNic(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	machine1 := &ufspb.Machine{
		Name: "machine-1",
		Device: &ufspb.Machine_ChromeBrowserMachine{
			ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
		},
	}
	registration.CreateMachine(tf.C, machine1)
	registration.CreateSwitch(tf.C, &ufspb.Switch{
		Name:         "test-switch",
		CapacityPort: 100,
	})
	ftt.Run("CreateNic", t, func(t *ftt.Test) {
		t.Run("Create new nic with nic_id", func(t *ftt.Test) {
			nic := mockNic("")
			nic.Machine = "machine-1"
			req := &ufsAPI.CreateNicRequest{
				Nic:   nic,
				NicId: "nic-1",
			}
			resp, err := tf.Fleet.CreateNic(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(nic))
		})

		t.Run("Create new nic - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.CreateNicRequest{
				Nic: nil,
			}
			resp, err := tf.Fleet.CreateNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new nic - Invalid mac", func(t *ftt.Test) {
			nic := mockNic("createNic-0")
			nic.MacAddress = "123"
			req := &ufsAPI.CreateNicRequest{
				Nic:   nic,
				NicId: "createNic-0",
			}
			resp, err := tf.Fleet.CreateNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidMac))
		})

		t.Run("Create new nic - Invalid input empty ID", func(t *ftt.Test) {
			nic := mockNic("")
			nic.Machine = "machine-1"
			req := &ufsAPI.CreateNicRequest{
				Nic:   nic,
				NicId: "",
			}
			resp, err := tf.Fleet.CreateNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new nic - Invalid input invalid characters", func(t *ftt.Test) {
			nic := mockNic("")
			nic.Machine = "machine-1"
			req := &ufsAPI.CreateNicRequest{
				Nic:   nic,
				NicId: "a.b)7&",
			}
			resp, err := tf.Fleet.CreateNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})

		t.Run("Create new nic - Invalid input empty machine", func(t *ftt.Test) {
			nic := mockNic("")
			req := &ufsAPI.CreateNicRequest{
				Nic:   nic,
				NicId: "nic-5",
			}
			resp, err := tf.Fleet.CreateNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyMachineName))
		})
	})
}

func TestUpdateNic(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	machine1 := &ufspb.Machine{
		Name: "machine-1",
		Device: &ufspb.Machine_ChromeBrowserMachine{
			ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
		},
	}
	registration.CreateMachine(tf.C, machine1)
	ftt.Run("UpdateNic", t, func(t *ftt.Test) {
		t.Run("Update existing nic", func(t *ftt.Test) {
			nic1 := &ufspb.Nic{
				Name:    "nic-1",
				Machine: "machine-1",
			}
			_, err := registration.CreateNic(tf.C, nic1)
			assert.Loosely(t, err, should.BeNil)

			nic2 := mockNic("nic-1")
			nic2.SwitchInterface = nil
			nic2.Machine = "machine-1"
			ureq := &ufsAPI.UpdateNicRequest{
				Nic: nic2,
			}
			resp, err := tf.Fleet.UpdateNic(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(nic2))
		})

		t.Run("Update nic - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateNicRequest{
				Nic: nil,
			}
			resp, err := tf.Fleet.UpdateNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update nic - Invalid mac", func(t *ftt.Test) {
			nic := mockNic("updateNic-0")
			nic.MacAddress = "123"
			nic.Machine = "machine-1"
			req := &ufsAPI.UpdateNicRequest{
				Nic: nic,
			}
			resp, err := tf.Fleet.UpdateNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidMac))
		})

		t.Run("Update nic - Invalid input empty name", func(t *ftt.Test) {
			nic := mockNic("")
			nic.Name = ""
			nic.Machine = "machine-1"
			req := &ufsAPI.UpdateNicRequest{
				Nic: nic,
			}
			resp, err := tf.Fleet.UpdateNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update nic - Invalid input invalid characters", func(t *ftt.Test) {
			nic := mockNic("a.b)7&")
			nic.Machine = "machine-1"
			req := &ufsAPI.UpdateNicRequest{
				Nic: nic,
			}
			resp, err := tf.Fleet.UpdateNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetNic(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("GetNic", t, func(t *ftt.Test) {
		t.Run("Get nic by existing ID", func(t *ftt.Test) {
			nic1 := &ufspb.Nic{
				Name: "nic-1",
			}
			registration.CreateNic(tf.C, nic1)
			nic1.Name = util.AddPrefix(util.NicCollection, "nic-1")

			req := &ufsAPI.GetNicRequest{
				Name: util.AddPrefix(util.NicCollection, "nic-1"),
			}
			resp, err := tf.Fleet.GetNic(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(nic1))
		})

		t.Run("Get nic - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetNicRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Get nic - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetNicRequest{
				Name: util.AddPrefix(util.NicCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListNics(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	nics := make([]*ufspb.Nic, 0, 4)
	for i := 0; i < 4; i++ {
		nic := &ufspb.Nic{
			Name: fmt.Sprintf("nic-%d", i),
		}
		resp, _ := registration.CreateNic(tf.C, nic)
		nic.Name = util.AddPrefix(util.NicCollection, nic.Name)
		nics = append(nics, resp)
	}
	ftt.Run("ListNics", t, func(t *ftt.Test) {
		t.Run("ListNics - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListNicsRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListNics(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListNics - Full listing - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListNicsRequest{}
			resp, err := tf.Fleet.ListNics(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Nics, should.Resemble(nics))
		})

		t.Run("ListNics - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListNicsRequest{
				Filter: "nic=mac-1 | rpm=rpm-2",
			}
			_, err := tf.Fleet.ListNics(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})

		t.Run("ListNics - filter format valid", func(t *ftt.Test) {
			req := &ufsAPI.ListNicsRequest{
				Filter: "switch=switch-1",
			}
			resp, err := tf.Fleet.ListNics(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Nics, should.BeNil)
		})
	})
}

func TestDeleteNic(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("DeleteNic", t, func(t *ftt.Test) {
		t.Run("Delete nic - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteNicRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete nic - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteNicRequest{
				Name: util.AddPrefix(util.NicCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteNic(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestRenameNic(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("RenameNic", t, func(t *ftt.Test) {
		t.Run("Rename an empty nic name", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameNic(tf.C, &ufsAPI.RenameNicRequest{
				Name: "",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Rename a nic to an empty name", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameNic(tf.C, &ufsAPI.RenameNicRequest{
				Name:    "oldNic",
				NewName: "",
			})
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("The new nic name after renaming doesn't follow nic name format", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameNic(tf.C, &ufsAPI.RenameNicRequest{
				Name:    "nics/oldNic",
				NewName: "newNic",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NicNameFormat))
		})
	})
}

func TestCreateKVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	rack1 := &ufspb.Rack{
		Name: "rack-1",
		Rack: &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
		},
	}
	registration.CreateRack(tf.C, rack1)
	ftt.Run("CreateKVM", t, func(t *ftt.Test) {
		t.Run("Create new KVM with KVM_id", func(t *ftt.Test) {
			KVM1 := mockKVM("")
			KVM1.Rack = "rack-1"
			req := &ufsAPI.CreateKVMRequest{
				KVM:   KVM1,
				KVMId: "KVM-1",
			}
			resp, err := tf.Fleet.CreateKVM(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(KVM1))
		})

		t.Run("Create new KVM - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.CreateKVMRequest{
				KVM: nil,
			}
			resp, err := tf.Fleet.CreateKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create kvm - Invalid mac", func(t *ftt.Test) {
			kvm := mockKVM("createKVM-0")
			kvm.MacAddress = "123"
			kvm.Rack = "rack-1"
			req := &ufsAPI.CreateKVMRequest{
				KVM:   kvm,
				KVMId: "createKVM-0",
			}
			resp, err := tf.Fleet.CreateKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidMac))
		})

		t.Run("Create new KVM - Invalid input empty ID", func(t *ftt.Test) {
			KVM1 := mockKVM("")
			KVM1.Rack = "rack-1"
			req := &ufsAPI.CreateKVMRequest{
				KVM:   KVM1,
				KVMId: "",
			}
			resp, err := tf.Fleet.CreateKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new KVM - Invalid input invalid characters", func(t *ftt.Test) {
			KVM1 := mockKVM("")
			KVM1.Rack = "rack-1"
			req := &ufsAPI.CreateKVMRequest{
				KVM:   KVM1,
				KVMId: "a.b)7&",
			}
			resp, err := tf.Fleet.CreateKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})

		t.Run("Create new kvm - Invalid input empty rack", func(t *ftt.Test) {
			req := &ufsAPI.CreateKVMRequest{
				KVM:   mockKVM("x"),
				KVMId: "kvm-5",
			}
			resp, err := tf.Fleet.CreateKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyRackName))
		})
	})
}

func TestUpdateKVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	rack1 := &ufspb.Rack{
		Name: "rack-1",
		Rack: &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
		},
	}
	registration.CreateRack(tf.C, rack1)
	ftt.Run("UpdateKVM", t, func(t *ftt.Test) {
		t.Run("Update existing KVM", func(t *ftt.Test) {
			KVM1 := &ufspb.KVM{
				Name: "kvm-1",
				Rack: "rack-1",
			}
			resp, err := registration.CreateKVM(tf.C, KVM1)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			KVM2 := mockKVM("KVM-1")
			KVM2.Rack = "rack-1"
			ureq := &ufsAPI.UpdateKVMRequest{
				KVM: KVM2,
			}
			resp, err = tf.Fleet.UpdateKVM(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(KVM2))
		})

		t.Run("Update KVM - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateKVMRequest{
				KVM: nil,
			}
			resp, err := tf.Fleet.UpdateKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update kvm - Invalid mac", func(t *ftt.Test) {
			kvm := mockKVM("updateKVM-0")
			kvm.MacAddress = "123"
			kvm.Rack = "rack-1"
			req := &ufsAPI.UpdateKVMRequest{
				KVM: kvm,
			}
			resp, err := tf.Fleet.UpdateKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidMac))
		})

		t.Run("Update KVM - Invalid input empty name", func(t *ftt.Test) {
			KVM3 := mockKVM("KVM-3")
			KVM3.Name = ""
			KVM3.Rack = "rack-1"
			req := &ufsAPI.UpdateKVMRequest{
				KVM: KVM3,
			}
			resp, err := tf.Fleet.UpdateKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update KVM - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.UpdateKVMRequest{
				KVM: mockKVM("a.b)7&"),
			}
			resp, err := tf.Fleet.UpdateKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetKVM(t *testing.T) {
	t.Parallel()
	ftt.Run("GetKVM", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		t.Run("Get KVM by existing ID", func(t *ftt.Test) {
			KVM1 := &ufspb.KVM{
				Name: "kvm-1",
			}
			_, err := registration.CreateKVM(tf.C, KVM1)
			assert.Loosely(t, err, should.BeNil)
			KVM1.Name = util.AddPrefix(util.KVMCollection, "kvm-1")

			req := &ufsAPI.GetKVMRequest{
				Name: util.AddPrefix(util.KVMCollection, "KVM-1"),
			}
			resp, err := tf.Fleet.GetKVM(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(KVM1))
		})
		t.Run("Get KVM by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetKVMRequest{
				Name: util.AddPrefix(util.KVMCollection, "KVM-2"),
			}
			resp, err := tf.Fleet.GetKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get KVM - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetKVMRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Get KVM - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetKVMRequest{
				Name: util.AddPrefix(util.KVMCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListKVMs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	KVMs := make([]*ufspb.KVM, 0, 4)
	for i := 0; i < 4; i++ {
		kvm := &ufspb.KVM{
			Name: fmt.Sprintf("kvm-%d", i),
		}
		resp, _ := registration.CreateKVM(tf.C, kvm)
		kvm.Name = util.AddPrefix(util.KVMCollection, kvm.Name)
		KVMs = append(KVMs, resp)
	}
	ftt.Run("ListKVMs", t, func(t *ftt.Test) {
		t.Run("ListKVMs - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListKVMsRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListKVMs(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListKVMs - Full listing - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListKVMsRequest{}
			resp, err := tf.Fleet.ListKVMs(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.KVMs, should.Resemble(KVMs))
		})

		t.Run("ListKVMs - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListKVMsRequest{
				Filter: "platform=chromeplatform-1 | rpm=rpm-2",
			}
			_, err := tf.Fleet.ListKVMs(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})

		t.Run("ListKVMs - filter format valid", func(t *ftt.Test) {
			req := &ufsAPI.ListKVMsRequest{
				Filter: "platform=chromeplatform-1",
			}
			resp, err := tf.Fleet.ListKVMs(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.KVMs, should.BeNil)
		})
	})
}

func TestDeleteKVM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("DeleteKVM", t, func(t *ftt.Test) {
		t.Run("Delete KVM by existing ID without references", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-2",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			KVM2 := &ufspb.KVM{
				Name: "kvm-2",
				Rack: "rack-2",
			}
			_, err = registration.CreateKVM(tf.C, KVM2)
			assert.Loosely(t, err, should.BeNil)

			dreq := &ufsAPI.DeleteKVMRequest{
				Name: util.AddPrefix(util.KVMCollection, "KVM-2"),
			}
			_, err = tf.Fleet.DeleteKVM(tf.C, dreq)
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.GetKVM(tf.C, "KVM-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Delete KVM - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteKVMRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete KVM - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteKVMRequest{
				Name: util.AddPrefix(util.KVMCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteKVM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestCreateRPM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	RPM1 := mockRPM("")
	RPM1.Rack = "rack-1"
	RPM2 := mockRPM("")
	rack1 := &ufspb.Rack{
		Name: "rack-1",
		Rack: &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
		},
	}
	registration.CreateRack(tf.C, rack1)
	ftt.Run("CreateRPM", t, func(t *ftt.Test) {
		t.Run("Create new RPM with RPM_id", func(t *ftt.Test) {
			req := &ufsAPI.CreateRPMRequest{
				RPM:   RPM1,
				RPMId: "RPM-1",
			}
			resp, err := tf.Fleet.CreateRPM(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(RPM1))
		})

		t.Run("Create new RPM - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.CreateRPMRequest{
				RPM: nil,
			}
			resp, err := tf.Fleet.CreateRPM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new RPM - Invalid input empty ID", func(t *ftt.Test) {
			req := &ufsAPI.CreateRPMRequest{
				RPM:   RPM2,
				RPMId: "",
			}
			resp, err := tf.Fleet.CreateRPM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new RPM - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.CreateRPMRequest{
				RPM:   RPM2,
				RPMId: "a.b)7&",
			}
			resp, err := tf.Fleet.CreateRPM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestUpdateRPM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	RPM2 := mockRPM("RPM-1")
	RPM3 := mockRPM("RPM-3")
	RPM4 := mockRPM("a.b)7&")
	rack1 := &ufspb.Rack{
		Name: "rack-1",
		Rack: &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
		},
	}
	registration.CreateRack(tf.C, rack1)
	ftt.Run("UpdateRPM", t, func(t *ftt.Test) {
		t.Run("Update existing RPM", func(t *ftt.Test) {
			_, err := registration.CreateRPM(tf.C, &ufspb.RPM{Name: "rpm-1", Rack: "rack-1"})
			assert.Loosely(t, err, should.BeNil)

			RPM2.Rack = "rack-1"
			ureq := &ufsAPI.UpdateRPMRequest{
				RPM: RPM2,
			}
			resp, err := tf.Fleet.UpdateRPM(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(RPM2))
		})

		t.Run("Update RPM - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateRPMRequest{
				RPM: nil,
			}
			resp, err := tf.Fleet.UpdateRPM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update RPM - Invalid input empty name", func(t *ftt.Test) {
			RPM3.Name = ""
			req := &ufsAPI.UpdateRPMRequest{
				RPM: RPM3,
			}
			resp, err := tf.Fleet.UpdateRPM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update RPM - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.UpdateRPMRequest{
				RPM: RPM4,
			}
			resp, err := tf.Fleet.UpdateRPM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetRPM(t *testing.T) {
	t.Parallel()
	ftt.Run("GetRPM", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		t.Run("Get RPM by existing ID", func(t *ftt.Test) {
			RPM1 := &ufspb.RPM{
				Name: "rpm-1",
			}
			_, err := registration.CreateRPM(tf.C, RPM1)
			assert.Loosely(t, err, should.BeNil)
			RPM1.Name = util.AddPrefix(util.RPMCollection, "rpm-1")

			req := &ufsAPI.GetRPMRequest{
				Name: util.AddPrefix(util.RPMCollection, "RPM-1"),
			}
			resp, err := tf.Fleet.GetRPM(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(RPM1))
		})
		t.Run("Get RPM - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetRPMRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetRPM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Get RPM - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetRPMRequest{
				Name: util.AddPrefix(util.RPMCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetRPM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListRPMs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	RPMs := make([]*ufspb.RPM, 0, 4)
	for i := 0; i < 4; i++ {
		rpm := &ufspb.RPM{
			Name: fmt.Sprintf("rpm-%d", i),
		}
		resp, _ := registration.CreateRPM(tf.C, rpm)
		resp.Name = util.AddPrefix(util.RPMCollection, resp.Name)
		RPMs = append(RPMs, resp)
	}
	ftt.Run("ListRPMs", t, func(t *ftt.Test) {
		t.Run("ListRPMs - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListRPMsRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListRPMs(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListRPMs - Full listing - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListRPMsRequest{
				PageSize: 2000,
			}
			resp, err := tf.Fleet.ListRPMs(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.RPMs, should.Resemble(RPMs))
		})

		t.Run("ListRPMs - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListRPMsRequest{
				Filter: "platform=chromeplatform-1 | rpm=rpm-2",
			}
			_, err := tf.Fleet.ListRPMs(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})
	})
}

func TestDeleteRPM(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("DeleteRPM", t, func(t *ftt.Test) {
		t.Run("Delete RPM by existing ID", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-2",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateRPM(tf.C, &ufspb.RPM{
				Name: "rpm-2", Rack: "rack-2"})
			assert.Loosely(t, err, should.BeNil)

			dreq := &ufsAPI.DeleteRPMRequest{
				Name: util.AddPrefix(util.RPMCollection, "RPM-2"),
			}
			_, err = tf.Fleet.DeleteRPM(tf.C, dreq)
			assert.Loosely(t, err, should.BeNil)

			greq := &ufsAPI.GetRPMRequest{
				Name: util.AddPrefix(util.RPMCollection, "RPM-2"),
			}
			res, err := tf.Fleet.GetRPM(tf.C, greq)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Delete RPM by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRPMRequest{
				Name: util.AddPrefix(util.RPMCollection, "RPM-2"),
			}
			_, err := tf.Fleet.DeleteRPM(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Delete RPM - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRPMRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteRPM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete RPM - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRPMRequest{
				Name: util.AddPrefix(util.RPMCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteRPM(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestCreateDrac(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	machine1 := &ufspb.Machine{
		Name: "machine-1",
		Device: &ufspb.Machine_ChromeBrowserMachine{
			ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
		},
	}
	registration.CreateMachine(tf.C, machine1)
	registration.CreateSwitch(tf.C, &ufspb.Switch{
		Name:         "test-switch",
		CapacityPort: 100,
	})
	ftt.Run("CreateDrac", t, func(t *ftt.Test) {
		t.Run("Create new drac - Happy path", func(t *ftt.Test) {
			setupTestVlan(ctx)
			drac := mockDrac("")
			drac.Machine = "machine-1"
			req := &ufsAPI.CreateDracRequest{
				Drac:   drac,
				DracId: "drac-1",
				NetworkOption: &ufsAPI.NetworkOption{
					Ip: "192.168.40.10",
				},
			}
			resp, err := tf.Fleet.CreateDrac(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(drac))

			// Verify network settings
			dhcp, err := configuration.GetDHCPConfig(ctx, "drac-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dhcp.GetIp(), should.Equal("192.168.40.10"))
			assert.Loosely(t, dhcp.GetVlan(), should.Equal("vlan-1"))
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "192.168.40.10"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeTrue)
		})

		t.Run("Create new drac - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.CreateDracRequest{
				Drac: nil,
			}
			resp, err := tf.Fleet.CreateDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new mac - Invalid mac", func(t *ftt.Test) {
			drac := mockDrac("createDrac-0")
			drac.MacAddress = "123"
			req := &ufsAPI.CreateDracRequest{
				Drac:   drac,
				DracId: "createDrac-0",
			}
			resp, err := tf.Fleet.CreateDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidMac))
		})

		t.Run("Create new drac - Invalid input empty ID", func(t *ftt.Test) {
			drac := mockDrac("")
			drac.Machine = "machine-1"
			req := &ufsAPI.CreateDracRequest{
				Drac:   drac,
				DracId: "",
			}
			resp, err := tf.Fleet.CreateDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new drac - Invalid input invalid characters", func(t *ftt.Test) {
			drac := mockDrac("")
			drac.Machine = "machine-1"
			req := &ufsAPI.CreateDracRequest{
				Drac:   drac,
				DracId: "a.b)7&",
			}
			resp, err := tf.Fleet.CreateDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})

		t.Run("Create new drac - Invalid input empty machine", func(t *ftt.Test) {
			drac := mockDrac("")
			req := &ufsAPI.CreateDracRequest{
				Drac:   drac,
				DracId: "drac-5",
			}
			resp, err := tf.Fleet.CreateDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyMachineName))
		})
	})
}

func TestUpdateDrac(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	machine1 := &ufspb.Machine{
		Name: "machine-1",
		Device: &ufspb.Machine_ChromeBrowserMachine{
			ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
		},
	}
	registration.CreateMachine(tf.C, machine1)
	ftt.Run("UpdateDrac", t, func(t *ftt.Test) {
		t.Run("Update existing drac", func(t *ftt.Test) {
			drac1 := &ufspb.Drac{
				Name:    "drac-1",
				Machine: "machine-1",
			}
			resp, err := registration.CreateDrac(tf.C, drac1)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			drac2 := mockDrac("drac-1")
			drac2.SwitchInterface = nil
			drac2.Machine = "machine-1"
			ureq := &ufsAPI.UpdateDracRequest{
				Drac: drac2,
			}
			resp, err = tf.Fleet.UpdateDrac(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(drac2))
		})

		t.Run("Update drac - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateDracRequest{
				Drac: nil,
			}
			resp, err := tf.Fleet.UpdateDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update drac - Invalid mac", func(t *ftt.Test) {
			drac := mockDrac("updateDrac-0")
			drac.MacAddress = "123"
			drac.Machine = "machine-1"
			req := &ufsAPI.UpdateDracRequest{
				Drac: drac,
			}
			resp, err := tf.Fleet.UpdateDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidMac))
		})

		t.Run("Update drac - Invalid input empty name", func(t *ftt.Test) {
			drac := mockDrac("")
			drac.Name = ""
			drac.Machine = "machine-1"
			req := &ufsAPI.UpdateDracRequest{
				Drac: drac,
			}
			resp, err := tf.Fleet.UpdateDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update drac - Invalid input invalid characters", func(t *ftt.Test) {
			drac := mockDrac("a.b)7&")
			drac.Machine = "machine-1"
			req := &ufsAPI.UpdateDracRequest{
				Drac: drac,
			}
			resp, err := tf.Fleet.UpdateDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetDrac(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()

	ftt.Run("GetDrac", t, func(t *ftt.Test) {
		t.Run("Get drac by existing ID", func(t *ftt.Test) {
			drac1 := &ufspb.Drac{
				Name: "drac-1",
			}
			registration.CreateDrac(tf.C, drac1)
			drac1.Name = util.AddPrefix(util.DracCollection, "drac-1")

			req := &ufsAPI.GetDracRequest{
				Name: util.AddPrefix(util.DracCollection, "drac-1"),
			}
			resp, err := tf.Fleet.GetDrac(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(drac1))
		})

		t.Run("Get drac - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetDracRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Get drac - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetDracRequest{
				Name: util.AddPrefix(util.DracCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListDracs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	dracs := make([]*ufspb.Drac, 0, 4)
	for i := 0; i < 4; i++ {
		drac := &ufspb.Drac{
			Name: fmt.Sprintf("drac-%d", i),
		}
		resp, _ := registration.CreateDrac(tf.C, drac)
		drac.Name = util.AddPrefix(util.DracCollection, drac.Name)
		dracs = append(dracs, resp)
	}
	ftt.Run("ListDracs", t, func(t *ftt.Test) {
		t.Run("ListDracs - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListDracsRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListDracs(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListDracs - Full listing - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListDracsRequest{}
			resp, err := tf.Fleet.ListDracs(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Dracs, should.Resemble(dracs))
		})

		t.Run("ListDracs - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListDracsRequest{
				Filter: "drac=mac-1 | rpm=rpm-2",
			}
			_, err := tf.Fleet.ListDracs(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})

		t.Run("ListDracs - filter format valid", func(t *ftt.Test) {
			req := &ufsAPI.ListDracsRequest{
				Filter: "switch=switch-1",
			}
			resp, err := tf.Fleet.ListDracs(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Dracs, should.BeNil)
		})
	})
}

func TestDeleteDrac(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("DeleteDrac", t, func(t *ftt.Test) {
		t.Run("Delete drac - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteDracRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete drac - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteDracRequest{
				Name: util.AddPrefix(util.DracCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteDrac(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestCreateSwitch(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	rack1 := &ufspb.Rack{
		Name: "rack-1",
		Rack: &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
		},
	}
	registration.CreateRack(tf.C, rack1)
	ftt.Run("CreateSwitch", t, func(t *ftt.Test) {
		t.Run("Create new switch with switch_id", func(t *ftt.Test) {
			switch1 := mockSwitch("")
			switch1.Rack = "rack-1"
			req := &ufsAPI.CreateSwitchRequest{
				Switch:   switch1,
				SwitchId: "Switch-1",
			}
			resp, err := tf.Fleet.CreateSwitch(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(switch1))
		})

		t.Run("Create new switch - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.CreateSwitchRequest{
				Switch: nil,
			}
			resp, err := tf.Fleet.CreateSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new switch - Invalid input empty ID", func(t *ftt.Test) {
			switch1 := mockSwitch("")
			switch1.Rack = "rack-1"
			req := &ufsAPI.CreateSwitchRequest{
				Switch:   switch1,
				SwitchId: "",
			}
			resp, err := tf.Fleet.CreateSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new switch - Invalid input invalid characters", func(t *ftt.Test) {
			switch1 := mockSwitch("")
			switch1.Rack = "rack-1"
			req := &ufsAPI.CreateSwitchRequest{
				Switch:   switch1,
				SwitchId: "a.b)7&",
			}
			resp, err := tf.Fleet.CreateSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})

		t.Run("Create new switch - Invalid input empty rack", func(t *ftt.Test) {
			req := &ufsAPI.CreateSwitchRequest{
				Switch:   mockSwitch("x"),
				SwitchId: "switch-5",
			}
			resp, err := tf.Fleet.CreateSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyRackName))
		})
	})
}

func TestUpdateSwitch(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	rack1 := &ufspb.Rack{
		Name: "rack-1",
		Rack: &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
		},
	}
	registration.CreateRack(tf.C, rack1)
	ftt.Run("UpdateSwitch", t, func(t *ftt.Test) {
		t.Run("Update existing switch", func(t *ftt.Test) {
			switch1 := &ufspb.Switch{
				Name: "switch-1",
				Rack: "rack-1",
			}
			resp, err := registration.CreateSwitch(tf.C, switch1)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			switch2 := mockSwitch("switch-1")
			switch2.Rack = "rack-1"
			ureq := &ufsAPI.UpdateSwitchRequest{
				Switch: switch2,
			}
			resp, err = tf.Fleet.UpdateSwitch(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(switch2))
		})

		t.Run("Update switch - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateSwitchRequest{
				Switch: nil,
			}
			resp, err := tf.Fleet.UpdateSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update switch - Invalid input empty name", func(t *ftt.Test) {
			switch1 := mockSwitch("")
			switch1.Name = ""
			switch1.Rack = "rack-1"
			req := &ufsAPI.UpdateSwitchRequest{
				Switch: switch1,
			}
			resp, err := tf.Fleet.UpdateSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update switch - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.UpdateSwitchRequest{
				Switch: mockSwitch("a.b)7&"),
			}
			resp, err := tf.Fleet.UpdateSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetSwitch(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("GetSwitch", t, func(t *ftt.Test) {
		t.Run("Get switch by existing ID", func(t *ftt.Test) {
			switch1 := &ufspb.Switch{
				Name: "switch-1",
			}
			_, err := registration.CreateSwitch(tf.C, switch1)
			assert.Loosely(t, err, should.BeNil)
			switch1.Name = util.AddPrefix(util.SwitchCollection, "switch-1")

			req := &ufsAPI.GetSwitchRequest{
				Name: util.AddPrefix(util.SwitchCollection, "switch-1"),
			}
			resp, err := tf.Fleet.GetSwitch(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(switch1))
		})
		t.Run("Get switch by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetSwitchRequest{
				Name: util.AddPrefix(util.SwitchCollection, "switch-2"),
			}
			resp, err := tf.Fleet.GetSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get switch - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetSwitchRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Get switch - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetSwitchRequest{
				Name: util.AddPrefix(util.SwitchCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListSwitches(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	switches := make([]*ufspb.Switch, 0, 4)
	for i := 0; i < 4; i++ {
		s := &ufspb.Switch{
			Name: fmt.Sprintf("switch-%d", i),
		}
		resp, _ := registration.CreateSwitch(tf.C, s)
		s.Name = util.AddPrefix(util.SwitchCollection, s.Name)
		switches = append(switches, resp)
	}
	ftt.Run("ListSwitches", t, func(t *ftt.Test) {
		t.Run("ListSwitches - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListSwitchesRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListSwitches(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListSwitches - Full listing - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListSwitchesRequest{}
			resp, err := tf.Fleet.ListSwitches(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Switches, should.Resemble(switches))
		})

		t.Run("ListSwitches - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListSwitchesRequest{
				Filter: "platform=chromeplatform-1 | rpm=rpm-2",
			}
			_, err := tf.Fleet.ListSwitches(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})
	})
}

func TestDeleteSwitch(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("DeleteSwitch", t, func(t *ftt.Test) {
		t.Run("Delete switch by existing ID without references", func(t *ftt.Test) {
			rack := &ufspb.Rack{
				Name: "rack-2",
				Rack: &ufspb.Rack_ChromeBrowserRack{
					ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
				},
			}
			_, err := registration.CreateRack(ctx, rack)
			assert.Loosely(t, err, should.BeNil)

			switch2 := &ufspb.Switch{
				Name: "switch-2",
				Rack: "rack-2",
			}
			_, err = registration.CreateSwitch(tf.C, switch2)
			assert.Loosely(t, err, should.BeNil)

			dreq := &ufsAPI.DeleteSwitchRequest{
				Name: util.AddPrefix(util.SwitchCollection, "switch-2"),
			}
			_, err = tf.Fleet.DeleteSwitch(tf.C, dreq)
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.GetSwitch(tf.C, "switch-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Delete switch - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteSwitchRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete switch - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteSwitchRequest{
				Name: util.AddPrefix(util.SwitchCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteSwitch(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestRenameSwitch(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("RenameSwitch", t, func(t *ftt.Test) {
		t.Run("Rename an empty switch name", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameSwitch(tf.C, &ufsAPI.RenameSwitchRequest{
				Name: "",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Rename a switch to an empty name", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameSwitch(tf.C, &ufsAPI.RenameSwitchRequest{
				Name:    "oldSwitch",
				NewName: "",
			})
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("The new switch name after renaming doesn't follow switch name format", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameSwitch(tf.C, &ufsAPI.RenameSwitchRequest{
				Name:    "switches/oldSwitch",
				NewName: "newSwitch",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.SwitchNameFormat))
		})
	})
}

func TestCreateAsset(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("Create Asset", t, func(t *ftt.Test) {
		t.Run("Create valid asset", func(t *ftt.Test) {
			asset := &ufspb.Asset{
				Name: "assets/C001001",
				Type: ufspb.AssetType_DUT,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
					Rack: "chromeos6-row2-rack3",
				},
			}
			req := &ufsAPI.CreateAssetRequest{
				Asset: asset,
			}
			rack := &ufspb.Rack{
				Name: "chromeos6-row2-rack3",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
			}
			rackReq := &ufsAPI.RackRegistrationRequest{
				Rack: rack,
			}
			rackResp, err := tf.Fleet.RackRegistration(tf.C, rackReq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rackResp, should.NotBeNil)
			assert.Loosely(t, rackResp, should.Resemble(rack))
			resp, err := tf.Fleet.CreateAsset(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Resemble(asset))
		})
		t.Run("Create asset - invalid name", func(t *ftt.Test) {
			asset := &ufspb.Asset{
				Name: "C001001",
				Type: ufspb.AssetType_DUT,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
					Rack: "chromeos6-row2-rack3",
				},
			}
			req := &ufsAPI.CreateAssetRequest{
				Asset: asset,
			}
			_, err := tf.Fleet.CreateAsset(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Create asset - empty name", func(t *ftt.Test) {
			asset := &ufspb.Asset{
				Name: " ",
				Type: ufspb.AssetType_DUT,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
					Rack: "chromeos6-row2-rack3",
				},
			}
			req := &ufsAPI.CreateAssetRequest{
				Asset: asset,
			}
			_, err := tf.Fleet.CreateAsset(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Create asset - missing location", func(t *ftt.Test) {
			asset := &ufspb.Asset{
				Name: "assets/C001001",
				Type: ufspb.AssetType_DUT,
			}
			req := &ufsAPI.CreateAssetRequest{
				Asset: asset,
			}
			_, err := tf.Fleet.CreateAsset(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Create asset - missing rack", func(t *ftt.Test) {
			asset := &ufspb.Asset{
				Name: "assets/C001001",
				Type: ufspb.AssetType_DUT,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
			}
			req := &ufsAPI.CreateAssetRequest{
				Asset: asset,
			}
			_, err := tf.Fleet.CreateAsset(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Create asset - zone unspecified", func(t *ftt.Test) {
			asset := &ufspb.Asset{
				Name: "assets/C001001",
				Type: ufspb.AssetType_DUT,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_UNSPECIFIED,
					Rack: "chromeos6-row2-rack3",
				},
			}
			req := &ufsAPI.CreateAssetRequest{
				Asset: asset,
			}
			_, err := tf.Fleet.CreateAsset(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}

func TestRenameAsset(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("RenameAsset", t, func(t *ftt.Test) {
		t.Run("Rename an empty asset name", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameAsset(tf.C, &ufsAPI.RenameAssetRequest{
				Name: "",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Rename an asset to an empty name", func(t *ftt.Test) {
			_, err := tf.Fleet.RenameAsset(tf.C, &ufsAPI.RenameAssetRequest{
				Name:    "oldAsset",
				NewName: "",
			})
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}
