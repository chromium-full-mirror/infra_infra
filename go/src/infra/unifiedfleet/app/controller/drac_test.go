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
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func mockDrac(id string) *ufspb.Drac {
	return &ufspb.Drac{
		Name: id,
	}
}

func TestCreateDrac(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	machine1 := &ufspb.Machine{
		Name: "machine-1",
		Device: &ufspb.Machine_ChromeBrowserMachine{
			ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
		},
	}
	registration.CreateMachine(ctx, machine1)
	ftt.Run("CreateDrac", t, func(t *ftt.Test) {
		t.Run("Create new drac with non existing machine", func(t *ftt.Test) {
			drac1 := &ufspb.Drac{
				Name:    "drac-1",
				Machine: "machine-5",
			}
			resp, err := CreateDrac(ctx, drac1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create drac - duplicated switch ports", func(t *ftt.Test) {
			drac := &ufspb.Drac{
				Name: "drac-create-1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "drac-create-switch-1",
					PortName: "25",
				},
			}
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)
			switch1 := &ufspb.Switch{
				Name: "drac-create-switch-1",
			}
			_, err = registration.CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)

			drac2 := &ufspb.Drac{
				Name:    "drac-create-2",
				Machine: "machine-1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "drac-create-switch-1",
					PortName: "25",
				},
			}
			_, err = CreateDrac(ctx, drac2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("switch port 25 of drac-create-switch-1 is already occupied"))
		})

		t.Run("Create new drac with existing machine with drac", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-10",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateDrac(ctx, &ufspb.Drac{
				Name:    "drac-5",
				Machine: "machine-10",
			})
			assert.Loosely(t, err, should.BeNil)

			drac := &ufspb.Drac{
				Name:    "drac-20",
				Machine: "machine-10",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.BrowserLabAdminRealm)
			_, err = CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is already a drac drac-5 associated with machine machine-10"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-20")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create new drac with existing machine without drac", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-15",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			drac := &ufspb.Drac{
				Name:    "drac-25",
				Machine: "machine-15",
			}
			resp, err := CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(drac))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-25")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("drac"))
		})

		t.Run("Create new drac with non existing switch", func(t *ftt.Test) {
			drac1 := &ufspb.Drac{
				Name:    "drac-1",
				Machine: "machine-1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch: "switch-1",
				},
			}
			resp, err := CreateDrac(ctx, drac1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no Switch with SwitchID switch-1"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Create new drac with existing switch", func(t *ftt.Test) {
			switch2 := &ufspb.Switch{
				Name: "switch-2",
			}
			_, err := registration.CreateSwitch(ctx, switch2)
			assert.Loosely(t, err, should.BeNil)

			drac2 := &ufspb.Drac{
				Name:    "drac-2",
				Machine: "machine-1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch: "switch-2",
				},
			}
			resp, err := CreateDrac(ctx, drac2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(drac2))

			s, err := state.GetStateRecord(ctx, "dracs/drac-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_SERVING))
		})

		t.Run("Create new drac - Permission denied: same realm and no create permission", func(t *ftt.Test) {
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

			drac2 := &ufspb.Drac{
				Name:    "drac-16",
				Machine: "machine-16",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = CreateDrac(ctx, drac2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Create new drac - Permission denied: different realm", func(t *ftt.Test) {
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

			drac2 := &ufspb.Drac{
				Name:    "drac-17",
				Machine: "machine-17",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsCreate, util.AtlLabAdminRealm)
			_, err = CreateDrac(ctx, drac2)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestUpdateDrac(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateDrac", t, func(t *ftt.Test) {
		t.Run("Update drac with non-existing drac", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-1",
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:    "drac-1",
				Machine: "machine-1",
			}
			resp, err := UpdateDrac(ctx, drac, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update drac with non existing switch", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-2",
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:    "drac-2",
				Machine: "machine-2",
			}
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			drac2 := &ufspb.Drac{
				Name:    "drac-2",
				Machine: "machine-2",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch: "switch-1",
				},
			}
			resp, err := UpdateDrac(ctx, drac2, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no Switch with SwitchID switch-1"))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update drac with new machine(already associated with drac) - failure", func(t *ftt.Test) {
			machine3 := &ufspb.Machine{
				Name: "machine-3",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err := registration.CreateMachine(ctx, machine3)
			assert.Loosely(t, err, should.BeNil)

			machine4 := &ufspb.Machine{
				Name: "machine-4",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			}
			_, err = registration.CreateMachine(ctx, machine4)
			assert.Loosely(t, err, should.BeNil)

			drac := &ufspb.Drac{
				Name:    "drac-3",
				Machine: "machine-3",
			}
			_, err = registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.CreateDrac(ctx, &ufspb.Drac{
				Name:    "drac-4",
				Machine: "machine-4",
			})
			assert.Loosely(t, err, should.BeNil)

			drac.Machine = "machine-4"
			_, err = UpdateDrac(ctx, drac, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is already a drac drac-4 associated with machine machine-4"))

			// Verify the changes - update fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update drac with same machine", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-5",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			drac := &ufspb.Drac{
				Name:    "drac-5",
				Machine: "machine-5",
			}
			_, err = registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			drac = &ufspb.Drac{
				Name:       "drac-5",
				MacAddress: "ab:cd:ef",
				Machine:    "machine-5",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateDrac(ctx, drac, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(drac))

			// Verify the changes
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.BeEmpty)
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("ab:cd:ef"))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("drac.mac_address"))
		})

		t.Run("Update drac with non existing machine", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-6.1",
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:    "drac-6",
				Machine: "machine-6.1",
			}
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			drac = &ufspb.Drac{
				Name:    "drac-6",
				Machine: "machine-6",
			}
			resp, err := UpdateDrac(ctx, drac, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no Machine with MachineID machine-6 in the system."))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Partial Update drac", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-7.1",
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:    "drac-7",
				Machine: "machine-7.1",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-7",
					PortName: "25",
				},
			}
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			drac1 := &ufspb.Drac{
				Name:       "drac-7",
				MacAddress: "drac-7-macaddress",
				SwitchInterface: &ufspb.SwitchInterface{
					PortName: "75",
				},
			}
			resp, err := UpdateDrac(ctx, drac1, &field_mask.FieldMask{Paths: []string{"portName", "macAddress"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetSwitchInterface().GetSwitch(), should.Match("switch-7"))
			assert.Loosely(t, resp.GetMacAddress(), should.Match("drac-7-macaddress"))
			assert.Loosely(t, resp.GetSwitchInterface().GetPortName(), should.Equal("75"))
		})

		t.Run("Partial Update drac mac address and new machine(same realm and not associated to any drac) - succeed", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-8.1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:       "drac-8",
				Machine:    "machine-8.1",
				MacAddress: "drac-8-address",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-8",
					PortName: "25",
				},
			}
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			machine1 = &ufspb.Machine{
				Name: "machine-8.1-1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			registration.CreateMachine(ctx, machine1)
			drac1 := &ufspb.Drac{
				Name:       "drac-8",
				Machine:    "machine-8.1-1",
				MacAddress: "drac-8-address",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			drac, err = UpdateDrac(ctx, drac1, &field_mask.FieldMask{Paths: []string{"macAddress", "machine"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, drac.GetMacAddress(), should.Equal("drac-8-address"))
			assert.Loosely(t, drac.GetMachine(), should.Equal("machine-8.1-1"))
		})

		t.Run("Partial Update drac mac address - duplicated mac address", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-8.1.1",
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:       "drac-8.1",
				Machine:    "machine-8.1.1",
				MacAddress: "drac-8.1-address",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-8.1",
					PortName: "25",
				},
			}
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)
			drac2 := &ufspb.Drac{
				Name:       "drac-8.2",
				MacAddress: "drac-8.2-address",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-8.1",
					PortName: "26",
				},
			}
			_, err = registration.CreateDrac(ctx, drac2)
			assert.Loosely(t, err, should.BeNil)

			drac1 := &ufspb.Drac{
				Name:       "drac-8.1",
				MacAddress: "drac-8.2-address",
			}
			_, err = UpdateDrac(ctx, drac1, &field_mask.FieldMask{Paths: []string{"macAddress"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("mac_address drac-8.2-address is already occupied"))
		})

		t.Run("Partial Update drac mac address - no update at all", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-9",
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:       "drac-9",
				Machine:    "machine-9",
				MacAddress: "drac-9-address",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-9",
					PortName: "25",
				},
			}
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			drac1 := &ufspb.Drac{
				Name:       "drac-9",
				MacAddress: "drac-9-address",
			}
			_, err = UpdateDrac(ctx, drac1, &field_mask.FieldMask{Paths: []string{"macAddress"}})
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Fully Update drac mac address - duplicated mac address", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-10",
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:       "drac-full",
				Machine:    "machine-10",
				MacAddress: "drac-full-address",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-drac-full",
					PortName: "25",
				},
			}
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)
			machine2 := &ufspb.Machine{
				Name: "machine-11",
			}
			registration.CreateMachine(ctx, machine2)
			drac2 := &ufspb.Drac{
				Name:       "drac-full-2",
				Machine:    "machine-11",
				MacAddress: "drac-full-address-2",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-drac-full",
					PortName: "26",
				},
			}
			_, err = registration.CreateDrac(ctx, drac2)
			assert.Loosely(t, err, should.BeNil)

			drac1 := &ufspb.Drac{
				Name:       "drac-full",
				MacAddress: "drac-full-address-2",
			}
			_, err = UpdateDrac(ctx, drac1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("mac_address drac-full-address-2 is already occupied"))
		})

		t.Run("Update drac -  Permission denied: same realm and no update permission", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-12",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			drac := &ufspb.Drac{
				Name:    "drac-12",
				Machine: "machine-12",
			}
			_, err = registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			drac = &ufspb.Drac{
				Name:       "drac-12",
				MacAddress: "ab:cd:ef:gh",
				Machine:    "machine-12",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.BrowserLabAdminRealm)
			_, err = UpdateDrac(ctx, drac, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update drac -  Permission denied: different realm", func(t *ftt.Test) {
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

			drac := &ufspb.Drac{
				Name:    "drac-13",
				Machine: "machine-13",
			}
			_, err = registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			drac = &ufspb.Drac{
				Name:       "drac-13",
				MacAddress: "ab:cd:ef:gh:ij",
				Machine:    "machine-13",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			_, err = UpdateDrac(ctx, drac, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update drac with new machine(same realm and not associated to any drac) - pass", func(t *ftt.Test) {
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
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err = registration.CreateMachine(ctx, machine4)
			assert.Loosely(t, err, should.BeNil)

			drac := &ufspb.Drac{
				Name:    "drac-14",
				Machine: "machine-14",
			}
			_, err = registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			drac.Machine = "machine-15"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			resp, err := UpdateDrac(ctx, drac, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(drac))

			// Verify the changes
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-14")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("machine-14"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("machine-15"))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("drac.machine"))
		})

		t.Run("Update drac with new machine(different realm without permission and not associated to any drac) - fail", func(t *ftt.Test) {
			machine3 := &ufspb.Machine{
				Name: "machine-16",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			_, err := registration.CreateMachine(ctx, machine3)
			assert.Loosely(t, err, should.BeNil)

			machine4 := &ufspb.Machine{
				Name: "machine-17",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			_, err = registration.CreateMachine(ctx, machine4)
			assert.Loosely(t, err, should.BeNil)

			drac := &ufspb.Drac{
				Name:    "drac-16",
				Machine: "machine-16",
			}
			_, err = registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			drac.Machine = "machine-17"
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateDrac(ctx, drac, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))

			// Verify the changes - update fails
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-16")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Partial Update drac with new machine(different realm without permission and not associated to any drac) - fail", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-18",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:    "drac-18",
				Machine: "machine-18",
			}
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			machine1 = &ufspb.Machine{
				Name: "machine-19",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			registration.CreateMachine(ctx, machine1)
			drac1 := &ufspb.Drac{
				Name:    "drac-18",
				Machine: "machine-19",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsUpdate, util.BrowserLabAdminRealm)
			_, err = UpdateDrac(ctx, drac1, &field_mask.FieldMask{Paths: []string{"machine"}})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Update drac with new machine(different realm with permission and not associated to any drac) - pass", func(t *ftt.Test) {
			machine3 := &ufspb.Machine{
				Name: "machine-20",
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
				Name: "machine-21",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			_, err = registration.CreateMachine(ctx, machine4)
			assert.Loosely(t, err, should.BeNil)

			drac := &ufspb.Drac{
				Name:    "drac-20",
				Machine: "machine-20",
			}
			_, err = registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			drac.Machine = "machine-21"
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.RegistrationsUpdate),
				),
			})
			resp, err := UpdateDrac(ctx, drac, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetMachine(), should.Equal("machine-21"))
		})

		t.Run("Partial Update drac with new machine(different realm with permission and not associated to any drac) - pass", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-22",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_ATL97,
				},
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:    "drac-22",
				Machine: "machine-22",
			}
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			machine1 = &ufspb.Machine{
				Name: "machine-23",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			registration.CreateMachine(ctx, machine1)

			drac1 := &ufspb.Drac{
				Name:    "drac-22",
				Machine: "machine-23",
			}
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate),
					authtest.MockPermission("user:user@example.com", util.BrowserLabAdminRealm, util.RegistrationsUpdate),
				),
			})
			resp, err := UpdateDrac(ctx, drac1, &field_mask.FieldMask{Paths: []string{"machine"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetMachine(), should.Equal("machine-23"))
		})

		t.Run("Update drac with new resource state", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-24",
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:    "drac-24",
				Machine: "machine-24",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-24",
					PortName: "25",
				},
			}
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			drac1 := &ufspb.Drac{
				Name:          "drac-24",
				ResourceState: ufspb.State_STATE_SERVING,
			}
			resp, err := UpdateDrac(ctx, drac1, &field_mask.FieldMask{Paths: []string{"resourceState"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetResourceState(), should.Equal(ufspb.State_STATE_SERVING))
		})

		t.Run("Update drac with new display name", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-26",
			}
			registration.CreateMachine(ctx, machine1)
			drac := &ufspb.Drac{
				Name:        "drac-26",
				DisplayName: "64d1c223-afe6-45a0-b2d9-96c6f8884181",
				Machine:     "machine-26",
				SwitchInterface: &ufspb.SwitchInterface{
					Switch:   "switch-26",
					PortName: "27",
				},
			}
			d, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, d, should.NotBeNil)
			assert.Loosely(t, d.GetDisplayName(), should.Equal("64d1c223-afe6-45a0-b2d9-96c6f8884181"))

			drac1 := &ufspb.Drac{
				Name:        "drac-26",
				DisplayName: "e08c1cf7-020c-4ca1-874b-4d41e65f85d5",
			}
			resp, err := UpdateDrac(ctx, drac1, &field_mask.FieldMask{Paths: []string{"displayName"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetDisplayName(), should.Equal("e08c1cf7-020c-4ca1-874b-4d41e65f85d5"))
		})
	})
}

