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

func mockMachineLSEPrototype(id string) *ufspb.MachineLSEPrototype {
	return &ufspb.MachineLSEPrototype{
		Name: id,
	}
}

func TestCreateMachineLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	machineLSEPrototype1 := mockMachineLSEPrototype("MachineLSEPrototype-1")
	machineLSEPrototype2 := mockMachineLSEPrototype("")
	ftt.Run("CreateMachineLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Create new machineLSEPrototype", func(t *ftt.Test) {
			resp, err := CreateMachineLSEPrototype(ctx, machineLSEPrototype1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype1))
		})
		t.Run("Create existing machineLSEPrototype", func(t *ftt.Test) {
			resp, err := CreateMachineLSEPrototype(ctx, machineLSEPrototype1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create machineLSEPrototype - invalid ID", func(t *ftt.Test) {
			resp, err := CreateMachineLSEPrototype(ctx, machineLSEPrototype2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateMachineLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	machineLSEPrototype1 := mockMachineLSEPrototype("MachineLSEPrototype-1")
	machineLSEPrototype2 := mockMachineLSEPrototype("MachineLSEPrototype-1")
	machineLSEPrototype3 := mockMachineLSEPrototype("MachineLSEPrototype-3")
	machineLSEPrototype4 := mockMachineLSEPrototype("")
	ftt.Run("UpdateMachineLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Update existing machineLSEPrototype", func(t *ftt.Test) {
			resp, err := CreateMachineLSEPrototype(ctx, machineLSEPrototype1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype1))

			resp, err = UpdateMachineLSEPrototype(ctx, machineLSEPrototype2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype2))
		})
		t.Run("Update non-existing machineLSEPrototype", func(t *ftt.Test) {
			resp, err := UpdateMachineLSEPrototype(ctx, machineLSEPrototype3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update machineLSEPrototype - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateMachineLSEPrototype(ctx, machineLSEPrototype4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetMachineLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	machineLSEPrototype1 := mockMachineLSEPrototype("MachineLSEPrototype-1")
	ftt.Run("GetMachineLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Get machineLSEPrototype by existing ID", func(t *ftt.Test) {
			resp, err := CreateMachineLSEPrototype(ctx, machineLSEPrototype1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype1))
			resp, err = GetMachineLSEPrototype(ctx, "MachineLSEPrototype-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype1))
		})
		t.Run("Get machineLSEPrototype by non-existing ID", func(t *ftt.Test) {
			resp, err := GetMachineLSEPrototype(ctx, "machineLSEPrototype-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get machineLSEPrototype - invalid ID", func(t *ftt.Test) {
			resp, err := GetMachineLSEPrototype(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListMachineLSEPrototypes(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	machineLSEPrototypes := make([]*ufspb.MachineLSEPrototype, 0, 4)
	for i := 0; i < 4; i++ {
		machineLSEPrototype1 := mockMachineLSEPrototype(fmt.Sprintf("machineLSEPrototype-%d", i))
		resp, _ := CreateMachineLSEPrototype(ctx, machineLSEPrototype1)
		machineLSEPrototypes = append(machineLSEPrototypes, resp)
	}
	ftt.Run("ListMachineLSEPrototypes", t, func(t *ftt.Test) {
		t.Run("List machineLSEPrototypes - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEPrototypes(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List machineLSEPrototypes - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEPrototypes(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototypes))
		})

		t.Run("List machineLSEPrototypes - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListMachineLSEPrototypes(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototypes[:3]))

			resp, _, err = ListMachineLSEPrototypes(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototypes[3:]))
		})
	})
}

func TestDeleteMachineLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	machineLSEPrototype2 := mockMachineLSEPrototype("machineLSEPrototype-2")
	ftt.Run("DeleteMachineLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Delete machineLSEPrototype successfully by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateMachineLSEPrototype(ctx, machineLSEPrototype2)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype2))

			err := DeleteMachineLSEPrototype(ctx, "machineLSEPrototype-2")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = GetMachineLSEPrototype(ctx, "machineLSEPrototype-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete machineLSEPrototype by non-existing ID", func(t *ftt.Test) {
			err := DeleteMachineLSEPrototype(ctx, "machineLSEPrototype-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete machineLSEPrototype - invalid ID", func(t *ftt.Test) {
			err := DeleteMachineLSEPrototype(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}
