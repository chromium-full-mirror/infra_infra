// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package state

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	"go.chromium.org/infra/unifiedfleet/app/config"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func mockDutState(id string) *chromeosLab.DutState {
	return &chromeosLab.DutState{
		Id:       &chromeosLab.ChromeOSDeviceID{Value: id},
		Hostname: fmt.Sprintf("hostname-%s", id),
		Servo:    chromeosLab.PeripheralState_NOT_CONNECTED,
	}
}

func mockDutStateWithRealm(id string, realm string) *chromeosLab.DutState {
	return &chromeosLab.DutState{
		Id:       &chromeosLab.ChromeOSDeviceID{Value: id},
		Hostname: fmt.Sprintf("hostname-%s", id),
		Servo:    chromeosLab.PeripheralState_NOT_CONNECTED,
		Realm:    realm,
	}
}

func TestUpdateDutState(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ftt.Run("UpdateDutState", t, func(t *ftt.Test) {
		t.Run("Update existing dut state", func(t *ftt.Test) {
			dutState1 := mockDutState("existing-dut-id")
			resp, err := UpdateDutStates(ctx, []*chromeosLab.DutState{dutState1})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp[0], should.Match(dutState1))

			dutState1.Servo = chromeosLab.PeripheralState_BAD_RIBBON_CABLE
			resp, err = UpdateDutStates(ctx, []*chromeosLab.DutState{dutState1})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp[0], should.Match(dutState1))

			getRes, err := GetDutState(ctx, "existing-dut-id")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, getRes, should.Match(dutState1))
		})
		t.Run("Update non-existing dut state", func(t *ftt.Test) {
			dutState1 := mockDutState("non-existing-dut-id")
			resp, err := UpdateDutStates(ctx, []*chromeosLab.DutState{dutState1})
			assert.Loosely(t, resp[0], should.Match(dutState1))
			assert.Loosely(t, err, should.BeNil)

			getRes, err := GetDutState(ctx, "non-existing-dut-id")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, getRes, should.Match(dutState1))
		})
		t.Run("Update dut state - invalid ID", func(t *ftt.Test) {
			dutState1 := mockDutState("")
			resp, err := UpdateDutStates(ctx, []*chromeosLab.DutState{dutState1})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsds.InternalError))
		})
	})
}

func TestDeleteDutState(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	ftt.Run("DeleteDutStates", t, func(t *ftt.Test) {
		t.Run("Delete dut state by existing ID", func(t *ftt.Test) {
			dutState1 := mockDutState("delete-dut-id1")
			dutState2 := mockDutState("delete-dut-id2")
			_, err := UpdateDutStates(ctx, []*chromeosLab.DutState{dutState1, dutState2})
			assert.Loosely(t, err, should.BeNil)

			resp, err := GetAllDutStates(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Passed(), should.HaveLength(2))

			resp2 := DeleteDutStates(ctx, []string{"delete-dut-id2"})
			assert.Loosely(t, resp2.Passed(), should.HaveLength(1))

			resp, err = GetAllDutStates(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Passed(), should.HaveLength(1))
			assert.Loosely(t, resp.Passed()[0].Data.(*chromeosLab.DutState).GetId().GetValue(), should.Equal("delete-dut-id1"))
			assert.Loosely(t, resp.Passed()[0].Data.(*chromeosLab.DutState).GetHostname(), should.Equal("hostname-delete-dut-id1"))
		})

		t.Run("Delete dut state by non-existing ID", func(t *ftt.Test) {
			resp := DeleteDutStates(ctx, []string{"delete-dut-non-existing-id"})
			assert.Loosely(t, resp.Failed(), should.HaveLength(1))
		})
	})
}

