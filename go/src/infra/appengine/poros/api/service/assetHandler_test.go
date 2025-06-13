// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"sort"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/datastore"

	proto "go.chromium.org/infra/appengine/poros/api/proto"
)

func mockCreateAssetRequest(name string, description string, asset_type string, assetResourcesToSave []*proto.AssetResourceModel) *proto.CreateAssetRequest {
	return &proto.CreateAssetRequest{
		Name:                 name,
		Description:          description,
		AssetType:            asset_type,
		AssetResourcesToSave: assetResourcesToSave,
	}
}

func mockAssetResource(assetResourceId string, assetId string, resourceId string, aliasName string) *proto.AssetResourceModel {
	return &proto.AssetResourceModel{
		AssetResourceId: assetResourceId,
		AssetId:         assetId,
		ResourceId:      resourceId,
		AliasName:       aliasName,
	}
}

func mockGetAssetConfigRequest(assetId string) *proto.GetAssetConfigurationRequest {
	return &proto.GetAssetConfigurationRequest{AssetId: assetId}
}

func mockGetHostConfigRequest(resourceIds []string) *proto.GetHostConfigurationRequest {
	return &proto.GetHostConfigurationRequest{ResourceIds: resourceIds}
}

func TestAssetCreateWithValidData(t *testing.T) {
	t.Parallel()
	assetResourcesToSave := []*proto.AssetResourceModel{mockAssetResource("", "", "Test ResourceId", "Test Alias Name")}
	assetRequest := mockCreateAssetRequest("Test Asset", "Test Asset description", "active_directory", assetResourcesToSave)
	ftt.Run("Create an asset in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		err := createDefaultResources(ctx)
		assert.Loosely(t, err, should.BeNil)
		handler := &AssetHandler{}
		response, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.BeNil)
		want := []string{assetRequest.GetName(), assetRequest.GetDescription(), assetRequest.GetAssetType()}
		get := []string{response.GetAsset().GetName(), response.GetAsset().GetDescription(), response.GetAsset().GetAssetType()}
		assert.Loosely(t, get, should.Match(want))
		assert.Loosely(t, response.GetAssetResources(), should.HaveLength(1))
		assert.Loosely(t, response.GetAssetResources(), should.Match(assetResourcesToSave))
	})
}

