// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filtering

import (
	"testing"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/testutils"
)

func TestFiltering(t *testing.T) {
	t.Parallel()

	t.Run(`FilterByDutID`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "dut_id = 2")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(1))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("2"))
	})

	t.Run(`FilterWithoutWhitespace`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "dut_id=2")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(1))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("2"))
	})

	t.Run(`FilterLessThan`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "dut_id < 2")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(1))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("1"))
	})

	t.Run(`FilterGreaterThan`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "dut_id > 2")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(1))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("3"))
	})

	t.Run(`FilterWithOrCondition`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "dut_id = 2 OR dut_id = 3")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(2))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("2"))
		assert.Loosely(t, filteredDevices[1].DutId, should.Equal("3"))
	})

	t.Run(`FilterWithParenthesisedOrCondition`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "dut_id = (2 OR 3)")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(2))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("2"))
		assert.Loosely(t, filteredDevices[1].DutId, should.Equal("3"))
	})

	t.Run(`SubFieldsAreCorrectlyFiltered`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").WithHostname("host1").WithPort(1001).Build(),
			testutils.NewDeviceBuilder().WithDutID("2").WithHostname("host1").WithPort(1001).Build(),
			testutils.NewDeviceBuilder().WithDutID("3").WithHostname("host2").WithPort(1001).Build(),
			testutils.NewDeviceBuilder().WithDutID("4").WithHostname("host1").WithPort(2002).Build(),
		}
		filteredDevices, err := FilterDevices(devices, "address.host = host1 AND address.port = 1001")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(2))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("1"))
		assert.Loosely(t, filteredDevices[1].DutId, should.Equal("2"))
	})

	t.Run(`FilterWithOrConditionOnTwoFields`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").WithHostname("host1").WithPort(1001).Build(),
			testutils.NewDeviceBuilder().WithDutID("2").WithHostname("host2").WithPort(1001).Build(),
			testutils.NewDeviceBuilder().WithDutID("3").WithHostname("host1").WithPort(2002).Build(),
			testutils.NewDeviceBuilder().WithDutID("4").WithHostname("host2").WithPort(2002).Build(),
		}
		filteredDevices, err := FilterDevices(devices, "address.host = host1 OR address.port = 1001")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(3))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("1"))
		assert.Loosely(t, filteredDevices[1].DutId, should.Equal("2"))
		assert.Loosely(t, filteredDevices[2].DutId, should.Equal("3"))
	})

	t.Run(`FieldCanBeComparedToOtherFieldOrValue`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").WithID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").WithID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").WithID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("4").WithID("1").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "dut_id = (id OR 3)")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(2))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("1"))
		assert.Loosely(t, filteredDevices[1].DutId, should.Equal("3"))
	})

	t.Run(`OrConditionTakesPrecedenceOverAnd`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "dut_id = 1 AND dut_id = 2 OR dut_id = 3") // a AND b OR c is converted to a AND (b OR c)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.BeEmpty)
	})

	t.Run(`ParenthesisTakesPrecedenceOverOr`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "(dut_id = 1 AND dut_id = 2) OR dut_id = 3")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(1))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("3"))
	})

	t.Run(`WhiteSpaceIsIgnored`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, " \t ( \n dut_id \n =   \t 1   AND  \t dut_id  \r\n  =   2  \r\n   )   OR \t  dut_id  \t  =  \n 3   \n \t ")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(1))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("3"))
	})

	t.Run(`FilterWithEmptyString_ReturnsAllDevices`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "         ")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(3))
	})

	t.Run(`NonexistentFieldIsTreatedAsLiteralAndIsNotMatched`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "nonexistent = 5")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.BeEmpty)
	})

	t.Run(`NonexistentSubField_ReturnsError`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		_, err := FilterDevices(devices, "nonexistent.nonexistent = 5")

		assert.Loosely(t, err, should.NotBeNil)
	})

	t.Run(`NonexistentFieldIsTreatedAsLiteralAndIsMatched`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "nonexistent = nonexistent")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(3))
	})

	// this is a valid syntax according to EBNF grammar for AIP-160, but it makes no sense and cannot be handled
	t.Run(`FilterWithNestedRestriction_ReturnsError`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").Build(),
			testutils.NewDeviceBuilder().WithDutID("2").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		_, err := FilterDevices(devices, "dut_id = (id = 3)")

		assert.Loosely(t, err, should.NotBeNil)
	})

	t.Run(`FilterWithSingleValueIsMatchedByArraysContainingTheValue`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").WithLabel("connectivity", []string{"wifi", "bluetooth"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("2").WithLabel("connectivity", []string{"wifi"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "labels.connectivity = wifi")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(2))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("1"))
		assert.Loosely(t, filteredDevices[1].DutId, should.Equal("2"))
	})

	t.Run(`MatchingArrayLabelToItself_ReturnsError`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").WithLabel("connectivity", []string{"wifi", "bluetooth"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("2").WithLabel("connectivity", []string{"wifi"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		_, err := FilterDevices(devices, "labels.connectivity = labels.connectivity")

		assert.Loosely(t, err, should.NotBeNil)
	})

	t.Run(`MatchingLabelToBaseDimension`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").WithHostname("192.168.0.1").WithLabel("rpm-host", []string{"192.168.0.1"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("2").WithHostname("192.168.0.1").Build(),
			testutils.NewDeviceBuilder().WithDutID("3").WithLabel("rpm-host", []string{"192.168.0.1"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("4").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "labels.rpm-host = address.host")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(1))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("1"))
	})

	t.Run(`MatchingTwoLabelsToEachOther`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").WithLabel("gateway-host", []string{"192.168.0.1"}).WithLabel("rpm-host", []string{"192.168.0.1"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("2").WithLabel("gateway-host", []string{"192.168.0.1"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("3").WithLabel("rpm-host", []string{"192.168.0.1"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("4").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "labels.rpm-host = labels.gateway-host")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(2))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("1"))
		assert.Loosely(t, filteredDevices[1].DutId, should.Equal("4"))
	})

	t.Run(`ArrayRightHandSideOperandIsNotSupported`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").WithLabel("gateway-host", []string{"192.168.0.1", "192.168.0.2"}).WithLabel("rpm-host", []string{"192.168.0.1"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("2").WithLabel("gateway-host", []string{"192.168.0.1"}).Build(),
		}
		_, err := FilterDevices(devices, "labels.rpm-host = labels.gateway-host")

		assert.Loosely(t, err, should.NotBeNil)
	})

	t.Run(`GreaterThanOperatorOnArray_ReturnsArraysWithAtLeastOneMatch`, func(t *testing.T) {
		devices := []*fleetconsolerpc.Device{
			testutils.NewDeviceBuilder().WithDutID("1").WithLabel("ssd-disks", []string{"256", "512"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("2").WithLabel("ssd-disks", []string{"256"}).Build(),
			testutils.NewDeviceBuilder().WithDutID("3").Build(),
		}
		filteredDevices, err := FilterDevices(devices, "labels.ssd-disks > 256")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filteredDevices, should.HaveLength(1))
		assert.Loosely(t, filteredDevices[0].DutId, should.Equal("1"))
	})
}
