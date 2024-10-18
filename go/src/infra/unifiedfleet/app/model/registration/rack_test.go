// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package registration

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/config"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/util"
)

func mockRack(id string, rackCapactiy int32, zone ufspb.Zone) *ufspb.Rack {
	return &ufspb.Rack{
		Name:       id,
		CapacityRu: rackCapactiy,
		Location: &ufspb.Location{
			Zone: zone,
		},
	}
}

func TestCreateRack(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	rack1 := mockRack("Rack-1", 5, ufspb.Zone_ZONE_CHROMEOS4)
	rack2 := mockRack("", 10, ufspb.Zone_ZONE_CHROMEOS4)
	ftt.Run("CreateRack", t, func(t *ftt.Test) {
		t.Run("Create new rack", func(t *ftt.Test) {
			resp, err := CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rack1))
		})
		t.Run("Create existing rack", func(t *ftt.Test) {
			resp, err := CreateRack(ctx, rack1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create rack - invalid ID", func(t *ftt.Test) {
			resp, err := CreateRack(ctx, rack2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateRack(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	rack1 := mockRack("Rack-1", 5, ufspb.Zone_ZONE_CHROMEOS4)
	rack2 := mockRack("Rack-1", 10, ufspb.Zone_ZONE_CHROMEOS4)
	rack3 := mockRack("Rack-3", 15, ufspb.Zone_ZONE_CHROMEOS4)
	rack4 := mockRack("", 20, ufspb.Zone_ZONE_CHROMEOS4)
	ftt.Run("UpdateRack", t, func(t *ftt.Test) {
		t.Run("Update existing rack", func(t *ftt.Test) {
			resp, err := CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rack1))

			resp, err = UpdateRack(ctx, rack2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rack2))
		})
		t.Run("Update non-existing rack", func(t *ftt.Test) {
			resp, err := UpdateRack(ctx, rack3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update rack - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateRack(ctx, rack4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetRack(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	rack1 := mockRack("Rack-1", 5, ufspb.Zone_ZONE_CHROMEOS4)
	ftt.Run("GetRack", t, func(t *ftt.Test) {
		t.Run("Get rack by existing ID", func(t *ftt.Test) {
			resp, err := CreateRack(ctx, rack1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rack1))
			resp, err = GetRack(ctx, "Rack-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rack1))
		})
		t.Run("Get rack by non-existing ID", func(t *ftt.Test) {
			resp, err := GetRack(ctx, "rack-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get rack - invalid ID", func(t *ftt.Test) {
			resp, err := GetRack(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetRackACL(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ctx = config.Use(ctx, &config.Config{
		ExperimentalAPI: &config.ExperimentalAPI{
			GetRackACL: 99,
		},
	})

	// realm "@internal:ufs/os-acs"
	rack := mockRack("rack-123", 100, ufspb.Zone_ZONE_CHROMEOS5)
	_, err := CreateRack(ctx, rack)
	if err != nil {
		t.Errorf("failed to create rack: %s", err)
	}

	ftt.Run("When a rack is created in a certain realm", t, func(t *ftt.Test) {

		t.Run("No user is rejected", func(t *ftt.Test) {
			resp, err := getRackACL(ctx, "rack-123")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Internal"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("A user without perms is rejected", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "email@google.com")
			resp, err := getRackACL(userCtx, "rack-123")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Permission"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("A user without the correct perm is rejected", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "email@google.com")
			mockRealmPerms(userCtx, util.AcsLabAdminRealm, util.ConfigurationsGet)
			mockRealmPerms(userCtx, util.AcsLabAdminRealm, util.RegistrationsCreate)
			resp, err := getRackACL(userCtx, "rack-123")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Permission"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("A user without the correct realm is rejected", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "email@google.com")
			mockRealmPerms(userCtx, util.SatLabInternalUserRealm, util.RegistrationsGet)
			resp, err := getRackACL(userCtx, "rack-123")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Permission"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("A user with the correct realm and permission is accepted", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "email@google.com")
			mockRealmPerms(userCtx, util.AcsLabAdminRealm, util.RegistrationsGet)
			resp, err := getRackACL(userCtx, "rack-123")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(rack))
		})
	})
}

func TestBatchGetRackACL(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ctx = config.Use(ctx, &config.Config{
		ExperimentalAPI: &config.ExperimentalAPI{
			GetRackACL: 99,
		},
	})

	// realm "@internal:ufs/os-acs"
	rack1 := mockRack("rack-1", 100, ufspb.Zone_ZONE_CHROMEOS5)
	_, err := CreateRack(ctx, rack1)
	if err != nil {
		t.Errorf("failed to create rack: %s", err)
	}

	// realm "@internal:ufs/os-atl"
	rack2 := mockRack("rack-2", 100, ufspb.Zone_ZONE_CHROMEOS4)
	_, err = CreateRack(ctx, rack2)
	if err != nil {
		t.Errorf("failed to create rack: %s", err)
	}

	ftt.Run("When two racks are created", t, func(t *ftt.Test) {
		t.Run("No user is rejected", func(t *ftt.Test) {
			resp, err := BatchGetRacksACL(ctx, []string{"rack-1", "rack-2"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Internal"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("A user without perms is rejected", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "email@google.com")
			resp, err := BatchGetRacksACL(userCtx, []string{"rack-1", "rack-2"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Permission"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("A user requesting racks without for at least one is rejected", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "email@google.com")
			mockRealmPerms(userCtx, util.AcsLabAdminRealm, util.RegistrationsGet)
			resp, err := BatchGetRacksACL(userCtx, []string{"rack-1", "rack-2"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Permission"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("A user requesting only racks they can access succeeds", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "email@google.com")
			mockRealmPerms(userCtx, util.AcsLabAdminRealm, util.RegistrationsGet)
			resp, err := BatchGetRacksACL(userCtx, []string{"rack-1"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.Rack{rack1}))
		})
		t.Run("A user with all realm perms can see racks in multiple realms", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "email@google.com")
			mockRealmPerms(userCtx, util.AcsLabAdminRealm, util.RegistrationsGet)
			mockRealmPerms(userCtx, util.AtlLabAdminRealm, util.RegistrationsGet)
			resp, err := BatchGetRacksACL(userCtx, []string{"rack-1", "rack-2"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match([]*ufspb.Rack{rack1, rack2}))
		})
	})
}

func TestListRacks(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	racks := make([]*ufspb.Rack, 0, 4)
	for i := 0; i < 4; i++ {
		rack1 := mockRack(fmt.Sprintf("rack-%d", i), 5, ufspb.Zone_ZONE_CHROMEOS4)
		resp, _ := CreateRack(ctx, rack1)
		racks = append(racks, resp)
	}
	ftt.Run("ListRacks", t, func(t *ftt.Test) {
		t.Run("List racks - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRacks(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List racks - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRacks(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(racks))
		})

		t.Run("List racks - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRacks(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(racks[:3]))

			resp, _, err = ListRacks(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(racks[3:]))
		})
	})
}

func TestListRacksACL(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	racks := make([]*ufspb.Rack, 0, 8)
	for i := 0; i < 4; i++ {
		rack := mockRack(fmt.Sprintf("rack-1%d", i), 4, ufspb.Zone_ZONE_CHROMEOS4)
		resp, _ := CreateRack(ctx, rack)
		racks = append(racks, resp)
	}
	for i := 0; i < 4; i++ {
		rack := mockRack(fmt.Sprintf("rack-2%d", i), 4, ufspb.Zone_ZONE_SFO36_BROWSER)
		resp, _ := CreateRack(ctx, rack)
		racks = append(racks, resp)
	}

	noPermUserCtx := mockUser(ctx, "none@google.com")

	somePermUserCtx := mockUser(ctx, "some@google.com")
	mockRealmPerms(somePermUserCtx, util.BrowserLabAdminRealm, util.RegistrationsList)

	allPermUserCtx := mockUser(ctx, "all@google.com")
	mockRealmPerms(allPermUserCtx, util.BrowserLabAdminRealm, util.RegistrationsList)
	mockRealmPerms(allPermUserCtx, util.AtlLabAdminRealm, util.RegistrationsList)

	ftt.Run("ListRacks", t, func(t *ftt.Test) {
		t.Run("List racks - anonymous call rejected", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRacksACL(ctx, 100, "", nil, false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List racks - filter on realm rejected", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRacksACL(allPermUserCtx, 100, "", map[string][]interface{}{"realm": nil}, false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List racks - happy path with no perms returns no results", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRacksACL(noPermUserCtx, 100, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List racks - happy path with partial perms returns partial results", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRacksACL(somePermUserCtx, 2, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(racks[4:6]))
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)

			resp2, nextPageToken2, err2 := ListRacksACL(somePermUserCtx, 100, nextPageToken, nil, false)
			assert.Loosely(t, err2, should.BeNil)
			assert.Loosely(t, resp2, should.Match(racks[6:]))
			assert.Loosely(t, nextPageToken2, should.BeEmpty)
		})
		t.Run("List racks - happy path with all perms returns all results", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRacksACL(allPermUserCtx, 4, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(racks[:4]))
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)

			resp2, nextPageToken2, err2 := ListRacksACL(allPermUserCtx, 100, nextPageToken, nil, false)
			assert.Loosely(t, err2, should.BeNil)
			assert.Loosely(t, resp2, should.Match(racks[4:]))
			assert.Loosely(t, nextPageToken2, should.BeEmpty)
		})
		t.Run("List racks - happy path with all perms and filters returns filtered results", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRacksACL(allPermUserCtx, 100, "", map[string][]interface{}{"zone": {"ZONE_CHROMEOS4"}}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(racks[:4]))
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List racks - happy path with all perms and filters with no matches returns no results", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRacksACL(allPermUserCtx, 100, "", map[string][]interface{}{"zone": {"fake"}}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
	})
}

func TestDeleteRack(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	rack1 := mockRack("rack-1", 5, ufspb.Zone_ZONE_CHROMEOS4)
	ftt.Run("DeleteRack", t, func(t *ftt.Test) {
		t.Run("Delete rack by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateRack(ctx, rack1)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(rack1))
			err := DeleteRack(ctx, "rack-1")
			assert.Loosely(t, err, should.BeNil)
			res, err := GetRack(ctx, "rack-1")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete rack by non-existing ID", func(t *ftt.Test) {
			err := DeleteRack(ctx, "rack-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete rack - invalid ID", func(t *ftt.Test) {
			err := DeleteRack(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestBatchUpdateRacks(t *testing.T) {
	t.Parallel()
	ftt.Run("BatchUpdateRacks", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		Racks := make([]*ufspb.Rack, 0, 4)
		for i := 0; i < 4; i++ {
			Rack1 := mockRack(fmt.Sprintf("Rack-%d", i), 10, ufspb.Zone_ZONE_CHROMEOS4)
			resp, err := CreateRack(ctx, Rack1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(Rack1))
			Racks = append(Racks, resp)
		}
		t.Run("BatchUpdate all Racks", func(t *ftt.Test) {
			resp, err := BatchUpdateRacks(ctx, Racks)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(Racks))
		})
		t.Run("BatchUpdate existing and invalid Racks", func(t *ftt.Test) {
			Rack5 := mockRack("", 10, ufspb.Zone_ZONE_CHROMEOS4)
			Racks = append(Racks, Rack5)
			resp, err := BatchUpdateRacks(ctx, Racks)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestQueryRackByPropertyName(t *testing.T) {
	t.Parallel()
	ftt.Run("QueryRackByPropertyName", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		dummyRack := &ufspb.Rack{
			Name: "Rack-1",
		}
		Rack1 := &ufspb.Rack{
			Name: "Rack-1",
			Rack: &ufspb.Rack_ChromeBrowserRack{
				ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
			},
			Tags: []string{"tag-1"},
		}
		resp, cerr := CreateRack(ctx, Rack1)
		assert.Loosely(t, cerr, should.BeNil)
		assert.Loosely(t, resp, should.Match(Rack1))

		Racks := make([]*ufspb.Rack, 0, 1)
		Racks = append(Racks, Rack1)

		dummyRacks := make([]*ufspb.Rack, 0, 1)
		dummyRacks = append(dummyRacks, dummyRack)
		t.Run("Query By existing Rack", func(t *ftt.Test) {
			resp, err := QueryRackByPropertyName(ctx, "tags", "tag-1", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(Racks))
		})
		t.Run("Query By non-existing Rack", func(t *ftt.Test) {
			resp, err := QueryRackByPropertyName(ctx, "tags", "tag-5", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Query By existing RackPrototype keysonly", func(t *ftt.Test) {
			resp, err := QueryRackByPropertyName(ctx, "tags", "tag-1", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dummyRacks))
		})
		t.Run("Query By non-existing RackPrototype", func(t *ftt.Test) {
			resp, err := QueryRackByPropertyName(ctx, "tags", "tag-5", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}
