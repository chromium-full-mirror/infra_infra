// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/config"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/history"
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/model/registration"
	"infra/unifiedfleet/app/util"
)

func mockRack(name, row string, zone ufspb.Zone) *ufspb.Rack {
	return &ufspb.Rack{
		Name: name,
		Location: &ufspb.Location{
			Zone:        zone,
			Row:         row,
			BarcodeName: name,
		},
	}
}

func mockAsset(name, model, row, rack, pos, host string, aType ufspb.AssetType, zone ufspb.Zone) *ufspb.Asset {
	return &ufspb.Asset{
		Name:  name,
		Type:  aType,
		Model: model,
		Location: &ufspb.Location{
			Zone:        zone,
			Row:         row,
			Rack:        rack,
			Position:    pos,
			BarcodeName: host,
		},
	}
}

func mockAssetInfo(serialNumber, costCenter, googleCodeName, model, buildTarget, referenceBoard, ethernetMacAddress, sku, phase string) *ufspb.AssetInfo {
	return &ufspb.AssetInfo{
		SerialNumber:       serialNumber,
		CostCenter:         costCenter,
		GoogleCodeName:     googleCodeName,
		Model:              model,
		BuildTarget:        buildTarget,
		ReferenceBoard:     referenceBoard,
		EthernetMacAddress: ethernetMacAddress,
		Sku:                sku,
		Phase:              phase,
	}
}

