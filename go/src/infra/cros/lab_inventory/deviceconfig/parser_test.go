// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package deviceconfig

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/golang/protobuf/jsonpb"

	"go.chromium.org/chromiumos/config/go/payload"
	"go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestParseConfigBundle(t *testing.T) {
	ftt.Run("Test config bundle parsing", t, func(t *ftt.Test) {
		var payloads payload.ConfigBundleList
		unmarshaller := &jsonpb.Unmarshaler{AllowUnknownFields: false}
		// Refer to https://chromium.googlesource.com/chromiumos/config/+/refs/heads/master/test/project/fake/fake/config.star for unittest check
		b, err := ioutil.ReadFile("test_device_config_v2.jsonproto")
		assert.Loosely(t, err, should.BeNil)
		buf, err := fixFieldMaskForConfigBundleList([]byte(b))
		assert.Loosely(t, err, should.BeNil)
		err = unmarshaller.Unmarshal(bytes.NewBuffer(buf), &payloads)
		assert.Loosely(t, err, should.BeNil)
		t.Run("Happy path", func(t *ftt.Test) {
			assert.Loosely(t, payloads.GetValues(), should.HaveLength(1))
			dcs := parseConfigBundle(payloads.GetValues()[0])
			// 6 sku-less device configs & 7 real device configs
			assert.Loosely(t, dcs, should.HaveLength(13))
			for _, dc := range dcs {
				assert.Loosely(t, dc.GetId().GetPlatformId().GetValue(), should.Equal("FAKE_PROGRAM"))
				modelWithSku := fmt.Sprintf("%s:%s", dc.GetId().GetModelId().GetValue(), dc.GetId().GetVariantId().GetValue())
				switch modelWithSku {
				case "FAKE-REF-DESIGN:", "PROJECT-A:", "PROJECT-B:", "PROJECT-C:", "PROJECT-WL:", "PROJECT-U:":
					// These are sku-less device config, every config entry is nil by default
					assert.Loosely(t, dc.GetFormFactor(), should.Equal(device.Config_FORM_FACTOR_UNSPECIFIED))
					assert.Loosely(t, dc.GetPower(), should.Equal(device.Config_POWER_SUPPLY_UNSPECIFIED))
					assert.Loosely(t, dc.GetHardwareFeatures(), should.BeNil)
					assert.Loosely(t, dc.GetStorage(), should.Equal(device.Config_STORAGE_UNSPECIFIED))
					assert.Loosely(t, dc.GetCpu(), should.Equal(device.Config_ARCHITECTURE_UNDEFINED))
				case "FAKE-REF-DESIGN:2147483647":
					assert.Loosely(t, dc.GetFormFactor(), should.Equal(device.Config_FORM_FACTOR_CLAMSHELL))
					assert.Loosely(t, dc.GetPower(), should.Equal(device.Config_POWER_SUPPLY_BATTERY))
					assert.Loosely(t, dc.GetHardwareFeatures(), should.Resemble([]device.Config_HardwareFeature{
						device.Config_HARDWARE_FEATURE_BLUETOOTH,
						device.Config_HARDWARE_FEATURE_INTERNAL_DISPLAY,
						device.Config_HARDWARE_FEATURE_STYLUS,
						device.Config_HARDWARE_FEATURE_TOUCHPAD,
						device.Config_HARDWARE_FEATURE_TOUCHSCREEN,
						device.Config_HARDWARE_FEATURE_DETACHABLE_KEYBOARD,
					}))
					assert.Loosely(t, dc.GetStorage(), should.Equal(device.Config_STORAGE_MMC))
					assert.Loosely(t, dc.GetCpu(), should.Equal(device.Config_ARCHITECTURE_UNDEFINED))
				case "FAKE-REF-DESIGN:0":
					fallthrough
				case "FAKE-REF-DESIGN:2":
					assert.Loosely(t, dc.GetFormFactor(), should.Equal(device.Config_FORM_FACTOR_CLAMSHELL))
					assert.Loosely(t, dc.GetPower(), should.Equal(device.Config_POWER_SUPPLY_BATTERY))
					assert.Loosely(t, dc.GetPower(), should.Equal(device.Config_POWER_SUPPLY_BATTERY))
					assert.Loosely(t, dc.GetHardwareFeatures(), should.Resemble([]device.Config_HardwareFeature{
						device.Config_HARDWARE_FEATURE_BLUETOOTH,
						device.Config_HARDWARE_FEATURE_INTERNAL_DISPLAY,
						device.Config_HARDWARE_FEATURE_STYLUS,
						device.Config_HARDWARE_FEATURE_TOUCHPAD,
						device.Config_HARDWARE_FEATURE_TOUCHSCREEN,
						device.Config_HARDWARE_FEATURE_DETACHABLE_KEYBOARD,
					}))
					assert.Loosely(t, dc.GetStorage(), should.Equal(device.Config_STORAGE_SSD))
					assert.Loosely(t, dc.GetCpu(), should.Equal(device.Config_ARCHITECTURE_UNDEFINED))
					assert.Loosely(t, dc.GetEc(), should.Equal(device.Config_EC_CHROME))
				case "PROJECT-A:32":
					assert.Loosely(t, dc.GetFormFactor(), should.Equal(device.Config_FORM_FACTOR_CONVERTIBLE))
					assert.Loosely(t, dc.GetPower(), should.Equal(device.Config_POWER_SUPPLY_BATTERY))
					assert.Loosely(t, dc.GetHardwareFeatures(), should.Resemble([]device.Config_HardwareFeature{
						device.Config_HARDWARE_FEATURE_BLUETOOTH,
						device.Config_HARDWARE_FEATURE_INTERNAL_DISPLAY,
						device.Config_HARDWARE_FEATURE_STYLUS,
						device.Config_HARDWARE_FEATURE_TOUCHPAD,
						device.Config_HARDWARE_FEATURE_TOUCHSCREEN,
						device.Config_HARDWARE_FEATURE_DETACHABLE_KEYBOARD,
					}))
					assert.Loosely(t, dc.GetStorage(), should.Equal(device.Config_STORAGE_MMC))
					assert.Loosely(t, dc.GetCpu(), should.Equal(device.Config_ARCHITECTURE_UNDEFINED))
				case "PROJECT-B:33":
					assert.Loosely(t, dc.GetFormFactor(), should.Equal(device.Config_FORM_FACTOR_CONVERTIBLE))
					assert.Loosely(t, dc.GetPower(), should.Equal(device.Config_POWER_SUPPLY_BATTERY))
					assert.Loosely(t, dc.GetHardwareFeatures(), should.Resemble([]device.Config_HardwareFeature{
						device.Config_HARDWARE_FEATURE_BLUETOOTH,
						device.Config_HARDWARE_FEATURE_INTERNAL_DISPLAY,
						device.Config_HARDWARE_FEATURE_STYLUS,
						device.Config_HARDWARE_FEATURE_TOUCHPAD,
						device.Config_HARDWARE_FEATURE_TOUCHSCREEN,
						device.Config_HARDWARE_FEATURE_DETACHABLE_KEYBOARD,
					}))
					assert.Loosely(t, dc.GetStorage(), should.Equal(device.Config_STORAGE_MMC))
					assert.Loosely(t, dc.GetCpu(), should.Equal(device.Config_ARCHITECTURE_UNDEFINED))
				case "PROJECT-C:34":
					assert.Loosely(t, dc.GetFormFactor(), should.Equal(device.Config_FORM_FACTOR_CLAMSHELL))
					assert.Loosely(t, dc.GetPower(), should.Equal(device.Config_POWER_SUPPLY_BATTERY))
					assert.Loosely(t, dc.GetHardwareFeatures(), should.Resemble([]device.Config_HardwareFeature{
						device.Config_HARDWARE_FEATURE_BLUETOOTH,
						device.Config_HARDWARE_FEATURE_INTERNAL_DISPLAY,
						device.Config_HARDWARE_FEATURE_STYLUS,
						device.Config_HARDWARE_FEATURE_TOUCHPAD,
						device.Config_HARDWARE_FEATURE_TOUCHSCREEN,
						device.Config_HARDWARE_FEATURE_DETACHABLE_KEYBOARD,
					}))
					assert.Loosely(t, dc.GetStorage(), should.Equal(device.Config_STORAGE_NVME))
					assert.Loosely(t, dc.GetCpu(), should.Equal(device.Config_ARCHITECTURE_UNDEFINED))
				case "PROJECT-WL:64":
					assert.Loosely(t, dc.GetFormFactor(), should.Equal(device.Config_FORM_FACTOR_CHROMEBIT))
					assert.Loosely(t, dc.GetPower(), should.Equal(device.Config_POWER_SUPPLY_AC_ONLY))
					assert.Loosely(t, dc.GetHardwareFeatures(), should.Resemble([]device.Config_HardwareFeature{
						device.Config_HARDWARE_FEATURE_BLUETOOTH,
						device.Config_HARDWARE_FEATURE_INTERNAL_DISPLAY,
						device.Config_HARDWARE_FEATURE_TOUCHPAD,
						device.Config_HARDWARE_FEATURE_TOUCHSCREEN,
						device.Config_HARDWARE_FEATURE_DETACHABLE_KEYBOARD,
					}))
					assert.Loosely(t, dc.GetStorage(), should.Equal(device.Config_STORAGE_NVME))
					assert.Loosely(t, dc.GetCpu(), should.Equal(device.Config_ARCHITECTURE_UNDEFINED))
				case "PROJECT-U:75":
					assert.Loosely(t, dc.GetFormFactor(), should.Equal(device.Config_FORM_FACTOR_CONVERTIBLE))
					assert.Loosely(t, dc.GetPower(), should.Equal(device.Config_POWER_SUPPLY_BATTERY))
					assert.Loosely(t, dc.GetHardwareFeatures(), should.Resemble([]device.Config_HardwareFeature{
						device.Config_HARDWARE_FEATURE_BLUETOOTH,
						device.Config_HARDWARE_FEATURE_INTERNAL_DISPLAY,
						device.Config_HARDWARE_FEATURE_STYLUS,
						device.Config_HARDWARE_FEATURE_TOUCHPAD,
						device.Config_HARDWARE_FEATURE_TOUCHSCREEN,
						device.Config_HARDWARE_FEATURE_DETACHABLE_KEYBOARD,
					}))
					assert.Loosely(t, dc.GetStorage(), should.Equal(device.Config_STORAGE_UFS))
					assert.Loosely(t, dc.GetCpu(), should.Equal(device.Config_ARCHITECTURE_UNDEFINED))
				default:
					t.Errorf("Invalid model:sku: %s", modelWithSku)
				}
			}
		})
	})
}
