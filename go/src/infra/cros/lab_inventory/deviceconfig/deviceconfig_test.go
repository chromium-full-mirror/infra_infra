// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package deviceconfig

import (
	"context"
	"os"
	"testing"

	"github.com/golang/mock/gomock"

	"go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/proto/gitiles"
	"go.chromium.org/luci/common/proto/gitiles/mock_gitiles"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"
)

var deviceConfigJSON = `
{
	"configs": [
		{
			"unkonwnField": "hahaha",
			"id": {
				"platformId": {
					"value": "Arcada"
				},
				"modelId": {
					"value": "arcada"
				},
				"variantId": {}
			},
			"hardwareFeatures": [
				"HARDWARE_FEATURE_BLUETOOTH",
				"HARDWARE_FEATURE_TOUCHSCREEN"
			],
			"power": "POWER_SUPPLY_BATTERY",
			"storage": "STORAGE_NVME",
			"videoAccelerationSupports": [
				"VIDEO_ACCELERATION_H264",
				"VIDEO_ACCELERATION_ENC_MJPG"
			],
			"soc": "SOC_WHISKEY_LAKE_U"
		},
		{
			"id": {
				"platformId": {
					"value": "Arcada"
				},
				"modelId": {
					"value": "arcada"
				},
				"variantId": {
					"value": "2"
				}
			},
			"hardwareFeatures": [
				"HARDWARE_FEATURE_TOUCHPAD",
				"HARDWARE_FEATURE_TOUCHSCREEN"
			],
			"power": "POWER_SUPPLY_BATTERY",
			"storage": "STORAGE_NVME",
			"videoAccelerationSupports": [
				"VIDEO_ACCELERATION_MJPG",
				"VIDEO_ACCELERATION_ENC_MJPG"
			],
			"soc": "SOC_WHISKEY_LAKE_U"
		}
	]
}
`

func TestUpdateDatastore(t *testing.T) {
	ftt.Run("Test update device config cache", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		ctl := gomock.NewController(t)
		defer ctl.Finish()

		gitilesMock := mock_gitiles.NewMockGitilesClient(ctl)
		gitilesMock.EXPECT().DownloadFile(gomock.Any(), gomock.Any()).Return(
			&gitiles.DownloadFileResponse{
				Contents: deviceConfigJSON,
			},
			nil,
		)

		err := UpdateDatastore(ctx, gitilesMock, "", "", "")
		assert.Loosely(t, err, should.BeNil)
		// There should be 2 entities created in datastore.
		var cfgs []*devcfgEntity
		datastore.GetTestable(ctx).Consistent(true)
		err = datastore.GetAll(ctx, datastore.NewQuery(entityKind), &cfgs)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cfgs, should.HaveLength(2))
	})
}

