// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package registration

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/config"
	ufsutil "go.chromium.org/infra/unifiedfleet/app/util"
)

func mockAsset(name, model, host string, assettype ufspb.AssetType, zone ufspb.Zone) *ufspb.Asset {
	return &ufspb.Asset{
		Name:  name,
		Type:  assettype,
		Model: model,
		Location: &ufspb.Location{
			BarcodeName: host,
			Zone:        zone,
		},
	}
}

func assertAssetEqual(t *ftt.Test, a, b *ufspb.Asset) {
	assert.Loosely(t, cmp.Equal(a, b, protocmp.Transform()), should.Equal(true))
}

func TestCreateAsset(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	asset1 := mockAsset("C001001", "krane", "cros4-row3-rack5-host4", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS4)
	asset2 := mockAsset("C001001", "Servo V4", "cros5-row3-rack5-host4", ufspb.AssetType_SERVO, ufspb.Zone_ZONE_CHROMEOS5)
	asset3 := mockAsset("", "eve", "cros6-row3-rack5-host4", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS6)
	asset4 := mockAsset("C002002", "eve", "cros7-row3-rack5-host4", ufspb.AssetType_UNDEFINED, ufspb.Zone_ZONE_CHROMEOS7)
	asset6 := mockAsset("C004004", "eve", "cros9-row3-rack5-host4", ufspb.AssetType_DUT, ufspb.Zone_ZONE_UNSPECIFIED)
	asset6.Location = nil
	ftt.Run("CreateAsset", t, func(t *ftt.Test) {
		t.Run("Create new asset", func(t *ftt.Test) {
			resp, err := CreateAsset(ctx, asset1)
			assert.Loosely(t, err, should.BeNil)
			assertAssetEqual(t, asset1, resp)
		})
		t.Run("Create existing asset", func(t *ftt.Test) {
			_, err := CreateAsset(ctx, asset2)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Create asset with invalid name", func(t *ftt.Test) {
			_, err := CreateAsset(ctx, asset3)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Create asset without type", func(t *ftt.Test) {
			_, err := CreateAsset(ctx, asset4)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Create asset without location", func(t *ftt.Test) {
			_, err := CreateAsset(ctx, asset6)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}

func TestUpdateAsset(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	asset1 := mockAsset("C001001", "krane", "cros4-row3-rack5-host4", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS4)
	asset2 := mockAsset("C001001", "Servo V3", "cros6-row3-rack5-host4", ufspb.AssetType_SERVO, ufspb.Zone_ZONE_CHROMEOS6)
	asset3 := mockAsset("C002002", "Whizz", "cros6-row3-rack5-host4", ufspb.AssetType_LABSTATION, ufspb.Zone_ZONE_CHROMEOS6)
	asset4 := mockAsset("", "Whizz-Labstation", "cros6-row3-rack5-host4", ufspb.AssetType_UNDEFINED, ufspb.Zone_ZONE_CHROMEOS6)
	ftt.Run("UpdateAsset", t, func(t *ftt.Test) {
		t.Run("Update existing Asset", func(t *ftt.Test) {
			resp, err := CreateAsset(ctx, asset1)
			assert.Loosely(t, err, should.BeNil)
			assertAssetEqual(t, asset1, resp)
			resp, err = UpdateAsset(ctx, asset2)
			assert.Loosely(t, err, should.BeNil)
			assertAssetEqual(t, asset2, resp)
		})
		t.Run("Update non-existinent asset", func(t *ftt.Test) {
			resp, err := UpdateAsset(ctx, asset3)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Update asset with invalid name", func(t *ftt.Test) {
			resp, err := UpdateAsset(ctx, asset4)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}

func TestGetAsset(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	asset1 := mockAsset("C001001", "krane", "cros4-row3-rack5-host4", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS4)
	ftt.Run("GetAsset", t, func(t *ftt.Test) {
		t.Run("Get asset by existing name", func(t *ftt.Test) {
			resp, err := CreateAsset(ctx, asset1)
			assert.Loosely(t, err, should.BeNil)
			assertAssetEqual(t, resp, asset1)
			resp, err = GetAsset(ctx, asset1.GetName())
			assert.Loosely(t, err, should.BeNil)
			assertAssetEqual(t, resp, asset1)
		})
		t.Run("Get asset by non-existent name", func(t *ftt.Test) {
			resp, err := GetAsset(ctx, "C009009")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Get asset by invalid name", func(t *ftt.Test) {
			resp, err := GetAsset(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}

func TestGetAssetACL(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ctx = config.Use(ctx, &config.Config{
		ExperimentalAPI: &config.ExperimentalAPI{
			GetAssetACL: 99,
		},
	})
	asset1 := mockAsset("C002002", "krane", "cros4-row3-rack5-host4", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS4)
	CreateAsset(ctx, asset1)
	ftt.Run("GetAssetACL", t, func(t *ftt.Test) {
		t.Run("GetAssetACL - Happy path", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "chromo@chromium.org")
			mockRealmPerms(userCtx, ufsutil.AtlLabAdminRealm, ufsutil.RegistrationsGet)
			resp, err := GetAssetACL(userCtx, asset1.GetName())
			assert.Loosely(t, err, should.BeNil)
			assertAssetEqual(t, resp, asset1)
		})
		t.Run("GetAssetACL - No user", func(t *ftt.Test) {
			resp, err := GetAssetACL(ctx, asset1.GetName())
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Internal"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("GetAssetACL - No perms", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "chromo@chromium.org")
			resp, err := GetAssetACL(userCtx, asset1.GetName())
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("PermissionDenied"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("GetAssetACL - Missing perms", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "chromo@chromium.org")
			mockRealmPerms(userCtx, ufsutil.AtlLabAdminRealm, ufsutil.RegistrationsList)
			mockRealmPerms(userCtx, ufsutil.AtlLabAdminRealm, ufsutil.InventoriesList)
			mockRealmPerms(userCtx, ufsutil.AtlLabAdminRealm, ufsutil.InventoriesGet)
			resp, err := GetAssetACL(userCtx, asset1.GetName())
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("PermissionDenied"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("GetAssetACL - Missing realms", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "chromo@chromium.org")
			mockRealmPerms(userCtx, ufsutil.BrowserLabAdminRealm, ufsutil.RegistrationsGet)
			mockRealmPerms(userCtx, ufsutil.AcsLabAdminRealm, ufsutil.RegistrationsGet)
			mockRealmPerms(userCtx, ufsutil.SatLabInternalUserRealm, ufsutil.RegistrationsGet)
			mockRealmPerms(userCtx, ufsutil.AtlLabChromiumAdminRealm, ufsutil.RegistrationsGet)
			resp, err := GetAssetACL(userCtx, asset1.GetName())
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("PermissionDenied"))
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}

func TestDeleteAsset(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	asset1 := mockAsset("C001001", "krane", "cros4-row3-rack5-host4", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS4)
	ftt.Run("DeleteAsset", t, func(t *ftt.Test) {
		t.Run("Delete asset by existing name", func(t *ftt.Test) {
			resp, cerr := CreateAsset(ctx, asset1)
			assert.Loosely(t, cerr, should.BeNil)
			assertAssetEqual(t, resp, asset1)
			err := DeleteAsset(ctx, asset1.GetName())
			assert.Loosely(t, err, should.BeNil)
			res, err := GetAsset(ctx, asset1.GetName())
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Delete asset by non-existing name", func(t *ftt.Test) {
			err := DeleteAsset(ctx, "C000000")
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Delete asset - invalid name", func(t *ftt.Test) {
			err := DeleteAsset(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}

func TestListAssets(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	assets := make([]*ufspb.Asset, 0, 10)
	for i := range 10 {
		asset := mockAsset(fmt.Sprintf("C00000%d", i), "eve", fmt.Sprintf("cros4-row3-rack5-host%d", i), ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS4)
		resp, _ := CreateAsset(ctx, asset)
		assets = append(assets, resp)
	}
	ftt.Run("ListAssets", t, func(t *ftt.Test) {
		t.Run("List assets - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListAssets(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("List assets - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListAssets(ctx, 10, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(assets))
		})

		t.Run("List assets - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListAssets(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(assets[:3]))

			resp, _, err = ListAssets(ctx, 7, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(assets[3:]))
		})
	})
}

func TestListAssetsACL(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	assets := make([]*ufspb.Asset, 0, 20)
	for i := range 10 {
		asset := mockAsset(fmt.Sprintf("C00000%d", i), "eve", fmt.Sprintf("chromeos4-row3-rack5-host%d", i), ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS4)
		resp, _ := CreateAsset(ctx, asset)
		assets = append(assets, resp)
	}
	for i := range 10 {
		asset := mockAsset(fmt.Sprintf("C00001%d", i), "eve", fmt.Sprintf("chromeos5-row3-rack5-host%d", i), ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS5)
		resp, _ := CreateAsset(ctx, asset)
		assets = append(assets, resp)
	}
	// User bat has permissions in atl lab
	ctxBat := auth.WithState(ctx, &authtest.FakeState{
		Identity: "user:bat@man.com",
		IdentityPermissions: []authtest.RealmPermission{
			{
				Realm:      ufsutil.AtlLabAdminRealm,
				Permission: ufsutil.RegistrationsList,
			},
		},
	})
	// User spider has permissions in both atl and acs lab
	ctxSpider := auth.WithState(ctx, &authtest.FakeState{
		Identity: "user:spider@man.com",
		IdentityPermissions: []authtest.RealmPermission{
			{
				Realm:      ufsutil.AtlLabAdminRealm,
				Permission: ufsutil.RegistrationsList,
			},
			{
				Realm:      ufsutil.AcsLabAdminRealm,
				Permission: ufsutil.RegistrationsList,
			},
		},
	})
	// User mermaid has no permissions
	ctxMermaid := auth.WithState(ctx, &authtest.FakeState{
		Identity: "user:mermaid@man.com",
	})
	// Anonymous User has no permissions
	ctx = auth.WithState(ctx, &authtest.FakeState{})
	ftt.Run("ListAssetsACL", t, func(t *ftt.Test) {
		t.Run("List assets - AnonymousUser", func(t *ftt.Test) {
			// User anonymous sees nothing
			resp, nextPageToken, err := ListAssetsACL(ctx, 100, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List assets - filter on realm", func(t *ftt.Test) {
			// User bat cannot filter on realm
			resp, nextPageToken, err := ListAssetsACL(ctxBat, 3, "", map[string][]any{"realm": {"test"}}, false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List assets - happy path, no permissions", func(t *ftt.Test) {
			// User mermaid has no permissions, sees nothing
			resp, nextPageToken, err := ListAssetsACL(ctxMermaid, 100, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List assets - happy path, single realm", func(t *ftt.Test) {
			// User bat has permissions only for ATL and sees only those assets
			resp, nextPageToken, err := ListAssetsACL(ctxBat, 3, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(assets[:3]))
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)

			resp, nextPageToken, err = ListAssetsACL(ctxBat, 100, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(assets[3:10]))
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List assets - happy path, two realms", func(t *ftt.Test) {
			// User spider has permissions for both ATL and ACS. sees all the assets
			resp, nextPageToken, err := ListAssetsACL(ctxSpider, 3, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(assets[:3]))
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)

			resp, nextPageToken, err = ListAssetsACL(ctxSpider, 7, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(assets[3:10]))
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)

			resp, nextPageToken, err = ListAssetsACL(ctxSpider, 100, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(assets[10:]))
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List assets - happy path, two realms, zone filter", func(t *ftt.Test) {
			// User spider has permissions for both ATL and ACS. But filters for chromeos4 so sees only those assets
			resp, nextPageToken, err := ListAssetsACL(ctxSpider, 3, "", map[string][]any{"zone": {"ZONE_CHROMEOS4"}}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(assets[:3]))
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)

			resp, nextPageToken, err = ListAssetsACL(ctxSpider, 100, nextPageToken, map[string][]any{"zone": {"ZONE_CHROMEOS4"}}, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(assets[3:10]))
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List assets - happy path, one realms, wrong zone filter", func(t *ftt.Test) {
			// User bat has permissions for ATL. Attempts to filter on zone chromeos5
			resp, nextPageToken, err := ListAssetsACL(ctxBat, 100, "", map[string][]any{"zone": {"ZONE_CHROMEOS5"}}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
	})
}

func TestBatchUpdateAssets(t *testing.T) {
	t.Parallel()
	ftt.Run("BatchUpdateAssets", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		assets := make([]*ufspb.Asset, 0, 4)
		for i := range 4 {
			asset := mockAsset(fmt.Sprintf("C0000%d0", i), "eve", fmt.Sprintf("cros4-row3-rack5-host%d", i), ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS4)
			resp, err := CreateAsset(ctx, asset)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(asset))
			asset.Model = "krane"
			assets = append(assets, resp)
		}
		t.Run("BatchUpdate all assets", func(t *ftt.Test) {
			resp, err := BatchUpdateAssets(ctx, assets)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(assets))
		})
		t.Run("BatchUpdate existing and invalid assets", func(t *ftt.Test) {
			asset := mockAsset("", "krane", "cros4-row3-rack5-host4", ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS4)
			assets = append(assets, asset)
			resp, err := BatchUpdateAssets(ctx, assets)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}

func TestGetAllAssets(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("GetAllAssets", t, func(t *ftt.Test) {
		t.Run("GetAllAssets - Emtpy database", func(t *ftt.Test) {
			resp, err := GetAllAssets(ctx)
			assert.Loosely(t, len(resp), should.BeZero)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("GetAllAssets - non-empty database", func(t *ftt.Test) {
			assets := make([]*ufspb.Asset, 0, 10)
			for i := range 10 {
				asset := mockAsset(fmt.Sprintf("C000300%d", i), "eve", fmt.Sprintf("cros6-row7-rack5-host%d", i), ufspb.AssetType_DUT, ufspb.Zone_ZONE_CHROMEOS4)
				resp, err := CreateAsset(ctx, asset)
				assert.Loosely(t, err, should.BeNil)
				assets = append(assets, resp)
			}
			resp, err := GetAllAssets(ctx)
			assert.Loosely(t, len(resp), should.Equal(10))
			assert.Loosely(t, len(assets), should.Equal(10))
			assert.Loosely(t, assets, should.Match(resp))
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestQueryAssetByPropertyName(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	// Add some assets to the database to test
	assets := []*ufspb.Asset{
		{
			Name:  "A1",
			Type:  ufspb.AssetType_DUT,
			Model: "test",
			Location: &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CHROMEOS6,
				Rack: "chromeos6-row1-rack1",
			},
			Info: &ufspb.AssetInfo{
				Model:       "test",
				BuildTarget: "notest",
				Phase:       "14",
			},
			Tags: []string{"dut", "no-battery"},
		},
		{
			Name:  "B1",
			Type:  ufspb.AssetType_LABSTATION,
			Model: "labtest",
			Location: &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CHROMEOS6,
				Rack: "chromeos6-row1-rack1",
			},
			Info: &ufspb.AssetInfo{
				Model:       "labtest",
				BuildTarget: "labnotest",
				Phase:       "15",
			},
			Tags: []string{"labstation", "pending decommission"},
		},
		{
			Name: "S1",
			Type: ufspb.AssetType_SERVO,
			Location: &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CHROMEOS6,
				Rack: "chromeos6-row1-rack1",
			},
			Tags: []string{"servo v4", "stable"},
		},
		{
			Name:  "A2",
			Type:  ufspb.AssetType_DUT,
			Model: "test",
			Location: &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CHROMEOS2,
				Rack: "chromeos2-row2-rack1",
			},
			Info: &ufspb.AssetInfo{
				Model:       "test",
				BuildTarget: "notest",
				Phase:       "14",
			},
			Tags: []string{"dut", "no-battery"},
		},
		{
			Name:  "B2",
			Type:  ufspb.AssetType_LABSTATION,
			Model: "labtest",
			Location: &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CHROMEOS2,
				Rack: "chromeos2-row2-rack1",
			},
			Info: &ufspb.AssetInfo{
				Model:       "labtest",
				BuildTarget: "labnotest",
				Phase:       "15",
			},
			Tags: []string{"labstation", "pending decommission"},
		},
		{
			Name: "S2",
			Type: ufspb.AssetType_SERVO,
			Location: &ufspb.Location{
				Zone: ufspb.Zone_ZONE_CHROMEOS2,
				Rack: "chromeo2-row2-rack1",
			},
			Tags: []string{"servo v4", "stable"},
		},
	}
	for _, asset := range assets {
		CreateAsset(ctx, asset)
	}
	ftt.Run("QueryAssetByPropertyName", t, func(t *ftt.Test) {
		t.Run("QueryAssetByPropertyName - NotFound", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "rack", "chromeosπ-rowk-rackh", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("QueryAssetByPropertyName - type; keys only", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "type", "LABSTATION", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, resp[0].Name, should.Equal("B1"))
			assert.Loosely(t, resp[1].Name, should.Equal("B2"))
		})
		t.Run("QueryAssetByPropertyName - zone; keys only", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "zone", "ZONE_CHROMEOS6", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(3))
			assert.Loosely(t, resp[0].Name, should.Equal("A1"))
			assert.Loosely(t, resp[1].Name, should.Equal("B1"))
			assert.Loosely(t, resp[2].Name, should.Equal("S1"))
		})
		t.Run("QueryAssetByPropertyName - phase; keys only", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "phase", "14", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, resp[0].Name, should.Equal("A1"))
			assert.Loosely(t, resp[1].Name, should.Equal("A2"))
		})
		t.Run("QueryAssetByPropertyName - tags; keys only", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "tags", "servo v4", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, resp[0].Name, should.Equal("S1"))
			assert.Loosely(t, resp[1].Name, should.Equal("S2"))
		})
		t.Run("QueryAssetByPropertyName - model; keys only", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "model", "test", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, resp[0].Name, should.Equal("A1"))
			assert.Loosely(t, resp[1].Name, should.Equal("A2"))
		})
		t.Run("QueryAssetByPropertyName - build_target; keys only", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "build_target", "labnotest", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, resp[0].Name, should.Equal("B1"))
			assert.Loosely(t, resp[1].Name, should.Equal("B2"))
		})
		t.Run("QueryAssetByPropertyName - type", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "type", "LABSTATION", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, resp[0].Name, should.Equal("B1"))
			assert.Loosely(t, resp[1].Name, should.Equal("B2"))
		})
		t.Run("QueryAssetByPropertyName - zone", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "zone", "ZONE_CHROMEOS6", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(3))
			assert.Loosely(t, resp[0], should.Match(assets[0])) // DUT A1
			assert.Loosely(t, resp[1], should.Match(assets[1])) // Labstation B1
			assert.Loosely(t, resp[2], should.Match(assets[2])) // Servo S1
		})
		t.Run("QueryAssetByPropertyName - phase", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "phase", "14", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, resp[0], should.Match(assets[0])) // DUT A1
			assert.Loosely(t, resp[1], should.Match(assets[3])) // DUT A2
		})
		t.Run("QueryAssetByPropertyName - tags", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "tags", "servo v4", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, resp[0], should.Match(assets[2])) // Servo S1
			assert.Loosely(t, resp[1], should.Match(assets[5])) // Servo S2
		})
		t.Run("QueryAssetByPropertyName - model", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "model", "test", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, resp[0], should.Match(assets[0])) // DUT A1
			assert.Loosely(t, resp[1], should.Match(assets[3])) // DUT A2
		})
		t.Run("QueryAssetByPropertyName - build_target", func(t *ftt.Test) {
			resp, err := QueryAssetByPropertyName(ctx, "build_target", "labnotest", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(2))
			assert.Loosely(t, resp[0], should.Match(assets[1])) // Labstation B1
			assert.Loosely(t, resp[1], should.Match(assets[4])) // Labstation B2
		})
	})
}
