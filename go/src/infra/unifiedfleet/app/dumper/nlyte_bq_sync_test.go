// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/controller"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func TestNlyteUpcertAsset(t *testing.T) {
	t.Parallel()

	ftt.Run("Nlyte Upcert Asset", t, func(t *ftt.Test) {
		ctx := memory.UseWithAppID(context.Background(), ("dev~infra-unified-fleet-system"))
		ctx = gologger.StdConfig.Use(ctx)
		ctx = logging.SetLevel(ctx, logging.Error)
		ctx = config.Use(ctx, &config.Config{})
		datastore.GetTestable(ctx).Consistent(true)
		ctx = nlyteUpcertTestFakeAuthDB(ctx, t.T)

		nlyteUpcertTestRackSetup(ctx, t.T)
		existingUFSAsset, existingUFSNlyteAsset := nlyteUpcertTestAssetSetup(ctx, t.T)

		t.Run("Create New Nlyte Asset", func(t *ftt.Test) {
			asset := &ufspb.Asset{
				Name: "test-create-asset",
				Type: ufspb.AssetType_DUT,
				Location: &ufspb.Location{
					Zone: ufspb.Zone_ZONE_CHROMEOS7,
					Rack: "chromeos7-row2-rack1",
				},
			}
			expected := &ufspb.Asset{}
			proto.Merge(expected, asset)
			expected.Name = "nlyte-" + asset.Name
			err := nlyteUpcertAsset(ctx, asset)
			assert.NoErr(t, err)
			actual, err := controller.GetAsset(ctx, "nlyte-test-create-asset")
			assert.NoErr(t, err)
			assert.Loosely(t, actual, should.NotBeNil)
			assert.That(t, actual.Location, should.Match(expected.Location))
		})

		t.Run("Update Existing UFS Asset", func(t *ftt.Test) {
			asset := &ufspb.Asset{}
			expected := &ufspb.Asset{}
			proto.Merge(asset, existingUFSAsset)
			asset.Location.Rack = "11-02"
			proto.Merge(expected, asset)
			expected.Name = "nlyte-" + expected.Name
			err := nlyteUpcertAsset(ctx, asset)
			assert.NoErr(t, err)
			originalAsset, err := controller.GetAsset(ctx, "test-update-ufs-asset")
			assert.NoErr(t, err)
			assert.Loosely(t, originalAsset, should.NotBeNil)
			assert.That(t, originalAsset, should.Match(existingUFSAsset))
			updatedAsset, err := controller.GetAsset(ctx, "nlyte-test-update-ufs-asset")
			assert.NoErr(t, err)
			assert.Loosely(t, updatedAsset, should.NotBeNil)
			assert.That(t, updatedAsset.Location, should.Match(expected.Location))
		})

		t.Run("Update Existing Nlyte Prefix Asset", func(t *ftt.Test) {
			asset := &ufspb.Asset{}
			expected := &ufspb.Asset{}
			proto.Merge(asset, existingUFSNlyteAsset)
			asset.Location.Rack = "11-02"
			proto.Merge(expected, asset)
			expected.Name = "nlyte-" + asset.Name
			err := nlyteUpcertAsset(ctx, asset)
			assert.NoErr(t, err)
			originalAsset, err := controller.GetAsset(ctx, "test-update-ufs-nlyte-asset")
			assert.NoErr(t, err)
			assert.Loosely(t, originalAsset, should.NotBeNil)
			assert.That(t, originalAsset, should.Match(existingUFSNlyteAsset))
			updatedAsset, err := controller.GetAsset(ctx, "nlyte-test-update-ufs-nlyte-asset")
			assert.NoErr(t, err)
			assert.Loosely(t, updatedAsset, should.NotBeNil)
			assert.Loosely(t, updatedAsset.Location, should.Match(expected.Location))
		})
	})
}

func nlyteUpcertTestFakeAuthDB(ctx context.Context, t *testing.T) context.Context {
	t.Helper()
	return auth.WithState(ctx, &authtest.FakeState{
		Identity: "user:user@example.com",
		FakeDB: authtest.NewFakeDB(
			authtest.MockMembership("user:user@example.com", "user"),
			authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsCreate),
			authtest.MockPermission("user:user@example.com", util.AcsLabAdminRealm, util.RegistrationsCreate),
			authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsGet),
			authtest.MockPermission("user:user@example.com", util.AcsLabAdminRealm, util.RegistrationsGet),
			authtest.MockPermission("user:user@example.com", util.AtlLabAdminRealm, util.RegistrationsUpdate),
			authtest.MockPermission("user:user@example.com", util.AcsLabAdminRealm, util.RegistrationsUpdate),
		),
	})
}

func nlyteUpcertTestRackSetup(ctx context.Context, t *testing.T) {
	t.Helper()
	if _, err := registration.CreateRack(ctx, &ufspb.Rack{Name: "chromeos7-row2-rack1", Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_CHROMEOS7}}); err != nil {
		t.Fatalf("Unable to create test new asset rack: %s", err)
	}
	if _, err := registration.CreateRack(ctx, &ufspb.Rack{Name: "10-01", Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_SFO36_OS}}); err != nil {
		t.Fatalf("Unable to create test existing UFS asset rack: %s", err)
	}
	if _, err := registration.CreateRack(ctx, &ufspb.Rack{Name: "11-02", Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_SFO36_OS}}); err != nil {
		t.Fatalf("Unable to create test existing Nlyte asset rack: %s", err)
	}
}

func nlyteUpcertTestAssetSetup(ctx context.Context, t *testing.T) (*ufspb.Asset, *ufspb.Asset) {
	t.Helper()
	existingUFSAsset := &ufspb.Asset{Name: "test-update-ufs-asset", Type: ufspb.AssetType_DUT, Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_SFO36_OS, Rack: "10-01"}}
	if _, err := controller.AssetRegistration(ctx, existingUFSAsset); err != nil {
		t.Fatalf("Unable to create test existing UFS asset: %s", err)
	}

	existingUFSNlyteAsset := &ufspb.Asset{Name: "test-update-ufs-nlyte-asset", Type: ufspb.AssetType_DUT, Location: &ufspb.Location{Zone: ufspb.Zone_ZONE_SFO36_OS, Rack: "10-01"}}
	existingUFSNlytePrefixAsset := &ufspb.Asset{}
	proto.Merge(existingUFSNlytePrefixAsset, existingUFSNlyteAsset)
	existingUFSNlytePrefixAsset.Name = "nlyte-" + existingUFSNlytePrefixAsset.Name
	if _, err := controller.AssetRegistration(ctx, existingUFSNlyteAsset); err != nil {
		t.Fatalf("Unable to create test existing UFS + Nlyte asset: %s", err)
	}
	if _, err := controller.AssetRegistration(ctx, existingUFSNlytePrefixAsset); err != nil {
		t.Fatalf("Unable to create test existing UFS + Nlyte prefix asset: %s", err)
	}
	return existingUFSAsset, existingUFSNlyteAsset
}