func TestGetCachedDeviceConfig(t *testing.T) {
	ctx := gaetesting.TestingContextWithAppID("go-test")

	ftt.Run("Test get device config from datastore", t, func(t *ftt.Test) {
		err := datastore.Put(ctx, []devcfgEntity{
			{ID: "platform.model.variant1"},
			{ID: "platform.model.variant2"},
			{
				ID:        "platform.model.variant3",
				DevConfig: []byte("bad data"),
			},
		})
		assert.Loosely(t, err, should.BeNil)

		t.Run("Happy path", func(t *ftt.Test) {
			devcfg, err := GetCachedConfig(ctx, []*device.ConfigId{
				{
					PlatformId: &device.PlatformId{Value: "platform"},
					ModelId:    &device.ModelId{Value: "model"},
					VariantId:  &device.VariantId{Value: "variant1"},
					BrandId:    &device.BrandId{Value: "brand1"},
				},
				{
					PlatformId: &device.PlatformId{Value: "platform"},
					ModelId:    &device.ModelId{Value: "model"},
					VariantId:  &device.VariantId{Value: "variant2"},
					BrandId:    &device.BrandId{Value: "brand2"},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, devcfg, should.HaveLength(2))
		})

		t.Run("Device id is case insensitive", func(t *ftt.Test) {
			devcfg, err := GetCachedConfig(ctx, []*device.ConfigId{
				{
					PlatformId: &device.PlatformId{Value: "PLATFORM"},
					ModelId:    &device.ModelId{Value: "model"},
					VariantId:  &device.VariantId{Value: "variant1"},
					BrandId:    &device.BrandId{Value: "brand1"},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, devcfg, should.HaveLength(1))
		})

		t.Run("Data unmarshal error", func(t *ftt.Test) {
			_, err := GetCachedConfig(ctx, []*device.ConfigId{
				{
					PlatformId: &device.PlatformId{Value: "platform"},
					ModelId:    &device.ModelId{Value: "model"},
					VariantId:  &device.VariantId{Value: "variant3"},
					BrandId:    &device.BrandId{Value: "brand3"},
				},
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("unmarshal config data"))
		})

		t.Run("Get nonexisting data", func(t *ftt.Test) {
			resp, err := GetCachedConfig(ctx, []*device.ConfigId{
				{
					PlatformId: &device.PlatformId{Value: "platform"},
					ModelId:    &device.ModelId{Value: "model"},
					VariantId:  &device.VariantId{Value: "variant-nonexisting"},
					BrandId:    &device.BrandId{Value: "nonexisting"},
				},
				{
					PlatformId: &device.PlatformId{Value: "platform"},
					ModelId:    &device.ModelId{Value: "model"},
					VariantId:  &device.VariantId{Value: "variant1"},
					BrandId:    &device.BrandId{Value: "brand1"},
				},
				{
					PlatformId: &device.PlatformId{Value: "platform"},
					ModelId:    &device.ModelId{Value: "model"},
					VariantId:  &device.VariantId{Value: "variant-nonexisting2"},
					BrandId:    &device.BrandId{Value: "nonexisting"},
				},
			})
			assert.Loosely(t, err, should.NotBeNil)
			errs := err.(errors.MultiError)
			assert.Loosely(t, errs, should.HaveLength(3))
			assert.Loosely(t, resp, should.HaveLength(3))
			assert.Loosely(t, errs[0].Error(), should.ContainSubstring("no such entity"))
			assert.Loosely(t, resp[0], should.BeNil)
			assert.Loosely(t, errs[1], should.BeNil)
			assert.Loosely(t, resp[1].(*device.Config), should.NotBeNil)
			assert.Loosely(t, errs[2].Error(), should.ContainSubstring("no such entity"))
			assert.Loosely(t, resp[2], should.BeNil)
		})
	})
}

func TestGetAllCachedConfig(t *testing.T) {
	ftt.Run("Test get all device config cache", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		err := datastore.Put(ctx, []devcfgEntity{
			{ID: "platform.model.variant1"},
			{ID: "platform.model.variant2"},
			{
				ID:        "platform.model.variant3",
				DevConfig: []byte("bad data"),
			},
		})
		assert.Loosely(t, err, should.BeNil)

		devConfigs, err := GetAllCachedConfig(ctx)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, devConfigs, should.HaveLength(2))
		for dc := range devConfigs {
			assert.Loosely(t, dc.GetId(), should.BeNil)
		}
	})
}

func TestDeviceConfigsExists(t *testing.T) {
	ctx := gaetesting.TestingContextWithAppID("go-test")

	ftt.Run("Test exists device config in datastore", t, func(t *ftt.Test) {
		err := datastore.Put(ctx, []devcfgEntity{
			{ID: "kunimitsu.lars.variant1"},
			{ID: "arcada.arcada.variant2"},
			{
				ID:        "platform.model.variant3",
				DevConfig: []byte("bad data"),
			},
		})
		assert.Loosely(t, err, should.BeNil)

		t.Run("Happy path", func(t *ftt.Test) {
			exists, err := DeviceConfigsExists(ctx, []*device.ConfigId{
				{
					PlatformId: &device.PlatformId{Value: "kunimitsu"},
					ModelId:    &device.ModelId{Value: "lars"},
					VariantId:  &device.VariantId{Value: "variant1"},
				},
				{
					PlatformId: &device.PlatformId{Value: "arcada"},
					ModelId:    &device.ModelId{Value: "arcada"},
					VariantId:  &device.VariantId{Value: "variant2"},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, exists[0], should.BeTrue)
			assert.Loosely(t, exists[1], should.BeTrue)
		})

		t.Run("check for nonexisting data", func(t *ftt.Test) {
			exists, err := DeviceConfigsExists(ctx, []*device.ConfigId{
				{
					PlatformId: &device.PlatformId{Value: "platform"},
					ModelId:    &device.ModelId{Value: "model"},
					VariantId:  &device.VariantId{Value: "variant-nonexisting"},
					BrandId:    &device.BrandId{Value: "nonexisting"},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, exists[0], should.BeFalse)
		})

		t.Run("check for existing and nonexisting data", func(t *ftt.Test) {
			exists, err := DeviceConfigsExists(ctx, []*device.ConfigId{
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
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, exists[0], should.BeFalse)
			assert.Loosely(t, exists[1], should.BeTrue)
		})
	})
}

type fakeGitClient struct {
	project string
}

type fakeGSClient struct{}

func (gc *fakeGitClient) GetFile(ctx context.Context, path string) (string, error) {
	if path != "generated/configs.jsonproto" {
		return "", nil
	}
	b, err := os.ReadFile("test_device_config_v2.jsonproto")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (gc *fakeGitClient) SwitchProject(ctx context.Context, project string) error {
	gc.project = project
	return nil
}

func (gsClient *fakeGSClient) GetFile(ctx context.Context, path string) ([]byte, error) {
	b, err := os.ReadFile("test_program_configs.json")
	if err != nil {
		return []byte{}, err
	}
	return b, nil
}

func TestUpdateDatastoreFromBoxter(t *testing.T) {
	ftt.Run("Test update device config from boxster", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		gitilesMock := &fakeGitClient{}
		t.Run("Happy path", func(t *ftt.Test) {
			err := UpdateDatastoreFromBoxster(ctx, gitilesMock, "generated/configs.jsonproto", nil, "", "", "")
			assert.Loosely(t, err, should.BeNil)
			// There should be 7 entities created in datastore as
			// test_device_config_v2.jsonproto contains 13 device configs:
			// 6 sku-less device configs & 7 real device configs.
			var cfgs []*devcfgEntity
			datastore.GetTestable(ctx).Consistent(true)
			err = datastore.GetAll(ctx, datastore.NewQuery(entityKind), &cfgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, cfgs, should.HaveLength(13))
		})
	})
}
