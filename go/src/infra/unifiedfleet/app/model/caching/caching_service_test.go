// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package caching

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

func mockCachingService(name string) *ufspb.CachingService {
	return &ufspb.CachingService{
		Name: name,
	}
}

func TestCreateCachingService(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("CreateCachingService", t, func(t *ftt.Test) {
		t.Run("Create new CachingService", func(t *ftt.Test) {
			cs := mockCachingService("127.0.0.1")
			resp, err := CreateCachingService(ctx, cs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(cs))
		})
		t.Run("Create existing CachingService", func(t *ftt.Test) {
			cs1 := mockCachingService("128.0.0.1")
			CreateCachingService(ctx, cs1)

			resp, err := CreateCachingService(ctx, cs1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
	})
}

func TestBatchCreateCachingServices(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("BatchUpdateCachingServices", t, func(t *ftt.Test) {
		t.Run("Create new CachingService", func(t *ftt.Test) {
			cs := mockCachingService("128.0.0.1")
			resp, err := BatchUpdateCachingServices(ctx, []*ufspb.CachingService{cs})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp[0], should.Resemble(cs))
		})
	})
}

func TestGetCachingService(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	cs1 := mockCachingService("cs-1")
	ftt.Run("GetCachingService", t, func(t *ftt.Test) {
		t.Run("Get CachingService by existing name/ID", func(t *ftt.Test) {
			resp, err := CreateCachingService(ctx, cs1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(cs1))
			resp, err = GetCachingService(ctx, "cs-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(cs1))
		})
		t.Run("Get CachingService by non-existing name/ID", func(t *ftt.Test) {
			resp, err := GetCachingService(ctx, "cs-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get CachingService - invalid name/ID", func(t *ftt.Test) {
			resp, err := GetCachingService(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestDeleteCachingService(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	cs1 := mockCachingService("cs-1")
	CreateCachingService(ctx, cs1)
	ftt.Run("DeleteCachingService", t, func(t *ftt.Test) {
		t.Run("Delete CachingService successfully by existing ID", func(t *ftt.Test) {
			err := DeleteCachingService(ctx, "cs-1")
			assert.Loosely(t, err, should.BeNil)

			resp, err := GetCachingService(ctx, "cs-1")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete CachingService by non-existing ID", func(t *ftt.Test) {
			err := DeleteCachingService(ctx, "cs-5")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete CachingService - invalid ID", func(t *ftt.Test) {
			err := DeleteCachingService(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListCachingServices(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	cachingServices := make([]*ufspb.CachingService, 0, 4)
	for i := 0; i < 4; i++ {
		cs := mockCachingService(fmt.Sprintf("cs-%d", i))
		resp, _ := CreateCachingService(ctx, cs)
		cachingServices = append(cachingServices, resp)
	}
	ftt.Run("ListCachingServices", t, func(t *ftt.Test) {
		t.Run("List CachingServices - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListCachingServices(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List CachingServices - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListCachingServices(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(cachingServices))
		})

		t.Run("List CachingServices - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListCachingServices(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(cachingServices[:3]))

			resp, _, err = ListCachingServices(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(cachingServices[3:]))
		})
	})
}
