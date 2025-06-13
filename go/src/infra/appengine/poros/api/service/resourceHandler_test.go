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

	proto "go.chromium.org/infra/appengine/poros/api/proto"
)

func mockCreateResourceRequest(name string, description string, Type string, operatingSystem string, imageProject string, imageFamily string) *proto.CreateResourceRequest {
	return &proto.CreateResourceRequest{
		Name:            name,
		Description:     description,
		Type:            Type,
		OperatingSystem: operatingSystem,
		ImageProject:    imageProject,
		ImageFamily:     imageFamily,
	}
}

func TestResourceCreateWithValidData(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource", "Test Resource description", "ad_joined_machine", "windows_machine", "image-project", "image-family")
	ftt.Run("Create a resource in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		model, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.BeNil)
		want := []string{resourceRequest.GetName(), resourceRequest.GetDescription(), resourceRequest.GetType(), resourceRequest.GetImageProject(), resourceRequest.GetImageFamily()}
		get := []string{model.GetName(), model.GetDescription(), model.GetType(), model.GetImageProject(), model.GetImageFamily()}
		assert.Loosely(t, get, should.Match(want))
		assert.Loosely(t, model.Deleted, should.Equal(false))
	})
}

func TestResourceCreateWithInvalidName(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("", "Test Resource description", "ad_joined_machine", "windows_machine", "image-project", "image-family")
	ftt.Run("Create a resource with invalid name in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		handler := &ResourceHandler{}
		_, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.NotBeNil)

		resourceRequest := mockCreateResourceRequest("Test Resource Name", "Test Resource description", "ad_joined_machine", "windows_machine", "image-project", "image-family")
		entity, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, entity.Name, should.Equal("Test Resource Name"))

		// Creating another resource with duplicate name; the Create operation should fail
		_, err = handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, err.Error(), should.Equal("Resource name must be unique"))
	})
}

