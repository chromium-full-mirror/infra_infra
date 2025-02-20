// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"

	. "go.chromium.org/infra/appengine/poros/api/entities"
	proto "go.chromium.org/infra/appengine/poros/api/proto"
)

func mockCreateAssetInstanceRequest(assetId string, statusValue int32) *proto.CreateAssetInstanceRequest {
	return &proto.CreateAssetInstanceRequest{
		AssetId: assetId,
		Status:  proto.DeploymentStatus_name[statusValue],
	}
}

func mockTriggerDeploymentRequest(entityType string, entityId string) *proto.TriggerDeploymentRequest {
	return &proto.TriggerDeploymentRequest{
		EntityType: entityType,
		EntityId:   entityId,
	}
}

func mockFetchLogsRequest(assetInstanceId string) *proto.FetchLogsRequest {
	return &proto.FetchLogsRequest{
		AssetInstanceId: assetInstanceId,
	}
}

func TestAssetInstanceCreateWithValidData(t *testing.T) {
	t.Parallel()
	request := mockCreateAssetInstanceRequest("Test AssetId", 0)
	ftt.Run("Create an AssetInstance in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetInstanceHandler{}
		model, err := handler.Create(ctx, request)
		assert.Loosely(t, err, should.BeNil)
		want := []string{request.GetAssetId(), request.GetStatus()}
		get := []string{model.GetAssetId(), model.GetStatus()}
		assert.Loosely(t, get, should.Match(want))
		assert.Loosely(t, model.CreatedAt.AsTime().Add(time.Hour*24*7), should.Match(model.DeleteAt.AsTime()))
	})
}

