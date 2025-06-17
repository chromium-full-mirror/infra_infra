// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"google.golang.org/genproto/protobuf/field_mask"

	crosLabAPI "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/external"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/history"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func TestMachineRegistration(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("TestMachineRegistration", t, func(t *ftt.Test) {
		t.Run("Register machine with already existing machine, nic and drac", func(t *ftt.Test) {
			nic := &ufspb.Nic{
				Name:    "nic-1",
				Machine: "machine-1",
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			drac := &ufspb.Drac{
				Name:    "drac-1",
				Machine: "machine-1",
			}
			_, err = registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			machine := &ufspb.Machine{
				Name: "machine-1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						NicObjects: []*ufspb.Nic{{
							Name:    "nic-1",
							Machine: "machine-1",
						}},
						DracObject: &ufspb.Drac{
							Name:    "drac-1",
							Machine: "machine-1",
						},
					},
				},
			}
			_, err = registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			_, err = MachineRegistration(ctx, machine)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(
				"Machine machine-1 already exists in the system.\n"+
					"Nic nic-1 already exists in the system.\n"+
					"Drac drac-1 already exists in the system.\n"))

			// No changes are recorded as the registration fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "nics/nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Register browser machine with duplicated serial number", func(t *ftt.Test) {
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.BrowserLabAdminRealm)
			existingMachine := &ufspb.Machine{
				Name:         "machine-with-duplicated-serial",
				SerialNumber: "duplicated_serial",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err := MachineRegistration(ctx, existingMachine)
			assert.Loosely(t, err, should.BeNil)

			machine := &ufspb.Machine{
				Name:         "machine-3",
				SerialNumber: "duplicated_serial",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err = MachineRegistration(ctx, machine)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("contains the same serial number"))

			// No changes are recorded as the registration fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "nics/nic-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Register machine with invalid machine(referencing non existing resources)", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-3",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						ChromePlatform: "chromePlatform-3",
						KvmInterface: &ufspb.KVMInterface{
							Kvm: "kvm-3",
						},
						RpmInterface: &ufspb.RPMInterface{
							Rpm: "rpm-3",
						},
					},
				},
			}
			_, err := MachineRegistration(ctx, machine)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot create machine machine-3:\n"+
				"There is no KVM with KVMID kvm-3 in the system.\n"+
				"There is no RPM with RPMID rpm-3 in the system.\n"+
				"There is no ChromePlatform with ChromePlatformID chromePlatform-3 in the system."))

			// No changes are recorded as the registration fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Register browser machine with invalid nic(referencing non existing resources)", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-3",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						NicObjects: []*ufspb.Nic{{
							Name: "nic-2",
							SwitchInterface: &ufspb.SwitchInterface{
								Switch: "switch-1",
							},
						}},
					},
				},
			}
			_, err := MachineRegistration(ctx, machine)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot create machine machine-3:\n"+
				"There is no Switch with SwitchID switch-1 in the system."))

			// No changes are recorded as the registration fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "nics/nic-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Register browser machine with invalid drac(referencing non existing resources)", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-3",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						DracObject: &ufspb.Drac{
							Name: "drac-2",
							SwitchInterface: &ufspb.SwitchInterface{
								Switch: "switch-1",
							},
						},
					},
				},
			}
			_, err := MachineRegistration(ctx, machine)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot create machine machine-3:\n"+
				"There is no Switch with SwitchID switch-1 in the system."))

			// No changes are recorded as the registration fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Register browser machine - duplicated kvm interface", func(t *ftt.Test) {
			_, err := registration.CreateKVM(ctx, &ufspb.KVM{
				Name: "kvm-browser-duplicate",
			})
			assert.Loosely(t, err, should.BeNil)
			machine := &ufspb.Machine{
				Name: "machine-browser-duplicate1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						KvmInterface: &ufspb.KVMInterface{
							Kvm:      "kvm-browser-duplicate",
							PortName: "A1",
						},
					},
				},
			}
			m, err := MachineRegistration(ctx, machine)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m, should.Match(machine))

			machine2 := &ufspb.Machine{
				Name: "machine-browser-duplicate2",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						KvmInterface: &ufspb.KVMInterface{
							Kvm:      "kvm-browser-duplicate",
							PortName: "A1",
						},
					},
				},
			}
			_, err = MachineRegistration(ctx, machine2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("kvm port A1 of kvm-browser-duplicate is already occupied"))
		})

		t.Run("Register browser machine happy path", func(t *ftt.Test) {
			_, err := registration.CreateKVM(ctx, &ufspb.KVM{
				Name: "kvm-browser-3",
			})
			assert.Loosely(t, err, should.BeNil)
			nics := []*ufspb.Nic{{
				Name: "nic-browser-3",
			}}
			drac := &ufspb.Drac{
				Name: "drac-browser-3",
			}
			machine := &ufspb.Machine{
				Name:         "machine-browser-3",
				SerialNumber: "machine-browser-3-serial-number",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						NicObjects: nics,
						DracObject: drac,
						KvmInterface: &ufspb.KVMInterface{
							Kvm:      "kvm-browser-3",
							PortName: "A1",
						},
					},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.BrowserLabAdminRealm)
			m, err := MachineRegistration(ctx, machine)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m, should.Match(machine))
			s, err := state.GetStateRecord(ctx, "machines/machine-browser-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
			dr, err := inventory.GetMachineLSEDeployment(ctx, m.GetSerialNumber())
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dr.GetHostname(), should.Equal(util.GetHostnameWithNoHostPrefix(m.GetSerialNumber())))
			assert.Loosely(t, dr.GetDeploymentIdentifier(), should.BeEmpty)
			assert.Loosely(t, dr.GetConfigsToPush(), should.BeNil)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-browser-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "nics/nic-browser-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("nic"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-browser-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("drac"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machineLSEDeployments/machine-browser-3-serial-number")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse_deployment"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machineLSEDeployments/machine-browser-3-serial-number")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Register OS machine happy path", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-os-3",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
			}
			asset := &ufspb.Asset{
				Name: "machine-os-3",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
					Rack: "chromeos6-test",
				},
			}
			r := mockRack("chromeos6-test", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := registration.CreateRack(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			_, err = AssetRegistration(ctx, asset)
			assert.Loosely(t, err, should.BeNil)
			m, err := MachineRegistration(ctx, machine)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m, should.NotBeNil)
			assert.Loosely(t, m, should.Match(machine))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-os-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
		})

		t.Run("Register machine - permission denied: same realm and no create permission", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-os-4",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
				Realm: util.AtlLabAdminRealm,
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.AtlLabAdminRealm)
			_, err := MachineRegistration(ctx, machine)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Register machine - permission denied: different realm", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-os-5",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
				Realm: util.AtlLabAdminRealm,
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.BrowserLabAdminRealm)
			_, err := MachineRegistration(ctx, machine)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestUpdateMachine(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateMachines", t, func(t *ftt.Test) {
		t.Run("Update a non-existing machine", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-10",
			}
			_, err := UpdateMachine(ctx, machine, nil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Update new machine with non existing resource", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-1",
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			machine = &ufspb.Machine{
				Name: "machine-1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						ChromePlatform: "chromePlatform-1",
					},
				},
			}
			_, err = UpdateMachine(ctx, machine, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot update machine machine-1"))
		})

		t.Run("Update machine with existing resources, but duplicated serial", func(t *ftt.Test) {
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			machine1 := &ufspb.Machine{
				Name:         "machine-update-with-duplicated-serial-1",
				SerialNumber: "update-duplicated-serial-1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)

			machine2 := &ufspb.Machine{
				Name:         "machine-update-with-duplicated-serial-2",
				SerialNumber: "update-duplicated-serial-2",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			_, err = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)

			machineToUpdate := &ufspb.Machine{
				Name:         "machine-update-with-duplicated-serial-1",
				SerialNumber: "update-duplicated-serial-2",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			_, err = UpdateMachine(ctx, machineToUpdate, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("contains the same serial number"))
		})

		t.Run("Update machine with existing resources", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-2",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS3,
				},
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			resp, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			registration.CreateNic(ctx, &ufspb.Nic{
				Name:    "nic-update-zone",
				Zone:    ufspb.Zone_ZONE_CHROMEOS3.String(),
				Machine: "machine-2",
			})
			registration.CreateDrac(ctx, &ufspb.Drac{
				Name:    "drac-update-zone",
				Zone:    ufspb.Zone_ZONE_CHROMEOS3.String(),
				Machine: "machine-2",
			})
			inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name: "lse-update-zone",
				Zone: ufspb.Zone_ZONE_CHROMEOS3.String(),
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{
						VmCapacity: 12,
					},
				},
				Machines: []string{"machine-2"},
			})
			inventory.BatchUpdateVMs(ctx, []*ufspb.VM{
				{
					Name:         "vm-update-zone",
					Zone:         ufspb.Zone_ZONE_CHROMEOS3.String(),
					MachineLseId: "lse-update-zone",
				},
			})

			chromePlatform2 := &ufspb.ChromePlatform{
				Name: "chromePlatform-2",
			}
			_, err = configuration.CreateChromePlatform(ctx, chromePlatform2)
			assert.Loosely(t, err, should.BeNil)

			machine = &ufspb.Machine{
				Name: "machine-2",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS2,
				},
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						ChromePlatform: "chromePlatform-2",
					},
				},
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AcsLabAdminRealm)
			resp, err = UpdateMachine(ctx, machine, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machine))
			lse, err := inventory.GetMachineLSE(ctx, "lse-update-zone")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse.GetZone(), should.Equal(ufspb.Zone_ZONE_CHROMEOS2.String()))
			nic, err := registration.GetNic(ctx, "nic-update-zone")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nic.GetZone(), should.Equal(ufspb.Zone_ZONE_CHROMEOS2.String()))
			drac, err := registration.GetDrac(ctx, "drac-update-zone")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, drac.GetZone(), should.Equal(ufspb.Zone_ZONE_CHROMEOS2.String()))
			vm, err := inventory.GetVM(ctx, "vm-update-zone")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, vm.GetZone(), should.Equal(ufspb.Zone_ZONE_CHROMEOS2.String()))
		})

		t.Run("Update machine kvm", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-update-kvm",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS3,
				},
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateKVM(ctx, &ufspb.KVM{
				Name: "kvm-update",
			})
			assert.Loosely(t, err, should.BeNil)

			machine = &ufspb.Machine{
				Name: "machine-update-kvm",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS2,
				},
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						KvmInterface: &ufspb.KVMInterface{
							Kvm:      "kvm-update",
							PortName: "A1",
						},
					},
				},
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AcsLabAdminRealm)
			resp, err := UpdateMachine(ctx, machine, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machine))
		})

		t.Run("Update machine serial number", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name:         "machine-full-update-serial",
				SerialNumber: "old-serial-full-update",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)
			_, err = inventory.UpdateMachineLSEDeployments(ctx, []*ufspb.MachineLSEDeployment{util.FormatDeploymentRecord("", machine.GetSerialNumber())})
			assert.Loosely(t, err, should.BeNil)

			machine1 := &ufspb.Machine{
				Name:         "machine-full-update-serial",
				SerialNumber: "serial-full-update",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			resp, err := UpdateMachine(ctx, machine1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetSerialNumber(), should.Match("serial-full-update"))
			dr, err := inventory.GetMachineLSEDeployment(ctx, "serial-full-update")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dr.GetHostname(), should.Equal(util.GetHostnameWithNoHostPrefix("serial-full-update")))
			_, err = inventory.GetMachineLSEDeployment(ctx, "old-serial-full-update")
			assert.Loosely(t, err, should.NotBeNil)

			// Verify the change events of deployment records
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machineLSEDeployments/serial-full-update")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse_deployment"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machineLSEDeployments/old-serial-full-update")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse_deployment"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machineLSEDeployments/serial-full-update")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machineLSEDeployments/old-serial-full-update")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
		})

		t.Run("Partial Update machine", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-3",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						ChromePlatform: "chromePlatform-3",
						KvmInterface: &ufspb.KVMInterface{
							Kvm:      "kvm-3",
							PortName: "A1",
						},
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			chromePlatform := &ufspb.ChromePlatform{
				Name: "chromePlatform-4",
			}
			_, err = configuration.CreateChromePlatform(ctx, chromePlatform)
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateKVM(ctx, &ufspb.KVM{
				Name: "kvm-4",
			})
			assert.Loosely(t, err, should.BeNil)

			machine1 := &ufspb.Machine{
				Name: "machine-3",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						ChromePlatform: "chromePlatform-4",
						KvmInterface: &ufspb.KVMInterface{
							Kvm:      "kvm-4",
							PortName: "A2",
						},
					},
				},
			}
			resp, err := UpdateMachine(ctx, machine1, &field_mask.FieldMask{Paths: []string{"platform", "kvm"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetChromeBrowserMachine().GetChromePlatform(), should.Match("chromePlatform-4"))
			assert.Loosely(t, resp.GetChromeBrowserMachine().GetKvmInterface().GetKvm(), should.Match("kvm-4"))
			assert.Loosely(t, resp.GetChromeBrowserMachine().GetKvmInterface().GetPortName(), should.Match("A1"))
		})

		t.Run("Partial Update machine - update serial number", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-update-serial",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)
			machine1 := &ufspb.Machine{
				Name:         "machine-update-serial",
				SerialNumber: "serial-update",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			resp, err := UpdateMachine(ctx, machine1, &field_mask.FieldMask{Paths: []string{"serialNumber"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetSerialNumber(), should.Match("serial-update"))
			dr, err := inventory.GetMachineLSEDeployment(ctx, "serial-update")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dr.GetHostname(), should.Equal(util.GetHostnameWithNoHostPrefix("serial-update")))

			// Verify the change events of deployment records
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machineLSEDeployments/serial-update")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse_deployment"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machineLSEDeployments/serial-update")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Partial Update machine - update serial number, machine has a corresponding lse", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-update-serial-with-lse",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)
			machineLSE1 := &ufspb.MachineLSE{
				Name:     "machinelse-update-serial-with-lse",
				Hostname: "machinelse-update-serial-with-lse",
				Machines: []string{"machine-update-serial-with-lse"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)

			// Partial update
			machine1 := &ufspb.Machine{
				Name:         "machine-update-serial-with-lse",
				SerialNumber: "serial-update-with-lse",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			resp, err := UpdateMachine(ctx, machine1, &field_mask.FieldMask{Paths: []string{"serialNumber"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetSerialNumber(), should.Match("serial-update-with-lse"))
			dr, err := inventory.GetMachineLSEDeployment(ctx, "serial-update-with-lse")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, dr.GetHostname(), should.Equal("machinelse-update-serial-with-lse"))

			// Verify the change events of deployment records
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machineLSEDeployments/serial-update-with-lse")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse_deployment"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machineLSEDeployments/serial-update-with-lse")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
		t.Run("Restore stricted fields - serial number, sku, hwid", func(t *ftt.Test) {
			name := "machine-update-serial-hwid-skuwith-lse"
			machine := &ufspb.Machine{
				Name:         name,
				SerialNumber: "serial-update-with-lse1",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						Sku:  "Sku1",
						Hwid: "hwid1",
					},
				},
			}
			respMachine, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respMachine, should.NotBeNil)
			assert.Loosely(t, respMachine.GetSerialNumber(), should.Match("serial-update-with-lse1"))
			assert.Loosely(t, respMachine.GetChromeosMachine().GetSku(), should.Match("Sku1"))
			assert.Loosely(t, respMachine.GetChromeosMachine().GetHwid(), should.Match("hwid1"))
			machineLSE1 := &ufspb.MachineLSE{
				Name:     name,
				Hostname: name,
				Machines: []string{name},
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{},
				},
			}
			_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)

			// No update
			machine1 := &ufspb.Machine{
				Name: name,
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
			}
			resp, err := UpdateMachine(ctx, machine1, &field_mask.FieldMask{Paths: []string{}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetSerialNumber(), should.Match("serial-update-with-lse1"))
			assert.Loosely(t, resp.GetChromeosMachine().GetSku(), should.Match("Sku1"))
			assert.Loosely(t, resp.GetChromeosMachine().GetHwid(), should.Match("hwid1"))

			// Update fields
			machine1 = &ufspb.Machine{
				Name:         name,
				SerialNumber: "serial-update-with-lse2",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						Sku:  "Sku2",
						Hwid: "hwid2",
					},
				},
			}
			resp, err = UpdateMachine(ctx, machine1, &field_mask.FieldMask{Paths: []string{}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetSerialNumber(), should.Match("serial-update-with-lse2"))
			assert.Loosely(t, resp.GetChromeosMachine().GetHwid(), should.Match("hwid2"))
			assert.Loosely(t, resp.GetChromeosMachine().GetSku(), should.Match("Sku2"))

			// Do not update if use paths.
			machine1 = &ufspb.Machine{
				Name:         name,
				SerialNumber: "serial-update-with-lse3",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						Sku:  "Sku3",
						Hwid: "hwid3",
					},
				},
			}
			resp, err = UpdateMachine(ctx, machine1, &field_mask.FieldMask{Paths: []string{"resourceState"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetSerialNumber(), should.Match("serial-update-with-lse2"))
			assert.Loosely(t, resp.GetChromeosMachine().GetHwid(), should.Match("hwid2"))
			assert.Loosely(t, resp.GetChromeosMachine().GetSku(), should.Match("Sku2"))
		})

		t.Run("Partial Update machine - duplicated kvm", func(t *ftt.Test) {
			_, err := registration.CreateKVM(ctx, &ufspb.KVM{
				Name: "kvm-update-duplicate1",
			})
			assert.Loosely(t, err, should.BeNil)
			machine := &ufspb.Machine{
				Name: "machine-update-duplicate1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						KvmInterface: &ufspb.KVMInterface{
							Kvm:      "kvm-update-duplicate1",
							PortName: "A1",
						},
					},
				},
			}
			_, err = registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)
			machine2 := &ufspb.Machine{
				Name: "machine-update-duplicate2",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						KvmInterface: &ufspb.KVMInterface{
							Kvm:      "kvm-update-duplicate2",
							PortName: "A1",
						},
					},
				},
			}
			_, err = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)

			machine1 := &ufspb.Machine{
				Name: "machine-update-duplicate2",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						KvmInterface: &ufspb.KVMInterface{
							Kvm:      "kvm-update-duplicate1",
							PortName: "A1",
						},
					},
				},
			}
			_, err = UpdateMachine(ctx, machine1, &field_mask.FieldMask{Paths: []string{"kvm"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("kvm port A1 of kvm-update-duplicate1 is already occupied"))
		})

		t.Run("Update machine - permission denied: same realm and no update permission", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-os-4",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.AtlLabAdminRealm)
			_, err = UpdateMachine(ctx, machine, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update machine - permission denied: different realm", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-os-5",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateMachine(ctx, machine, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update machine(realm name) - different realm with permission success", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-os-6",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
				Realm: util.AtlLabAdminRealm,
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			machine.Realm = util.BrowserLabAdminRealm
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.RegistrationsUpdate),
				),
			})
			resp, err := UpdateMachine(ctx, machine, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machine))
		})

		t.Run("Update machine(realm name) - permission denied: different realm without permission", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-os-7",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
				Realm: util.AtlLabAdminRealm,
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			machine.Realm = util.BrowserLabAdminRealm
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			_, err = UpdateMachine(ctx, machine, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Partial Update attached device machine", func(t *ftt.Test) {
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

			machine1 := &ufspb.Machine{
				Name: "adm-1",
				Device: &ufspb.Machine_AttachedDevice{
					AttachedDevice: &ufspb.AttachedDevice{
						DeviceType:   ufspb.AttachedDeviceType_ATTACHED_DEVICE_TYPE_ANDROID_PHONE,
						Manufacturer: "test-man-1",
						BuildTarget:  "test-target-1",
						Model:        "test-model-1",
					},
				},
			}
			resp, err := UpdateMachine(ctx, machine1, &field_mask.FieldMask{Paths: []string{
				"admManufacturer",
				"admDeviceType",
				"admBuildTarget",
				"admModel",
			}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetAttachedDevice().GetDeviceType(), should.Equal(ufspb.AttachedDeviceType_ATTACHED_DEVICE_TYPE_ANDROID_PHONE))
			assert.Loosely(t, resp.GetAttachedDevice().GetManufacturer(), should.Equal("test-man-1"))
			assert.Loosely(t, resp.GetAttachedDevice().GetBuildTarget(), should.Equal("test-target-1"))
			assert.Loosely(t, resp.GetAttachedDevice().GetModel(), should.Equal("test-model-1"))
		})
	})
}

func TestUpdateMachineDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateMachines with DeviceLabels", t, func(t *ftt.Test) {
		t.Run("Update machine with device labels happy path", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-update-with-devicelabels-1",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS5,
					Rack: "chromeos5-test",
				},
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			lse1 := &ufspb.MachineLSE{
				Name:     "machine-update-with-devicelabels-lse-1",
				Machines: []string{"machine-update-with-devicelabels-1"},
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{},
				},
			}
			_, err = inventory.CreateMachineLSE(ctx, lse1)
			assert.Loosely(t, err, should.BeNil)
			su1 := &ufspb.SchedulingUnit{
				Name:        "machine-update-with-devicelabels-su-1",
				MachineLSEs: []string{"machine-update-with-devicelabels-lse-1"},
			}
			_, err = inventory.CreateSchedulingUnit(ctx, su1)
			assert.Loosely(t, err, should.BeNil)

			machine2 := &ufspb.Machine{
				Name: "machine-update-with-devicelabels-1",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS7,
					Rack: "chromeos7-test",
				},
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AcsLabAdminRealm)
			_, err = UpdateMachine(ctx, machine2, &field_mask.FieldMask{Paths: []string{"zone", "rack"}})
			assert.Loosely(t, err, should.BeNil)

			// DUT labels
			resp, err := inventory.GetDeviceLabels(ctx, util.AddPrefix(util.MachineLSECollection, "machine-update-with-devicelabels-lse-1"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetName(), should.Equal("machineLSEs/machine-update-with-devicelabels-lse-1"))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/machine-update-with-devicelabels-lse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			// Scheduling Unit labels
			resp, err = inventory.GetDeviceLabels(ctx, util.AddPrefix(util.SchedulingUnitCollection, "machine-update-with-devicelabels-su-1"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetName(), should.Equal("schedulingunits/machine-update-with-devicelabels-su-1"))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufspb.ResourceType_RESOURCE_TYPE_SCHEDULING_UNIT))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/schedulingunits/machine-update-with-devicelabels-su-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
		})
	})
}

