// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
)

func TestUpdateAssetInfoHelper(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("Testing updateAssetInfoHelper", t, func(t *ftt.Test) {
		t.Run("Update non-existing asset", func(t *ftt.Test) {
			// Shouldn't work as we didn't create the asset
			err := updateAssetInfoHelper(ctx, &ufspb.AssetInfo{
				AssetTag: "test-tag",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Entity not found"))
		})
		t.Run("Update asset with missing machine", func(t *ftt.Test) {
			a1 := &ufspb.Asset{
				Name:  "test-tag",
				Model: "test-model",
				Type:  ufspb.AssetType_DUT,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CROS_GOOGLER_DESK,
					Rack: "test-rack",
				},
			}
			// Create an asset.
			_, err := registration.CreateAsset(ctx, a1)
			assert.Loosely(t, err, should.BeNil)
			// Update a dut asset without machine.
			err = updateAssetInfoHelper(ctx, &ufspb.AssetInfo{
				AssetTag:    "test-tag",
				Model:       "test-model",
				BuildTarget: "test-target",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Entity not found"))
		})
		t.Run("Update asset info - Happy path", func(t *ftt.Test) {
			l1 := &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CROS_GOOGLER_DESK,
				Rack: "test-rack",
			}
			a1 := &ufspb.Asset{
				Name:     "test-tag1",
				Model:    "test-model",
				Type:     ufspb.AssetType_DUT,
				Location: l1,
			}
			m1 := &ufspb.Machine{
				Name: "test-tag1",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						Model: "test-model",
					},
				},
				Location: l1,
			}
			// Create an asset
			_, err := registration.CreateAsset(ctx, a1)
			assert.Loosely(t, err, should.BeNil)
			// Create a corresponding machine
			_, err = registration.CreateMachine(ctx, m1)
			assert.Loosely(t, err, should.BeNil)
			// Update buildtarget for the asset
			err = updateAssetInfoHelper(ctx, &ufspb.AssetInfo{
				AssetTag:      "test-tag1",
				Model:         "test-model",
				BuildTarget:   "test-target",
				HasWifiBt:     true,
				WifiBluetooth: "test-wifichip",
			})
			assert.Loosely(t, err, should.BeNil)
			// Machine should reflect the change
			m2, err := registration.GetMachine(ctx, "test-tag1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m2.GetChromeosMachine().GetBuildTarget(), should.Equal("test-target"))
			assert.Loosely(t, m2.GetChromeosMachine().GetWifiBluetooth(), should.Equal("test-wifichip"))
			assert.Loosely(t, m2.GetChromeosMachine().GetHasWifiBt(), should.BeTrue)
		})
		t.Run("Update asset info - Machine avoids HWID, phase, sku and mac", func(t *ftt.Test) {
			l1 := &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CROS_GOOGLER_DESK,
				Rack: "test-rack",
			}
			a1 := &ufspb.Asset{
				Name:     "test-tag2",
				Model:    "test-model",
				Type:     ufspb.AssetType_DUT,
				Location: l1,
			}
			m1 := &ufspb.Machine{
				Name: "test-tag2",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						Model:      "test-model",
						MacAddress: "FF:FF:FF:EE:EE:EE",
						Sku:        "21",
						Phase:      "PVT",
						Hwid:       "TESTHWID 123FGHEASFG",
					},
				},
				Location: l1,
			}
			// Create an asset
			_, err := registration.CreateAsset(ctx, a1)
			assert.Loosely(t, err, should.BeNil)
			// Create a corresponding machine
			_, err = registration.CreateMachine(ctx, m1)
			assert.Loosely(t, err, should.BeNil)
			// Update buildtarget for the asset
			err = updateAssetInfoHelper(ctx, &ufspb.AssetInfo{
				AssetTag:           "test-tag2",
				Model:              "test-model",
				Sku:                "0",
				EthernetMacAddress: "11:11:11:22:22:22",
				Phase:              "EVT",
				Hwid:               "NOTEST HWID124RFGG",
			})
			assert.Loosely(t, err, should.BeNil)
			// Machine should not reflect the change
			m2, err := registration.GetMachine(ctx, "test-tag2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m2.GetChromeosMachine().GetSku(), should.Equal("21"))
			assert.Loosely(t, m2.GetChromeosMachine().GetHwid(), should.Equal("TESTHWID 123FGHEASFG"))
			assert.Loosely(t, m2.GetChromeosMachine().GetPhase(), should.Equal("PVT"))
			assert.Loosely(t, m2.GetChromeosMachine().GetMacAddress(), should.Equal("FF:FF:FF:EE:EE:EE"))
			// Asset should record the change
			a2, err := registration.GetAsset(ctx, "test-tag2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, a2.GetInfo().GetPhase(), should.Equal("EVT"))
			assert.Loosely(t, a2.GetInfo().GetEthernetMacAddress(), should.Equal("11:11:11:22:22:22"))
			assert.Loosely(t, a2.GetInfo().GetHwid(), should.Equal("NOTEST HWID124RFGG"))
			assert.Loosely(t, a2.GetInfo().GetSku(), should.Equal("0"))
		})
	})
}
