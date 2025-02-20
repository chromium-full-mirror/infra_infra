// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"testing"

	"google.golang.org/grpc/codes"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	"go.chromium.org/infra/unifiedfleet/app/external"
	"go.chromium.org/infra/unifiedfleet/app/model/history"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func TestCreateLabstation(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ftt.Run("CreateLabstation", t, func(t *ftt.Test) {
		t.Run("CreateLabstation - RPM conflict", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-101",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			_, err2 := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-102",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, err2, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-101", "machine-101").withRpm("rpm-101", ".A1").build()
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Resemble(labstation1))

			// Create 2nd labstation with the same RPM details - should fail
			labstation2 := newMockLabstationBuilder("labstation-102", "machine-102").withRpm("rpm-101", ".A1").build()
			res2, err2 := CreateLabstation(ctx, labstation2)
			assert.Loosely(t, res2, should.BeNil)
			assert.Loosely(t, err2, should.NotBeNil)
			assert.Loosely(t, err2.Error(), should.ContainSubstring("The rpm powerunit_name and powerunit_outlet is already in use by labstation-101"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-102")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.BeEmpty)
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-102")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.BeEmpty)
			labstation3, err := GetMachineLSE(ctx, "labstation-102")
			assert.Loosely(t, labstation3, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}

func TestUpdateLabstation(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ftt.Run("UpdateLabstation", t, func(t *ftt.Test) {
		t.Run("UpdateLabstation - Non-existent labstation", func(t *ftt.Test) {
			labstation1 := newMockLabstationBuilder("labstation-1", "machine-1").build()
			// Labstation doesn't exist. Must return error
			res, err := UpdateLabstation(ctx, labstation1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Failed to get existing Labstation"))
			assert.Loosely(t, res, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			_, err = GetMachineLSE(ctx, "labstation-1")
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("UpdateLabstation - Delete machine, mask update", func(t *ftt.Test) {
			// Reset a machine by setting machines to nil and machines update mask
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-2",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			labstation2 := newMockLabstationBuilder("labstation-2", "machine-2").build()
			res, err := CreateLabstation(ctx, labstation2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Match(labstation2))
			labstation2 = newMockLabstationBuilder("labstation-2", "").build()
			// Attempt to delete machine. Should fail.
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("machines"))
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("InvalidArgument"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			labstation3, err := GetMachineLSE(ctx, "labstation-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetMachines(), should.Match([]string{"machine-2"}))
		})
		t.Run("UpdateLabstation - Delete machine", func(t *ftt.Test) {
			// Reset a machine in maskless update.
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-3",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-3", "machine-3").build()
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Match(labstation1))
			labstation1 = newMockLabstationBuilder("labstation-3", "").build()
			// Attempt to delete the machine in maskless update. Should fail.
			res, err = UpdateLabstation(ctx, labstation1, nil)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Empty Machine ID"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			labstation3, err := GetMachineLSE(ctx, "labstation-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetMachines(), should.Match([]string{"machine-3"}))
		})
		t.Run("UpdateLabstation - Reset rpm using update mask", func(t *ftt.Test) {
			// Delete rpm using update mask and setting rpm name to nil
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-4",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-4", "machine-4").build()
			labstation1.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Rpm = &chromeosLab.OSRPM{
				PowerunitName:   "rpm-4",
				PowerunitOutlet: ".A4",
				PowerunitType:   chromeosLab.OSRPM_TYPE_SENTRY,
			}
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Match(labstation1))
			// rpm of labstation2 is nil by default.
			labstation2 := newMockLabstationBuilder("labstation-4", "machine-4").build()
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("labstation.rpm.host"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetRpm(), should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("rpm-4"))
			assert.Loosely(t, changes[1].NewValue, should.BeEmpty)
			assert.Loosely(t, changes[2].OldValue, should.Equal(".A4"))
			assert.Loosely(t, changes[2].NewValue, should.BeEmpty)
			assert.Loosely(t, changes[3].OldValue, should.Equal("TYPE_SENTRY"))
			assert.Loosely(t, changes[3].NewValue, should.Equal("TYPE_UNKNOWN"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			labstation3, err := GetMachineLSE(ctx, "labstation-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetRpm(), should.BeNil)
			s, err := state.GetStateRecord(ctx, "hosts/labstation-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})
		t.Run("UpdateLabstation - Reset rpm outlet using update mask", func(t *ftt.Test) {
			// Reset rpm outlet using a mask update. Should fail.
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-5",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-5", "machine-5").build()
			labstation1.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Rpm = &chromeosLab.OSRPM{
				PowerunitName:   "rpm-5",
				PowerunitOutlet: ".A5",
				PowerunitType:   chromeosLab.OSRPM_TYPE_SENTRY,
			}
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Match(labstation1))
			labstation2 := newMockLabstationBuilder("labstation-5", "machine-5").build()
			labstation2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Rpm = &chromeosLab.OSRPM{PowerunitOutlet: ".A6"}
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("labstation.rpm.host", "labstation.rpm.outlet"))
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot update outlet"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			// Reset the rpm outlet. Should fail, can only reset complete rpm.
			labstation2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Rpm = &chromeosLab.OSRPM{
				PowerunitOutlet: "",
			}
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("labstation.rpm.outlet"))
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot remove rpm outlet"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			// Reset the rpm outlet and update rpm host. Should fail.
			labstation2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Rpm = &chromeosLab.OSRPM{
				PowerunitName:   "rpm-6",
				PowerunitOutlet: "",
				PowerunitType:   chromeosLab.OSRPM_TYPE_SENTRY,
			}
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("labstation.rpm.outlet", "labstation.rpm.name"))
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot remove rpm outlet"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			labstation3, err := GetMachineLSE(ctx, "labstation-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetRpm(), should.Match(&chromeosLab.OSRPM{
				PowerunitName:   "rpm-5",
				PowerunitOutlet: ".A5",
				PowerunitType:   chromeosLab.OSRPM_TYPE_SENTRY,
			}))
			s, err := state.GetStateRecord(ctx, "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			// No update to machines of rpm. Should not be in needs_deploy.
			assert.Loosely(t, s.GetState(), should.NotEqual(ufspb.State_STATE_DEPLOYED_PRE_SERVING))
		})
		t.Run("UpdateLabstation - Update/Delete pools", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-6",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-6", "machine-6").build()
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			labstation2 := newMockLabstationBuilder("labstation-6", "machine-6").build()
			// Add a pool to the labstation.
			labstation2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Pools = []string{"labstation_main"}
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("labstation.pools"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetPools(), should.Match([]string{"labstation_main"}))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("[]"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("[labstation_main]"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			labstation3, err := GetMachineLSE(ctx, "labstation-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetPools(), should.Match([]string{"labstation_main"}))
			// Reset pools assigned to labstation.
			labstation2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Pools = nil
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("labstation.pools"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetPools(), should.HaveLength(0))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("[labstation_main]"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("[]"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			labstation3, err = GetMachineLSE(ctx, "labstation-6")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetPools(), should.BeNil)
			s, err := state.GetStateRecord(ctx, "hosts/labstation-6")
			assert.Loosely(t, err, should.BeNil)
			// No update to machines of rpm. Should not be in needs_deploy.
			assert.Loosely(t, s.GetState(), should.NotEqual(ufspb.State_STATE_DEPLOYED_PRE_SERVING))
		})
		t.Run("UpdateLabstation - Update/Delete hive", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-13",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-13", "machine-13").build()
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			labstation2 := newMockLabstationBuilder("labstation-13", "machine-13").build()
			// Add a hive to the labstation.
			labstation2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Hive = "test-hive"
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("labstation.hive"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetHive(), should.Equal("test-hive"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-13")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[1].OldValue, should.Equal(""))
			assert.Loosely(t, changes[1].NewValue, should.Equal("test-hive"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-13")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			labstation3, err := GetMachineLSE(ctx, "labstation-13")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetHive(), should.Equal("test-hive"))
			// Reset hive assigned to labstation.
			labstation2.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Hive = ""
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("labstation.hive"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetHive(), should.Equal(""))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-13")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("test-hive"))
			assert.Loosely(t, changes[2].NewValue, should.Equal(""))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-13")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			labstation3, err = GetMachineLSE(ctx, "labstation-13")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetHive(), should.Equal(""))
			s, err := state.GetStateRecord(ctx, "hosts/labstation-13")
			assert.Loosely(t, err, should.BeNil)
			// No update to machines of rpm. Should not be in needs_deploy.
			assert.Loosely(t, s.GetState(), should.NotEqual(ufspb.State_STATE_DEPLOYED_PRE_SERVING))
		})
		t.Run("UpdateLabstation - Update/Delete tags", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-7",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-7", "machine-7").build()
			labstation1.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Pools = []string{"labstation_main"}
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Match(labstation1))
			labstation2 := newMockLabstationBuilder("labstation-7", "machine-7").build()
			// Add a tag to the labstation.
			labstation2.Tags = []string{"decommission"}
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("tags"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetTags(), should.Match([]string{"decommission"}))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			labstation3, err := GetMachineLSE(ctx, "labstation-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetTags(), should.Match([]string{"decommission"}))
			// Append another tag to the labstation.
			labstation2.Tags = []string{"needs_replacement"}
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("tags"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetTags(), should.Match([]string{"decommission", "needs_replacement"}))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			labstation3, err = GetMachineLSE(ctx, "labstation-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetTags(), should.Match([]string{"decommission", "needs_replacement"}))
			// Clear all tags from the labstation.
			labstation2.Tags = nil
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("tags"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.GetTags(), should.BeNil)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(4))
			labstation3, err = GetMachineLSE(ctx, "labstation-7")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetTags(), should.BeNil)
			s, err := state.GetStateRecord(ctx, "hosts/labstation-7")
			assert.Loosely(t, err, should.BeNil)
			// No update to machines of rpm. Should not be in needs_deploy.
			assert.Loosely(t, s.GetState(), should.NotEqual(ufspb.State_STATE_DEPLOYED_PRE_SERVING))
		})
		t.Run("UpdateLabstation - Update/Delete description", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-8",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-8", "machine-8").build()
			labstation1.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Pools = []string{"labstation_main"}
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Match(labstation1))
			labstation2 := newMockLabstationBuilder("labstation-8", "machine-8").build()
			// Add a description  to the labstation.
			labstation2.Description = "[12 Jan 2021] crbug.com/35007"
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("description"))
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("[12 Jan 2021] crbug.com/35007"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			labstation3, err := GetMachineLSE(ctx, "labstation-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetDescription(), should.Equal("[12 Jan 2021] crbug.com/35007"))
			// Reset labstation description.
			labstation2.Description = ""
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("description"))
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("[12 Jan 2021] crbug.com/35007"))
			assert.Loosely(t, changes[2].NewValue, should.BeEmpty)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			labstation3, err = GetMachineLSE(ctx, "labstation-8")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetDescription(), should.BeEmpty)
			s, err := state.GetStateRecord(ctx, "hosts/labstation-8")
			assert.Loosely(t, err, should.BeNil)
			// No update to machines of rpm. Should not be in needs_deploy.
			assert.Loosely(t, s.GetState(), should.NotEqual(ufspb.State_STATE_DEPLOYED_PRE_SERVING))
		})
		t.Run("UpdateLabstation - Update/Delete deploymentTicket", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-9",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-9", "machine-9").build()
			labstation1.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Pools = []string{"labstation_main"}
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Match(labstation1))
			labstation2 := newMockLabstationBuilder("labstation-9", "machine-9").build()
			// Add a deployment ticket to the labstation.
			labstation2.DeploymentTicket = "crbug.com/35007"
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("deploymentTicket"))
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("crbug.com/35007"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			labstation3, err := GetMachineLSE(ctx, "labstation-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetDeploymentTicket(), should.Equal("crbug.com/35007"))
			// Reset deployment ticket to the labstation.
			labstation2.DeploymentTicket = ""
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("deploymentTicket"))
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("crbug.com/35007"))
			assert.Loosely(t, changes[2].NewValue, should.BeEmpty)
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(3))
			labstation3, err = GetMachineLSE(ctx, "labstation-9")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetDeploymentTicket(), should.BeEmpty)
			s, err := state.GetStateRecord(ctx, "hosts/labstation-9")
			assert.Loosely(t, err, should.BeNil)
			// No update to machines of rpm. Should not be in needs_deploy.
			assert.Loosely(t, s.GetState(), should.NotEqual(ufspb.State_STATE_DEPLOYED_PRE_SERVING))
		})
		t.Run("UpdateLabstation - Update labstation state", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-10",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-10", "machine-10").build()
			labstation1.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Pools = []string{"labstation_main"}
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Match(labstation1))
			labstation2 := newMockLabstationBuilder("labstation-10", "machine-10").build()
			// Set labstation state to serving.
			labstation2.ResourceState = ufspb.State_STATE_SERVING
			res, err = UpdateLabstation(ctx, labstation2, mockFieldMask("resourceState"))
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			labstation3, err := GetMachineLSE(ctx, "labstation-10")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.ResourceState, should.Equal(ufspb.State_STATE_SERVING))
			// State record should not be needs_deploy
			s, err := state.GetStateRecord(ctx, "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			// No update to machines of rpm. Should not be in needs_deploy.
			assert.Loosely(t, s.GetState(), should.NotEqual(ufspb.State_STATE_DEPLOYED_PRE_SERVING))
		})

		t.Run("UpdateLabstation - RPM conflict when changing rpm name", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-11",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			_, err2 := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-12",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, err2, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-11", "machine-11").withRpm("rpm-11", ".A1").build()
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Resemble(labstation1))
			labstation2 := newMockLabstationBuilder("labstation-12", "machine-12").withRpm("rpm-12", ".A1").build()
			res2, err2 := CreateLabstation(ctx, labstation2)
			assert.Loosely(t, err2, should.BeNil)
			assert.Loosely(t, res2, should.NotBeNil)
			assert.Loosely(t, res2, should.Resemble(labstation2))

			// When updating via shivas the machineLSE is filled only with the updated data
			updatedLabstation2 := newMockLabstationBuilder("labstation-12", "machine-12").withRpm("rpm-11", "").build()

			// Attempt to update machine. Should fail.
			res, err = UpdateLabstation(ctx, updatedLabstation2, nil)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("The rpm powerunit_name and powerunit_outlet is already in use by labstation-11"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-12")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-12")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			labstation3, err := GetMachineLSE(ctx, "labstation-12")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetMachines(), should.Resemble([]string{"machine-12"}))
		})

		t.Run("UpdateLabstation - RPM conflict when changing rpm outlet", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-14",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			_, err2 := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-15",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, err2, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-14", "machine-14").withRpm("rpm-14", ".A1").build()
			res, err := CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res, should.Resemble(labstation1))
			labstation2 := newMockLabstationBuilder("labstation-15", "machine-15").withRpm("rpm-14", ".A2").build()
			res2, err2 := CreateLabstation(ctx, labstation2)
			assert.Loosely(t, err2, should.BeNil)
			assert.Loosely(t, res2, should.NotBeNil)
			assert.Loosely(t, res2, should.Resemble(labstation2))

			// When updating via shivas the machineLSE is filled only with the updated data
			updatedLabstation2 := newMockLabstationBuilder("labstation-15", "machine-15").withRpm("", ".A1").build()

			// Attempt to update machine. Should fail.
			res, err = UpdateLabstation(ctx, updatedLabstation2, nil)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("The rpm powerunit_name and powerunit_outlet is already in use by labstation-14"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-15")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-15")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			labstation3, err := GetMachineLSE(ctx, "labstation-15")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, labstation3.GetMachines(), should.Resemble([]string{"machine-15"}))
		})
	})
}

