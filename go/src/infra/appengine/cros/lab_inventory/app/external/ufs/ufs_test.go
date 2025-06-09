// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufs

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/appengine/cros/lab_inventory/app/config"
	"go.chromium.org/infra/appengine/cros/lab_inventory/app/external"
	"go.chromium.org/infra/appengine/cros/lab_inventory/app/frontend/fake"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

type testFixture struct {
	T testing.TB
	C context.Context
}

func newTestFixtureWithContext(ctx context.Context, t testing.TB) (testFixture, func()) {
	tf := testFixture{T: t, C: ctx}
	mc := gomock.NewController(t)
	validate := func() {
		mc.Finish()
	}
	return tf, validate
}

func testingContext() context.Context {
	c := gaetesting.TestingContextWithAppID("dev~infra-lab-inventory")
	c = config.Use(c, &config.Config{
		Readers: &config.LuciAuthGroup{
			Value: "fake_group",
		},
	})
	return c
}

func TestGetUFSDevicesByHostnames(t *testing.T) {
	// t.Parallel()
	ftt.Run("GetUFSDevicesByHostnames", t, func(t *ftt.Test) {
		ctx := testingContext()
		ctx = external.WithTestingContext(ctx)
		ufsClient, _ := GetUFSClient(ctx)
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		t.Run("Happy path - 2 passed", func(t *ftt.Test) {
			devices, failedDevices := GetUFSDevicesByHostnames(tf.C, ufsClient, []string{"test-dut", "test-labstation"})
			assert.Loosely(t, failedDevices, should.BeEmpty)
			assert.Loosely(t, devices, should.HaveLength(2))
			for _, d := range devices {
				var machine *ufspb.Machine
				if d.GetDut() != nil {
					nb, err := proto.Marshal(d.GetDut())
					assert.Loosely(t, err, should.BeNil)
					ob, err := proto.Marshal(fake.GetMockDUT().GetChromeosMachineLse().GetDeviceLse().GetDut())
					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, nb, should.Resemble(ob))
					machine = fake.GetMockMachineForDUT()
				} else {
					nb, err := proto.Marshal(d.GetLabstation())
					assert.Loosely(t, err, should.BeNil)
					ob, err := proto.Marshal(fake.GetMockLabstation().GetChromeosMachineLse().GetDeviceLse().GetLabstation())
					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, nb, should.Resemble(ob))
					machine = fake.GetMockMachineForLabstation()
				}
				assert.Loosely(t, d.GetSerialNumber(), should.Equal(machine.GetSerialNumber()))
				assert.Loosely(t, d.GetId().GetValue(), should.Equal(machine.GetName()))
				assert.Loosely(t, d.GetDeviceConfigId().GetPlatformId().GetValue(), should.Equal(machine.GetChromeosMachine().GetBuildTarget()))
				assert.Loosely(t, d.GetDeviceConfigId().GetModelId().GetValue(), should.Equal(machine.GetChromeosMachine().GetModel()))
				assert.Loosely(t, d.GetDeviceConfigId().GetVariantId().GetValue(), should.Equal(machine.GetChromeosMachine().GetSku()))
				assert.Loosely(t, d.GetManufacturingId().GetValue(), should.Equal(machine.GetChromeosMachine().GetHwid()))
			}
		})

		t.Run("Get non existing device", func(t *ftt.Test) {
			devices, failedDevices := GetUFSDevicesByHostnames(tf.C, ufsClient, []string{"test-dut", "test-labstation", "ghost"})
			assert.Loosely(t, failedDevices, should.HaveLength(1))
			assert.Loosely(t, devices, should.HaveLength(2))
			assert.Loosely(t, failedDevices[0].ErrorMsg, should.ContainSubstring("No MachineLSE found"))
		})
	})
}

func TestCopyUFSDutToInvV2Dut(t *testing.T) {
	ftt.Run("Verify CopyUFSDutToInvV2Dut", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			dut := fake.GetMockDUT()
			newDUT := CopyUFSDutToInvV2Dut(dut.GetChromeosMachineLse().GetDeviceLse().GetDut())
			nb, err := proto.Marshal(newDUT)
			assert.Loosely(t, err, should.BeNil)
			ob, err := proto.Marshal(dut.GetChromeosMachineLse().GetDeviceLse().GetDut())
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nb, should.Resemble(ob))
		})
	})
}

func TestCopyUFSLabstationToInvV2Labstation(t *testing.T) {
	ftt.Run("Verify CopyUFSLabstationToInvV2Labstation", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			labstation := fake.GetMockLabstation()
			newL := CopyUFSLabstationToInvV2Labstation(labstation.GetChromeosMachineLse().GetDeviceLse().GetLabstation())
			nb, err := proto.Marshal(newL)
			assert.Loosely(t, err, should.BeNil)
			ob, err := proto.Marshal(labstation.GetChromeosMachineLse().GetDeviceLse().GetLabstation())
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nb, should.Resemble(ob))
		})
	})
}