func TestDeleteDrac(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteDrac", t, func(t *ftt.Test) {
		t.Run("Delete drac error by non-existing ID", func(t *ftt.Test) {
			err := DeleteDrac(ctx, "drac-10")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Delete drac successfully by existing ID without references", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-2",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			registration.CreateMachine(ctx, machine1)

			drac := mockDrac("drac-2")
			drac.Machine = "machine-2"
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)
			_, err = state.BatchUpdateStates(ctx, []*ufspb.StateRecord{
				{
					ResourceName: "dracs/drac-2",
					State:        ufspb.State_STATE_SERVING,
				},
			})
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.AtlLabAdminRealm)
			err = DeleteDrac(ctx, "drac-2")
			assert.Loosely(t, err, should.BeNil)

			resp, err := registration.GetDrac(ctx, "drac-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			_, err = state.GetStateRecord(ctx, "dracs/drac-2")
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("drac"))
		})

		t.Run("Delete drac successfully together with deleting ip", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-3",
			}
			registration.CreateMachine(ctx, machine1)
			drac := mockDrac("drac-ip")
			drac.Machine = "machine-3"
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)
			_, err = configuration.BatchUpdateDHCPs(ctx, []*ufspb.DHCPConfig{
				{
					Hostname: "drac-ip",
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

			err = DeleteDrac(ctx, "drac-ip")
			assert.Loosely(t, err, should.BeNil)
			ip, err := configuration.QueryIPByPropertyName(ctx, map[string]string{"ipv4_str": "1.2.3.4"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ip, should.HaveLength(1))
			assert.Loosely(t, ip[0].GetOccupied(), should.BeFalse)
			_, err = configuration.GetDHCPConfig(ctx, "drac-ip")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			resp, err := registration.GetDrac(ctx, "drac-ip")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "dracs/drac-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("drac"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dracs/drac-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "dhcps/drac-ip")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
		})

		t.Run("Delete drac - permission denied: same realm and no delete permission", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-4",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			registration.CreateMachine(ctx, machine1)

			drac := mockDrac("drac-4")
			drac.Machine = "machine-4"
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsGet, util.AtlLabAdminRealm)
			err = DeleteDrac(ctx, "drac-4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})

		t.Run("Delete drac - permission denied: different realm", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name: "machine-5",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS1,
				},
			}
			registration.CreateMachine(ctx, machine1)

			drac := mockDrac("drac-5")
			drac.Machine = "machine-5"
			_, err := registration.CreateDrac(ctx, drac)
			assert.Loosely(t, err, should.BeNil)

			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.RegistrationsDelete, util.BrowserLabAdminRealm)
			err = DeleteDrac(ctx, "drac-5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestListDracs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	dracsWithSwitch := make([]*ufspb.Drac, 0, 2)
	dracs := make([]*ufspb.Drac, 0, 4)
	for i := 0; i < 4; i++ {
		drac := mockDrac(fmt.Sprintf("drac-%d", i))
		if i%2 == 0 {
			drac.SwitchInterface = &ufspb.SwitchInterface{Switch: "switch-12"}
		}
		resp, _ := registration.CreateDrac(ctx, drac)
		if i%2 == 0 {
			dracsWithSwitch = append(dracsWithSwitch, resp)
		}
		dracs = append(dracs, resp)
	}
	ftt.Run("ListDracs", t, func(t *ftt.Test) {
		t.Run("List Dracs - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListDracs(ctx, 5, "", "invalid=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name invalid"))
		})

		t.Run("List Dracs - filter switch - happy path", func(t *ftt.Test) {
			resp, _, _ := ListDracs(ctx, 5, "", "switch=switch-12", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(dracsWithSwitch))
		})

		t.Run("ListDracs - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListDracs(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(dracs))
		})
	})
}

func TestBatchGetDracs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetDracs", t, func(t *ftt.Test) {
		t.Run("Batch get dracs - happy path", func(t *ftt.Test) {
			entities := make([]*ufspb.Drac, 4)
			for i := 0; i < 4; i++ {
				entities[i] = &ufspb.Drac{
					Name: fmt.Sprintf("drac-batchGet-%d", i),
				}
			}
			_, err := registration.BatchUpdateDracs(ctx, entities)
			assert.Loosely(t, err, should.BeNil)
			resp, err := registration.BatchGetDracs(ctx, []string{"drac-batchGet-0", "drac-batchGet-1", "drac-batchGet-2", "drac-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(entities))
		})
		t.Run("Batch get dracs  - missing id", func(t *ftt.Test) {
			resp, err := registration.BatchGetDracs(ctx, []string{"drac-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("drac-batchGet-non-existing"))
		})
		t.Run("Batch get dracs  - empty input", func(t *ftt.Test) {
			resp, err := registration.BatchGetDracs(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = registration.BatchGetDracs(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}
