// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/config"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func mockMachineLSE(id string) *ufspb.MachineLSE {
	return &ufspb.MachineLSE{
		Name: id,
	}
}

func mockMachineLSEWithOwnership(id string, ownership *ufspb.OwnershipData) *ufspb.MachineLSE {
	machine := mockMachineLSE(id)
	machine.Ownership = ownership
	return machine
}

func mockMachineLSEWithRealm(id, realm string) *ufspb.MachineLSE {
	return &ufspb.MachineLSE{
		Name:  id,
		Realm: realm,
	}
}

func TestCreateMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	machineLSE1 := mockMachineLSE("machineLSE-1")
	machineLSE2 := mockMachineLSE("")

	ownershipData := &ufspb.OwnershipData{
		PoolName:         "pool1",
		SwarmingInstance: "test-swarming",
		Customer:         "test-customer",
		SecurityLevel:    "test-security-level",
	}
	machineLSE3Ownership := mockMachineLSEWithOwnership("machineLSE-3", ownershipData)
	ftt.Run("CreateMachineLSE", t, func(t *ftt.Test) {
		t.Run("Create new machineLSE", func(t *ftt.Test) {
			resp, err := CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE1))
		})
		t.Run("Create existing machineLSE", func(t *ftt.Test) {
			resp, err := CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create machineLSE - invalid ID", func(t *ftt.Test) {
			resp, err := CreateMachineLSE(ctx, machineLSE2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
		t.Run("Create machineLSE with ownership data - ownership is not saved", func(t *ftt.Test) {
			resp, err := CreateMachineLSE(ctx, machineLSE3Ownership)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE3Ownership))
			assert.Loosely(t, resp.Ownership, should.BeNil)
		})
	})
}

func TestUpdateMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	machineLSE1 := mockMachineLSE("machineLSE-1")
	machineLSE2 := mockMachineLSE("machineLSE-1")
	machineLSE2.Hostname = "Linux Server"
	machineLSE3 := mockMachineLSE("machineLSE-3")
	machineLSE4 := mockMachineLSE("")
	ftt.Run("UpdateMachineLSE", t, func(t *ftt.Test) {
		t.Run("Update existing machineLSE", func(t *ftt.Test) {
			resp, err := CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE1))

			resp, err = UpdateMachineLSE(ctx, machineLSE2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE2))
		})
		t.Run("Update non-existing machineLSE", func(t *ftt.Test) {
			resp, err := UpdateMachineLSE(ctx, machineLSE3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update machineLSE - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateMachineLSE(ctx, machineLSE4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateMachineOwnership(t *testing.T) {
	// Tests the ownership update scenarios for a machine
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ownershipData := &ufspb.OwnershipData{
		PoolName:         "pool1",
		SwarmingInstance: "test-swarming",
		Customer:         "test-customer",
		SecurityLevel:    "test-security-level",
	}
	ownershipData2 := &ufspb.OwnershipData{
		PoolName:         "pool2",
		SwarmingInstance: "test-swarming",
		Customer:         "test-customer",
		SecurityLevel:    "test-security-level",
	}
	machineLSE1 := mockMachineLSEWithOwnership("machineLSE-1", ownershipData)
	machineLSE2 := mockMachineLSEWithOwnership("machineLSE-1", ownershipData2)

	ftt.Run("UpdateMachine", t, func(t *ftt.Test) {
		t.Run("Update existing machine with ownership data", func(t *ftt.Test) {
			resp, err := CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE1))

			// Ownership data should be updated
			resp, err = UpdateMachineLSEOwnership(ctx, resp.Name, ownershipData)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetOwnership(), should.Match(ownershipData))

			// Regular Update calls should not override ownership data
			resp, err = UpdateMachineLSE(ctx, machineLSE2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE2))

			resp, err = GetMachineLSE(ctx, "machineLSE-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetOwnership(), should.Match(ownershipData))
		})
		t.Run("Update non-existing machine with ownership", func(t *ftt.Test) {
			resp, err := UpdateMachineLSEOwnership(ctx, "dummy", ownershipData)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update machine with ownership - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateMachineLSEOwnership(ctx, "", ownershipData)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	machineLSE1 := mockMachineLSE("machineLSE-1")
	ftt.Run("GetMachineLSE", t, func(t *ftt.Test) {
		t.Run("Get machineLSE by existing ID", func(t *ftt.Test) {
			resp, err := CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE1))
			resp, err = GetMachineLSE(ctx, "machineLSE-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE1))
		})
		t.Run("Get machineLSE by non-existing ID", func(t *ftt.Test) {
			resp, err := GetMachineLSE(ctx, "machineLSE-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get machineLSE - invalid ID", func(t *ftt.Test) {
			resp, err := GetMachineLSE(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetMachineLSEACL(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ctx = config.Use(ctx, &config.Config{
		ExperimentalAPI: &config.ExperimentalAPI{
			GetMachineLSEACL: 99,
		},
	})
	machineLSE1 := mockMachineLSEWithRealm("machineLSE-1", util.BrowserLabAdminRealm)
	CreateMachineLSE(ctx, machineLSE1)
	ftt.Run("GetMachineLSEACL", t, func(t *ftt.Test) {
		t.Run("GetMachineLSEACL - no user", func(t *ftt.Test) {
			resp, err := GetMachineLSEACL(ctx, "machineLSE-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Internal"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("GetMachineLSEACL - no perms", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "nombre@chromium.org")
			resp, err := GetMachineLSEACL(userCtx, "machineLSE-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("PermissionDenied"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("GetMachineLSEACL - missing perms", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "nombre@chromium.org")
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.RegistrationsList)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.RegistrationsGet)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.RegistrationsDelete)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.RegistrationsCreate)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.InventoriesList)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.InventoriesDelete)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.InventoriesCreate)
			resp, err := GetMachineLSEACL(userCtx, "machineLSE-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("PermissionDenied"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("GetMachineLSEACL - missing realms", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "nombre@chromium.org")
			mockRealmPerms(userCtx, util.AtlLabAdminRealm, util.InventoriesGet)
			mockRealmPerms(userCtx, util.AtlLabChromiumAdminRealm, util.InventoriesGet)
			mockRealmPerms(userCtx, util.AcsLabAdminRealm, util.InventoriesGet)
			mockRealmPerms(userCtx, util.AtlLabAdminRealm, util.InventoriesGet)
			mockRealmPerms(userCtx, util.SatLabInternalUserRealm, util.InventoriesGet)
			resp, err := GetMachineLSEACL(userCtx, "machineLSE-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("PermissionDenied"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("GetMachineLSEACL - happy path", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "nombre@chromium.org")
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.InventoriesGet)
			resp, err := GetMachineLSEACL(userCtx, "machineLSE-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(machineLSE1))
		})
	})
}

func TestListMachineLSEs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	machineLSEs := make([]*ufspb.MachineLSE, 0, 4)
	for i := range 4 {
		machineLSE1 := mockMachineLSE(fmt.Sprintf("machineLSE-%d", i))
		resp, _ := CreateMachineLSE(ctx, machineLSE1)
		machineLSEs = append(machineLSEs, resp)
	}
	ftt.Run("ListMachineLSEs", t, func(t *ftt.Test) {
		t.Run("List machineLSEs - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEs(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List machineLSEs - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEs(ctx, 4, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, resp, should.Match(machineLSEs))
		})

		t.Run("List machineLSEs - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEs(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs[:3]))

			resp, _, err = ListMachineLSEs(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs[3:]))
		})
	})
}

func TestListMachineLSEsACL(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	browserMachineLSEs := make([]*ufspb.MachineLSE, 0, 4)
	acsMachineLSEs := make([]*ufspb.MachineLSE, 0, 4)
	for i := range 4 {
		machineLSE1 := mockMachineLSEWithRealm(fmt.Sprintf("machineLSE-%d", i), util.BrowserLabAdminRealm)
		resp, _ := CreateMachineLSE(ctx, machineLSE1)
		browserMachineLSEs = append(browserMachineLSEs, resp)
	}
	for i := range 4 {
		machineLSE1 := mockMachineLSEWithRealm(fmt.Sprintf("machineLSE-%d", i+4), util.AcsLabAdminRealm)
		resp, _ := CreateMachineLSE(ctx, machineLSE1)
		acsMachineLSEs = append(acsMachineLSEs, resp)
	}
	// User bat has permissions in browser lab
	ctxBat := auth.WithState(ctx, &authtest.FakeState{
		Identity: "user:bat@man.com",
		IdentityPermissions: []authtest.RealmPermission{
			{
				Realm:      util.BrowserLabAdminRealm,
				Permission: util.InventoriesList,
			},
		},
	})
	// User spider has permissions in both browser and acs lab
	ctxSpider := auth.WithState(ctx, &authtest.FakeState{
		Identity: "user:spider@man.com",
		IdentityPermissions: []authtest.RealmPermission{
			{
				Realm:      util.BrowserLabAdminRealm,
				Permission: util.InventoriesList,
			},
			{
				Realm:      util.AcsLabAdminRealm,
				Permission: util.InventoriesList,
			},
		},
	})
	// User mermaid has no permissions
	ctxMermaid := auth.WithState(ctx, &authtest.FakeState{
		Identity: "user:mermaid@man.com",
	})
	// Anonymous User has no permissions
	ctx = auth.WithState(ctx, &authtest.FakeState{})
	ftt.Run("ListMachineLSEsACL", t, func(t *ftt.Test) {
		t.Run("List machineLSEs ACLed - anonymous user", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEsACL(ctx, 5, "", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("List machineLSEs ACLed - Filter on realm", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEsACL(ctxBat, 4, "", map[string][]any{"realm": {"test"}}, false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Cannot filter on realm"))
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})

		t.Run("List machineLSEs ACLed - Happy path, no permissions", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEsACL(ctxMermaid, 4, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})

		t.Run("List machineLSEs ACLed - Happy path, single realm", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEsACL(ctxBat, 10, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, resp, should.Match(browserMachineLSEs))
		})

		t.Run("List machineLSEs ACLed - Happy path, two realms", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEsACL(ctxSpider, 4, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, resp, should.Match(browserMachineLSEs))
			// Get the remaining machineLSEs
			resp, nextPageToken, err = ListMachineLSEsACL(ctxSpider, 10, nextPageToken, nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, resp, should.Match(acsMachineLSEs))
		})

	})
}

// TestListMachineLSEsByIdPrefixSearch tests the functionality for listing
// machineLSEs by searching for name/id prefix
func TestListMachineLSEsByIdPrefixSearch(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	machineLSEs := make([]*ufspb.MachineLSE, 0, 4)
	for i := range 4 {
		machineLSE1 := mockMachineLSE(fmt.Sprintf("machineLSE-%d", i))
		resp, _ := CreateMachineLSE(ctx, machineLSE1)
		machineLSEs = append(machineLSEs, resp)
	}
	ftt.Run("ListMachinesByIdPrefixSearch", t, func(t *ftt.Test) {
		t.Run("List machines - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEsByIdPrefixSearch(ctx, 5, "abc", "machineLSE-", false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List machines - Full listing with valid prefix and no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEsByIdPrefixSearch(ctx, 4, "", "machineLSE-", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs))
		})

		t.Run("List machines - Full listing with invalid prefix", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEsByIdPrefixSearch(ctx, 4, "", "machineLSE1-", false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("List machines - listing with valid prefix and pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEsByIdPrefixSearch(ctx, 3, "", "machineLSE-", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs[:3]))

			resp, _, err = ListMachineLSEsByIdPrefixSearch(ctx, 2, nextPageToken, "machineLSE-", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs[3:]))
		})
	})
}

func TestDeleteMachineLSE(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	machineLSE1 := mockMachineLSE("machineLSE-1")
	ownershipData := &ufspb.OwnershipData{
		PoolName:         "pool1",
		SwarmingInstance: "test-swarming",
	}
	machineLSE2 := mockMachineLSEWithOwnership("machineLSE-2", ownershipData)
	ftt.Run("DeleteMachineLSE", t, func(t *ftt.Test) {
		t.Run("Delete machineLSE by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE1))
			err := DeleteMachineLSE(ctx, "machineLSE-1")
			assert.Loosely(t, err, should.BeNil)
			res, err := GetMachineLSE(ctx, "machineLSE-1")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete machineLSE by non-existing ID", func(t *ftt.Test) {
			err := DeleteMachineLSE(ctx, "machineLSE-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete machineLSE - invalid ID", func(t *ftt.Test) {
			err := DeleteMachineLSE(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
		t.Run("Delete machineLSE - with ownershipdata", func(t *ftt.Test) {
			resp, cerr := CreateMachineLSE(ctx, machineLSE2)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE2))

			// Ownership data should be updated
			resp, err := UpdateMachineLSEOwnership(ctx, resp.Name, ownershipData)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetOwnership(), should.Match(ownershipData))

			err = DeleteMachineLSE(ctx, "machineLSE-2")
			assert.Loosely(t, err, should.BeNil)
			res, err := GetMachineLSE(ctx, "machineLSE-2")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
	})
}

func TestDeleteMachineLSEs(t *testing.T) {
	t.Parallel()
	ftt.Run("BatchDeleteMachineLSEs", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		machineLSEs := make([]*ufspb.MachineLSE, 0, 4)
		for i := range 4 {
			machineLSE1 := mockMachineLSE(fmt.Sprintf("machineLSE-%d", i))
			resp, err := CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE1))
			machineLSEs = append(machineLSEs, resp)
		}
		t.Run("BatchDelete all machineLSEs without realm", func(t *ftt.Test) {
			resp, err := QueryMachineLSEByPropertyName(ctx, "realm", "", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs))

			_ = DeleteMachineLSEs(ctx, []string{"machineLSE-1", "machineLSE-2"})
			_, err = GetMachineLSE(ctx, "machineLSE-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			lse, err := GetMachineLSE(ctx, "machineLSE-3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse, should.NotBeNil)

		})
	})
}

func TestBatchUpdateMachineLSEs(t *testing.T) {
	t.Parallel()
	ftt.Run("BatchUpdateMachineLSEs", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		machineLSEs := make([]*ufspb.MachineLSE, 0, 4)
		for i := range 4 {
			machineLSE1 := mockMachineLSE(fmt.Sprintf("machineLSE-%d", i))
			resp, err := CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSE1))
			machineLSEs = append(machineLSEs, resp)
		}
		t.Run("BatchUpdate all machineLSEs", func(t *ftt.Test) {
			resp, err := BatchUpdateMachineLSEs(ctx, machineLSEs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs))
		})
		t.Run("BatchUpdate existing and invalid machineLSEs", func(t *ftt.Test) {
			machineLSE5 := mockMachineLSE("")
			machineLSEs = append(machineLSEs, machineLSE5)
			resp, err := BatchUpdateMachineLSEs(ctx, machineLSEs)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestQueryMachineLSEByPropertyName(t *testing.T) {
	t.Parallel()
	ftt.Run("QueryMachineLSEByPropertyName", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		dummymachineLSE := &ufspb.MachineLSE{
			Name: "machineLSE-1",
		}

		dummymachineLSE2 := &ufspb.MachineLSE{
			Name:  "machineLSE-2",
			Realm: "",
		}
		resp2, cerr2 := CreateMachineLSE(ctx, dummymachineLSE2)
		assert.Loosely(t, cerr2, should.BeNil)
		assert.Loosely(t, resp2, should.Match(dummymachineLSE2))

		machineLSE1 := &ufspb.MachineLSE{
			Name:                "machineLSE-1",
			Machines:            []string{"machine-1", "machine-2"},
			MachineLsePrototype: "machineLsePrototype-1",
			LogicalZone:         ufspb.LogicalZone_LOGICAL_ZONE_DRILLZONE_SFO36,
			Realm:               "realm-1",
		}
		resp, cerr := CreateMachineLSE(ctx, machineLSE1)
		assert.Loosely(t, cerr, should.BeNil)
		assert.Loosely(t, resp, should.Match(machineLSE1))

		machineLSEs := make([]*ufspb.MachineLSE, 0, 1)
		machineLSEs = append(machineLSEs, machineLSE1)

		dummymachineLSEs := make([]*ufspb.MachineLSE, 0, 1)
		dummymachineLSEs = append(dummymachineLSEs, dummymachineLSE)
		t.Run("Query By existing Machine", func(t *ftt.Test) {
			resp, err := QueryMachineLSEByPropertyName(ctx, "machine_ids", "machine-1", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs))
		})
		t.Run("Query By non-existing Machine", func(t *ftt.Test) {
			resp, err := QueryMachineLSEByPropertyName(ctx, "machine_ids", "machine-5", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Query By existing MachineLsePrototype keysonly", func(t *ftt.Test) {
			resp, err := QueryMachineLSEByPropertyName(ctx, "machinelse_prototype_id", "machineLsePrototype-1", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dummymachineLSEs))
		})
		t.Run("Query By non-existing MachineLsePrototype", func(t *ftt.Test) {
			resp, err := QueryMachineLSEByPropertyName(ctx, "machinelse_prototype_id", "machineLsePrototype-2", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Query By LogicalZone", func(t *ftt.Test) {
			resp, err := QueryMachineLSEByPropertyName(ctx, "logical_zone", "LOGICAL_ZONE_DRILLZONE_SFO36", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs))
		})
		t.Run("Query By Realm - Valid Realm", func(t *ftt.Test) {
			resp, err := QueryMachineLSEByPropertyName(ctx, "realm", "realm-1", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs))
		})
		t.Run("Query By Realm - Empty Realm", func(t *ftt.Test) {
			resp, err := QueryMachineLSEByPropertyName(ctx, "realm", "", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.MachineLSE{dummymachineLSE2}))
		})
	})
}

func TestListAllMachineLSEs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	machineLSEs := make([]*ufspb.MachineLSE, 0, 4)
	for i := range 4 {
		machineLSE1 := mockMachineLSE(fmt.Sprintf("machineLSE-%d", i))
		machineLSE1.Description = "Test machineLSE"
		resp, _ := CreateMachineLSE(ctx, machineLSE1)
		machineLSEs = append(machineLSEs, resp)
	}
	ftt.Run("ListAllMachineLSEs", t, func(t *ftt.Test) {
		t.Run("List all machineLSEs - keysOnly", func(t *ftt.Test) {
			resp, _ := ListAllMachineLSEs(ctx, true)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, len(resp), should.Equal(4))
			for i := range 4 {
				assert.Loosely(t, resp[i].GetName(), should.Equal(fmt.Sprintf("machineLSE-%d", i)))
				assert.Loosely(t, resp[i].GetDescription(), should.BeEmpty)
			}
		})

		t.Run("List all machineLSEs", func(t *ftt.Test) {
			resp, _ := ListAllMachineLSEs(ctx, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(machineLSEs))
		})
	})
}