func TestRenameLabstation(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ftt.Run("renameLabstation", t, func(t *ftt.Test) {
		t.Run("renameLabstation - Missing Delete permission", func(t *ftt.Test) {
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesUpdate),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesCreate),
				),
			})
			machine1 := &ufspb.Machine{
				Name:  "machine-1l",
				Realm: util.AtlLabAdminRealm,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			machine2 := &ufspb.Machine{
				Name:  "machine-1d",
				Realm: util.AtlLabAdminRealm,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)
			labstation1 := newMockLabstationBuilder("labstation-1", "machine-1l").build()
			_, err = CreateLabstation(ctx, labstation1)
			assert.Loosely(t, err, should.BeNil)
			dut1 := mockDUT("dut-1", "machine-1d", "labstation-1", "serial-1", "power-1", ".A1", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(ctx, dut1)
			assert.Loosely(t, err, should.BeNil)
			_, err = RenameMachineLSE(ctx, "labstation-1", "labstation-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(codes.PermissionDenied.String()))
		})
		t.Run("renameLabstation - Missing Update permission", func(t *ftt.Test) {
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesCreate),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesDelete),
				),
			})
			machine1 := &ufspb.Machine{
				Name:  "machine-2l",
				Realm: util.AtlLabAdminRealm,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			machine2 := &ufspb.Machine{
				Name:  "machine-2d",
				Realm: util.AtlLabAdminRealm,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)
			labstation2 := newMockLabstationBuilder("labstation-2", "machine-2l").build()
			_, err = CreateLabstation(ctx, labstation2)
			assert.Loosely(t, err, should.BeNil)
			dut2 := mockDUT("dut-2", "machine-2d", "labstation-2", "serial-2", "power-2", ".A2", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(ctx, dut2)
			assert.Loosely(t, err, should.BeNil)
			_, err = RenameMachineLSE(ctx, "labstation-2", "labstation-3")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(codes.PermissionDenied.String()))
		})
		t.Run("renameLabstation - Missing Create permission", func(t *ftt.Test) {
			createCtx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesCreate),
				),
			})
			machine1 := &ufspb.Machine{
				Name:  "machine-3l",
				Realm: util.AtlLabAdminRealm,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			machine2 := &ufspb.Machine{
				Name:  "machine-3d",
				Realm: util.AtlLabAdminRealm,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(createCtx, machine1)
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateMachine(createCtx, machine2)
			assert.Loosely(t, err, should.BeNil)
			labstation2 := newMockLabstationBuilder("labstation-3", "machine-3l").build()
			_, err = CreateLabstation(createCtx, labstation2)
			assert.Loosely(t, err, should.BeNil)
			dut2 := mockDUT("dut-3", "machine-3d", "labstation-3", "serial-3", "power-3", ".A3", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(createCtx, dut2)
			assert.Loosely(t, err, should.BeNil)
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesUpdate),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesDelete),
				),
			})
			_, err = RenameMachineLSE(ctx, "labstation-3", "labstation-4")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(codes.PermissionDenied.String()))
		})
		t.Run("renameLabstation - Happy path", func(t *ftt.Test) {
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:user@example.com",
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:user@example.com", "user"),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesCreate),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesDelete),
					authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.InventoriesUpdate),
				),
			})
			machine1 := &ufspb.Machine{
				Name:  "machine-4l",
				Realm: util.AtlLabAdminRealm,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			machine2 := &ufspb.Machine{
				Name:  "machine-4d",
				Realm: util.AtlLabAdminRealm,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "test",
						Model:       "test",
					},
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			_, err = registration.CreateMachine(ctx, machine2)
			assert.Loosely(t, err, should.BeNil)
			labstation2 := newMockLabstationBuilder("labstation-4", "machine-4l").build()
			_, err = CreateLabstation(ctx, labstation2)
			assert.Loosely(t, err, should.BeNil)
			dut2 := mockDUT("dut-4", "machine-4d", "labstation-4", "serial-4", "power-4", ".A4", int32(9999), []string{"DUT_POOL_QUOTA"}, "")
			_, err = CreateDUT(ctx, dut2)
			assert.Loosely(t, err, should.BeNil)
			_, err = RenameMachineLSE(ctx, "labstation-4", "labstation-5")
			assert.Loosely(t, err, should.BeNil)
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-4")
			assert.Loosely(t, err, should.BeNil)
			// One snapshot at registration and another at rename
			assert.Loosely(t, msgs, should.HaveLength(2))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			// One snapshot at registration
			assert.Loosely(t, msgs, should.HaveLength(1))
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "hosts/dut-4")
			assert.Loosely(t, err, should.BeNil)
			// One snapshot at registration and another at servo host change
			assert.Loosely(t, msgs, should.HaveLength(2))
			// State record for new dut should be same.
			s, err := state.GetStateRecord(ctx, "hosts/dut-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
			// State record for old labstation should not exist.
			_, err = state.GetStateRecord(ctx, "hosts/labstation-4")
			assert.Loosely(t, err, should.NotBeNil)
			// State record for new labstation should be same as old one..
			s, err = state.GetStateRecord(ctx, "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetState(), should.Equal(ufspb.State_STATE_REGISTERED))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[0].OldValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("labstation-4"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("labstation-5"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/labstation-5")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].OldValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("labstation-4"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("labstation-5"))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "hosts/dut-4")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].EventLabel, should.Equal("machine_lse.chromeos_machine_lse.dut.servo.hostname"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("labstation-4"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("labstation-5"))
			assert.Loosely(t, changes[0].OldValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
		})
	})
}