func TestAssetInstanceCreateWithInvalidAssetId(t *testing.T) {
	t.Parallel()
	request := mockCreateAssetInstanceRequest("", 0)
	ftt.Run("Create an AssetInstance with invalid assetId in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetInstanceHandler{}
		_, err := handler.Create(ctx, request)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetInstanceCreateWithInvalidStatus(t *testing.T) {
	t.Parallel()
	request := mockCreateAssetInstanceRequest("Test Asset Id", -1)
	ftt.Run("Create an AssetInstance with invalid status in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetInstanceHandler{}
		_, err := handler.Create(ctx, request)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetInstanceUpdateWithValidData(t *testing.T) {
	t.Parallel()
	createRequest := mockCreateAssetInstanceRequest("Test AssetId", 0)
	ftt.Run("Update an AssetInstance with valid data in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetInstanceHandler{}
		entity, err := handler.Create(ctx, createRequest)
		assert.Loosely(t, err, should.BeNil)

		// Update AssetInstance with some new value and the operation should not throw any error
		entity.AssetId = "Test AssetId Updated"
		entity.Status = proto.DeploymentStatus(1).String()
		timestamp := time.Now().UTC()
		entity.DeleteAt = timestamppb.New(timestamp)

		updateRequest := &proto.UpdateAssetInstanceRequest{
			AssetInstance: entity,
			UpdateMask:    &fieldmaskpb.FieldMask{Paths: []string{"asset_id", "status", "delete_at"}},
		}
		updatedEntity, err := handler.Update(ctx, updateRequest)
		assert.Loosely(t, err, should.BeNil)
		want := []string{"Test AssetId Updated", proto.DeploymentStatus_name[1]}
		get := []string{updatedEntity.GetAssetId(), updatedEntity.GetStatus()}
		assert.Loosely(t, get, should.Match(want))
		assert.Loosely(t, timestamp.Format(time.UnixDate), should.Equal(updatedEntity.DeleteAt.AsTime().Format(time.UnixDate)))

		// Retrieve the updated AssetInstance and make sure that the values were correctly updated
		getRequest := &proto.GetAssetInstanceRequest{
			AssetInstanceId: entity.GetAssetInstanceId(),
		}
		readEntity, err := handler.Get(ctx, getRequest)
		want = []string{"Test AssetId Updated", proto.DeploymentStatus_name[1]}
		get = []string{readEntity.GetAssetId(), readEntity.GetStatus()}
		assert.Loosely(t, get, should.Match(want))
		assert.Loosely(t, timestamp.Format(time.UnixDate), should.Equal(readEntity.DeleteAt.AsTime().Format(time.UnixDate)))
	})
}

func TestAssetInstanceUpdateWithInvalidAssetId(t *testing.T) {
	t.Parallel()
	createRequest := mockCreateAssetInstanceRequest("Test AssetId", 0)
	ftt.Run("Update an AssetInstance with invalid assetId in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetInstanceHandler{}
		entity, err := handler.Create(ctx, createRequest)
		assert.Loosely(t, err, should.BeNil)
		entity.AssetId = ""
		entity.Status = proto.DeploymentStatus(1).String()

		updateRequest := &proto.UpdateAssetInstanceRequest{
			AssetInstance: entity,
			UpdateMask:    &fieldmaskpb.FieldMask{Paths: []string{"asset_id", "status"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the AssetInstance as AssetId is empty
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAssetInstanceUpdateWithInvalidStatus(t *testing.T) {
	t.Parallel()
	createRequest := mockCreateAssetInstanceRequest("Test AssetId", 0)
	ftt.Run("Update an AssetInstance with invalid deployment status in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetInstanceHandler{}
		entity, err := handler.Create(ctx, createRequest)
		assert.Loosely(t, err, should.BeNil)
		entity.AssetId = "Test AssetId Updated"
		entity.Status = proto.DeploymentStatus(-1).String()

		updateRequest := &proto.UpdateAssetInstanceRequest{
			AssetInstance: entity,
			UpdateMask:    &fieldmaskpb.FieldMask{Paths: []string{"asset_id", "status"}},
		}
		readEntity, err := handler.Update(ctx, updateRequest)
		// should not save the AssetInstance as DeploymentStatus is invalid
		assert.Loosely(t, readEntity, should.BeNil)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestGetAssetInstanceWithValidData(t *testing.T) {
	createRequest := mockCreateAssetInstanceRequest("Test RAssetId", 0)
	ftt.Run("Get a AssetInstance based on id from datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetInstanceHandler{}
		entity, err := handler.Create(ctx, createRequest)
		assert.Loosely(t, err, should.BeNil)
		getRequest := &proto.GetAssetInstanceRequest{
			AssetInstanceId: entity.GetAssetInstanceId(),
		}
		readEntity, err := handler.Get(ctx, getRequest)
		assert.Loosely(t, err, should.BeNil)

		want := []string{entity.GetAssetId(), entity.GetStatus()}
		get := []string{readEntity.GetAssetId(), readEntity.GetStatus()}
		assert.Loosely(t, get, should.Match(want))
	})
}

func TestListAssetInstance(t *testing.T) {
	t.Parallel()
	createRequest1 := mockCreateAssetInstanceRequest("Test AssetId1", 0)
	createRequest2 := mockCreateAssetInstanceRequest("Test AssetId2", 1)
	ftt.Run("Get all AssetInstances from datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		handler := &AssetInstanceHandler{}
		_, err := handler.Create(ctx, createRequest1)
		assert.Loosely(t, err, should.BeNil)
		_, err = handler.Create(ctx, createRequest2)
		assert.Loosely(t, err, should.BeNil)
		// Verify
		response, err := handler.List(ctx, &proto.ListAssetInstancesRequest{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, response.GetAssetInstances(), should.HaveLength(2))
		asset_instances := response.GetAssetInstances()
		want := []string{"Test AssetId1", "Test AssetId2"}
		get := []string{asset_instances[0].GetAssetId(), asset_instances[1].GetAssetId()}
		sort.Strings(get)
		assert.Loosely(t, get, should.Match(want))
		want = []string{"STATUS_PENDING", "STATUS_RUNNING"}
		get = []string{asset_instances[0].GetStatus(), asset_instances[1].GetStatus()}
		sort.Strings(get)
		assert.Loosely(t, get, should.Match(want))
	})
}

func TestTriggerDeployment_TypeAssetInstance(t *testing.T) {
	t.Parallel()
	createRequest := mockCreateAssetInstanceRequest("Test AssetId", 0)
	ftt.Run("Test TriggerDeployment function for Type AssetInstance", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &AssetInstanceHandler{}
		entity, _ := handler.Create(ctx, createRequest)
		assert.Loosely(t, entity.ProjectId, should.BeEmpty)
		assert.Loosely(t, entity.Status, should.Equal("STATUS_PENDING"))
		triggerRequest := mockTriggerDeploymentRequest("AssetInstance", entity.AssetInstanceId)
		triggerDeploymentResponse, err := handler.TriggerDeployment(ctx, triggerRequest)
		assert.Loosely(t, err, should.BeNil)

		assetInstanceEntity, _ := getAssetInstanceById(ctx, entity.AssetInstanceId)
		assert.Loosely(t, assetInstanceEntity.ProjectId, should.NotBeEmpty)
		assert.Loosely(t, assetInstanceEntity.Status, should.Equal("STATUS_RUNNING"))

		want := []string{"Test AssetId", assetInstanceEntity.AssetInstanceId}
		get := []string{triggerDeploymentResponse.AssetId, triggerDeploymentResponse.AssetInstanceId}
		assert.Loosely(t, get, should.Match(want))
		assert.Loosely(t, triggerDeploymentResponse.ProjectId, should.NotBeEmpty)
		assert.Loosely(t, triggerDeploymentResponse.ProjectPrefix, should.NotBeEmpty)
	})
}

func TestTriggerDeployment_TypeAsset(t *testing.T) {
	t.Parallel()
	assetResourcesToSave := []*proto.AssetResourceModel{mockAssetResource("", "", "ResourceId", "Alias name")}
	assetRequest := mockCreateAssetRequest("Test Asset", "Test Asset description", "active_directory", assetResourcesToSave)
	ftt.Run("Test TriggerDeployment function for Type Asset", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		err := createDefaultResources(ctx)
		assert.Loosely(t, err, should.BeNil)
		handler := &AssetHandler{}
		asset, err := handler.Create(ctx, assetRequest)
		assert.Loosely(t, err, should.BeNil)

		assetInstanceHandler := &AssetInstanceHandler{}
		triggerRequest := mockTriggerDeploymentRequest("Asset", asset.Asset.AssetId)
		triggerDeploymentResponse, err := assetInstanceHandler.TriggerDeployment(ctx, triggerRequest)
		assert.Loosely(t, err, should.BeNil)

		assetInstance, err := getAssetInstanceById(ctx, triggerDeploymentResponse.AssetInstanceId)
		assert.Loosely(t, err, should.BeNil)

		assert.Loosely(t, assetInstance.ProjectId, should.NotBeEmpty)
		assert.Loosely(t, assetInstance.Status, should.Equal("STATUS_RUNNING"))

		want := []string{asset.Asset.AssetId, assetInstance.AssetInstanceId}
		get := []string{triggerDeploymentResponse.AssetId, triggerDeploymentResponse.AssetInstanceId}
		assert.Loosely(t, get, should.Match(want))
		assert.Loosely(t, triggerDeploymentResponse.ProjectId, should.NotBeEmpty)
		assert.Loosely(t, triggerDeploymentResponse.ProjectPrefix, should.NotBeEmpty)
	})
}

func TestDeploymentProject(t *testing.T) {
	t.Parallel()
	projectList := gcpProjectList()
	createData := [][]string{
		{"Test AssetId1", "STATUS_PENDING", ""},
		{"Test AssetId2", "STATUS_RUNNING", projectList[0]},
		{"Test AssetId3", "STATUS_COMPLETED", projectList[1]},
	}

	ftt.Run("Select a project for deployment", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		for _, data := range createData {
			id := uuid.New().String()
			entity := &AssetInstanceEntity{
				AssetInstanceId: id,
				AssetId:         data[0],
				Status:          data[1],
				ProjectId:       data[2],
				CreatedBy:       auth.CurrentUser(ctx).Email,
				CreatedAt:       time.Now().UTC(),
			}
			err := datastore.Put(ctx, entity)
			assert.Loosely(t, err, should.BeNil)
		}

		project, err := deploymentProject(ctx)
		assert.Loosely(t, err, should.BeNil)

		assert.Loosely(t, project, should.Equal(projectList[2]))
	})
}

func TestDeploymentProject_NoAvailableProject(t *testing.T) {
	t.Parallel()
	projectList := gcpProjectList()
	ftt.Run("Should throw an error since all projects are deployed at the moment", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		for i, project := range projectList {
			id := uuid.New().String()
			entity := &AssetInstanceEntity{
				AssetInstanceId: id,
				AssetId:         fmt.Sprintf("Test AssetId %v", i),
				Status:          "STATUS_RUNNING",
				ProjectId:       project,
				CreatedBy:       auth.CurrentUser(ctx).Email,
				CreatedAt:       time.Now().UTC(),
			}
			err := datastore.Put(ctx, entity)
			assert.Loosely(t, err, should.BeNil)
		}

		project, err := deploymentProject(ctx)
		assert.Loosely(t, project, should.BeEmpty)
		assert.Loosely(t, err.Error(), should.Equal("No Projects available at the moment"))
	})
}

func TestFetchLogs(t *testing.T) {
	t.Parallel()
	ftt.Run("Fetch Asset Logs", t, func(t *ftt.Test) {
		id := uuid.New().String()
		timestamp := time.Now().UTC()
		entity := &AssetInstanceEntity{
			AssetInstanceId: id,
			AssetId:         "Test Asset Id",
			Status:          "STATUS_RUNNING",
			Logs:            "My Test Logs",
			CreatedBy:       "test@test.com",
			CreatedAt:       timestamp,
			DeleteAt:        timestamp.Add(time.Hour * 24 * 7),
		}

		ctx := memory.Use(context.Background())
		err := datastore.Put(ctx, entity)
		assert.Loosely(t, err, should.BeNil)

		request := mockFetchLogsRequest(id)
		handler := &AssetInstanceHandler{}
		logs, err := handler.FetchLogs(ctx, request)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, logs.Logs, should.Equal("My Test Logs"))
	})
}
