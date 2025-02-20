// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	"go.chromium.org/infra/unifiedfleet/app/model/history"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func TestUpdateDutState(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx, _ = util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	osCtx := withAuthorizedAtlUser(ctx)
	noPermsCtx := withAuthorizedNoPermsUser(ctx)
	ftt.Run("UpdateDutState", t, func(t *ftt.Test) {
		t.Run("Update dut state - missing state", func(t *ftt.Test) {
			_, err := UpdateDutState(ctx, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("dut state must not be null"))
		})
		t.Run("Update dut state with non-existing host in dut state storage", func(t *ftt.Test) {
			ds1 := mockDutState("update-dutstate-id1", "update-dutstate-hostname1")
			_, err := UpdateDutState(ctx, ds1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Entity not found"))
		})
		t.Run("Update dut state - happy path with existing dut state", func(t *ftt.Test) {
			ds1 := mockDutState("update-dutstate-id2", "update-dutstate-hostname2")
			ds1.Servo = chromeosLab.PeripheralState_WORKING
			ds1.Chameleon = chromeosLab.PeripheralState_WORKING
			ds1.StorageState = chromeosLab.HardwareState_HARDWARE_ACCEPTABLE

			// Use osCtx in setup
			_, err := inventory.CreateMachineLSE(osCtx, &ufspb.MachineLSE{
				Name:     "update-dutstate-hostname2",
				Hostname: "update-dutstate-hostname2",
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
				Realm: util.AtlLabAdminRealm,
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = state.UpdateDutStates(osCtx, []*chromeosLab.DutState{ds1})
			assert.Loosely(t, err, should.BeNil)
			oldDS, err := state.GetDutState(osCtx, "update-dutstate-id2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, oldDS.GetServo(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, oldDS.GetChameleon(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, oldDS.GetStorageState(), should.Equal(chromeosLab.HardwareState_HARDWARE_ACCEPTABLE))

			// Use osCtx in testing, as in prod, ctx is forced to include namespace.
			ds2 := mockDutState("update-dutstate-id2", "update-dutstate-hostname2")
			ds2.Servo = chromeosLab.PeripheralState_BROKEN
			ds2.Chameleon = chromeosLab.PeripheralState_BROKEN
			ds2.StorageState = chromeosLab.HardwareState_HARDWARE_NEED_REPLACEMENT
			_, err = UpdateDutState(osCtx, ds2)
			assert.Loosely(t, err, should.BeNil)

			// Verify with osCtx
			newDS, err := state.GetDutState(osCtx, "update-dutstate-id2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, newDS.GetServo(), should.Equal(chromeosLab.PeripheralState_BROKEN))
			assert.Loosely(t, newDS.GetChameleon(), should.Equal(chromeosLab.PeripheralState_BROKEN))
			assert.Loosely(t, newDS.GetStorageState(), should.Equal(chromeosLab.HardwareState_HARDWARE_NEED_REPLACEMENT))
			// Verify changes
			changes, err := history.QueryChangesByPropertyName(osCtx, "name", "dutstates/update-dutstate-id2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(4))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dut_state.servo"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(chromeosLab.PeripheralState_WORKING.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(chromeosLab.PeripheralState_BROKEN.String()))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("dut_state.chameleon"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(chromeosLab.PeripheralState_WORKING.String()))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(chromeosLab.PeripheralState_BROKEN.String()))
			assert.Loosely(t, changes[2].GetEventLabel(), should.Equal("dut_state.storage_state"))
			assert.Loosely(t, changes[2].GetOldValue(), should.Equal(chromeosLab.HardwareState_HARDWARE_ACCEPTABLE.String()))
			assert.Loosely(t, changes[2].GetNewValue(), should.Equal(chromeosLab.HardwareState_HARDWARE_NEED_REPLACEMENT.String()))
			msgs, err := history.QuerySnapshotMsgByPropertyName(osCtx, "resource_name", "dutstates/update-dutstate-id2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
		t.Run("Update dut state - no perms", func(t *ftt.Test) {
			ds3 := mockDutState("update-dutstate-id3", "update-dutstate-hostname3")
			ds3.Servo = chromeosLab.PeripheralState_WORKING
			ds3.Chameleon = chromeosLab.PeripheralState_WORKING
			ds3.StorageState = chromeosLab.HardwareState_HARDWARE_ACCEPTABLE

			// Use osCtx in setup
			_, err := inventory.CreateMachineLSE(osCtx, &ufspb.MachineLSE{
				Name:     "update-dutstate-hostname3",
				Hostname: "update-dutstate-hostname3",
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
				Realm: util.AtlLabAdminRealm,
			})
			assert.Loosely(t, err, should.BeNil)

			_, err = UpdateDutState(noPermsCtx, ds3)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Permission"))
		})
	})
}

func TestUpdateDutStateWithDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx, _ = util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	ctx = withAuthorizedAtlUser(ctx)
	ftt.Run("UpdateDutState with DeviceLabels", t, func(t *ftt.Test) {
		t.Run("Update dut state with device labels happy path", func(t *ftt.Test) {
			ds1 := mockDutState("update-dutstate-devicelabels-1", "update-dutstate-devicelabels-hostname1")
			_, err := inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:     "update-dutstate-devicelabels-hostname1",
				Hostname: "update-dutstate-devicelabels-hostname1",
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{},
				},
				Realm: util.AtlLabAdminRealm,
			})
			assert.Loosely(t, err, should.BeNil)

			su1 := &ufspb.SchedulingUnit{
				Name:        "update-dutstate-su-1",
				MachineLSEs: []string{"update-dutstate-devicelabels-hostname1"},
			}
			_, err = inventory.CreateSchedulingUnit(ctx, su1)
			assert.Loosely(t, err, should.BeNil)

			_, err = UpdateDutState(ctx, ds1)
			assert.Loosely(t, err, should.BeNil)

			// DUT labels
			resp, err := inventory.GetDeviceLabels(ctx, util.AddPrefix(util.MachineLSECollection, "update-dutstate-devicelabels-hostname1"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetName(), should.Equal("machineLSEs/update-dutstate-devicelabels-hostname1"))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/update-dutstate-devicelabels-hostname1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))

			// Scheduling Unit labels
			resp, err = inventory.GetDeviceLabels(ctx, util.AddPrefix(util.SchedulingUnitCollection, "update-dutstate-su-1"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetName(), should.Equal("schedulingunits/update-dutstate-su-1"))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufspb.ResourceType_RESOURCE_TYPE_SCHEDULING_UNIT))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "devicelabels/schedulingunits/update-dutstate-su-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
		})
	})
}

func TestUpdateDutStateWithMasks(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	osCtx, _ := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	createNewState := func(id int) (idString, hostname string) {
		idString = fmt.Sprintf("update-dutstate-id%d", id)
		hostname = fmt.Sprintf("update-dutstate-hostname%d", id)
		ds1 := mockDutState(idString, hostname)
		ds1.DutStateReason = "Haha"
		ds1.Servo = chromeosLab.PeripheralState_WORKING
		ds1.Chameleon = chromeosLab.PeripheralState_WORKING
		ds1.StorageState = chromeosLab.HardwareState_HARDWARE_ACCEPTABLE
		ds1.RepairRequests = []chromeosLab.DutState_RepairRequest{
			chromeosLab.DutState_REPAIR_REQUEST_PROVISION,
		}

		// Use osCtx in setup
		_, err := inventory.CreateMachineLSE(osCtx, &ufspb.MachineLSE{
			Name:     hostname,
			Hostname: hostname,
			Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
				ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
			},
		})
		assert.Loosely(t, err, should.BeNil)
		_, err = state.UpdateDutStates(osCtx, []*chromeosLab.DutState{ds1})
		assert.Loosely(t, err, should.BeNil)
		oldDS, err := state.GetDutState(osCtx, idString)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, oldDS.GetDutStateReason(), should.Equal("Haha"))
		assert.Loosely(t, oldDS.GetServo(), should.Equal(chromeosLab.PeripheralState_WORKING))
		assert.Loosely(t, oldDS.GetChameleon(), should.Equal(chromeosLab.PeripheralState_WORKING))
		assert.Loosely(t, oldDS.GetStorageState(), should.Equal(chromeosLab.HardwareState_HARDWARE_ACCEPTABLE))
		assert.Loosely(t, oldDS.GetRepairRequests(), should.Match([]chromeosLab.DutState_RepairRequest{
			chromeosLab.DutState_REPAIR_REQUEST_PROVISION,
		}))
		return
	}
	ftt.Run("UpdateDutStateWithMasks", t, func(t *ftt.Test) {
		t.Run("Update dut state - missing state", func(t *ftt.Test) {
			_, err := UpdateDutStateWithMasks(ctx, nil, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("dut state must not be null"))
		})
		t.Run("Update dut state with non-existing host in dut state storage", func(t *ftt.Test) {
			ds1 := mockDutState("update-dutstate-id1", "update-dutstate-hostname1")
			_, err := UpdateDutStateWithMasks(ctx, nil, ds1)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Entity not found"))
		})
		t.Run("Update dut state - happy path with no masks", func(t *ftt.Test) {
			id, hostname := createNewState(2)
			// Use osCtx in testing, as in prod, ctx is forced to include namespace.
			ds2 := mockDutState(id, hostname)
			ds2.DutStateReason = "new-reason"
			ds2.Servo = chromeosLab.PeripheralState_BROKEN
			ds2.Chameleon = chromeosLab.PeripheralState_BROKEN
			ds2.StorageState = chromeosLab.HardwareState_HARDWARE_NEED_REPLACEMENT
			ds2.RepairRequests = []chromeosLab.DutState_RepairRequest{
				chromeosLab.DutState_REPAIR_REQUEST_REIMAGE_BY_USBKEY,
			}
			var maskSet map[string]bool
			_, err := UpdateDutStateWithMasks(osCtx, maskSet, ds2)
			assert.Loosely(t, err, should.BeNil)

			// Verify with osCtx
			newDS, err := state.GetDutState(osCtx, id)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, newDS.GetDutStateReason(), should.Equal("Haha"))
			assert.Loosely(t, newDS.GetServo(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, newDS.GetChameleon(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, newDS.GetStorageState(), should.Equal(chromeosLab.HardwareState_HARDWARE_ACCEPTABLE))
			assert.Loosely(t, newDS.GetRepairRequests(), should.Match([]chromeosLab.DutState_RepairRequest{
				chromeosLab.DutState_REPAIR_REQUEST_PROVISION,
			}))
			// Verify changes
			changes, err := history.QueryChangesByPropertyName(osCtx, "name", fmt.Sprintf("dutstates/%s", id))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
			msgs, err := history.QuerySnapshotMsgByPropertyName(osCtx, "resource_name", fmt.Sprintf("dutstates/%s", id))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
		t.Run("Update dut state - happy path with servo masks", func(t *ftt.Test) {
			id, hostname := createNewState(3)
			// Use osCtx in testing, as in prod, ctx is forced to include namespace.
			ds2 := mockDutState(id, hostname)
			ds2.DutStateReason = "new-reason"
			ds2.Servo = chromeosLab.PeripheralState_BROKEN
			ds2.Chameleon = chromeosLab.PeripheralState_BROKEN
			ds2.StorageState = chromeosLab.HardwareState_HARDWARE_NEED_REPLACEMENT
			ds2.RepairRequests = []chromeosLab.DutState_RepairRequest{
				chromeosLab.DutState_REPAIR_REQUEST_REIMAGE_BY_USBKEY,
			}
			maskSet := map[string]bool{
				"dut_state.servo": true,
			}
			_, err := UpdateDutStateWithMasks(osCtx, maskSet, ds2)
			assert.Loosely(t, err, should.BeNil)

			// Verify with osCtx
			newDS, err := state.GetDutState(osCtx, id)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, newDS.GetDutStateReason(), should.Equal("Haha"))
			assert.Loosely(t, newDS.GetServo(), should.Equal(chromeosLab.PeripheralState_BROKEN)) // only value to apply
			assert.Loosely(t, newDS.GetChameleon(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, newDS.GetStorageState(), should.Equal(chromeosLab.HardwareState_HARDWARE_ACCEPTABLE))
			assert.Loosely(t, newDS.GetRepairRequests(), should.Match([]chromeosLab.DutState_RepairRequest{
				chromeosLab.DutState_REPAIR_REQUEST_PROVISION,
			}))
			// Verify changes
			changes, err := history.QueryChangesByPropertyName(osCtx, "name", fmt.Sprintf("dutstates/%s", id))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dut_state.servo"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(chromeosLab.PeripheralState_WORKING.String()))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(chromeosLab.PeripheralState_BROKEN.String()))
			msgs, err := history.QuerySnapshotMsgByPropertyName(osCtx, "resource_name", fmt.Sprintf("dutstates/%s", id))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
		t.Run("Update dut state - happy path with state reason and repair_requests masks", func(t *ftt.Test) {
			id, hostname := createNewState(4)
			// Use osCtx in testing, as in prod, ctx is forced to include namespace.
			ds2 := mockDutState(id, hostname)
			ds2.DutStateReason = "new-reason"
			ds2.Servo = chromeosLab.PeripheralState_BROKEN
			ds2.Chameleon = chromeosLab.PeripheralState_BROKEN
			ds2.StorageState = chromeosLab.HardwareState_HARDWARE_NEED_REPLACEMENT
			ds2.RepairRequests = []chromeosLab.DutState_RepairRequest{
				chromeosLab.DutState_REPAIR_REQUEST_REIMAGE_BY_USBKEY,
				chromeosLab.DutState_REPAIR_REQUEST_UPDATE_USBKEY_IMAGE,
			}
			maskSet := map[string]bool{
				"dut_state.reason":          true,
				"dut_state.repair_requests": true,
			}
			_, err := UpdateDutStateWithMasks(osCtx, maskSet, ds2)
			assert.Loosely(t, err, should.BeNil)

			// Verify with osCtx
			newDS, err := state.GetDutState(osCtx, id)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, newDS.GetDutStateReason(), should.Equal("new-reason"))
			assert.Loosely(t, newDS.GetServo(), should.Equal(chromeosLab.PeripheralState_WORKING)) // only value to apply
			assert.Loosely(t, newDS.GetChameleon(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, newDS.GetStorageState(), should.Equal(chromeosLab.HardwareState_HARDWARE_ACCEPTABLE))
			assert.Loosely(t, newDS.GetRepairRequests(), should.Match([]chromeosLab.DutState_RepairRequest{
				chromeosLab.DutState_REPAIR_REQUEST_REIMAGE_BY_USBKEY,
				chromeosLab.DutState_REPAIR_REQUEST_UPDATE_USBKEY_IMAGE,
			}))
			// Verify changes
			changes, err := history.QueryChangesByPropertyName(osCtx, "name", fmt.Sprintf("dutstates/%s", id))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("dut_state.reason"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("Haha"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("new-reason"))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("dut_state.repair_requests"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("[REPAIR_REQUEST_PROVISION]"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("[REPAIR_REQUEST_REIMAGE_BY_USBKEY REPAIR_REQUEST_UPDATE_USBKEY_IMAGE]"))

			msgs, err := history.QuerySnapshotMsgByPropertyName(osCtx, "resource_name", fmt.Sprintf("dutstates/%s", id))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
	})
}

func TestGetDutState(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	osCtx, _ := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	ftt.Run("GetDutState", t, func(t *ftt.Test) {
		t.Run("Get dut state by id with non-existing host in dut state storage", func(t *ftt.Test) {
			_, err := GetDutState(ctx, "id1", "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Entity not found"))
		})

		t.Run("Get dut state by hostname with non-existing host in dut state storage", func(t *ftt.Test) {
			_, err := GetDutState(ctx, "", "hostname1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Dut State not found for hostname1."))
		})

		t.Run("Get dut state by id - happy path with existing dut state", func(t *ftt.Test) {
			ds1 := mockDutState("update-dutstate-id2", "update-dutstate-hostname2")
			ds1.Servo = chromeosLab.PeripheralState_WORKING
			ds1.Chameleon = chromeosLab.PeripheralState_WORKING
			ds1.StorageState = chromeosLab.HardwareState_HARDWARE_ACCEPTABLE

			_, err := state.UpdateDutStates(osCtx, []*chromeosLab.DutState{ds1})
			assert.Loosely(t, err, should.BeNil)

			oldDS, err := GetDutState(osCtx, "update-dutstate-id2", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, oldDS.GetServo(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, oldDS.GetChameleon(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, oldDS.GetStorageState(), should.Equal(chromeosLab.HardwareState_HARDWARE_ACCEPTABLE))
		})

		t.Run("Get dut state by hostname - happy path with existing dut state", func(t *ftt.Test) {
			ds1 := mockDutState("update-dutstate-id3", "update-dutstate-hostname3")
			ds1.Servo = chromeosLab.PeripheralState_WORKING
			ds1.Chameleon = chromeosLab.PeripheralState_WORKING
			ds1.StorageState = chromeosLab.HardwareState_HARDWARE_ACCEPTABLE

			_, err := state.UpdateDutStates(osCtx, []*chromeosLab.DutState{ds1})
			assert.Loosely(t, err, should.BeNil)

			oldDS, err := GetDutState(osCtx, "", "update-dutstate-hostname3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, oldDS.GetServo(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, oldDS.GetChameleon(), should.Equal(chromeosLab.PeripheralState_WORKING))
			assert.Loosely(t, oldDS.GetStorageState(), should.Equal(chromeosLab.HardwareState_HARDWARE_ACCEPTABLE))
		})
	})
}

func TestListDutStates(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	dutStates := make([]*chromeosLab.DutState, 0, 4)
	for i := 0; i < 4; i++ {
		cs := mockDutState(fmt.Sprintf("cs-machine-%d", i), fmt.Sprintf("cs-dut-%d", i))
		dutStates = append(dutStates, cs)
	}
	dutStates, _ = state.UpdateDutStates(ctx, dutStates)
	ftt.Run("ListDutStates", t, func(t *ftt.Test) {
		t.Run("ListDutStates - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListDutStates(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(dutStates))
		})
	})
}

func mockDutState(id, hostname string) *chromeosLab.DutState {
	return &chromeosLab.DutState{
		Id: &chromeosLab.ChromeOSDeviceID{
			Value: id,
		},
		Hostname: hostname,
	}
}
