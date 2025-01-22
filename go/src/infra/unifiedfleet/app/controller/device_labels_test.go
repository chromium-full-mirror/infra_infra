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

	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/external"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/history"
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/model/registration"
	"infra/unifiedfleet/app/util"
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
			for i := 0; i < 4; i++ {
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
