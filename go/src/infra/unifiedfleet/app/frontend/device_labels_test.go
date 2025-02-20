// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func TestGetDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("GetDeviceLabels", t, func(t *ftt.Test) {
		t.Run("Get DeviceLabels by existing ID", func(t *ftt.Test) {
			createResp, err := inventory.BatchUpdateDeviceLabels(ctx, []*ufspb.DeviceLabels{
				{
					Name:         util.AddPrefix(util.VMCollection, "devicelabels-1"),
					ResourceType: ufspb.ResourceType_RESOURCE_TYPE_BROWSER_DEVICE,
				},
			})
			assert.Loosely(t, err, should.BeNil)
			deviceLabels1 := createResp[0]
			deviceLabels1.Name = util.AddPrefix(util.DeviceLabelsCollection, deviceLabels1.Name)

			req := &ufsAPI.GetDeviceLabelsRequest{
				Hostname: util.AddPrefix(util.DeviceLabelsCollection, "vms/devicelabels-1"),
			}
			resp, err := tf.Fleet.GetDeviceLabels(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(deviceLabels1))
		})
		t.Run("Get DeviceLabels by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetDeviceLabelsRequest{
				Hostname: util.AddPrefix(util.DeviceLabelsCollection, "vms/devicelabels-2"),
			}
			resp, err := tf.Fleet.GetDeviceLabels(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})
		t.Run("Get DeviceLabels - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetDeviceLabelsRequest{
				Hostname: "",
			}
			resp, err := tf.Fleet.GetDeviceLabels(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
	})
}

func TestListDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
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
		t.Run("List DeviceLabels - page_size negative - error", func(t *ftt.Test) {
			resp, err := tf.Fleet.ListDeviceLabels(tf.C, &ufsAPI.ListDeviceLabelsRequest{
				PageSize: -5,
			})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("List DeviceLabels - invalid filter - error", func(t *ftt.Test) {
			resp, err := tf.Fleet.ListDeviceLabels(tf.C, &ufsAPI.ListDeviceLabelsRequest{
				Filter: "fruit=orange",
			})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("List DeviceLabels - happy path", func(t *ftt.Test) {
			resp, err := tf.Fleet.ListDeviceLabels(tf.C, &ufsAPI.ListDeviceLabelsRequest{
				Filter:   "resourcetype=RESOURCE_TYPE_CHROMEOS_DEVICE",
				PageSize: 5,
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetLabels(), should.HaveLength(2))
			assert.Loosely(t, ufsAPI.ParseResources(resp.GetLabels(), "Name"), should.Match([]string{"devicelabels/machineLSEs/devicelabels-list-1", "devicelabels/machineLSEs/devicelabels-list-2"}))
		})
	})
}
