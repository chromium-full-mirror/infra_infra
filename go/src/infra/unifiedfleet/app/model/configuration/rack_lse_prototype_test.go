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

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
)

func mockRackLSEPrototype(id string) *ufspb.RackLSEPrototype {
	return &ufspb.RackLSEPrototype{
		Name: id,
	}
}

func TestCreateRackLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	rackLSEPrototype1 := mockRackLSEPrototype("RackLSEPrototype-1")
	rackLSEPrototype2 := mockRackLSEPrototype("")
	ftt.Run("CreateRackLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Create new rackLSEPrototype", func(t *ftt.Test) {
			resp, err := CreateRackLSEPrototype(ctx, rackLSEPrototype1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype1))
		})
		t.Run("Create existing rackLSEPrototype", func(t *ftt.Test) {
			resp, err := CreateRackLSEPrototype(ctx, rackLSEPrototype1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create rackLSEPrototype - invalid ID", func(t *ftt.Test) {
			resp, err := CreateRackLSEPrototype(ctx, rackLSEPrototype2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateRackLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	rackLSEPrototype1 := mockRackLSEPrototype("RackLSEPrototype-1")
	rackLSEPrototype2 := mockRackLSEPrototype("RackLSEPrototype-1")
	rackLSEPrototype3 := mockRackLSEPrototype("RackLSEPrototype-3")
	rackLSEPrototype4 := mockRackLSEPrototype("")
	ftt.Run("UpdateRackLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Update existing rackLSEPrototype", func(t *ftt.Test) {
			resp, err := CreateRackLSEPrototype(ctx, rackLSEPrototype1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype1))

			resp, err = UpdateRackLSEPrototype(ctx, rackLSEPrototype2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype2))
		})
		t.Run("Update non-existing rackLSEPrototype", func(t *ftt.Test) {
			resp, err := UpdateRackLSEPrototype(ctx, rackLSEPrototype3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update rackLSEPrototype - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateRackLSEPrototype(ctx, rackLSEPrototype4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetRackLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	rackLSEPrototype1 := mockRackLSEPrototype("RackLSEPrototype-1")
	ftt.Run("GetRackLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Get rackLSEPrototype by existing ID", func(t *ftt.Test) {
			resp, err := CreateRackLSEPrototype(ctx, rackLSEPrototype1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype1))
			resp, err = GetRackLSEPrototype(ctx, "RackLSEPrototype-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype1))
		})
		t.Run("Get rackLSEPrototype by non-existing ID", func(t *ftt.Test) {
			resp, err := GetRackLSEPrototype(ctx, "rackLSEPrototype-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get rackLSEPrototype - invalid ID", func(t *ftt.Test) {
			resp, err := GetRackLSEPrototype(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListRackLSEPrototypes(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	rackLSEPrototypes := make([]*ufspb.RackLSEPrototype, 0, 4)
	for i := range 4 {
		rackLSEPrototype1 := mockRackLSEPrototype(fmt.Sprintf("rackLSEPrototype-%d", i))
		resp, _ := CreateRackLSEPrototype(ctx, rackLSEPrototype1)
		rackLSEPrototypes = append(rackLSEPrototypes, resp)
	}
	ftt.Run("ListRackLSEPrototypes", t, func(t *ftt.Test) {
		t.Run("List rackLSEPrototypes - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRackLSEPrototypes(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List rackLSEPrototypes - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRackLSEPrototypes(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototypes))
		})

		t.Run("List rackLSEPrototypes - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRackLSEPrototypes(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototypes[:3]))

			resp, _, err = ListRackLSEPrototypes(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototypes[3:]))
		})
	})
}

func TestDeleteRackLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	rackLSEPrototype2 := mockRackLSEPrototype("rackLSEPrototype-2")
	ftt.Run("DeleteRackLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Delete rackLSEPrototype successfully by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateRackLSEPrototype(ctx, rackLSEPrototype2)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype2))

			err := DeleteRackLSEPrototype(ctx, "rackLSEPrototype-2")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = GetRackLSEPrototype(ctx, "rackLSEPrototype-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete rackLSEPrototype by non-existing ID", func(t *ftt.Test) {
			err := DeleteRackLSEPrototype(ctx, "rackLSEPrototype-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete rackLSEPrototype - invalid ID", func(t *ftt.Test) {
			err := DeleteRackLSEPrototype(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}
