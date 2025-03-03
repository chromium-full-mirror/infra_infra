// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/dutstate"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	"go.chromium.org/infra/unifiedfleet/app/external"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/history"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func TestCreateDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("CreateDeviceLabels", t, func(t *ftt.Test) {
		t.Run("Create new DeviceLabels", func(t *ftt.Test) {
			deviceLabels1 := &ufspb.DeviceLabels{
				Name:         util.AddPrefix(util.MachineLSECollection, "devicelabels-create-1"),
				ResourceType: ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE,
				Labels:       map[string]*ufspb.DeviceLabelValues{},
			}
			resp, err := CreateDeviceLabels(ctx, deviceLabels1, "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/devicelabels-create-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
		})
	})
}

func TestUpdateDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ftt.Run("UpdateDeviceLabels", t, func(t *ftt.Test) {
		t.Run("Update non-existing DeviceLabels", func(t *ftt.Test) {
			deviceLabels1 := &ufspb.DeviceLabels{
				Name:         util.AddPrefix(util.MachineLSECollection, "devicelabels-update-1"),
				ResourceType: ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE,
				Labels:       map[string]*ufspb.DeviceLabelValues{},
			}
			resp, err := UpdateDeviceLabels(ctx, deviceLabels1, nil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/devicelabels-update-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update DeviceLabels happy path", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name:         "devicelabels-update-machine-2",
				SerialNumber: "devicelabels-update-machine-2-serial",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_BROWSER_GOOGLER_DESK,
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			lse1 := &ufspb.MachineLSE{
				Name:     "devicelabels-update-lse-2",
				Machines: []string{"devicelabels-update-machine-2"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			_, err = inventory.CreateMachineLSE(ctx, lse1)
			assert.Loosely(t, err, should.BeNil)

			deviceLabels1 := &ufspb.DeviceLabels{
				Name:         util.AddPrefix(util.MachineLSECollection, "devicelabels-update-lse-2"),
				ResourceType: ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE,
				Labels:       map[string]*ufspb.DeviceLabelValues{"field": {LabelValues: []string{"value1"}}},
			}
			deviceLabels2 := &ufspb.DeviceLabels{
				Name:         util.AddPrefix(util.MachineLSECollection, "devicelabels-update-lse-2"),
				ResourceType: ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE,
				Labels:       map[string]*ufspb.DeviceLabelValues{"field": {LabelValues: []string{"value2"}}},
			}
			resp, err := CreateDeviceLabels(ctx, deviceLabels1, "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)

			ctx = initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesUpdate, util.BrowserLabAdminRealm)
			resp, err = UpdateDeviceLabels(ctx, deviceLabels2, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/devicelabels-update-lse-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("device_labels"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/devicelabels-update-lse-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
		})
	})
}

func TestDeleteDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("DeleteDeviceLabels", t, func(t *ftt.Test) {
		t.Run("Delete non-existing Device Labels", func(t *ftt.Test) {
			err := DeleteDeviceLabels(ctx, "machineLSEs/devicelabels-delete-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/devicelabels-delete-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})
		t.Run("Delete Device Labels - happy path", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name:         "devicelabels-delete-machine-2",
				SerialNumber: "devicelabels-delete-machine-2-serial",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_BROWSER_GOOGLER_DESK,
				},
			}
			_, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)
			lse1 := &ufspb.MachineLSE{
				Name:     "devicelabels-delete-lse-2",
				Machines: []string{"devicelabels-delete-machine-2"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
			}
			_, err = inventory.CreateMachineLSE(ctx, lse1)
			assert.Loosely(t, err, should.BeNil)

			deviceLabels1 := &ufspb.DeviceLabels{
				Name:         util.AddPrefix(util.MachineLSECollection, "devicelabels-delete-lse-2"),
				ResourceType: ufspb.ResourceType_RESOURCE_TYPE_BROWSER_DEVICE,
				Labels:       map[string]*ufspb.DeviceLabelValues{},
			}
			ctx = initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesDelete, util.BrowserLabAdminRealm)
			_, err = CreateDeviceLabels(ctx, deviceLabels1, "")
			assert.Loosely(t, err, should.BeNil)

			err = DeleteDeviceLabels(ctx, "machineLSEs/devicelabels-delete-lse-2")
			assert.Loosely(t, err, should.BeNil)

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "devicelabels/machineLSEs/devicelabels-delete-lse-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("device_labels"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "devicelabels/machineLSEs/devicelabels-delete-lse-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeTrue)
		})
	})
}

func TestListDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	deviceLabelsList := []*ufspb.DeviceLabels{
		{
			Name:         util.AddPrefix(util.MachineLSECollection, "devicelabels-list-1"),
			ResourceType: ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE,
			Labels:       map[string]*ufspb.DeviceLabelValues{},
		},
		{
			Name:         util.AddPrefix(util.MachineLSECollection, "devicelabels-list-2"),
			ResourceType: ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE,
			Labels:       map[string]*ufspb.DeviceLabelValues{},
		},
		{
			Name:         util.AddPrefix(util.MachineLSECollection, "devicelabels-list-3"),
			ResourceType: ufspb.ResourceType_RESOURCE_TYPE_ATTACHED_DEVICE,
			Labels:       map[string]*ufspb.DeviceLabelValues{},
		},
		{
			Name:         util.AddPrefix(util.SchedulingUnitCollection, "devicelabels-list-4"),
			ResourceType: ufspb.ResourceType_RESOURCE_TYPE_SCHEDULING_UNIT,
			Labels:       map[string]*ufspb.DeviceLabelValues{},
		},
	}
	ftt.Run("ListDeviceLabels", t, func(t *ftt.Test) {
		_, err := inventory.BatchUpdateDeviceLabels(ctx, deviceLabelsList)
		assert.Loosely(t, err, should.BeNil)
		t.Run("List DeviceLabels - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListDeviceLabels(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(deviceLabelsList))
		})
		t.Run("List DeviceLabels - resource type filter", func(t *ftt.Test) {
			resp, _, err := ListDeviceLabels(ctx, 3, "", "resourcetype=RESOURCE_TYPE_CHROMEOS_DEVICE", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, resp[0].GetName(), should.Equal("machineLSEs/devicelabels-list-1"))
		})
	})
}

func TestBatchGetDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetDeviceLabels", t, func(t *ftt.Test) {
		t.Run("Batch get device labels - happy path", func(t *ftt.Test) {
			entities := make([]*ufspb.DeviceLabels, 4)
			for i := range 4 {
				nameSuffix := fmt.Sprintf("devicelabels-batchGet-%d", i)
				entities[i] = &ufspb.DeviceLabels{
					Name:         util.AddPrefix(util.MachineLSECollection, nameSuffix),
					ResourceType: ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE,
				}
			}
			resp, err := inventory.BatchUpdateDeviceLabels(ctx, entities)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			resp, err = inventory.BatchGetDeviceLabels(ctx, []string{"machineLSEs/devicelabels-batchGet-0", "machineLSEs/devicelabels-batchGet-1", "machineLSEs/devicelabels-batchGet-2", "machineLSEs/devicelabels-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(entities))
		})
		t.Run("Batch get device labels  - missing id", func(t *ftt.Test) {
			resp, err := inventory.BatchGetDeviceLabels(ctx, []string{"vm-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("vm-batchGet-non-existing"))
		})
		t.Run("Batch get device labels  - empty input", func(t *ftt.Test) {
			resp, err := inventory.BatchGetDeviceLabels(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = inventory.BatchGetDeviceLabels(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}

func TestGetMachineLSELabels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ctx = external.WithTestingContext(ctx)
	ftt.Run("GetMachineLSELabels", t, func(t *ftt.Test) {
		t.Run("GetMachineLSELabels - no/other lse", func(t *ftt.Test) {
			lse1 := &ufspb.MachineLSE{
				Name:     "machinelse-1",
				Machines: []string{"machine-1"},
				Lse:      nil,
			}
			resp, err := GetMachineLSELabels(ctx, lse1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("GetMachineLSELabels - browser lse", func(t *ftt.Test) {
			lse1 := &ufspb.MachineLSE{
				Name:     "machinelse-2",
				Machines: []string{"machine-2"},
				Lse: &ufspb.MachineLSE_ChromeBrowserMachineLse{
					ChromeBrowserMachineLse: &ufspb.ChromeBrowserMachineLSE{},
				},
				ResourceState: ufspb.State_STATE_SERVING,
				Realm:         util.BrowserLabAdminRealm,
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesCreate, util.BrowserLabAdminRealm)
			lseResp, err := inventory.CreateMachineLSE(ctx, lse1)
			assert.Loosely(t, err, should.BeNil)

			resp, err := GetMachineLSELabels(ctx, lseResp)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetName(), should.Equal(util.AddPrefix(util.MachineLSECollection, lse1.GetName())))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufspb.ResourceType_RESOURCE_TYPE_BROWSER_DEVICE))
			assert.Loosely(t, resp.GetRealm(), should.Equal(util.BrowserLabAdminRealm))
			labels := resp.GetLabels()
			assert.Loosely(t, labels, should.HaveLength(3))
			assert.Loosely(t, labels["ufs_zone"], should.NotBeNil)
			assert.Loosely(t, labels["ufs_zone"].GetLabelValues(), should.NotBeEmpty)
			assert.Loosely(t, labels["ufs_zone"].GetLabelValues()[0], should.Equal(lseResp.GetZone()))
			expectedState := dutstate.ConvertFromUFSState(lseResp.GetResourceState()).String()
			assert.Loosely(t, labels["ufs_state"], should.NotBeNil)
			assert.Loosely(t, labels["ufs_state"].GetLabelValues(), should.NotBeEmpty)
			assert.Loosely(t, labels["ufs_state"].GetLabelValues()[0], should.Equal(expectedState))
			assert.Loosely(t, labels["dut_state"], should.NotBeNil)
			assert.Loosely(t, labels["dut_state"].GetLabelValues(), should.NotBeEmpty)
			assert.Loosely(t, labels["dut_state"].GetLabelValues()[0], should.Equal(expectedState))
		})
		t.Run("GetMachineLSELabels - attached device lse", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name:         "machine-3",
				SerialNumber: "machine-3-serial",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_BROWSER_GOOGLER_DESK,
				},
			}
			machineResp, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)

			lse1 := &ufspb.MachineLSE{
				Name:     "machinelse-3",
				Machines: []string{"machine-3"},
				Lse: &ufspb.MachineLSE_AttachedDeviceLse{
					AttachedDeviceLse: &ufspb.AttachedDeviceLSE{},
				},
				Realm: "made up realm",
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesCreate, util.BrowserLabAdminRealm)
			lseResp, err := inventory.CreateMachineLSE(ctx, lse1)
			assert.Loosely(t, err, should.BeNil)

			resp, err := GetMachineLSELabels(ctx, lseResp)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetName(), should.Equal(util.AddPrefix(util.MachineLSECollection, lse1.GetName())))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufspb.ResourceType_RESOURCE_TYPE_ATTACHED_DEVICE))
			assert.Loosely(t, resp.GetRealm(), should.Equal(machineResp.GetRealm()))
			labels := resp.GetLabels()
			assert.Loosely(t, labels, should.NotBeEmpty)
		})
		t.Run("GetMachineLSELabels - chromeos lse", func(t *ftt.Test) {
			machine1 := &ufspb.Machine{
				Name:         "machine-4",
				SerialNumber: "machine-4-serial",
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CROS_GOOGLER_DESK,
				},
			}
			machineResp, err := registration.CreateMachine(ctx, machine1)
			assert.Loosely(t, err, should.BeNil)

			lse1 := &ufspb.MachineLSE{
				Name:     "machinelse-4",
				Machines: []string{"machine-4"},
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hostname: "machinelse-4",
										Peripherals: &chromeosLab.Peripherals{
											Servo: &chromeosLab.Servo{},
											Rpm:   &chromeosLab.OSRPM{},
										},
									},
								},
							},
						},
					},
				},
			}
			ctx := initializeFakeAuthDB(ctx, "user:user@example.com", util.InventoriesCreate, util.AtlLabAdminRealm)
			lseResp, err := inventory.CreateMachineLSE(ctx, lse1)
			assert.Loosely(t, err, should.BeNil)

			resp, err := GetMachineLSELabels(ctx, lseResp)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetName(), should.Equal(util.AddPrefix(util.MachineLSECollection, lse1.GetName())))
			assert.Loosely(t, resp.GetResourceType(), should.Equal(ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE))
			assert.Loosely(t, resp.GetRealm(), should.Equal(machineResp.GetRealm()))

			labels := resp.GetLabels()
			assert.Loosely(t, labels, should.NotBeEmpty)
		})
	})
}
