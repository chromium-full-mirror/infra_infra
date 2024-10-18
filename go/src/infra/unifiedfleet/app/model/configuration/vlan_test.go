// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "infra/unifiedfleet/api/v1/models"
	. "infra/unifiedfleet/app/model/datastore"
)

func mockVlan(id string) *ufspb.Vlan {
	return &ufspb.Vlan{
		Name: id,
	}
}

func TestCreateVlan(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	vlan1 := mockVlan("Vlan-1")
	vlan2 := mockVlan("")
	ftt.Run("CreateVlan", t, func(t *ftt.Test) {
		t.Run("Create new vlan", func(t *ftt.Test) {
			resp, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan1))
		})
		t.Run("Create existing vlan", func(t *ftt.Test) {
			resp, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create vlan - invalid ID", func(t *ftt.Test) {
			resp, err := CreateVlan(ctx, vlan2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateVlan(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	vlan1 := mockVlan("Vlan-1")
	vlan2 := mockVlan("Vlan-1")
	vlan3 := mockVlan("Vlan-3")
	vlan4 := mockVlan("")
	ftt.Run("UpdateVlan", t, func(t *ftt.Test) {
		t.Run("Update existing vlan", func(t *ftt.Test) {
			resp, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan1))

			resp, err = UpdateVlan(ctx, vlan2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan2))
		})
		t.Run("Update non-existing vlan", func(t *ftt.Test) {
			resp, err := UpdateVlan(ctx, vlan3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update vlan - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateVlan(ctx, vlan4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetVlan(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	vlan1 := mockVlan("Vlan-1")
	ftt.Run("GetVlan", t, func(t *ftt.Test) {
		t.Run("Get vlan by existing ID", func(t *ftt.Test) {
			resp, err := CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan1))
			resp, err = GetVlan(ctx, "Vlan-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan1))
		})
		t.Run("Get vlan by non-existing ID", func(t *ftt.Test) {
			resp, err := GetVlan(ctx, "vlan-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get vlan - invalid ID", func(t *ftt.Test) {
			resp, err := GetVlan(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListVlans(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	vlans := make([]*ufspb.Vlan, 0, 4)
	for i := 0; i < 4; i++ {
		vlan1 := mockVlan(fmt.Sprintf("vlan-%d", i))
		resp, _ := CreateVlan(ctx, vlan1)
		vlans = append(vlans, resp)
	}
	ftt.Run("ListVlans", t, func(t *ftt.Test) {
		t.Run("List vlans - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListVlans(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List vlans - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListVlans(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlans))
		})

		t.Run("List vlans - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListVlans(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlans[:3]))

			resp, _, err = ListVlans(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlans[3:]))
		})
	})
}

func TestDeleteVlan(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	vlan2 := mockVlan("vlan-2")
	ftt.Run("DeleteVlan", t, func(t *ftt.Test) {
		t.Run("Delete vlan successfully by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateVlan(ctx, vlan2)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan2))

			err := DeleteVlan(ctx, "vlan-2")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = GetVlan(ctx, "vlan-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete vlan by non-existing ID", func(t *ftt.Test) {
			err := DeleteVlan(ctx, "vlan-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete vlan - invalid ID", func(t *ftt.Test) {
			err := DeleteVlan(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}