func TestResourceCreateWithInvalidDescription(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource", "", "ad_joined_machine", "windows_machine", "image-project", "image-family")
	ftt.Run("Create a resource with invalid description in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		_, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestResourceCreateWithInvalidType(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource", "Test Resource Description", "", "windows_machine", "image-project", "image-family")
	ftt.Run("Create a resource with invalid type in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		_, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestResourceCreateOfTypeMachineWithInvalidOperatingSystem(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource", "Test Resource Description", "ad_joined_machine", "", "image-project", "image-family")
	ftt.Run("Create a resource with invalid operating system in datastore if type if machine", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		_, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestResourceCreateOfTypeMachineWithInvalidImageProject(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource", "Test Resource Description", "ad_joined_machine", "windows_machine", "", "image-family")
	ftt.Run("Create a resource with invalid image project in datastore if type is machine", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		_, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestResourceCreateOfTypeMachineWithInvalidImageFamily(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource", "Test Resource Description", "ad_joined_machine", "windows_machine", "image-project", "")
	ftt.Run("Create a resource with invalid image family in datastore if type is machine", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		_, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestResourceUpdateWithValidData(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource Name", "Test Resource description", "ad_joined_machine", "windows_machine", "image-project", "image-family")
	ftt.Run("Update a resource with valid data in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		entity, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.BeNil)

		// Update resource with some new value and the operation should not throw any error
		entity.Name = "Test Resource Name Updated"
		entity.Description = "Test Resource description Updated"
		entity.Type = "ad_joined_machine"
		entity.OperatingSystem = "windows_system"
		entity.ImageProject = "image-project-updated"
		entity.ImageFamily = "image-family-updated"

		updateRequest := &proto.UpdateResourceRequest{
			Resource:   entity,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "type", "operating_system", "image_project", "image_family"}},
		}
		updatedEntity, err := handler.Update(ctx, updateRequest)
		assert.Loosely(t, err, should.BeNil)
		want := []string{"Test Resource Name Updated", "Test Resource description Updated", "ad_joined_machine", "windows_system", "image-project-updated", "image-family-updated"}
		get := []string{updatedEntity.GetName(), updatedEntity.GetDescription(), updatedEntity.GetType(), updatedEntity.GetOperatingSystem(), updatedEntity.GetImageProject(), updatedEntity.GetImageFamily()}
		assert.Loosely(t, get, should.Match(want))

		// Retrieve the updated resource and make sure that the values were correctly updated
		getRequest := &proto.GetResourceRequest{
			ResourceId: entity.GetResourceId(),
		}
		readEntity, err := handler.Get(ctx, getRequest)
		assert.Loosely(t, err, should.BeNil)
		want = []string{"Test Resource Name Updated", "Test Resource description Updated", "ad_joined_machine", "windows_system", "image-project-updated", "image-family-updated"}
		get = []string{readEntity.GetName(), readEntity.GetDescription(), readEntity.GetType(), readEntity.GetOperatingSystem(), readEntity.GetImageProject(), readEntity.GetImageFamily()}
		assert.Loosely(t, get, should.Match(want))
	})
}

func TestResourceUpdateWithInvalidName(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource Name", "Test Resource description", "ad_joined_machine", "windows_system", "image-project", "image-family")
	ftt.Run("Update a resource with invalid name in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		entity, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.BeNil)
		entity.Name = ""
		entity.Description = "Test Resource description Updated"
		entity.Type = "ad_joined_machine"
		entity.OperatingSystem = "windows_system"
		entity.ImageProject = "image-project"
		entity.ImageFamily = "image-family"

		updateRequest := &proto.UpdateResourceRequest{
			Resource:   entity,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "type", "operating_system", "image_project", "image_family"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the resource as name is empty
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestResourceUpdateWithDuplicateName(t *testing.T) {
	t.Parallel()
	createRequest1 := mockCreateResourceRequest("Test Resource Name1", "Test Resource description", "ad_joined_machine", "windows_system", "image-project", "image-family")
	createRequest2 := mockCreateResourceRequest("Test Resource Name2", "Test Resource description", "ad_joined_machine", "windows_system", "image-project", "image-family")
	ftt.Run("Update a resource with duplicate name in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		handler := &ResourceHandler{}
		entity, err := handler.Create(ctx, createRequest1)
		assert.Loosely(t, err, should.BeNil)
		_, err = handler.Create(ctx, createRequest2)
		assert.Loosely(t, err, should.BeNil)
		entity.Name = "Test Resource Name2"

		updateRequest := &proto.UpdateResourceRequest{
			Resource:   entity,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the resource as name is not unique
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, err.Error(), should.Equal("Resource name must be unique"))
	})
}

func TestResourceUpdateWithInvalidDescription(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource Name", "Test Resource description", "ad_joined_machine", "windows_system", "image-project", "image-family")
	ftt.Run("Update a resource with invalid descriprion in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		entity, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.BeNil)
		entity.Name = "Test Resource Name Updated"
		entity.Description = ""
		entity.Type = "ad_joined_machine"
		entity.OperatingSystem = "windows_system"
		entity.ImageProject = "image-project"
		entity.ImageFamily = "image-family"

		updateRequest := &proto.UpdateResourceRequest{
			Resource:   entity,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "type", "operating_system", "image-project", "image-family"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the resource as description is empty
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestResourceUpdateWithInvalidType(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource Name", "Test Resource description", "ad_joined_machine", "windows_system", "image-project", "image-family")
	ftt.Run("Update a resource with invalid type in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		entity, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.BeNil)
		entity.Name = "Test Resource Name Updated"
		entity.Description = "Test Resource description"
		entity.Type = ""
		entity.OperatingSystem = "windows_system"
		entity.ImageProject = "image-project"
		entity.ImageFamily = "image-family"

		updateRequest := &proto.UpdateResourceRequest{
			Resource:   entity,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "type", "operating_system", "image-project", "image-family"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the resource as type is missing
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestResourceUpdateOfTypeMachineWithInvalidOperatingSystem(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource Name", "Test Resource description", "ad_joined_machine", "windows_system", "image-project", "image-family")
	ftt.Run("Update a resource with invalid operating system in datastore if type is machine", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		entity, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.BeNil)
		entity.Name = "Test Resource Name Updated"
		entity.Description = "Test Resource description"
		entity.Type = "ad_joined_machine"
		entity.OperatingSystem = ""
		entity.ImageProject = "image-project"
		entity.ImageFamily = "image-family"

		updateRequest := &proto.UpdateResourceRequest{
			Resource:   entity,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "type", "operating_system", "image-project", "image-family"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the resource with type machine as operating system is missing
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestResourceUpdateOfTypeMachineWithInvalidImageProject(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource Name", "Test Resource description", "ad_joined_machine", "windows_system", "image-project", "image-family")
	ftt.Run("Update a resource with invalid image project in datastore if type is machine", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		entity, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.BeNil)
		entity.Name = "Test Resource Name Updated"
		entity.Description = "Test Resource description"
		entity.Type = "ad_joined_machine"
		entity.OperatingSystem = "windows_machine"
		entity.ImageProject = ""
		entity.ImageFamily = "image-family"

		updateRequest := &proto.UpdateResourceRequest{
			Resource:   entity,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "type", "operating_system", "image_project", "image_family"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the resource with type machine as image project is missing
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestResourceUpdateOfTypeMachineWithInvalidImageFamily(t *testing.T) {
	t.Parallel()
	resourceRequest := mockCreateResourceRequest("Test Resource Name", "Test Resource description", "ad_joined_machine", "windows_system", "image-project", "image-family")
	ftt.Run("Update a resource with invalid image family in datastore if type is machine", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		entity, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.BeNil)
		entity.Name = "Test Resource Name Updated"
		entity.Description = "Test Resource description"
		entity.Type = "ad_joined_machine"
		entity.OperatingSystem = "windows_machine"
		entity.ImageProject = "image-project"
		entity.ImageFamily = ""

		updateRequest := &proto.UpdateResourceRequest{
			Resource:   entity,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name", "description", "type", "operating_system", "image_project", "image_family"}},
		}
		_, err = handler.Update(ctx, updateRequest)
		// should not save the resource with type machine as image family is missing
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestGetResourceWithValidData(t *testing.T) {
	resourceRequest := mockCreateResourceRequest("Test Resource", "Test Resource description", "ad_joined_machine", "windows_system", "image-project", "image-family")
	ftt.Run("Get a resource based on id from datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		entity, err := handler.Create(ctx, resourceRequest)
		assert.Loosely(t, err, should.BeNil)
		getRequest := &proto.GetResourceRequest{
			ResourceId: entity.GetResourceId(),
		}
		readEntity, err := handler.Get(ctx, getRequest)
		assert.Loosely(t, err, should.BeNil)

		want := []string{entity.GetName(), entity.GetDescription(), entity.GetType(), entity.GetOperatingSystem(), entity.GetImageProject(), entity.GetImageFamily()}
		get := []string{readEntity.GetName(), readEntity.GetDescription(), readEntity.GetType(), readEntity.GetOperatingSystem(), readEntity.GetImageProject(), readEntity.GetImageFamily()}
		assert.Loosely(t, get, should.Match(want))
	})
}

func TestListResources(t *testing.T) {
	t.Parallel()

	resourceRequest1 := mockCreateResourceRequest("Test Resource1", "Test Resource description", "ad_joined_machine", "windows_system", "image-project-1", "image-family-1")
	resourceRequest2 := mockCreateResourceRequest("Test Resource2", "Test Resource description2", "ad_joined_machine", "windows_system", "image-project-2", "image-family-2")

	ftt.Run("Get all resources from datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		datastore.GetTestable(ctx).Consistent(true)
		handler := &ResourceHandler{}
		_, err := handler.Create(ctx, resourceRequest1)
		assert.Loosely(t, err, should.BeNil)
		_, err = handler.Create(ctx, resourceRequest2)
		assert.Loosely(t, err, should.BeNil)
		// Verify
		response, err := handler.List(ctx, &proto.ListResourcesRequest{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, response.GetResources(), should.HaveLength(2))
		resources := response.GetResources()
		want := []string{"Test Resource1", "Test Resource2"}
		get := []string{resources[0].GetName(), resources[1].GetName()}
		sort.Strings(get)
		assert.Loosely(t, get, should.Match(want))
	})
}

func TestResourceDeleteWithValidData(t *testing.T) {
	t.Parallel()
	createRequest := mockCreateResourceRequest("Test Resource", "Test Resource description", "ad_joined_machine", "windows_machine", "image-project", "image-family")
	ftt.Run("Create a resource in datastore", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())
		handler := &ResourceHandler{}
		model, err := handler.Create(ctx, createRequest)
		assert.Loosely(t, err, should.BeNil)
		deleteRequest := &proto.DeleteResourceRequest{ResourceId: model.ResourceId}
		_, err = handler.Delete(ctx, deleteRequest)
		assert.Loosely(t, err, should.BeNil)
		getRequest := &proto.GetResourceRequest{ResourceId: model.ResourceId}
		readEntity, err := handler.Get(ctx, getRequest)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, readEntity.Deleted, should.Equal(true))
	})
}