func TestAssetCreateWithInvalidName(t *testing.T) {
	t.Parallel()
	assetRequest := mockCreateAssetRequest("", "Test Asset description", "active_directory", []*proto.AssetResourceModel{})
	ftt.Run("Create an asset with invalid name in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		handler := &AssetHandler{}
		_, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetCreateWithInvalidDescription(t *testing.T) {
	t.Parallel()
	assetRequest := mockCreateAssetRequest("Test Asset", "", "active_directory", []*proto.AssetResourceModel{})
	ftt.Run("Create an asset with invalid description in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		handler := &AssetHandler{}
		_, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetCreateWithInvalidAssetType(t *testing.T) {
	t.Parallel()
	assetRequest := mockCreateAssetRequest("Test Asset", "Test Asset description", "", []*proto.AssetResourceModel{})
	ftt.Run("Create an asset with invalid asset_type in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		handler := &AssetHandler{}
		_, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetCreateWithInvalidAssetResource(t *testing.T) {
	t.Parallel()
	assetRequest := mockCreateAssetRequest("Test Name", "Test Description", "active_directory",
		[]*proto.AssetResourceModel{mockAssetResource("", "", "", "")})
	ftt.Run("Create an asset with invalid asset_resource in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		handler := &AssetHandler{}
		_, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetUpdateWithValidData(t *testing.T) {
	t.Parallel()
	assetResourcesToSave := []*proto.AssetResourceModel{mockAssetResource("", "", "ResourceId", "Alias name")}
	assetResourcesToDelete := []*proto.AssetResourceModel{}
	assetRequest := mockCreateAssetRequest("Test Asset", "Test Asset description", "active_directory", assetResourcesToSave)
	ftt.Run("Update an asset with valid data in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		handler := &AssetHandler{}
		createAssetesponse, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.BeNil)

		// Update asset with some new value and the operation should not throw any error
		entity := createAssetesponse.GetAsset()
		entity.Name = "Test Asset Name Updated"
		entity.Description = "Test Asset Description Updated"
		entity.AssetType = "active_directory_updated"
		assetResourcesToSave[0].ResourceId = "ResourceId Updated"
		assetResourcesToSave[0].AliasName = "Alias Name Updated"

		updateRequest := &proto.UpdateAssetRequest{
			Asset:                   entity,
			AssetUpdateMask:         &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "asset_type"}},
			AssetResourcesToSave:    assetResourcesToSave,
			AssetResourceUpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"resource_id", "alias_name"}},
			AssetResourcesToDelete:  assetResourcesToDelete,
		}
		updateAssetResponse, err := handler.Update(ctx, updateRequest)
		assert.Loosely(t, err, should.BeNil)

		// Retrieve the updated asset and make sure that the values were correctly updated
		getRequest := &proto.GetAssetRequest{
			AssetId: entity.GetAssetId(),
		}
		readEntity, err := handler.Get(ctx, getRequest)
		assert.Loosely(t, err, should.BeNil)
		want := []string{"Test Asset Name Updated", "Test Asset Description Updated", "active_directory_updated"}
		get := []string{readEntity.GetName(), readEntity.GetDescription(), readEntity.GetAssetType()}
		assert.Loosely(t, get, should.Match(want))

		//Retrieve the updated asset_resource to make sure the update goes through
		assetResourceHanlder := &AssetResourceHandler{}
		req := &proto.GetAssetResourceRequest{AssetResourceId: updateAssetResponse.GetAssetResources()[0].GetAssetResourceId()}

		readAssetResource, err := assetResourceHanlder.Get(ctx, req)
		assert.Loosely(t, err, should.BeNil)
		want = []string{"ResourceId Updated", "Alias Name Updated"}
		get = []string{readAssetResource.GetResourceId(), readAssetResource.GetAliasName()}
		assert.Loosely(t, get, should.Match(want))
	})
}

func TestAssetUpdateWithInvalidName(t *testing.T) {
	t.Parallel()
	assetRequest := mockCreateAssetRequest("Test Asset Name", "Test Asset description", "active_directory", []*proto.AssetResourceModel{})
	ftt.Run("Update an asset with invalid name in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		handler := &AssetHandler{}
		response, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.BeNil)
		entity := response.GetAsset()
		entity.Name = ""
		entity.Description = "Test Asset Description"
		entity.AssetType = "active_directory"

		updateRequest := &proto.UpdateAssetRequest{
			Asset:                   entity,
			AssetUpdateMask:         &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "asset_type"}},
			AssetResourceUpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"resource_id", "alias_name"}},
			AssetResourcesToSave:    []*proto.AssetResourceModel{},
			AssetResourcesToDelete:  []*proto.AssetResourceModel{},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the asset as name is empty
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetUpdateWithInvalidDescription(t *testing.T) {
	t.Parallel()
	assetRequest := mockCreateAssetRequest("Test Asset Name", "Test Asset description", "active_directory", []*proto.AssetResourceModel{})
	ftt.Run("Update an asset with invalid name in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		handler := &AssetHandler{}
		response, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.BeNil)
		entity := response.GetAsset()
		entity.Name = "Test Asset Name"
		entity.Description = ""
		entity.AssetType = "active_directory"

		updateRequest := &proto.UpdateAssetRequest{
			Asset:                   entity,
			AssetUpdateMask:         &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "asset_type"}},
			AssetResourceUpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"resource_id", "alias_name"}},
			AssetResourcesToSave:    []*proto.AssetResourceModel{},
			AssetResourcesToDelete:  []*proto.AssetResourceModel{},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the asset as name is empty
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetUpdateWithInvalidAssetType(t *testing.T) {
	t.Parallel()
	assetRequest := mockCreateAssetRequest("Test Asset Name", "Test Asset description", "active_directory", []*proto.AssetResourceModel{})
	ftt.Run("Update an asset with invalid name in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		handler := &AssetHandler{}
		response, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.BeNil)
		entity := response.GetAsset()
		entity.Name = "Test Asset Name"
		entity.Description = "Test Asset description"
		entity.AssetType = ""

		updateRequest := &proto.UpdateAssetRequest{
			Asset:                   entity,
			AssetUpdateMask:         &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "asset_type"}},
			AssetResourceUpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"resource_id", "alias_name"}},
			AssetResourcesToSave:    []*proto.AssetResourceModel{},
			AssetResourcesToDelete:  []*proto.AssetResourceModel{},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the asset as name is empty
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetUpdateWithInvalidAssetResource(t *testing.T) {
	t.Parallel()
	assetResourcesToSave := []*proto.AssetResourceModel{mockAssetResource("", "", "ResourceId", "Alias name")}
	assetResourcesToDelete := []*proto.AssetResourceModel{}
	assetRequest := mockCreateAssetRequest("Test Asset Name", "Test Asset description", "active_directory", assetResourcesToSave)
	ftt.Run("Update an asset with invalid name in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		handler := &AssetHandler{}
		response, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.BeNil)
		assetResourcesToSave[0].ResourceId = ""

		updateRequest := &proto.UpdateAssetRequest{
			Asset:                   response.GetAsset(),
			AssetUpdateMask:         &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "asset_type"}},
			AssetResourceUpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"resource_id", "alias_name"}},
			AssetResourcesToSave:    assetResourcesToSave,
			AssetResourcesToDelete:  assetResourcesToDelete,
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the asset as the asset_resource is invalid
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestGetAssetWithValidData(t *testing.T) {
	assetRequest := mockCreateAssetRequest("Test Asset", "Test Asset description", "active_directory", []*proto.AssetResourceModel{})
	ftt.Run("Get an assets based on id from datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		handler := &AssetHandler{}
		response, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.BeNil)
		getRequest := &proto.GetAssetRequest{
			AssetId: response.GetAsset().GetAssetId(),
		}
		readEntity, err := handler.Get(ctx, getRequest)
		assert.Loosely(t, err, should.BeNil)

		want := []string{response.GetAsset().GetName(), response.GetAsset().GetDescription(), response.GetAsset().GetAssetType()}
		get := []string{readEntity.GetName(), readEntity.GetDescription(), readEntity.GetAssetType()}
		assert.Loosely(t, get, should.Match(want))
	})
}

