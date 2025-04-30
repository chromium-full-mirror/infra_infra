// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"

	"go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/chromiumos/infra/proto/go/lab"
	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"

	api "go.chromium.org/infra/appengine/cros/lab_inventory/api/v1"
	"go.chromium.org/infra/appengine/cros/lab_inventory/app/config"
	"go.chromium.org/infra/cros/lab_inventory/deviceconfig"
)

type testFixture struct {
	T testing.TB
	C context.Context

	Inventory          *InventoryServerImpl
	DecoratedInventory *api.DecoratedInventory
}

func newTestFixtureWithContext(ctx context.Context, t testing.TB) (testFixture, func()) {
	tf := testFixture{T: t, C: ctx}
	mc := gomock.NewController(t)

	tf.Inventory = &InventoryServerImpl{}
	tf.DecoratedInventory = &api.DecoratedInventory{
		Service: tf.Inventory,
		Prelude: checkAccess,
	}

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

type devcfgEntity struct {
	_kind     string `gae:"$kind,DevConfig"`
	ID        string `gae:"$id"`
	DevConfig []byte `gae:",noindex"`
	Updated   time.Time
}

func TestDeviceConfigsExists(t *testing.T) {
	t.Parallel()

	ftt.Run("Test exists device config in datastore", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		err := ds.Put(ctx, []devcfgEntity{
			{ID: "kunimitsu.lars.variant1"},
			{ID: "sarien.arcada.variant2"},
			{
				ID:        "platform.model.variant3",
				DevConfig: []byte("bad data"),
			},
		})
		assert.Loosely(t, err, should.BeNil)

		t.Run("Happy path", func(t *ftt.Test) {
			resp, err := tf.Inventory.DeviceConfigsExists(ctx, &api.DeviceConfigsExistsRequest{
				ConfigIds: []*device.ConfigId{
					{
						PlatformId: &device.PlatformId{Value: "lars"},
						ModelId:    &device.ModelId{Value: "lars"},
						VariantId:  &device.VariantId{Value: "variant1"},
					},
					{
						PlatformId: &device.PlatformId{Value: "arcada"},
						ModelId:    &device.ModelId{Value: "arcada"},
						VariantId:  &device.VariantId{Value: "variant2"},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Exists[0], should.BeTrue)
			assert.Loosely(t, resp.Exists[1], should.BeTrue)
		})

		t.Run("check for nonexisting data", func(t *ftt.Test) {
			resp, err := tf.Inventory.DeviceConfigsExists(ctx, &api.DeviceConfigsExistsRequest{
				ConfigIds: []*device.ConfigId{
					{
						PlatformId: &device.PlatformId{Value: "platform"},
						ModelId:    &device.ModelId{Value: "model"},
						VariantId:  &device.VariantId{Value: "variant-nonexisting"},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Exists[0], should.BeFalse)
		})

		t.Run("check for existing and nonexisting data", func(t *ftt.Test) {
			resp, err := tf.Inventory.DeviceConfigsExists(ctx, &api.DeviceConfigsExistsRequest{
				ConfigIds: []*device.ConfigId{
					{
						PlatformId: &device.PlatformId{Value: "platform"},
						ModelId:    &device.ModelId{Value: "model"},
						VariantId:  &device.VariantId{Value: "variant-nonexisting"},
					},
					{
						PlatformId: &device.PlatformId{Value: "arcada"},
						ModelId:    &device.ModelId{Value: "arcada"},
						VariantId:  &device.VariantId{Value: "variant2"},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Exists[0], should.BeFalse)
			assert.Loosely(t, resp.Exists[1], should.BeTrue)
		})
	})
}

func mockServo(servoHost string) *lab.Servo {
	return &lab.Servo{
		ServoHostname: servoHost,
		ServoPort:     8888,
		ServoSerial:   "SERVO1",
		ServoType:     "v3",
	}
}

func mockDut(hostname, id, servoHost string) *lab.ChromeOSDevice {
	return &lab.ChromeOSDevice{
		Id: &lab.ChromeOSDeviceID{
			Value: id,
		},
		Device: &lab.ChromeOSDevice_Dut{
			Dut: &lab.DeviceUnderTest{
				Hostname: hostname,
				Peripherals: &lab.Peripherals{
					Servo:       mockServo(servoHost),
					SmartUsbhub: false,
				},
			},
		},
	}
}

func mockLabstation(hostname, id string) *lab.ChromeOSDevice {
	return &lab.ChromeOSDevice{
		Id: &lab.ChromeOSDeviceID{
			Value: id,
		},
		Device: &lab.ChromeOSDevice_Labstation{
			Labstation: &lab.Labstation{
				Hostname: hostname,
			},
		},
	}
}

func mockDevCfg(board string, model string, variant string) *device.Config {
	return &device.Config{
		Id: &device.ConfigId{
			PlatformId: &device.PlatformId{Value: board},
			ModelId:    &device.ModelId{Value: model},
			VariantId:  &device.VariantId{Value: variant},
		},
	}
}

func mockDevCfgEntity(devCfg *device.Config) (*devcfgEntity, error) {
	cfgBytes, err := proto.Marshal(devCfg)
	if err != nil {
		return nil, err
	}

	return &devcfgEntity{
		ID:        deviceconfig.GetDeviceConfigIDStr(devCfg.GetId()),
		DevConfig: cfgBytes,
	}, nil

}

func TestListDeviceConfigs(t *testing.T) {
	t.Parallel()

	ftt.Run("When device configs exist in datastore", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContext()
		ds.GetTestable(ctx).Consistent(true)
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()

		devCfg1 := mockDevCfg("board1", "model1", "variant1")
		cfgEntity1, err := mockDevCfgEntity(devCfg1)
		assert.Loosely(t, err, should.BeNil)

		devCfg2 := mockDevCfg("board2", "model2", "variant2")
		cfgEntity2, err := mockDevCfgEntity(devCfg2)
		assert.Loosely(t, err, should.BeNil)

		err = ds.Put(ctx, []devcfgEntity{
			*cfgEntity1,
			*cfgEntity2,
		})
		assert.Loosely(t, err, should.BeNil)

		t.Run("ListDeviceConfigs should return all configs", func(t *ftt.Test) {
			expected := &api.ListDeviceConfigsResponse{
				DeviceConfigs: []*device.Config{devCfg1, devCfg2},
			}
			resp2, err := tf.Inventory.ListDeviceConfigs(ctx, &api.ListDeviceConfigsRequest{})
			assert.Loosely(t, err, should.BeNil)
			if resp2 != nil {
				sort.Slice(resp2.DeviceConfigs, func(i int, j int) bool {
					// Id field is unique for device configs in real life, so we can use it to sort.
					return resp2.DeviceConfigs[i].GetId().String() < resp2.DeviceConfigs[j].GetId().String()
				})
			}
			assert.Loosely(t, resp2, should.Resemble(expected))
		})
	})
}
