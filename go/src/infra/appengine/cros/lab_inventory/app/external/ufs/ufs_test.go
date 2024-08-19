// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufs

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/chromiumos/infra/proto/go/lab"
	"go.chromium.org/luci/appengine/gaetesting"

	api "infra/appengine/cros/lab_inventory/api/v1"
	"infra/appengine/cros/lab_inventory/app/config"
	"infra/appengine/cros/lab_inventory/app/external"
	"infra/appengine/cros/lab_inventory/app/frontend/fake"
	ufspb "infra/unifiedfleet/api/v1/models"
)

type testFixture struct {
	T *testing.T
	C context.Context
}

func newTestFixtureWithContext(ctx context.Context, t *testing.T) (testFixture, func()) {
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
	Convey("GetUFSDevicesByHostnames", t, func() {
		ctx := testingContext()
		ctx = external.WithTestingContext(ctx)
		ufsClient, _ := GetUFSClient(ctx)
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		Convey("Happy path - 2 passed", func() {
			devices, failedDevices := GetUFSDevicesByHostnames(tf.C, ufsClient, []string{"test-dut", "test-labstation"})
			So(failedDevices, ShouldBeEmpty)
			So(devices, ShouldHaveLength, 2)
			for _, d := range devices {
				var machine *ufspb.Machine
				if d.GetDut() != nil {
					nb, err := proto.Marshal(d.GetDut())
					So(err, ShouldBeNil)
					ob, err := proto.Marshal(fake.GetMockDUT().GetChromeosMachineLse().GetDeviceLse().GetDut())
					So(err, ShouldBeNil)
					So(nb, ShouldResemble, ob)
					machine = fake.GetMockMachineForDUT()
				} else {
					nb, err := proto.Marshal(d.GetLabstation())
					So(err, ShouldBeNil)
					ob, err := proto.Marshal(fake.GetMockLabstation().GetChromeosMachineLse().GetDeviceLse().GetLabstation())
					So(err, ShouldBeNil)
					So(nb, ShouldResemble, ob)
					machine = fake.GetMockMachineForLabstation()
				}
				So(d.GetSerialNumber(), ShouldEqual, machine.GetSerialNumber())
				So(d.GetId().GetValue(), ShouldEqual, machine.GetName())
				So(d.GetDeviceConfigId().GetPlatformId().GetValue(), ShouldEqual, machine.GetChromeosMachine().GetBuildTarget())
				So(d.GetDeviceConfigId().GetModelId().GetValue(), ShouldEqual, machine.GetChromeosMachine().GetModel())
				So(d.GetDeviceConfigId().GetVariantId().GetValue(), ShouldEqual, machine.GetChromeosMachine().GetSku())
				So(d.GetManufacturingId().GetValue(), ShouldEqual, machine.GetChromeosMachine().GetHwid())
			}
		})

		Convey("Get non existing device", func() {
			devices, failedDevices := GetUFSDevicesByHostnames(tf.C, ufsClient, []string{"test-dut", "test-labstation", "ghost"})
			So(failedDevices, ShouldHaveLength, 1)
			So(devices, ShouldHaveLength, 2)
			So(failedDevices[0].ErrorMsg, ShouldContainSubstring, "No MachineLSE found")
		})
	})
}

func TestCopyUFSDutToInvV2Dut(t *testing.T) {
	Convey("Verify CopyUFSDutToInvV2Dut", t, func() {
		Convey("happy path", func() {
			dut := fake.GetMockDUT()
			newDUT := CopyUFSDutToInvV2Dut(dut.GetChromeosMachineLse().GetDeviceLse().GetDut())
			nb, err := proto.Marshal(newDUT)
			So(err, ShouldBeNil)
			ob, err := proto.Marshal(dut.GetChromeosMachineLse().GetDeviceLse().GetDut())
			So(err, ShouldBeNil)
			So(nb, ShouldResemble, ob)
		})
	})
}

func TestCopyUFSLabstationToInvV2Labstation(t *testing.T) {
	Convey("Verify CopyUFSLabstationToInvV2Labstation", t, func() {
		Convey("happy path", func() {
			labstation := fake.GetMockLabstation()
			newL := CopyUFSLabstationToInvV2Labstation(labstation.GetChromeosMachineLse().GetDeviceLse().GetLabstation())
			nb, err := proto.Marshal(newL)
			So(err, ShouldBeNil)
			ob, err := proto.Marshal(labstation.GetChromeosMachineLse().GetDeviceLse().GetLabstation())
			So(err, ShouldBeNil)
			So(nb, ShouldResemble, ob)
		})
	})
}

func mockInvV2DutState(id string) *lab.DutState {
	return &lab.DutState{
		Id: &lab.ChromeOSDeviceID{
			Value: id,
		},
		Servo:                  lab.PeripheralState_WORKING,
		StorageState:           lab.HardwareState_HARDWARE_NORMAL,
		WorkingBluetoothBtpeer: 1,
		Cr50Phase:              lab.DutState_CR50_PHASE_PVT,
	}
}

func mockInvV2DutMeta(id string) *api.DutMeta {
	return &api.DutMeta{
		ChromeosDeviceId: id,
		SerialNumber:     "test-machine-dut-2-serial",
		HwID:             "testdut2hwid",
		DeviceSku:        "testdut2variant",
	}
}

func mockInvV2LabMeta(id string) *api.LabMeta {
	return &api.LabMeta{
		ChromeosDeviceId: id,
		SmartUsbhub:      true,
		ServoType:        "v3",
		ServoTopology: &lab.ServoTopology{
			Main: &lab.ServoTopologyItem{
				Type: "v3",
			},
		},
	}
}

func mockIV2ChromeOSDeviceDUT(assetTag, hostname, model, board, variant, servohost, servoserial string, servoport int32) *lab.ChromeOSDevice {
	return &lab.ChromeOSDevice{
		Id: &lab.ChromeOSDeviceID{
			Value: assetTag,
		},
		DeviceConfigId: &device.ConfigId{
			ModelId: &device.ModelId{
				Value: model,
			},
			PlatformId: &device.PlatformId{
				Value: board,
			},
			VariantId: &device.VariantId{
				Value: variant,
			},
		},
		Device: &lab.ChromeOSDevice_Dut{
			Dut: &lab.DeviceUnderTest{
				Hostname: hostname,
				Peripherals: &lab.Peripherals{
					Servo: &lab.Servo{
						ServoHostname: servohost,
						ServoPort:     servoport,
						ServoSerial:   servoserial,
					},
				},
			},
		},
	}
}

func mockIV2ChromeOSDeviceLabstation(assetTag, hostname, model, board, variant string) *lab.ChromeOSDevice {
	return &lab.ChromeOSDevice{
		Id: &lab.ChromeOSDeviceID{
			Value: assetTag,
		},
		DeviceConfigId: &device.ConfigId{
			ModelId: &device.ModelId{
				Value: model,
			},
			PlatformId: &device.PlatformId{
				Value: board,
			},
			VariantId: &device.VariantId{
				Value: variant,
			},
		},
		Device: &lab.ChromeOSDevice_Labstation{
			Labstation: &lab.Labstation{
				Hostname: hostname,
			},
		},
	}
}