func TestListAssets(t *testing.T) {
	t.Parallel()
	assetRequest1 := mockCreateAssetRequest("Test Asset1", "Test Asset description", "active_directory", []*proto.AssetResourceModel{})
	assetRequest2 := mockCreateAssetRequest("Test Asset2", "Test Asset description", "active_directory", []*proto.AssetResourceModel{})
	ftt.Run("Get all assets from datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		handler := &AssetHandler{}
		_, err := handler.Create(ctx, assetRequest1)
		assert.Loosely(t, err, should.BeNil)
		_, err = handler.Create(ctx, assetRequest2)
		assert.Loosely(t, err, should.BeNil)
		// Verify
		response, err := handler.List(ctx, &proto.ListAssetsRequest{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, response.GetAssets(), should.HaveLength(2))
		assets := response.GetAssets()
		want := []string{"Test Asset1", "Test Asset2"}
		get := []string{assets[0].GetName(), assets[1].GetName()}
		sort.Strings(get)
		assert.Loosely(t, get, should.Match(want))
	})
}

func TestAssetConfigWithValidDetails(t *testing.T) {
	t.Parallel()

	ftt.Run("Test Generated Asset Configuration with valid data", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)
		asset, _, _, err := generateAssetAndResources(ctx)
		assert.Loosely(t, err, should.BeNil)

		handler := &AssetHandler{}
		mockRequest := mockGetAssetConfigRequest(asset.AssetId)
		res, err := handler.GetAssetConfiguration(ctx, mockRequest)
		assert.Loosely(t, err, should.BeNil)

		assetConfig := &proto.AssetConfiguration{}
		err = protojson.Unmarshal([]byte(res.Config), assetConfig)
		assert.Loosely(t, err, should.BeNil)

		assert.Loosely(t, assetConfig.AssetId, should.Equal(asset.AssetId))

		assert.Loosely(t, len(assetConfig.Resources), should.Equal(1))
	})
}

func TestHostConfigWithValidDetails(t *testing.T) {
	t.Parallel()

	ftt.Run("Test Generated Host Configuration with valid data", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)

		_, _, resource, err := generateAssetAndResources(ctx)
		assert.Loosely(t, err, should.BeNil)

		handler := &AssetHandler{}
		mockRequest := mockGetHostConfigRequest([]string{resource.ResourceId})
		res, err := handler.GetHostConfiguration(ctx, mockRequest)
		assert.Loosely(t, err, should.BeNil)

		hostConfig := &proto.HostConfiguration{}
		err = protojson.Unmarshal([]byte(res.Config), hostConfig)
		assert.Loosely(t, err, should.BeNil)

		assert.Loosely(t, len(hostConfig.Resources), should.Equal(1))
	})
}