func TestUpdateDutMeta(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateDutLab for an OS machine", t, func(t *ftt.Test) {
		t.Run("Update a non-OS machine", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-dutmeta-1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			})
			assert.Loosely(t, err, should.BeNil)

			err = UpdateDutMeta(ctx, &ufspb.DutMeta{
				ChromeosDeviceId: "machine-dutmeta-1",
				Hostname:         "machinelse-labmeta-1",
				SerialNumber:     "fake-serial",
			})
			// Update is skipped without error
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Update a OS machine - happy path", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-dutmeta-2",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
			}
			req, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, req.GetChromeosMachine().GetHwid(), should.BeEmpty)
			assert.Loosely(t, req.GetChromeosMachine().GetSku(), should.BeEmpty)

			err = UpdateDutMeta(ctx, &ufspb.DutMeta{
				ChromeosDeviceId: "machine-dutmeta-2",
				Hostname:         "machinelse-dutmeta-2",
				SerialNumber:     "fake-serial",
				HwID:             "fake-hwid",
				DeviceSku:        "fake-devicesku",
			})
			assert.Loosely(t, err, should.BeNil)
			req, err = registration.GetMachine(ctx, "machine-dutmeta-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetSerialNumber(), should.Equal("fake-serial"))
			assert.Loosely(t, req.GetChromeosMachine().GetHwid(), should.Equal("fake-hwid"))
			assert.Loosely(t, req.GetChromeosMachine().GetSku(), should.Equal("fake-devicesku"))
		})
	})
}