func TestAssetRegistration(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	// Set values to HaRT config. Else the tests trigger a request and wait for response. Timing out eventually
	config.Get(ctx).Hart = &config.PubSub{
		Project:   "nonexistent-testing-project",
		Topic:     "vertical migration of zooplankton",
		BatchSize: 0,
	}
	ftt.Run("Testing AssetRegistration", t, func(t *ftt.Test) {
		t.Run("Register asset with existing rack", func(t *ftt.Test) {
			r := mockRack("chromeos6-row2-rack3", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := registration.CreateRack(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			a := mockAsset("C001001", "eve", "2", "chromeos6-row2-rack3", "1", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("asset"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))

			machine, err := registration.GetMachine(ctx, "C001001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, machine, should.NotBeNil)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/C001001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
		})
		t.Run("Register asset with non-existent rack", func(t *ftt.Test) {
			a := mockAsset("C001002", "eve", "2", "chromeos6-row3-rack3", "1", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err := AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.NotBeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001002")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})
		t.Run("Register asset with invalid name", func(t *ftt.Test) {
			r := mockRack("chromeos6-row4-rack3", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := registration.CreateRack(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			a := mockAsset("", "eve", "4", "chromeos6-row4-rack3", "1", "chromeos6-row4-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Register existing asset", func(t *ftt.Test) {
			r := mockRack("chromeos6-row2-rack4", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := registration.CreateRack(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			a := mockAsset("C001004", "eve", "2", "chromeos6-row2-rack4", "1", "chromeos6-row2-rack4-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("asset"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			_, err = AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Register asset(servo) with existing rack", func(t *ftt.Test) {
			r := mockRack("chromeos6-row2-rack3-servo1", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := registration.CreateRack(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			a := mockAsset("C001001-servo", "eve", "2", "chromeos6-row2-rack3-servo1", "1", "", ufspb.AssetType_SERVO, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001001-servo")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("asset"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))

			_, err = registration.GetMachine(ctx, "C001001-servo")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
	})
}

func TestUpdateAsset(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("Testing UpdateAsset", t, func(t *ftt.Test) {
		t.Run("Update non existent asset", func(t *ftt.Test) {
			r := mockRack("chromeos6-row2-rack3", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := registration.CreateRack(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			b := mockAsset("C001001", "eve", "2", "chromeos6-row2-rack3", "1", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = UpdateAsset(ctx, b, &field_mask.FieldMask{Paths: []string{"type", "model"}})
			fmt.Println(err)
			assert.Loosely(t, err.Error(), should.ContainSubstring("unable to update asset C001001"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Move asset to non-existent rack", func(t *ftt.Test) {
			// Give the user update permissions
			ctx = initializeFakeAuthDB(ctx, "user:tes@ter.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			a := mockAsset("C001001", "eve", "2", "chromeos6-row2-rack3", "1", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err := AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.BeNil)
			b := mockAsset("C001001", "eve", "2", "chromeos6-row2-rack4", "1", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = UpdateAsset(ctx, b, &field_mask.FieldMask{Paths: []string{"location.rack"}})
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no Rack with RackID chromeos6-row2-rack4"))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
		})

		t.Run("Move Asset to existing rack", func(t *ftt.Test) {
			// Give the user update permissions
			ctx = initializeFakeAuthDB(ctx, "user:tes@ter.com", util.RegistrationsUpdate, util.AtlLabAdminRealm)
			r := mockRack("chromeos6-row2-rack5", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := registration.CreateRack(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			r = mockRack("chromeos6-row2-rack6", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err = registration.CreateRack(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			a := mockAsset("C001002", "eve", "2", "chromeos6-row2-rack5", "1", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.BeNil)
			b := mockAsset("C001002", "eve", "2", "chromeos6-row2-rack6", "2", "chromeos6-row2-rack4-host2", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = UpdateAsset(ctx, b, &field_mask.FieldMask{Paths: []string{"location.rack"}})
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001002")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "name", "assets/C001002")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(0))
		})

		t.Run("Update Asset info of an asset", func(t *ftt.Test) {
			r := mockRack("chromeos6-row2-rack7", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := registration.CreateRack(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			a := mockAsset("C001003", "eve", "2", "chromeos6-row2-rack5", "3", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.BeNil)
			ai := mockAssetInfo("", "", "", "", "", "", "", "", "DVT")
			a.Info = ai
			_, err = UpdateAsset(ctx, a, &field_mask.FieldMask{Paths: []string{"info.phase"}})
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001003")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("asset"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("asset.info.phase"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("DVT"))
			assert.Loosely(t, changes[1].GetOldValue(), should.BeEmpty)

			machine, err := registration.GetMachine(ctx, "C001003")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, machine, should.NotBeNil)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/C001003")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine.chrome_os_machine.phase"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("DVT"))
			assert.Loosely(t, changes[1].GetOldValue(), should.BeEmpty)
		})

		t.Run("Update Asset with invalid mask", func(t *ftt.Test) {
			a := mockAsset("C001004", "eve", "2", "chromeos6-row2-rack3", "1", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err := AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.BeNil)
			b := mockAsset("C001004", "eve", "2", "chromeos6-row2-rack3", "1", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			// Attempt to update name of the asset
			_, err = UpdateAsset(ctx, b, &field_mask.FieldMask{Paths: []string{"name"}})
			assert.Loosely(t, err, should.NotBeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001004")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			// Attempt to update name of the asset in asset info
			_, err = UpdateAsset(ctx, b, &field_mask.FieldMask{Paths: []string{"info.asset_tag"}})
			assert.Loosely(t, err, should.NotBeNil)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "assets/C001004")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			// Attempt to update timestamp of the asset
			_, err = UpdateAsset(ctx, b, &field_mask.FieldMask{Paths: []string{"update_time"}})
			assert.Loosely(t, err, should.NotBeNil)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "assets/C001004")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			// Attempt to clear zone of the asset
			b.Location.Zone = ufspb.Zone_ZONE_UNSPECIFIED
			_, err = UpdateAsset(ctx, b, &field_mask.FieldMask{Paths: []string{"location.zone"}})
			assert.Loosely(t, err, should.NotBeNil)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "assets/C001004")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			b.Location.Zone = ufspb.Zone_ZONE_CHROMEOS6
			// Attempt to clear rack of the asset
			b.Location.Rack = ""
			_, err = UpdateAsset(ctx, b, &field_mask.FieldMask{Paths: []string{"location.rack"}})
			assert.Loosely(t, err, should.NotBeNil)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "assets/C001004")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
		})

		t.Run("Update Asset from servo to DUT", func(t *ftt.Test) {
			r := mockRack("chromeos6-row2-rack8", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := registration.CreateRack(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			a := mockAsset("C001003-servo", "eve", "2", "chromeos6-row2-rack8", "3", "chromeos6-row2-rack3-host1", ufspb.AssetType_SERVO, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = registration.CreateAsset(ctx, a)
			assert.Loosely(t, err, should.BeNil)

			a.Type = ufspb.AssetType_DUT
			_, err = UpdateAsset(ctx, a, &field_mask.FieldMask{Paths: []string{"type"}})
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001003-servo")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("asset.type"))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal("DUT"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal("SERVO"))

			machine, err := registration.GetMachine(ctx, "C001003-servo")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, machine, should.NotBeNil)
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/C001003-servo")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
		})

		t.Run("Update Asset from DUT to servo", func(t *ftt.Test) {
			// Give the user update permissions
			ctx = withAuthorizedAtlUser(ctx)
			r := mockRack("chromeos6-row2-rack9", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := registration.CreateRack(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			a := mockAsset("C001003-DUT", "eve", "2", "chromeos6-row2-rack9", "3", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			asset, err := AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.BeNil)

			asset.Type = ufspb.AssetType_SERVO
			_, err = UpdateAsset(ctx, a, &field_mask.FieldMask{Paths: []string{"type"}})
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001003-DUT")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("asset"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("asset.type"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("SERVO"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("DUT"))

			_, err = registration.GetMachine(ctx, "C001003-DUT")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/C001003-DUT")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(LifeCycleRetire))
		})
	})
}

func TestGetAsset(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("Testing GetAsset", t, func(t *ftt.Test) {
		t.Run("Get existing assets", func(t *ftt.Test) {
			r := mockRack("chromeos6-row2-rack3", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := RackRegistration(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			a := mockAsset("C001001", "eve", "2", "chromeos6-row2-rack3", "1", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.BeNil)
			respA, err := GetAsset(ctx, "C001001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respA, should.Resemble(a))
		})
		t.Run("Get non existing assets", func(t *ftt.Test) {
			respA, err := GetAsset(ctx, "C001004")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, respA, should.BeNil)
		})
		t.Run("Get invalid assets", func(t *ftt.Test) {
			respB, err := GetAsset(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, respB, should.BeNil)
		})
	})
}

func createArrayOfMockAssets(n int, prefix, zone, assetType, model string) []*ufspb.Asset {
	var assets []*ufspb.Asset
	for i := 0; i < n; i++ {
		aType := ufspb.AssetType_UNDEFINED
		if assetType == "dut" {
			aType = ufspb.AssetType_DUT
		} else if assetType == "labstation" {
			aType = ufspb.AssetType_LABSTATION
		}
		a := mockAsset(fmt.Sprintf("%s00%d", prefix, i), model, "3", fmt.Sprintf("%s-row3-rack3", zone), fmt.Sprintf("%d", i), fmt.Sprintf("%s-row3-rack3-host%d", zone, i), aType, util.ToUFSZone(zone))
		assets = append(assets, a)
	}
	return assets
}

func TestListAssets(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	r := mockRack("chromeos6-row3-rack3", "3", ufspb.Zone_ZONE_CHROMEOS6)
	RackRegistration(ctx, r)
	r = mockRack("chromeos2-row3-rack3", "3", ufspb.Zone_ZONE_CHROMEOS2)
	RackRegistration(ctx, r)
	dutChromeos6 := createArrayOfMockAssets(4, "EVE6", "chromeos6", "dut", "eve")
	labstationsChromeos6 := createArrayOfMockAssets(4, "FIZ6", "chromeos6", "labstation", "fizz")
	guadoChromeos2 := createArrayOfMockAssets(4, "GUA2", "chromeos2", "labstation", "guado")
	fizzChromeos2 := createArrayOfMockAssets(4, "FIZ2", "chromeos2", "labstation", "fizz")
	assets := append(dutChromeos6, labstationsChromeos6...)
	assets = append(assets, guadoChromeos2...)
	assets = append(assets, fizzChromeos2...)
	chromeos2Assets := append(fizzChromeos2, guadoChromeos2...)
	chromeos6Assets := append(dutChromeos6, labstationsChromeos6...)
	labstationAssets := append(fizzChromeos2, labstationsChromeos6...)
	labstationAssets = append(labstationAssets, guadoChromeos2...)
	for _, asset := range assets {
		AssetRegistration(ctx, asset)
	}
	ftt.Run("Testing ListAssets", t, func(t *ftt.Test) {
		t.Run("List all existing assets", func(t *ftt.Test) {
			respAssets, _, err := ListAssets(ctx, 16, "", "", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respAssets, should.HaveLength(16))
		})
		t.Run("List assets by zone", func(t *ftt.Test) {
			respAssets, _, err := ListAssets(ctx, 10, "", "zone=chromeos2", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respAssets, should.HaveLength(8))
			assert.Loosely(t, respAssets, should.Resemble(chromeos2Assets))
			respAssets, _, err = ListAssets(ctx, 10, "", "zone=chromeos6", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respAssets, should.HaveLength(8))
			assert.Loosely(t, respAssets, should.Resemble(chromeos6Assets))
		})
		t.Run("List assets by model", func(t *ftt.Test) {
			respAssets, _, err := ListAssets(ctx, 10, "", "model=guado", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respAssets, should.HaveLength(4))
			assert.Loosely(t, respAssets, should.Resemble(guadoChromeos2))
			respAssets, _, err = ListAssets(ctx, 10, "", "model=eve", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respAssets, should.HaveLength(4))
			assert.Loosely(t, respAssets, should.Resemble(dutChromeos6))
		})
		t.Run("List assets by type", func(t *ftt.Test) {
			respAssets, _, err := ListAssets(ctx, 10, "", "assettype=dut", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respAssets, should.HaveLength(4))
			assert.Loosely(t, respAssets, should.Resemble(dutChromeos6))
			respAssets, _, err = ListAssets(ctx, 12, "", "assettype=labstation", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respAssets, should.HaveLength(12))
			assert.Loosely(t, respAssets, should.Resemble(labstationAssets))
		})
		t.Run("List assets by combination of filters", func(t *ftt.Test) {
			respAssets, _, err := ListAssets(ctx, 10, "", "assettype=dut&model=eve", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respAssets, should.HaveLength(4))
			assert.Loosely(t, respAssets, should.Resemble(dutChromeos6))
			respAssets, _, err = ListAssets(ctx, 10, "", "assettype=labstation&zone=chromeos2", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respAssets, should.HaveLength(8))
			assert.Loosely(t, respAssets, should.Resemble(chromeos2Assets))
			respAssets, _, err = ListAssets(ctx, 10, "", "assettype=labstation&zone=chromeos2&model=guado", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, respAssets, should.HaveLength(4))
			assert.Loosely(t, respAssets, should.Resemble(guadoChromeos2))
		})
	})
}

func TestDeleteAsset(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("Testing DeleteAsset", t, func(t *ftt.Test) {
		t.Run("Delete existing assets", func(t *ftt.Test) {
			// Give the user update permissions
			ctx = initializeFakeAuthDB(ctx, "user:tes@ter.com", util.RegistrationsDelete, util.AtlLabAdminRealm)
			r := mockRack("chromeos6-row2-rack3", "2", ufspb.Zone_ZONE_CHROMEOS6)
			_, err := RackRegistration(ctx, r)
			assert.Loosely(t, err, should.BeNil)
			a := mockAsset("C001001", "eve", "2", "chromeos6-row2-rack3", "1", "chromeos6-row2-rack3-host1", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
			_, err = AssetRegistration(ctx, a)
			assert.Loosely(t, err, should.BeNil)
			err = DeleteAsset(ctx, "C001001")
			assert.Loosely(t, err, should.BeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("asset"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("asset"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal(LifeCycleRetire))
		})
		t.Run("Delete non existing assets", func(t *ftt.Test) {
			err := DeleteAsset(ctx, "C001004")
			assert.Loosely(t, err, should.NotBeNil)
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/C001004")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})
		t.Run("Delete invalid assets", func(t *ftt.Test) {
			err := DeleteAsset(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Delete existing assets with machine associated - pass", func(t *ftt.Test) {
			asset := &ufspb.Asset{
				Name: "asset-1",
				Type: ufspb.AssetType_DUT,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
			}
			_, err := registration.CreateAsset(ctx, asset)
			assert.Loosely(t, err, should.BeNil)

			machine := &ufspb.Machine{
				Name: "asset-1",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
			}
			_, err = registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			err = DeleteAsset(ctx, "asset-1")
			assert.Loosely(t, err, should.BeNil)

			_, err = registration.GetMachine(ctx, "asset-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/asset-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("asset"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))

			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/asset-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(1))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRetire))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRetire))

			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "assets/asset-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)

			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machines/asset-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			assert.Loosely(t, msgs[0].Delete, should.BeTrue)
		})
		t.Run("Delete existing assets with host associated - fail", func(t *ftt.Test) {
			asset := &ufspb.Asset{
				Name: "asset-2",
				Type: ufspb.AssetType_DUT,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS6,
				},
			}
			_, err := registration.CreateAsset(ctx, asset)
			assert.Loosely(t, err, should.BeNil)

			machine := &ufspb.Machine{
				Name: "asset-2",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
			}
			_, err = registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)

			dut := &ufspb.MachineLSE{
				Name:     "dut-2",
				Hostname: "dut-2",
				Machines: []string{"asset-2"},
			}
			_, err = inventory.CreateMachineLSE(ctx, dut)
			assert.Loosely(t, err, should.BeNil)

			err = DeleteAsset(ctx, "asset-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("DUT dut-2 is referring this Asset"))
		})
	})
}

func TestUpdateAssetMeta(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateAssetMeta for an OS machine", t, func(t *ftt.Test) {
		t.Run("Update a non-OS machine", func(t *ftt.Test) {
			_, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "machine-assetmeta-1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{},
				},
			})
			assert.Loosely(t, err, should.BeNil)

			err = UpdateAssetMeta(ctx, &ufspb.DutMeta{
				ChromeosDeviceId: "machine-assetmeta-1",
				Hostname:         "machinelse-labmeta-1",
				SerialNumber:     "fake-serial",
			})
			// Update is skipped without error
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Update a OS machine - happy path", func(t *ftt.Test) {
			machine := &ufspb.Machine{
				Name: "machine-assetmeta-2",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{},
				},
			}
			req, err := registration.CreateMachine(ctx, machine)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req.GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, req.GetChromeosMachine().GetHwid(), should.BeEmpty)
			assert.Loosely(t, req.GetChromeosMachine().GetSku(), should.BeEmpty)

			asset := &ufspb.Asset{
				Name: "machine-assetmeta-2",
				Info: &ufspb.AssetInfo{
					AssetTag: "machine-assetmeta-2",
				},
				Type:     ufspb.AssetType_DUT,
				Location: &ufspb.Location{},
			}
			asset, err = registration.CreateAsset(ctx, asset)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, asset.GetInfo().GetSerialNumber(), should.BeEmpty)
			assert.Loosely(t, asset.GetInfo().GetHwid(), should.BeEmpty)
			assert.Loosely(t, asset.GetInfo().GetSku(), should.BeEmpty)

			err = UpdateAssetMeta(ctx, &ufspb.DutMeta{
				ChromeosDeviceId: "machine-assetmeta-2",
				Hostname:         "machinelse-assetmeta-2",
				SerialNumber:     "fake-serial",
				HwID:             "fake-hwid",
				DeviceSku:        "fake-devicesku",
			})
			assert.Loosely(t, err, should.BeNil)
			asset, err = registration.GetAsset(ctx, "machine-assetmeta-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, asset.GetInfo().GetSerialNumber(), should.Equal("fake-serial"))
			assert.Loosely(t, asset.GetInfo().GetHwid(), should.Equal("fake-hwid"))
			assert.Loosely(t, asset.GetInfo().GetSku(), should.Equal("fake-devicesku"))
		})
	})
}

func TestRenameAsset(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("Testing RenameAsset", t, func(t *ftt.Test) {
		t.Run("Rename non-existing asset", func(t *ftt.Test) {
			_, err := RenameAsset(ctx, "test-asset-1", "test-asset-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(codes.FailedPrecondition.String()))
		})
		t.Run("Rename asset to an existing asset", func(t *ftt.Test) {
			ctx = withAuthorizedAtlUser(ctx)
			r := mockRack("chromeos6-row3-rack3", "3", ufspb.Zone_ZONE_CHROMEOS2)
			RackRegistration(ctx, r)
			// Create EVE6000 and EVE6001 assets
			assets := createArrayOfMockAssets(2, "EVE6", "chromeos6", "dut", "eve")
			for _, asset := range assets {
				_, err := AssetRegistration(ctx, asset)
				assert.Loosely(t, err, should.BeNil)
			}
			_, err := RenameAsset(ctx, "EVE6000", "EVE6001")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(codes.FailedPrecondition.String()))
		})
		t.Run("Rename asset - happy path", func(t *ftt.Test) {
			ctx = withAuthorizedAtlUser(ctx)
			r := mockRack("chromeos6-row3-rack3", "3", ufspb.Zone_ZONE_CHROMEOS2)
			RackRegistration(ctx, r)
			// Create EVE6000 and EVE6001 assets
			assets := createArrayOfMockAssets(1, "EVE7", "chromeos6", "dut", "eve")
			for _, asset := range assets {
				_, err := AssetRegistration(ctx, asset)
				assert.Loosely(t, err, should.BeNil)
			}
			rsp, err := RenameAsset(ctx, "EVE7000", "EVE7001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rsp, should.NotBeNil)
			assert.Loosely(t, rsp.GetName(), should.ContainSubstring("EVE7001"))
			// Validate asset changes record in history
			// Two snapshots. One at registration and another at retirement
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "assets/EVE7000")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "assets/EVE7000")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[0].OldValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("EVE7000"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("EVE7001"))
			// One snapshot at registration.
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "assets/EVE7001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "assets/EVE7001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[0].OldValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("EVE7000"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("EVE7001"))
			// Validate machine changes record in history
			// Two snapshots. One at registration and another at retirement
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machines/EVE7000")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/EVE7000")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(3))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[0].OldValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("REGISTRATION"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[2].OldValue, should.Equal("EVE7000"))
			assert.Loosely(t, changes[2].NewValue, should.Equal("EVE7001"))
			// One snapshot at registration.
			msgs, err = history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machines/EVE7001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(1))
			changes, err = history.QueryChangesByPropertyName(ctx, "name", "machines/EVE7001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			// Verify all changes recorded by the history.
			assert.Loosely(t, changes[0].OldValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[0].NewValue, should.Equal("RENAME"))
			assert.Loosely(t, changes[1].OldValue, should.Equal("EVE7000"))
			assert.Loosely(t, changes[1].NewValue, should.Equal("EVE7001"))
			// Verify state is not changed
			machine, err := registration.GetMachine(ctx, "EVE7001")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, machine.GetResourceState(), should.Equal(ufspb.State_STATE_REGISTERED))
		})
	})
}

func TestUpdateIndexInAsset(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	registration.CreateAsset(ctx, &ufspb.Asset{
		Name: "asset-update-index",
		Type: ufspb.AssetType_DUT,
		Location: &ufspb.Location{
			Zone: ufspb.Zone_ZONE_CHROMEOS6,
			Rack: "index-rack-old",
		},
	})
	ftt.Run("Testing updateIndexInAsset", t, func(t *ftt.Test) {
		t.Run("updateIndexInAsset - update index rack", func(t *ftt.Test) {
			err := updateIndexInAsset(ctx, "rack", "index-rack-old", "index-rack-new", &HistoryClient{})
			assert.Loosely(t, err, should.BeNil)
			asset, err := registration.GetAsset(ctx, "asset-update-index")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, asset.GetLocation().GetRack(), should.Equal("index-rack-new"))
		})
	})
}
