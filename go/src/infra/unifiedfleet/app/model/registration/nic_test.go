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

func mockNic(id string) *ufspb.Nic {
	return &ufspb.Nic{
		Name: id,
	}
}

func TestCreateNic(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	nic1 := mockNic("Nic-1")
	nic2 := mockNic("")
	ftt.Run("CreateNic", t, func(t *ftt.Test) {
		t.Run("Create new nic", func(t *ftt.Test) {
			resp, err := CreateNic(ctx, nic1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nic1))
		})
		t.Run("Create existing nic", func(t *ftt.Test) {
			resp, err := CreateNic(ctx, nic1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create nic - invalid ID", func(t *ftt.Test) {
			resp, err := CreateNic(ctx, nic2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateNic(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	nic1 := mockNic("Nic-1")
	nic2 := mockNic("Nic-1")
	nic3 := mockNic("Nic-3")
	nic4 := mockNic("")
	ftt.Run("UpdateNic", t, func(t *ftt.Test) {
		t.Run("Update existing nic", func(t *ftt.Test) {
			resp, err := CreateNic(ctx, nic1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nic1))

			resp, err = UpdateNic(ctx, nic2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nic2))
		})
		t.Run("Update non-existing nic", func(t *ftt.Test) {
			resp, err := UpdateNic(ctx, nic3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update nic - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateNic(ctx, nic4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetNic(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	nic1 := mockNic("Nic-1")
	ftt.Run("GetNic", t, func(t *ftt.Test) {
		t.Run("Get nic by existing ID", func(t *ftt.Test) {
			resp, err := CreateNic(ctx, nic1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nic1))
			resp, err = GetNic(ctx, "Nic-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nic1))
		})
		t.Run("Get nic by non-existing ID", func(t *ftt.Test) {
			resp, err := GetNic(ctx, "nic-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get nic - invalid ID", func(t *ftt.Test) {
			resp, err := GetNic(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListNics(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	nics := make([]*ufspb.Nic, 0, 4)
	for i := range 4 {
		nic1 := mockNic(fmt.Sprintf("nic-%d", i))
		resp, _ := CreateNic(ctx, nic1)
		nics = append(nics, resp)
	}
	ftt.Run("ListNics", t, func(t *ftt.Test) {
		t.Run("List nics - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListNics(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List nics - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListNics(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nics))
		})

		t.Run("List nics - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListNics(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nics[:3]))

			resp, _, err = ListNics(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nics[3:]))
		})
	})
}

func TestDeleteNic(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	nic2 := mockNic("nic-2")
	ftt.Run("DeleteNic", t, func(t *ftt.Test) {
		t.Run("Delete nic successfully by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateNic(ctx, nic2)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(nic2))

			err := DeleteNic(ctx, "nic-2")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = GetNic(ctx, "nic-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete nic by non-existing ID", func(t *ftt.Test) {
			err := DeleteNic(ctx, "nic-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete nic - invalid ID", func(t *ftt.Test) {
			err := DeleteNic(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestBatchUpdateNics(t *testing.T) {
	t.Parallel()
	ftt.Run("BatchUpdateNics", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		nics := make([]*ufspb.Nic, 0, 4)
		for i := range 4 {
			nic1 := mockNic(fmt.Sprintf("nic-%d", i))
			resp, err := CreateNic(ctx, nic1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nic1))
			nics = append(nics, resp)
		}
		t.Run("BatchUpdate all nics", func(t *ftt.Test) {
			resp, err := BatchUpdateNics(ctx, nics)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nics))
		})
		t.Run("BatchUpdate existing and non-existing nics", func(t *ftt.Test) {
			Nic5 := mockNic("")
			nics = append(nics, Nic5)
			resp, err := BatchUpdateNics(ctx, nics)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestQueryNicByPropertyName(t *testing.T) {
	t.Parallel()
	ftt.Run("QueryNicByPropertyName", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		dummyNic := &ufspb.Nic{
			Name: "nic-15",
		}
		nic1 := &ufspb.Nic{
			Name: "nic-15",
			SwitchInterface: &ufspb.SwitchInterface{
				Switch: "switch-1",
			},
		}
		resp, cerr := CreateNic(ctx, nic1)
		assert.Loosely(t, cerr, should.BeNil)
		assert.Loosely(t, resp, should.Match(nic1))

		nics := make([]*ufspb.Nic, 0, 1)
		nics = append(nics, nic1)

		nics1 := make([]*ufspb.Nic, 0, 1)
		nics1 = append(nics1, dummyNic)
		t.Run("Query By existing Switch keysonly", func(t *ftt.Test) {
			resp, err := QueryNicByPropertyName(ctx, "switch_id", "switch-1", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nics1))
		})
		t.Run("Query By non-existing Switch", func(t *ftt.Test) {
			resp, err := QueryNicByPropertyName(ctx, "switch_id", "switch-2", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Query By existing Switch", func(t *ftt.Test) {
			resp, err := QueryNicByPropertyName(ctx, "switch_id", "switch-1", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(nics))
		})
	})
}