func TestUpdateRecoveryDutData(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	const serialNumber = "serialNumber"
	const hwID = "hwID"
	const deviceSku = "deviceSku"
	const dlmSkuId = "12345"
	storageType := crosLabAPI.StorageType_HDD
	dutData := &ufsAPI.ChromeOsRecoveryData_DutData{
		SerialNumber: serialNumber,
		HwID:         hwID,
		DeviceSku:    deviceSku,
		DlmSkuId:     dlmSkuId,
		StorageType:  storageType,
	}
	ftt.Run("UpdateRecoveryDutData for an OS machine", t, func(t *ftt.Test) {
		t.Run("Update a non-OS machine", func(t *ftt.Test) {
			const machineName = "machine-dutdata-1"
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: machineName,
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			asset := &ufspb.Asset{
				Name: machineName,
				Info: &ufspb.AssetInfo{
					AssetTag: machineName,
				},
				Type:     ufspb.AssetType_DUT,
				Location: &ufspb.Location{},
			}
			asset, err = registration.CreateAsset(ctx, asset)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, asset.GetInfo().GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, asset.GetInfo().GetHwid(), should.BeEmpty)
			assert.Loosely(t, asset.GetInfo().GetSku(), should.BeEmpty)

			err = updateRecoveryDutData(ctx, machineName, dutData)
			// Update is skipped without error
			assert.Loosely(t, err, should.BeNil)
			req, err := registration.GetMachine(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, req.GetChromeosMachine().GetHwid(), should.BeEmpty)
			assert.Loosely(t, req.GetChromeosMachine().GetSku(), should.BeEmpty)
			assert.Loosely(t, req.GetChromeosMachine().GetDlmSkuId(), should.BeEmpty)
			assert.Loosely(t, req.GetChromeosMachine().GetStorageType(), should.Equal(crosLabAPI.StorageType_UNSPECIFIED))

			asset, err = registration.GetAsset(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, asset.GetInfo().GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, asset.GetInfo().GetHwid(), should.BeEmpty)
			assert.Loosely(t, asset.GetInfo().GetSku(), should.BeEmpty)
		})
		t.Run("Update a OS machine - successful path", func(t *ftt.Test) {
			const machineName = "machine-dutdata-2"
			const assetTag = "machine-testassetdata-2"
			machine := &ufspb.Machine{
				Name: machineName,
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
			}
			req, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, req.GetChromeosMachine().GetHwid(), should.BeEmpty)
			assert.Loosely(t, req.GetChromeosMachine().GetSku(), should.BeEmpty)
			assert.Loosely(t, req.GetChromeosMachine().GetDlmSkuId(), should.BeEmpty)
			asset := &ufspb.Asset{
				Name: machineName,
				Info: &ufspb.AssetInfo{
					AssetTag: assetTag,
				},
				Type:     ufspb.AssetType_DUT,
				Location: &ufspb.Location{},
			}
			asset, err = registration.CreateAsset(ctx, asset)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, asset.GetInfo().GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, asset.GetInfo().GetHwid(), should.BeEmpty)
			assert.Loosely(t, asset.GetInfo().GetSku(), should.BeEmpty)

			err = updateRecoveryDutData(ctx, machineName, dutData)
			assert.Loosely(t, err, should.BeNil)
			req, err = registration.GetMachine(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetSerialNumber(), should.Equal(serialNumber))
			assert.Loosely(t, req.GetChromeosMachine().GetHwid(), should.Equal(hwID))
			assert.Loosely(t, req.GetChromeosMachine().GetSku(), should.Equal(deviceSku))
			assert.Loosely(t, req.GetChromeosMachine().GetDlmSkuId(), should.Equal(dlmSkuId))
			assert.Loosely(t, req.GetChromeosMachine().GetStorageType(), should.Equal(storageType))

			asset, err = registration.GetAsset(ctx, machineName)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, asset.GetInfo().GetSerialNumber(), should.Equal(serialNumber))
			assert.Loosely(t, asset.GetInfo().GetHwid(), should.Equal(hwID))
			assert.Loosely(t, asset.GetInfo().GetSku(), should.Equal(deviceSku))
		})
	})
}