func TestGetDutStateACL(t *testing.T) {
	t.Parallel()
	// manually turn on config
	alwaysUseACLConfig := config.Config{
		ExperimentalAPI: &config.ExperimentalAPI{
			GetDutStateACL: 99,
		},
	}

	ctx := gaetesting.TestingContextWithAppID("go-test")
	ctx = config.Use(ctx, &alwaysUseACLConfig)

	dutState1 := mockDutStateWithRealm("dut-state-1", util.BrowserLabAdminRealm)
	dutState2 := mockDutStateWithRealm("dut-state-2", util.SatLabInternalUserRealm)
	_, err := UpdateDutStates(ctx, []*chromeosLab.DutState{dutState1, dutState2})
	if err != nil {
		fmt.Println("Not able to instantiate DutStates in TestGetDutStateACL")
	}

	ftt.Run("GetDutStateACL", t, func(t *ftt.Test) {
		t.Run("GetDutStateACL - no user", func(t *ftt.Test) {
			resp, err := GetDutStateACL(ctx, "dut-state-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Internal"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("GetDutStateACL - no perms", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "nombre@chromium.org")
			resp, err := GetDutStateACL(userCtx, "dut-state-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("PermissionDenied"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("GetDutStateACL - missing perms", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "nombre@chromium.org")
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.RegistrationsList)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.RegistrationsGet)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.RegistrationsDelete)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.RegistrationsCreate)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.InventoriesList)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.InventoriesDelete)
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.InventoriesCreate)
			resp, err := GetDutStateACL(userCtx, "dut-state-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("PermissionDenied"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("GetDutStateACL - missing realms", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "nombre@chromium.org")
			mockRealmPerms(userCtx, util.AtlLabAdminRealm, util.ConfigurationsGet)
			mockRealmPerms(userCtx, util.AtlLabChromiumAdminRealm, util.ConfigurationsGet)
			mockRealmPerms(userCtx, util.AcsLabAdminRealm, util.ConfigurationsGet)
			mockRealmPerms(userCtx, util.AtlLabAdminRealm, util.ConfigurationsGet)
			mockRealmPerms(userCtx, util.SatLabInternalUserRealm, util.ConfigurationsGet)
			resp, err := GetDutStateACL(userCtx, "dut-state-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("PermissionDenied"))
			assert.Loosely(t, resp, should.BeNil)
			// 2nd dut-state of different realm
			user2Ctx := mockUser(ctx, "name@chromium.org")
			mockRealmPerms(user2Ctx, util.BrowserLabAdminRealm, util.ConfigurationsGet)
			resp, err = GetDutStateACL(user2Ctx, "dut-state-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("PermissionDenied"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("GetDutStateACL - happy path", func(t *ftt.Test) {
			userCtx := mockUser(ctx, "nombre@chromium.org")
			mockRealmPerms(userCtx, util.BrowserLabAdminRealm, util.ConfigurationsGet)
			resp, err := GetDutStateACL(userCtx, "dut-state-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(dutState1))
			// 2nd dut-state of different realm
			user2Ctx := mockUser(ctx, "name@chromium.org")
			mockRealmPerms(user2Ctx, util.SatLabInternalUserRealm, util.ConfigurationsGet)
			resp, err = GetDutStateACL(user2Ctx, "dut-state-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(dutState2))
		})
	})
}
func TestListDutStatesACL(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	dutStates := make([]*chromeosLab.DutState, 0, 8)
	for i := range 4 {
		broswerDutState := mockDutStateWithRealm(fmt.Sprintf("dut-state-%d", i), util.BrowserLabAdminRealm)
		dutStates = append(dutStates, broswerDutState)
	}
	for i := range 4 {
		satlabDutState := mockDutStateWithRealm(fmt.Sprintf("dut-state-%d", i+4), util.SatLabInternalUserRealm)
		dutStates = append(dutStates, satlabDutState)
	}
	_, err := UpdateDutStates(ctx, dutStates)
	if err != nil {
		fmt.Println("Not able to instantiate DutStates in TestGetDutStateACL")
	}

	noPermUserCtx := mockUser(ctx, "none@google.com")

	somePermUserCtx := mockUser(ctx, "some@google.com")
	mockRealmPerms(somePermUserCtx, util.BrowserLabAdminRealm, util.ConfigurationsList)

	allPermUserCtx := mockUser(ctx, "all@google.com")
	mockRealmPerms(allPermUserCtx, util.BrowserLabAdminRealm, util.ConfigurationsList)
	mockRealmPerms(allPermUserCtx, util.SatLabInternalUserRealm, util.ConfigurationsList)
	ftt.Run("ListDutStates", t, func(t *ftt.Test) {
		t.Run("List DutStates - anonymous call rejected", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDutStatesACL(ctx, 100, "", nil, false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List DutStates - filter on realm rejected", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDutStatesACL(allPermUserCtx, 100, "", map[string][]interface{}{"realm": nil}, false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List DutStates - happy path with no perms returns no results", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDutStatesACL(noPermUserCtx, 100, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
		t.Run("List DutStates - happy path with partial perms returns partial results", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDutStatesACL(somePermUserCtx, 2, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dutStates[:2]))
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)

			resp2, nextPageToken2, err2 := ListDutStatesACL(somePermUserCtx, 100, nextPageToken, nil, false)
			assert.Loosely(t, err2, should.BeNil)
			assert.Loosely(t, resp2, should.Match(dutStates[2:4]))
			assert.Loosely(t, nextPageToken2, should.BeEmpty)
		})
		t.Run("List DutStates - happy path with all perms returns all results", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDutStatesACL(allPermUserCtx, 4, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dutStates[:4]))
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)

			resp2, nextPageToken2, err2 := ListDutStatesACL(allPermUserCtx, 100, nextPageToken, nil, false)
			assert.Loosely(t, err2, should.BeNil)
			assert.Loosely(t, resp2, should.Match(dutStates[4:]))
			assert.Loosely(t, nextPageToken2, should.BeEmpty)
		})
		t.Run("List DutStates - happy path with all perms and filters with no matches returns no results", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDutStatesACL(allPermUserCtx, 100, "", map[string][]interface{}{"hostname": {"fake"}}, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
		})
	})
}