func TestDefaultResourcesWithActiveDirectory(t *testing.T) {
	t.Parallel()

	ftt.Run("Test GetDefaultResources with Asset type active_directory", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		createDefaultResources(ctx)

		_, _, _, err := generateAssetAndResources(ctx)
		assert.Loosely(t, err, should.BeNil)

		handler := &AssetHandler{}
		resourceRequest := &proto.GetDefaultResourcesRequest{AssetType: "active_directory"}
		response, err := handler.GetDefaultResources(ctx, resourceRequest)
		defaultResources := response.GetAssetResources()
		assert.Loosely(t, err, should.BeNil)
		want := []string{"primary", "test1.com", "domain-controll", "Joe"}
		get := []string{defaultResources[0].GetAliasName(), defaultResources[1].GetAliasName(), defaultResources[2].GetAliasName(), defaultResources[3].GetAliasName()}
		assert.Loosely(t, defaultResources, should.HaveLength(4))
		assert.Loosely(t, want, should.Match(get))
	})
}

func createDefaultResources(ctx context.Context) error {
	resHandler := &ResourceHandler{}
	resourceData := [][]string{
		{"Network", "Resource of Type Network", "network", "", "", ""},
		{"User", "Resource of Type User", "user", "", "", ""},
		{"win2008r2", "Resource of Domain Controller Machine", "domain_controller_machine", "windows_machine", "project-1", "family-1"},
		{"Active Directory Domain", "Resource of Type Active Directory Domain", "ad_domain", "", "", ""},
	}

	for _, data := range resourceData {
		resourceRequest := mockCreateResourceRequest(data[0], data[1], data[2], data[3], data[4], data[5])
		_, err := resHandler.Create(ctx, resourceRequest)
		if err != nil {
			return err
		}
	}

	return nil
}

func generateAssetAndResources(ctx context.Context) (*proto.AssetModel, *proto.AssetResourceModel, *proto.ResourceModel, error) {
	resHandler := &ResourceHandler{}
	resourceRequest := mockCreateResourceRequest("win2016mock", "mock windows machine", "ad_joined_machine", "windows_machine", "project-2", "family-2")
	resource, err := resHandler.Create(ctx, resourceRequest)
	if err != nil {
		return nil, nil, nil, err
	}

	assetHandler := &AssetHandler{}
	var assetsToSave []*proto.AssetResourceModel
	assetRequest := mockCreateAssetRequest("Test Asset Name", "Test Asset description", "active_directory", assetsToSave)
	asset, err := assetHandler.Create(ctx, assetRequest)
	if err != nil {
		return nil, nil, nil, err
	}

	assetResHandler := &AssetResourceHandler{}
	assetResourceRequest := mockCreateAssetResourceRequest(asset.Asset.AssetId, resource.ResourceId, "win-1")
	assetResource, err := assetResHandler.Create(ctx, assetResourceRequest)
	if err != nil {
		return nil, nil, nil, err
	}

	_, err = getByAssetResourceId(ctx, assetResource.AssetResourceId)
	if err != nil {
		return nil, nil, nil, err
	}

	return asset.Asset, assetResource, resource, nil
}

func TestAssetDeleteWithValidData(t *testing.T) {
	t.Parallel()
	createRequest := mockCreateAssetRequest("Test Asset1", "Test Asset description", "active_directory", []*proto.AssetResourceModel{})
	ftt.Run("Create an asset in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetHandler{}
		model, err := handler.Create(ctx, createRequest)
		assert.Loosely(t, err, should.BeNil)
		deleteRequest := &proto.DeleteAssetRequest{AssetId: model.Asset.AssetId}
		_, err = handler.Delete(ctx, deleteRequest)
		assert.Loosely(t, err, should.BeNil)
		getRequest := &proto.GetAssetRequest{AssetId: model.Asset.AssetId}
		readEntity, err := handler.Get(ctx, getRequest)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, readEntity.Deleted, should.Equal(true))
	})
}
