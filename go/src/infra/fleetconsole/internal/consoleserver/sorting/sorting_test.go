// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sorting

import (
	"testing"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/testutils"
)

func TestSortDescriptor(t *testing.T) {
	t.Parallel()

	t.Run(`SortByIDAscending_ReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("2").Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("1").Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "dut_id")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DutId, should.Equal("1"))
	})

	t.Run(`SortByDutIDAscending_ReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithID("2").Build()
		d2 := testutils.NewDeviceBuilder().WithID("1").Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "id")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].Id, should.Equal("1"))
	})

	t.Run(`SortByHostnameAscending_ReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").WithHostname("2").Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").WithHostname("1").Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "address.host")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DutId, should.Equal("2"))
	})

	t.Run(`SortByPortAscending_ReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").WithPort(9999).Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").WithPort(1001).Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "address.port")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DutId, should.Equal("2"))
	})

	t.Run(`SortByStateAscending_ReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").WithState(fleetconsolerpc.DeviceState_DEVICE_STATE_AVAILABLE).Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").WithState(fleetconsolerpc.DeviceState_DEVICE_STATE_UNSPECIFIED).Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "state")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DutId, should.Equal("2"))
	})

	t.Run(`SortByTypeAscending_ReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").WithType(fleetconsolerpc.DeviceType_DEVICE_TYPE_VIRTUAL).Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").WithType(fleetconsolerpc.DeviceType_DEVICE_TYPE_UNSPECIFIED).Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "type")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DutId, should.Equal("2"))
	})

	t.Run(`SortByDutIDDescending_ReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "dut_id desc")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DutId, should.Equal("2"))
	})

	t.Run(`SortWithNonexistentField_ReturnsError`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "nonexistent")

		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, devices, should.BeNil)
	})

	t.Run(`SortWithWhitespaceInFieldName_IgnoresWhitespaceAndReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, " \ndut_id  \t\t  ")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DutId, should.Equal("1"))
	})

	t.Run(`SortWithWhitespaceInDescendingModifier_IgnoresWhitespaceAndReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, " \ndut_id  \t\t  desc \t")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DutId, should.Equal("2"))
	})

	t.Run(`SortWithNonexistentNestedField_ReturnsError`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "nonexistent.abcd")

		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, devices, should.BeNil)
	})

	t.Run(`SortWithNonexistentLabel_IgnoresFieldAndReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "labels.abcd")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DutId, should.Equal("1"))
	})

	t.Run(`SortByLabelAscending_ReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithLabel("label1", []string{"2"}).Build()
		d2 := testutils.NewDeviceBuilder().WithLabel("label1", []string{"1"}).Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "labels.label1")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DeviceSpec.Labels["label1"].Values[0], should.Equal("1"))
	})

	t.Run(`SortByLabelDescending_ReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithLabel("label1", []string{"1"}).Build()
		d2 := testutils.NewDeviceBuilder().WithLabel("label1", []string{"2"}).Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "labels.label1 desc")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DeviceSpec.Labels["label1"].Values[0], should.Equal("2"))
	})

	t.Run(`SortWithNonexistentLabel_IgnoresFieldAndReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").WithLabel("label1", []string{"1"}).Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2}, "labels.label1")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DutId, should.Equal("1"))
	})

	t.Run(`SortByMultipleFields_ReturnsSortedDevices`, func(t *testing.T) {
		d1 := testutils.NewDeviceBuilder().WithDutID("1").Build()
		d2 := testutils.NewDeviceBuilder().WithDutID("2").WithLabel("label1", []string{"1"}).Build()
		d3 := testutils.NewDeviceBuilder().WithDutID("3").WithLabel("label1", []string{"1"}).Build()

		devices, err := SortDevices([]*fleetconsolerpc.Device{d1, d2, d3}, "labels.label1, dut_id desc")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devices[0].DutId, should.Equal("1"))
		assert.Loosely(t, devices[1].DutId, should.Equal("3"))
		assert.Loosely(t, devices[2].DutId, should.Equal("2"))
	})
}
