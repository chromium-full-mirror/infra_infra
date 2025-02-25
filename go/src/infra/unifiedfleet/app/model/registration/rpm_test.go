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

func mockRPM(id string) *ufspb.RPM {
	return &ufspb.RPM{
		Name: id,
	}
}

func TestCreateRPM(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	RPM1 := mockRPM("RPM-1")
	RPM2 := mockRPM("")
	ftt.Run("CreateRPM", t, func(t *ftt.Test) {
		t.Run("Create new RPM", func(t *ftt.Test) {
			resp, err := CreateRPM(ctx, RPM1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(RPM1))
		})
		t.Run("Create existing RPM", func(t *ftt.Test) {
			resp, err := CreateRPM(ctx, RPM1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create RPM - invalid ID", func(t *ftt.Test) {
			resp, err := CreateRPM(ctx, RPM2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateRPM(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	RPM1 := mockRPM("RPM-1")
	RPM2 := mockRPM("RPM-1")
	RPM3 := mockRPM("RPM-3")
	RPM4 := mockRPM("")
	ftt.Run("UpdateRPM", t, func(t *ftt.Test) {
		t.Run("Update existing RPM", func(t *ftt.Test) {
			resp, err := CreateRPM(ctx, RPM1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(RPM1))

			resp, err = UpdateRPM(ctx, RPM2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(RPM2))
		})
		t.Run("Update non-existing RPM", func(t *ftt.Test) {
			resp, err := UpdateRPM(ctx, RPM3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update RPM - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateRPM(ctx, RPM4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetRPM(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	RPM1 := mockRPM("RPM-1")
	ftt.Run("GetRPM", t, func(t *ftt.Test) {
		t.Run("Get RPM by existing ID", func(t *ftt.Test) {
			resp, err := CreateRPM(ctx, RPM1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(RPM1))
			resp, err = GetRPM(ctx, "RPM-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(RPM1))
		})
		t.Run("Get RPM by non-existing ID", func(t *ftt.Test) {
			resp, err := GetRPM(ctx, "RPM-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get RPM - invalid ID", func(t *ftt.Test) {
			resp, err := GetRPM(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListRPMs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	RPMs := make([]*ufspb.RPM, 0, 4)
	for i := range 4 {
		RPM1 := mockRPM(fmt.Sprintf("RPM-%d", i))
		resp, _ := CreateRPM(ctx, RPM1)
		RPMs = append(RPMs, resp)
	}
	ftt.Run("ListRPMs", t, func(t *ftt.Test) {
		t.Run("List RPMs - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRPMs(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List RPMs - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRPMs(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(RPMs))
		})

		t.Run("List RPMs - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListRPMs(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(RPMs[:3]))

			resp, _, err = ListRPMs(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(RPMs[3:]))
		})
	})
}

func TestDeleteRPM(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	RPM4 := mockRPM("RPM-4")
	ftt.Run("DeleteRPM", t, func(t *ftt.Test) {
		t.Run("Delete RPM successfully by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateRPM(ctx, RPM4)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(RPM4))

			err := DeleteRPM(ctx, "RPM-4")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = GetRPM(ctx, "RPM-4")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete RPM by non-existing ID", func(t *ftt.Test) {
			err := DeleteRPM(ctx, "RPM-6")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete RPM - invalid ID", func(t *ftt.Test) {
			err := DeleteRPM(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}
