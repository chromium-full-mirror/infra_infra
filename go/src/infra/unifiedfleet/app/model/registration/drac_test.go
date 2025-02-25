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

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
)

func mockDrac(id string) *ufspb.Drac {
	return &ufspb.Drac{
		Name: id,
	}
}

func TestCreateDrac(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	drac1 := mockDrac("Drac-1")
	drac2 := mockDrac("")
	ftt.Run("CreateDrac", t, func(t *ftt.Test) {
		t.Run("Create new drac", func(t *ftt.Test) {
			resp, err := CreateDrac(ctx, drac1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(drac1))
		})
		t.Run("Create existing drac", func(t *ftt.Test) {
			resp, err := CreateDrac(ctx, drac1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create drac - invalid ID", func(t *ftt.Test) {
			resp, err := CreateDrac(ctx, drac2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateDrac(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	drac1 := mockDrac("Drac-1")
	drac2 := mockDrac("Drac-1")
	drac3 := mockDrac("Drac-3")
	drac4 := mockDrac("")
	ftt.Run("UpdateDrac", t, func(t *ftt.Test) {
		t.Run("Update existing drac", func(t *ftt.Test) {
			resp, err := CreateDrac(ctx, drac1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(drac1))

			resp, err = UpdateDrac(ctx, drac2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(drac2))
		})
		t.Run("Update non-existing drac", func(t *ftt.Test) {
			resp, err := UpdateDrac(ctx, drac3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update drac - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateDrac(ctx, drac4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetDrac(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	drac1 := mockDrac("Drac-1")
	ftt.Run("GetDrac", t, func(t *ftt.Test) {
		t.Run("Get drac by existing ID", func(t *ftt.Test) {
			resp, err := CreateDrac(ctx, drac1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(drac1))
			resp, err = GetDrac(ctx, "Drac-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(drac1))
		})
		t.Run("Get drac by non-existing ID", func(t *ftt.Test) {
			resp, err := GetDrac(ctx, "drac-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get drac - invalid ID", func(t *ftt.Test) {
			resp, err := GetDrac(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListDracs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	dracs := make([]*ufspb.Drac, 0, 4)
	for i := range 4 {
		drac1 := mockDrac(fmt.Sprintf("drac-%d", i))
		resp, _ := CreateDrac(ctx, drac1)
		dracs = append(dracs, resp)
	}
	ftt.Run("ListDracs", t, func(t *ftt.Test) {
		t.Run("List dracs - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDracs(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List dracs - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDracs(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dracs))
		})

		t.Run("List dracs - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListDracs(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dracs[:3]))

			resp, _, err = ListDracs(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dracs[3:]))
		})
	})
}

func TestDeleteDrac(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	drac2 := mockDrac("drac-2")
	ftt.Run("DeleteDrac", t, func(t *ftt.Test) {
		t.Run("Delete drac successfully by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateDrac(ctx, drac2)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(drac2))

			err := DeleteDrac(ctx, "drac-2")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = GetDrac(ctx, "drac-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete drac by non-existing ID", func(t *ftt.Test) {
			err := DeleteDrac(ctx, "drac-3")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete drac - invalid ID", func(t *ftt.Test) {
			err := DeleteDrac(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestBatchUpdateDracs(t *testing.T) {
	t.Parallel()
	ftt.Run("BatchUpdateDracs", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		dracs := make([]*ufspb.Drac, 0, 4)
		for i := range 4 {
			drac1 := mockDrac(fmt.Sprintf("drac-%d", i))
			resp, err := CreateDrac(ctx, drac1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(drac1))
			dracs = append(dracs, resp)
		}
		t.Run("BatchUpdate all dracs", func(t *ftt.Test) {
			resp, err := BatchUpdateDracs(ctx, dracs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dracs))
		})
		t.Run("BatchUpdate existing and non-existing dracs", func(t *ftt.Test) {
			Drac5 := mockDrac("")
			dracs = append(dracs, Drac5)
			resp, err := BatchUpdateDracs(ctx, dracs)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestQueryDracByPropertyName(t *testing.T) {
	t.Parallel()
	ftt.Run("QueryDracByPropertyName", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		dummyDrac := &ufspb.Drac{
			Name: "drac-15",
		}
		drac1 := &ufspb.Drac{
			Name: "drac-15",
			SwitchInterface: &ufspb.SwitchInterface{
				Switch: "switch-1",
			},
		}
		resp, cerr := CreateDrac(ctx, drac1)
		assert.Loosely(t, cerr, should.BeNil)
		assert.Loosely(t, resp, should.Match(drac1))

		dracs := make([]*ufspb.Drac, 0, 1)
		dracs = append(dracs, drac1)

		dracs1 := make([]*ufspb.Drac, 0, 1)
		dracs1 = append(dracs1, dummyDrac)
		t.Run("Query By existing Switch keysonly", func(t *ftt.Test) {
			resp, err := QueryDracByPropertyName(ctx, "switch_id", "switch-1", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dracs1))
		})
		t.Run("Query By non-existing Switch", func(t *ftt.Test) {
			resp, err := QueryDracByPropertyName(ctx, "switch_id", "switch-2", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Query By existing Switch", func(t *ftt.Test) {
			resp, err := QueryDracByPropertyName(ctx, "switch_id", "switch-1", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(dracs))
		})
	})
}
