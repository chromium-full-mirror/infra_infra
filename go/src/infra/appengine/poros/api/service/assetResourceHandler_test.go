// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"sort"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/datastore"

	proto "infra/appengine/poros/api/proto"
)

func mockCreateAssetResourceRequest(assetId string, resourceId string, aliasName string) *proto.CreateAssetResourceRequest {
	return &proto.CreateAssetResourceRequest{
		AssetId:    assetId,
		ResourceId: resourceId,
		AliasName:  aliasName,
	}
}

func TestAssetResourceCreateWithValidData(t *testing.T) {
	t.Parallel()
	assetResourceRequest := mockCreateAssetResourceRequest("Test AsseId", "Test ResourceId", "Test AliasName")
	ftt.Run("Create an asset_resource entity in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetResourceHandler{}
		model, err := handler.Create(ctx, assetResourceRequest)
		assert.Loosely(t, err, should.BeNil)
		want := []string{assetResourceRequest.GetAssetId(), assetResourceRequest.GetResourceId(), assetResourceRequest.GetAliasName()}
		get := []string{model.GetAssetId(), model.GetResourceId(), model.GetAliasName()}
		assert.Loosely(t, get, should.Resemble(want))
	})
}

func TestAssetResourceCreateWithInvalidAssetId(t *testing.T) {
	t.Parallel()
	assetResourceRequest := mockCreateAssetResourceRequest("", "Test ResourceId", "Test AliasName")
	ftt.Run("Create an asset_resource entity in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetResourceHandler{}
		_, err := handler.Create(ctx, assetResourceRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetResourceCreateWithInvalidResourceId(t *testing.T) {
	t.Parallel()
	assetResourceRequest := mockCreateAssetResourceRequest("Test AssetId", "", "Test AliasName")
	ftt.Run("Create an asset_resource entity in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetResourceHandler{}
		_, err := handler.Create(ctx, assetResourceRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetResourceCreateWithInvalidAliasName(t *testing.T) {
	t.Parallel()
	assetResourceRequest := mockCreateAssetResourceRequest("Test AssetId", "Test ResourceId", "")
	ftt.Run("Create an asset_resource entity in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetResourceHandler{}
		_, err := handler.Create(ctx, assetResourceRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetResourceUpdateWithValidData(t *testing.T) {
	t.Parallel()
	assetResourceRequest := mockCreateAssetResourceRequest("Test AssetId", "Test ResourceId", "Test Alias Name")
	ftt.Run("Update an asset_resource with valid data in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetResourceHandler{}
		entity, err := handler.Create(ctx, assetResourceRequest)
		assert.Loosely(t, err, should.BeNil)

		// Update asset_resource with some new value and the operation should not throw any error
		entity.ResourceId = "Test ResourceId Updated"
		entity.AliasName = "Test Alias Name Updated"

		updateRequest := &proto.UpdateAssetResourceRequest{
			AssetResource: entity,
			UpdateMask:    &fieldmaskpb.FieldMask{Paths: []string{"resource_id", "alias_name"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		assert.Loosely(t, err, should.BeNil)

		// Retrieve the updated asset_resource and make sure that the values were correctly updated
		getRequest := &proto.GetAssetResourceRequest{
			AssetResourceId: entity.GetAssetResourceId(),
		}
		readEntity, err := handler.Get(ctx, getRequest)
		want := []string{"Test AssetId", "Test ResourceId Updated", "Test Alias Name Updated"}
		get := []string{readEntity.GetAssetId(), readEntity.GetResourceId(), readEntity.GetAliasName()}
		assert.Loosely(t, get, should.Resemble(want))
	})
}

func TestAssetResourceUpdateWithInvalidAsset(t *testing.T) {
	t.Parallel()
	assetResourceRequest := mockCreateAssetResourceRequest("Test AssetId", "Test ResourceId", "Test Alias Name")

	ftt.Run("Update an asset_resource with invalid asset_id in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetResourceHandler{}
		entity, err := handler.Create(ctx, assetResourceRequest)
		assert.Loosely(t, err, should.BeNil)
		entity.AssetId = ""
		entity.ResourceId = "Test ResourceId"
		entity.AliasName = "Test Alias Name Updated"

		updateRequest := &proto.UpdateAssetResourceRequest{
			AssetResource: entity,
			UpdateMask:    &fieldmaskpb.FieldMask{Paths: []string{"aliasId", "resourceId", "aliasName"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the asset_resource as resource_id is empty
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetResourceUpdateWithInvalidResource(t *testing.T) {
	t.Parallel()
	assetResourceRequest := mockCreateAssetResourceRequest("Test AssetId", "Test ResourceId", "Test Alias Name")

	ftt.Run("Update an asset_resource with invalid resource_id in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetResourceHandler{}
		entity, err := handler.Create(ctx, assetResourceRequest)
		assert.Loosely(t, err, should.BeNil)
		entity.ResourceId = ""
		entity.AliasName = "Test Alias Name Updated"

		updateRequest := &proto.UpdateAssetResourceRequest{
			AssetResource: entity,
			UpdateMask:    &fieldmaskpb.FieldMask{Paths: []string{"resourceId", "aliasName"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the asset_resource as resource_id is empty
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetResourceUpdateWithInvalidAliasName(t *testing.T) {
	t.Parallel()
	assetResourceRequest := mockCreateAssetResourceRequest("Test AssetId", "Test ResourceId", "Test Alias Name")

	ftt.Run("Update an asset_resource with invalid alias_name in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetResourceHandler{}
		entity, err := handler.Create(ctx, assetResourceRequest)
		assert.Loosely(t, err, should.BeNil)
		entity.ResourceId = "Test ResourceId"
		entity.AliasName = ""

		updateRequest := &proto.UpdateAssetResourceRequest{
			AssetResource: entity,
			UpdateMask:    &fieldmaskpb.FieldMask{Paths: []string{"resourceId", "aliasName"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the asset_resource as alias_name is empty
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestGetAssetResourceWithValidData(t *testing.T) {
	assetResourceRequest := mockCreateAssetResourceRequest("Test AssetId", "Test ResourceId", "Test AliasName")
	ftt.Run("Get an asset_resource based on id from datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetResourceHandler{}
		entity, err := handler.Create(ctx, assetResourceRequest)
		assert.Loosely(t, err, should.BeNil)
		getRequest := &proto.GetAssetResourceRequest{
			AssetResourceId: entity.GetAssetResourceId(),
		}
		readEntity, err := handler.Get(ctx, getRequest)
		assert.Loosely(t, err, should.BeNil)

		want := []string{entity.GetAssetResourceId(), entity.GetAssetId(), entity.GetResourceId(), entity.GetAliasName()}
		get := []string{readEntity.GetAssetResourceId(), readEntity.GetAssetId(), readEntity.GetResourceId(), readEntity.GetAliasName()}
		assert.Loosely(t, get, should.Resemble(want))
	})
}

func TestListAssetResources(t *testing.T) {
	assetResourceRequest1 := mockCreateAssetResourceRequest("Test AssetId1", "Test ResourceId1", "Test AliasName1")
	assetResourceRequest2 := mockCreateAssetResourceRequest("Test AssetId2", "Test ResourceId2", "Test AliasName2")
	ftt.Run("Get an asset_resource based on id from datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		handler := &AssetResourceHandler{}
		_, err := handler.Create(ctx, assetResourceRequest1)
		assert.Loosely(t, err, should.BeNil)
		_, err = handler.Create(ctx, assetResourceRequest2)
		assert.Loosely(t, err, should.BeNil)
		// Verify
		response, err := handler.List(ctx, &proto.ListAssetResourcesRequest{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, response.GetAssetResources(), should.HaveLength(2))
		asset_resources := response.GetAssetResources()
		want := []string{"Test AssetId1", "Test AssetId2"}
		get := []string{asset_resources[0].GetAssetId(), asset_resources[1].GetAssetId()}
		sort.Strings(get)
		assert.Loosely(t, get, should.Resemble(want))
		want = []string{"Test ResourceId1", "Test ResourceId2"}
		get = []string{asset_resources[0].GetResourceId(), asset_resources[1].GetResourceId()}
		sort.Strings(get)
		assert.Loosely(t, get, should.Resemble(want))
		want = []string{"Test AliasName1", "Test AliasName2"}
		get = []string{asset_resources[0].GetAliasName(), asset_resources[1].GetAliasName()}
		sort.Strings(get)
		assert.Loosely(t, get, should.Resemble(want))
	})
}