func TestDeleteMachine(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteMachine", t, func(t *ftt.Test) {
		t.Run("Delete machine by existing ID with machineLSE reference", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name:  "machine-3",
				Realm: util.AtlLabAdminRealm,
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)

			machineLSE1 := &ufspb.MachineLSE{
				Name:     "machinelse-1",
				Machines: []string{"machine-3"},
			}
			_, err = inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.AtlLabAdminRealm)
			err = DeleteMachine(ctx, "machine-3")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("is occupied"))

			resp, _ := registration.GetMachine(ctx, "machine-3")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(machine1))

			// No changes are recorded as the deletion fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Delete machine by existing ID without references", func(t *ftt.Test) {
			machine2 := &ufspb.Machine{
				Name: "machine-4",
			}
			_, err := registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)

			err = DeleteMachine(ctx, "machine-4")
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.GetMachine(ctx, "machine-4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Delete machine with nics and drac - happy path", func(t *ftt.Test) {
			nic := &ufspb.Nic{
				Name:    "nic-5",
				Machine: "machine-5",
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)

			drac := &ufspb.Drac{
				Name:    "drac-5",
				Machine: "machine-5",
			}
			_, err = registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			_, err = configuration.BatchUpdateDHCPs(ctx, []*ufspb.DHCPConfig{
				{
					Hostname: "drac-5",
					Ip:       "1.2.3.5",
				},
			})
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateIPs(ctx, []*ufspb.IP{
				{
					Id:       "ip3",
					Occupied: true,
					Ipv4Str:  "1.2.3.5",
					Vlan:     "fake_vlan",
					Ipv4:     uint32(100),
				},
			})
			assert.Loosely(t, err, should.BeNil)

			machine := &ufspb.Machine{
				Name: "machine-5",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err = registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			err = DeleteMachine(ctx, "machine-5")
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.GetMachine(ctx, "machine-4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = registration.GetNic(ctx, "nic-5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = registration.GetDrac(ctx, "drac-5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			_, err = configuration.GetDHCPConfig(ctx, "drac-5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			resIPs, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "1.2.3.5"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resIPs, should.HaveLength(1))
			assert.Loosely(t, resIPs[0].Occupied, should.BeFalse)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetName(), should.Equal("machines/machine-4"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machines/machine-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
		})

		t.Run("Delete machine - Permission denied: same realm with no delete permission", func(t *ftt.Test) {
			machine2 := &ufspb.Machine{
				Name: "machine-6",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			_, err := registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)

			// same realm different permission
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.AtlLabAdminRealm)
			err = DeleteMachine(ctx, "machine-6")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Delete machine - Permission denied: different realm", func(t *ftt.Test) {
			machine2 := &ufspb.Machine{
				Name: "machine-7",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			_, err := registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)

			// different realm
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.BrowserLabAdminRealm)
			err = DeleteMachine(ctx, "machine-7")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestReplaceMachine(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("ReplaceMachines", t, func(t *ftt.Test) {
		t.Run("Repalce an old Machine with new machine with MachineLSE reference", func(t *ftt.Test) {
			oldMachine1 := &ufspb.Machine{
				Name: "machine-4",
			}
			_, cerr := registration.CreateMachine(ctx, oldMachine1)
			assert.Loosely(t, cerr, should.BeNil)

			machineLSE1 := &ufspb.MachineLSE{
				Name:     "machinelse-1",
				Machines: []string{"machine-0", "machine-50", "machine-4", "machine-7"},
			}
			mresp, merr := inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, merr, should.BeNil)
			assert.Loosely(t, mresp, should.Match(machineLSE1))

			newMachine2 := &ufspb.Machine{
				Name: "machine-100",
			}
			rresp, rerr := ReplaceMachine(ctx, oldMachine1, newMachine2)
			assert.Loosely(t, rerr, should.BeNil)
			assert.Loosely(t, rresp, should.Match(newMachine2))

			mresp, merr = inventory.GetMachineLSE(ctx, "machinelse-1")
			assert.Loosely(t, merr, should.BeNil)
			assert.Loosely(t, mresp.GetMachines(), should.Match([]string{"machine-0", "machine-50", "machine-100", "machine-7"}))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/machine-100")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machines/machine-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machines/machine-100")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})

		t.Run("Repalce an old Machine with already existing machine", func(t *ftt.Test) {
			existingMachine1 := &ufspb.Machine{
				Name: "machine-105",
			}
			_, cerr := registration.CreateMachine(ctx, existingMachine1)
			assert.Loosely(t, cerr, should.BeNil)

			oldMachine1 := &ufspb.Machine{
				Name: "machine-5",
			}
			_, cerr = registration.CreateMachine(ctx, oldMachine1)
			assert.Loosely(t, cerr, should.BeNil)

			newMachine2 := &ufspb.Machine{
				Name: "machine-105",
			}
			rresp, rerr := ReplaceMachine(ctx, oldMachine1, newMachine2)
			assert.Loosely(t, rerr, should.NotBeNil)
			assert.Loosely(t, rresp, should.BeNil)
			assert.Loosely(t, rerr.Error(), should.ContainSubstring(AlreadyExists))

			// No change are recorded as the replacement fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/machine-105")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})
	})
}

func TestRenameMachine(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("RenameMachine", t, func(t *ftt.Test) {
		t.Run("Rename a Machine with new machine name", func(t *ftt.Test) {
			nic := &ufspb.Nic{
				Name:    "machine-10:nic-10",
				Machine: "machine-10",
			}
			_, err := registration.CreateNic(ctx, nic)
			assert.Loosely(t, err, should.BeNil)
			drac := &ufspb.Drac{
				Name:    "drac-10",
				Machine: "machine-10",
			}
			_, err = registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)
			host := &ufspb.MachineLSE{
				Name:     "machinelse-10",
				Machines: []string{"machine-10"},
				Nic:      "machine-10:nic-10",
			}
			_, err = inventory.CreateMachineLSE(ctx, host)
			assert.Loosely(t, err, should.BeNil)
			machine := &ufspb.Machine{
				Name: "machine-10",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Realm: util.BrowserLabAdminRealm,
			}
			_, err = registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			res, err := RenameMachine(ctx, "machine-10", "machine-202")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.Name, should.Equal("machine-202"))
			assert.Loosely(t, res.GetChromeBrowserMachine().GetDisplayName(), should.Equal("machine-202"))

			_, err = registration.GetMachine(ctx, "machine-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			_, err = registration.GetNic(ctx, "machine-10:nic-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			nic, err = registration.GetNic(ctx, "machine-202:nic-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nic.GetMachine(), should.Equal("machine-202"))
			drac, err = registration.GetDrac(ctx, "drac-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, drac.GetMachine(), should.Equal("machine-202"))
			host, err = inventory.GetMachineLSE(ctx, "machinelse-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, host.GetMachines(), should.Match([]string{"machine-202"}))
			assert.Loosely(t, host.GetNic(), should.Match("machine-202:nic-10"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machines/machine-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("machine-10"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("machine-202"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine.name"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/machine-202")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("machine-10"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("machine-202"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine.name"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "nics/machine-10:nic-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("nic"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("machine-10:nic-10"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("machine-202:nic-10"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("nic.name"))
			assert.Loosely(t, changes[2].GetOldValue(), should.Equal("machine-10"))
			assert.Loosely(t, changes[2].GetNewValue(), should.Equal("machine-202"))
			assert.Loosely(t, changes[2].GetEventLabel(), should.Equal("nic.machine"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "nics/machine-202:nic-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRename))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("nic"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("machine-10:nic-10"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("machine-202:nic-10"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("nic.name"))
			assert.Loosely(t, changes[2].GetOldValue(), should.Equal("machine-10"))
			assert.Loosely(t, changes[2].GetNewValue(), should.Equal("machine-202"))
			assert.Loosely(t, changes[2].GetEventLabel(), should.Equal("nic.machine"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("machine-10"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("machine-202"))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("drac.machine"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/machinelse-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("[machine-10]"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("[machine-202]"))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse.machines"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("machine-10:nic-10"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("machine-202:nic-10"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine_lse.nic"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machines/machine-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machines/machine-202")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "nics/machine-10:nic-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "nics/machine-202:nic-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dracs/drac-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/machinelse-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
		t.Run("Rename a non-existing Machine", func(t *ftt.Test) {
			_, err := RenameMachine(ctx, "machine-11", "machine-211")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Rename a Machine to an already existing machine name", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-12",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			machine = &ufspb.Machine{
				Name: "machine-212",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err = registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			_, err = RenameMachine(ctx, "machine-12", "machine-212")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Machine machine-212 already exists in the system"))
		})
		t.Run("Rename a Machine - permission denied: same realm and no update permission", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-13",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = RenameMachine(ctx, "machine-13", "machine-313")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
		t.Run("Rename a Machine - permission denied: different realm", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-14",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			_, err = RenameMachine(ctx, "machine-13", "machine-313")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestRenameMachineDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ctx = useTestingCfg(ctx)

	ftt.Run("RenameMachine with DeviceLabels", t, func(t *ftt.Test) {
		t.Run("Rename a Machine with device labels happy path", func(t *ftt.Test) {
			// Setup
			machine1 := &ufspb.Machine{
				Name: "machine-rename-with-devicelabels-1",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS5,
					Rack: "chromeos5-test",
				},
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						ReferenceBoard: "test",
						BuildTarget:    "test",
						Model:          "test",
						Hwid:           "test",
						Sku:            "100",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.NoErr(t, err)

			lse1 := mockDutMachineLSE("machine-rename-with-devicelabels-lse-1")
			lse1.Machines = []string{"machine-rename-with-devicelabels-1"}
			host, err := inventory.CreateMachineLSE(ctx, lse1)
			assert.NoErr(t, err)
			assert.That(t, host.GetMachines()[0], should.Equal("machine-rename-with-devicelabels-1"))

			// RenameMachine
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AcsLabAdminRealm)
			res, err := RenameMachine(ctx, "machine-rename-with-devicelabels-1", "machine-rename-with-devicelabels-1-renamed")
			assert.NoErr(t, err)
			assert.That(t, res.Name, should.Equal("machine-rename-with-devicelabels-1-renamed"))

			// Validate label change
			_, err = registration.GetMachine(ctx, "machine-rename-with-devicelabels-1")
			assert.ErrIsLike(t, err, NotFound)
			host, err = inventory.GetMachineLSE(ctx, "machine-rename-with-devicelabels-lse-1")
			assert.NoErr(t, err)
			assert.That(t, host.GetMachines(), should.Match([]string{"machine-rename-with-devicelabels-1-renamed"}))
			resp, err := inventory.GetDeviceLabels(ctx, util.AddPrefix(util.MachineLSECollection, "machine-rename-with-devicelabels-lse-1"))
			assert.NoErr(t, err)
			assert.Loosely(t, resp, should.NotBeNil)
			labels := resp.GetLabels()
			assert.Loosely(t, labels, should.NotBeNil)
			assert.Loosely(t, labels["dut_id"], should.NotBeNil)
			assert.Loosely(t, labels["dut_id"].GetLabelValues(), should.NotBeEmpty)
			assert.That(t, labels["dut_id"].GetLabelValues()[0], should.Equal("machine-rename-with-devicelabels-1-renamed"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/machine-rename-with-devicelabels-lse-1")
			assert.NoErr(t, err)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/machine-rename-with-devicelabels-lse-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
	})
}

func TestListMachines(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	machinesWithChromeplatform := make([]*ufspb.Machine, 0, 2)
	machines := make([]*ufspb.Machine, 0, 4)
	for i := range 4 {
		machine := &ufspb.Machine{
			Name: fmt.Sprintf("machinefilter-%d", i),
			Device: &ufspb.Machine_ChromeBrowserMachine{
				ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
			},
		}
		if i%2 == 0 {
			machine.GetChromeBrowserMachine().ChromePlatform = "cp-12"
		}
		resp, _ := registration.CreateMachine(ctx, machine)
		if i%2 == 0 {
			machinesWithChromeplatform = append(machinesWithChromeplatform, resp)
		}
		machines = append(machines, resp)
	}
	ftt.Run("ListMachines", t, func(t *ftt.Test) {
		t.Run("List Machines - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListMachines(ctx, 5, "", "invalid=mx-1", false, false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("invalid field name invalid"))
		})

		t.Run("List Machines - filter chromeplatform - happy path", func(t *ftt.Test) {
			resp, _, _ := ListMachines(ctx, 5, "", "platform=cp-12", false, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(machinesWithChromeplatform))
		})

		t.Run("ListMachines - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListMachines(ctx, 5, "", "", false, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(machines))
		})
	})
}

func TestBatchGetMachines(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	// manually turn on config
	alwaysUseACLConfig := config.Config{
		ExperimentalAPI: &config.ExperimentalAPI{
			GetMachineACL: 99,
		},
	}
	ctx = config.Use(ctx, &alwaysUseACLConfig)
	ctx = auth.WithState(ctx, &authtest.FakeState{
		Identity: "user:atl@lab.com",
		IdentityPermissions: []authtest.RealmPermission{
			{
				Realm:      util.AcsLabAdminRealm,
				Permission: util.RegistrationsGet,
			},
		},
	})

	ftt.Run("BatchGetMachines", t, func(t *ftt.Test) {
		t.Run("Batch get machine - happy path", func(t *ftt.Test) {
			entities := make([]*ufspb.Machine, 4)
			for i := range 4 {
				entities[i] = &ufspb.Machine{
					Name: fmt.Sprintf("machine-batchGet-%d", i),
					Location: &ufspb.Location{
						Zone: ufspb.Zone_ZONE_CHROMEOS5,
					},
				}
			}
			_, err := registration.BatchUpdateMachines(ctx, entities)
			assert.Loosely(t, err, should.BeNil)
			resp, err := BatchGetMachines(ctx, []string{"machine-batchGet-0", "machine-batchGet-1", "machine-batchGet-2", "machine-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(entities))
		})
		t.Run("Batch get machines  - missing id", func(t *ftt.Test) {
			resp, err := BatchGetMachines(ctx, []string{"machine-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("machine-batchGet-non-existing"))
		})
		t.Run("Batch get machines  - empty input", func(t *ftt.Test) {
			resp, err := BatchGetMachines(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = BatchGetMachines(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}

func TestUpdateIndexInMachine(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	registration.CreateMachine(ctx, &ufspb.Machine{
		Name: "machine-update-index",
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_CHROMEOS6,
			Rack: "index-rack-old",
		},
		Device: &ufspb.Machine_ChromeBrowserMachine{
			ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
		},
	})
	ftt.Run("Testing updateIndexInMachine", t, func(t *ftt.Test) {
		t.Run("updateIndexInMachine - update index rack", func(t *ftt.Test) {
			err := updateIndexInMachine(ctx, "rack", "index-rack-old", "index-rack-new", &HistoryClient{})
			assert.Loosely(t, err, should.BeNil)
			machine, err := registration.GetMachine(ctx, "machine-update-index")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, machine.GetLocation().GetRack(), should.Equal("index-rack-new"))
		})
	})
}
