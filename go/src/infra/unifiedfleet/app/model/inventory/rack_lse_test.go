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

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
)

func mockRackLSE(id string) *ufspb.RackLSE {
	return &ufspb.RackLSE{
		Name: id,
	}
}

func TestCreateRackLSE(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	rackLSE1 := mockRackLSE("rackLSE-1")
	rackLSE2 := mockRackLSE("")
	ftt.Run("CreateRackLSE", t, func(t *ftt.Test) {
		t.Run("Create new rackLSE", func(t *ftt.Test) {
			resp, err := CreateRackLSE(ctx, rackLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSE1))
		})
		t.Run("Create existing rackLSE", func(t *ftt.Test) {
			resp, err := CreateRackLSE(ctx, rackLSE1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create rackLSE - invalid ID", func(t *ftt.Test) {
			resp, err := CreateRackLSE(ctx, rackLSE2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateRackLSE(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	rackLSE1 := mockRackLSE("rackLSE-1")
	rackLSE2 := mockRackLSE("rackLSE-1")
	rackLSE3 := mockRackLSE("rackLSE-3")
	rackLSE4 := mockRackLSE("")
	ftt.Run("UpdateRackLSE", t, func(t *ftt.Test) {
		t.Run("Update existing rackLSE", func(t *ftt.Test) {
			resp, err := CreateRackLSE(ctx, rackLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSE1))

			resp, err = UpdateRackLSE(ctx, rackLSE2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSE2))
		})
		t.Run("Update non-existing rackLSE", func(t *ftt.Test) {
			resp, err := UpdateRackLSE(ctx, rackLSE3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update rackLSE - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateRackLSE(ctx, rackLSE4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetRackLSE(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	rackLSE1 := mockRackLSE("rackLSE-1")
	ftt.Run("GetRackLSE", t, func(t *ftt.Test) {
		t.Run("Get rackLSE by existing ID", func(t *ftt.Test) {
			resp, err := CreateRackLSE(ctx, rackLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSE1))
			resp, err = GetRackLSE(ctx, "rackLSE-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSE1))
		})
		t.Run("Get rackLSE by non-existing ID", func(t *ftt.Test) {
			resp, err := GetRackLSE(ctx, "rackLSE-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get rackLSE - invalid ID", func(t *ftt.Test) {
			resp, err := GetRackLSE(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListRackLSEs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	rackLSEs := make([]*ufspb.RackLSE, 0, 4)
	for i := range 4 {
		rackLSE1 := mockRackLSE(fmt.Sprintf("rackLSE-%d", i))
		resp, _ := CreateRackLSE(ctx, rackLSE1)
		rackLSEs = append(rackLSEs, resp)
	}
	ftt.Run("ListRackLSEs", t, func(t *ftt.Test) {
		t.Run("List rackLSEs - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRackLSEs(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List rackLSEs - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRackLSEs(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEs))
		})

		t.Run("List rackLSEs - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRackLSEs(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEs[:3]))

			resp, _, err = ListRackLSEs(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEs[3:]))
		})
	})
}

func TestDeleteRackLSE(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	rackLSE1 := mockRackLSE("rackLSE-1")
	ftt.Run("DeleteRackLSE", t, func(t *ftt.Test) {
		t.Run("Delete rackLSE by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateRackLSE(ctx, rackLSE1)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSE1))
			err := DeleteRackLSE(ctx, "rackLSE-1")
			assert.Loosely(t, err, should.BeNil)
			res, err := GetRackLSE(ctx, "rackLSE-1")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete rackLSE by non-existing ID", func(t *ftt.Test) {
			err := DeleteRackLSE(ctx, "rackLSE-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete rackLSE - invalid ID", func(t *ftt.Test) {
			err := DeleteRackLSE(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestBatchUpdateRackLSEs(t *testing.T) {
	t.Parallel()
	ftt.Run("BatchUpdateRackLSEs", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		rackLSEs := make([]*ufspb.RackLSE, 0, 4)
		for i := range 4 {
			rackLSE1 := mockRackLSE(fmt.Sprintf("rackLSE-%d", i))
			resp, err := CreateRackLSE(ctx, rackLSE1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSE1))
			rackLSEs = append(rackLSEs, resp)
		}
		t.Run("BatchUpdate all rackLSEs", func(t *ftt.Test) {
			resp, err := BatchUpdateRackLSEs(ctx, rackLSEs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEs))
		})
		t.Run("BatchUpdate existing and invalid rackLSEs", func(t *ftt.Test) {
			rackLSE5 := mockRackLSE("")
			rackLSEs = append(rackLSEs, rackLSE5)
			resp, err := BatchUpdateRackLSEs(ctx, rackLSEs)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestQueryRackLSEByPropertyName(t *testing.T) {
	t.Parallel()
	ftt.Run("QueryRackLSEByPropertyName", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		dummyrackLSE := &ufspb.RackLSE{
			Name: "rackLSE-1",
		}
		rackLSE1 := &ufspb.RackLSE{
			Name:             "rackLSE-1",
			Racks:            []string{"rack-1", "rack-2"},
			RackLsePrototype: "rackLsePrototype-1",
		}
		resp, cerr := CreateRackLSE(ctx, rackLSE1)
		assert.Loosely(t, cerr, should.BeNil)
		assert.Loosely(t, resp, should.Match(rackLSE1))

		rackLSEs := make([]*ufspb.RackLSE, 0, 1)
		rackLSEs = append(rackLSEs, rackLSE1)

		dummyrackLSEs := make([]*ufspb.RackLSE, 0, 1)
		dummyrackLSEs = append(dummyrackLSEs, dummyrackLSE)
		t.Run("Query By existing Rack", func(t *ftt.Test) {
			resp, err := QueryRackLSEByPropertyName(ctx, "rack_ids", "rack-1", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEs))
		})
		t.Run("Query By non-existing Rack", func(t *ftt.Test) {
			resp, err := QueryRackLSEByPropertyName(ctx, "rack_ids", "rack-5", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Query By existing RackLsePrototype keysonly", func(t *ftt.Test) {
			resp, err := QueryRackLSEByPropertyName(ctx, "racklse_prototype_id", "rackLsePrototype-1", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dummyrackLSEs))
		})
		t.Run("Query By non-existing RackLsePrototype", func(t *ftt.Test) {
			resp, err := QueryRackLSEByPropertyName(ctx, "racklse_prototype_id", "rackLsePrototype-2", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}
