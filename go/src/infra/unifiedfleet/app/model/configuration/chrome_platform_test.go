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

func mockChromePlatform(id, desc string) *ufspb.ChromePlatform {
	return &ufspb.ChromePlatform{
		Name:        id,
		Description: desc,
	}
}

func TestCreateChromePlatform(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	chromePlatform1 := mockChromePlatform("ChromePlatform-1", "Camera")
	chromePlatform2 := mockChromePlatform("", "Sensor")
	ftt.Run("CreateChromePlatform", t, func(t *ftt.Test) {
		t.Run("Create new chromePlatform", func(t *ftt.Test) {
			resp, err := CreateChromePlatform(ctx, chromePlatform1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform1))
		})
		t.Run("Create existing chromePlatform", func(t *ftt.Test) {
			resp, err := CreateChromePlatform(ctx, chromePlatform1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create chromePlatform - invalid ID", func(t *ftt.Test) {
			resp, err := CreateChromePlatform(ctx, chromePlatform2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateChromePlatform(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	chromePlatform1 := mockChromePlatform("ChromePlatform-1", "Camera")
	chromePlatform2 := mockChromePlatform("ChromePlatform-1", "Printer")
	chromePlatform3 := mockChromePlatform("ChromePlatform-3", "Sensor")
	chromePlatform4 := mockChromePlatform("", "Scanner")
	ftt.Run("UpdateChromePlatform", t, func(t *ftt.Test) {
		t.Run("Update existing chromePlatform", func(t *ftt.Test) {
			resp, err := CreateChromePlatform(ctx, chromePlatform1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform1))

			resp, err = UpdateChromePlatform(ctx, chromePlatform2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform2))
		})
		t.Run("Update non-existing chromePlatform", func(t *ftt.Test) {
			resp, err := UpdateChromePlatform(ctx, chromePlatform3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update chromePlatform - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateChromePlatform(ctx, chromePlatform4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetChromePlatform(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	chromePlatform1 := mockChromePlatform("ChromePlatform-1", "Camera")
	ftt.Run("GetChromePlatform", t, func(t *ftt.Test) {
		t.Run("Get chromePlatform by existing ID", func(t *ftt.Test) {
			resp, err := CreateChromePlatform(ctx, chromePlatform1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform1))
			resp, err = GetChromePlatform(ctx, "ChromePlatform-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform1))
		})
		t.Run("Get chromePlatform by non-existing ID", func(t *ftt.Test) {
			resp, err := GetChromePlatform(ctx, "chromePlatform-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get chromePlatform - invalid ID", func(t *ftt.Test) {
			resp, err := GetChromePlatform(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListChromePlatforms(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	chromePlatforms := make([]*ufspb.ChromePlatform, 0, 4)
	for i := 0; i < 4; i++ {
		chromePlatform1 := mockChromePlatform(fmt.Sprintf("chromePlatform-%d", i), "Camera")
		resp, _ := CreateChromePlatform(ctx, chromePlatform1)
		chromePlatforms = append(chromePlatforms, resp)
	}
	ftt.Run("ListChromePlatforms", t, func(t *ftt.Test) {
		t.Run("List chromePlatforms - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListChromePlatforms(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List chromePlatforms - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListChromePlatforms(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatforms))
		})

		t.Run("List chromePlatforms - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListChromePlatforms(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatforms[:3]))

			resp, _, err = ListChromePlatforms(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatforms[3:]))
		})
	})
}

func TestDeleteChromePlatform(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	chromePlatform2 := mockChromePlatform("chromePlatform-2", "Camera")
	ftt.Run("DeleteChromePlatform", t, func(t *ftt.Test) {
		t.Run("Delete chromePlatform successfully by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateChromePlatform(ctx, chromePlatform2)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform2))

			err := DeleteChromePlatform(ctx, "chromePlatform-2")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = GetChromePlatform(ctx, "chromePlatform-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete chromePlatform by non-existing ID", func(t *ftt.Test) {
			err := DeleteChromePlatform(ctx, "chromePlatform-2")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete chromePlatform - invalid ID", func(t *ftt.Test) {
			err := DeleteChromePlatform(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}
