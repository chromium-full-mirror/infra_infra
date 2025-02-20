// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
)

func mockDeviceLabels(id string) *ufspb.DeviceLabels {
	return &ufspb.DeviceLabels{
		Name: id,
	}
}

func TestBatchUpdateDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	deviceLabels1 := mockDeviceLabels("deviceLabels-1")
	deviceLabels2 := mockDeviceLabels("deviceLabels-2")
	deviceLabels3 := mockDeviceLabels("")
	ftt.Run("Batch Update DeviceLabels", t, func(t *ftt.Test) {
		t.Run("BatchUpdate all deviceLabels", func(t *ftt.Test) {
			resp, err := BatchUpdateDeviceLabels(ctx, []*ufspb.DeviceLabels{deviceLabels1, deviceLabels2})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.DeviceLabels{deviceLabels1, deviceLabels2}))
		})
		t.Run("BatchUpdate existing deviceLabels", func(t *ftt.Test) {
			deviceLabels2.ResourceType = ufspb.ResourceType_RESOURCE_TYPE_SCHEDULING_UNIT
			_, err := BatchUpdateDeviceLabels(ctx, []*ufspb.DeviceLabels{deviceLabels1, deviceLabels2})
			assert.Loosely(t, err, should.BeNil)
			deviceLabels, err := GetDeviceLabels(ctx, "deviceLabels-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, deviceLabels.GetResourceType(), should.Equal(ufspb.ResourceType_RESOURCE_TYPE_SCHEDULING_UNIT))
		})
		t.Run("BatchUpdate invalid deviceLabels", func(t *ftt.Test) {
			resp, err := BatchUpdateDeviceLabels(ctx, []*ufspb.DeviceLabels{deviceLabels1, deviceLabels2, deviceLabels3})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ftt.Run("GetDeviceLabels", t, func(t *ftt.Test) {
		t.Run("Get DeviceLabels by non-existing ID", func(t *ftt.Test) {
			resp, err := GetDeviceLabels(ctx, "empty")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get DeviceLabels - invalid ID", func(t *ftt.Test) {
			resp, err := GetDeviceLabels(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	deviceLabels1 := &ufspb.DeviceLabels{
		Name:         "deviceLabels-1",
		ResourceType: ufspb.ResourceType_RESOURCE_TYPE_SCHEDULING_UNIT,
	}
	deviceLabels2 := &ufspb.DeviceLabels{
		Name:         "deviceLabels-2",
		ResourceType: ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE,
	}
	deviceLabels3 := &ufspb.DeviceLabels{
		Name:         "deviceLabels-3",
		ResourceType: ufspb.ResourceType_RESOURCE_TYPE_ATTACHED_DEVICE,
	}
	deviceLabels4 := mockDeviceLabels("deviceLabels-4")
	deviceLabels := []*ufspb.DeviceLabels{deviceLabels1, deviceLabels2, deviceLabels3, deviceLabels4}

	ftt.Run("ListDeviceLabels", t, func(t *ftt.Test) {
		_, err := BatchUpdateDeviceLabels(ctx, deviceLabels)
		assert.Loosely(t, err, should.BeNil)
		t.Run("List deviceLabels - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDeviceLabels(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List deviceLabels - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDeviceLabels(ctx, 4, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, resp, should.Match(deviceLabels))
		})

		t.Run("List deviceLabels - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDeviceLabels(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(deviceLabels[:3]))

			resp, _, err = ListDeviceLabels(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(deviceLabels[3:]))
		})
	})
	ftt.Run("ListDeviceLabels with Filters", t, func(t *ftt.Test) {
		_, err := BatchUpdateDeviceLabels(ctx, deviceLabels)
		assert.Loosely(t, err, should.BeNil)
		filterMap := make(map[string][]interface{})
		t.Run("List deviceLabels - Filter by state", func(t *ftt.Test) {
			filterMap["resource_type"] = []interface{}{ufspb.ResourceType_RESOURCE_TYPE_SCHEDULING_UNIT.String()}
			resp, nextPageToken, err := ListDeviceLabels(ctx, 2, "", filterMap, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.DeviceLabels{deviceLabels1}))
		})
	})
}

func TestDeleteDeviceLabels(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	deviceLabels1 := mockDeviceLabels("deviceLabels-delete1")
	ftt.Run("DeleteDeviceLabels", t, func(t *ftt.Test) {
		t.Run("Delete DeviceLabels by existing ID", func(t *ftt.Test) {
			_, err := BatchUpdateDeviceLabels(ctx, []*ufspb.DeviceLabels{deviceLabels1})
			assert.Loosely(t, err, should.BeNil)
			DeleteDeviceLabelsList(ctx, []string{"deviceLabels-delete1"})
			deviceLabels, err := GetDeviceLabels(ctx, "deviceLabels-delete1")
			assert.Loosely(t, deviceLabels, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete deviceLabels by non-existing ID", func(t *ftt.Test) {
			res := DeleteDeviceLabelsList(ctx, []string{"deviceLabels-delete2"})
			assert.Loosely(t, res.Failed(), should.HaveLength(1))
		})
		t.Run("Delete machineLSE - invalid ID", func(t *ftt.Test) {
			res := DeleteDeviceLabelsList(ctx, []string{""})
			assert.Loosely(t, res.Failed(), should.HaveLength(1))
		})
	})
}

func TestQueryDeviceLabelsByPropertyName(t *testing.T) {
	t.Parallel()
	ftt.Run("QueryDeviceLabelsByPropertyName", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		deviceLabels1 := mockDeviceLabels("deviceLabels-queryByProperty1")
		deviceLabels1.ResourceType = ufspb.ResourceType_RESOURCE_TYPE_BROWSER_DEVICE
		_, err := BatchUpdateDeviceLabels(ctx, []*ufspb.DeviceLabels{deviceLabels1})
		assert.Loosely(t, err, should.BeNil)

		t.Run("Query By existing resource type", func(t *ftt.Test) {
			resp, err := QueryDeviceLabelsByPropertyName(ctx, "resource_type", ufspb.ResourceType_RESOURCE_TYPE_BROWSER_DEVICE.String(), false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0], should.Match(deviceLabels1))
		})
		t.Run("Query By non-existing resource type", func(t *ftt.Test) {
			resp, err := QueryDeviceLabelsByPropertyName(ctx, "resource_type", "999", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}
