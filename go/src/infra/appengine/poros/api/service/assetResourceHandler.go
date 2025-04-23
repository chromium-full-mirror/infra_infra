// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/google/uuid"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"

	"go.chromium.org/infra/appengine/poros/api/entities"
	proto "go.chromium.org/infra/appengine/poros/api/proto"
)

type AssetResourceHandler struct {
	proto.UnimplementedAssetResourceServer
}

func toAssetResourceEntity(model *proto.AssetResourceModel) *entities.AssetResourceEntity {
	if model != nil {
		return &entities.AssetResourceEntity{
			AssetResourceId: model.AssetResourceId,
			AssetId:         model.AssetId,
			ResourceId:      model.ResourceId,
			AliasName:       model.AliasName,
			CreatedAt:       model.CreatedAt.AsTime(),
			CreatedBy:       model.CreatedBy,
			ModifiedAt:      model.ModifiedAt.AsTime(),
			ModifiedBy:      model.ModifiedBy,
			Default:         model.Default,
		}
	}
	return nil
}

func toAssetResourceModel(entity *entities.AssetResourceEntity) *proto.AssetResourceModel {
	if entity != nil {
		return &proto.AssetResourceModel{
			AssetResourceId: entity.AssetResourceId,
			AssetId:         entity.AssetId,
			ResourceId:      entity.ResourceId,
			AliasName:       entity.AliasName,
			CreatedAt:       timestamppb.New(entity.CreatedAt),
			CreatedBy:       entity.CreatedBy,
			ModifiedAt:      timestamppb.New(entity.ModifiedAt),
			ModifiedBy:      entity.ModifiedBy,
			Default:         entity.Default,
		}
	}
	return nil
}

func validateAssetResourceEntity(entity *entities.AssetResourceEntity) error {
	// validate AssetResourceId, AssetId, ResourceId, and AliasName
	if entity.AssetResourceId == "" {
		return errors.New("asset resource id cannot be empty")
	}
	if entity.AssetId == "" {
		return errors.New("asset id cannot be empty")
	}
	if entity.ResourceId == "" {
		return errors.New("resource id cannot be empty")
	}
	if entity.AliasName == "" {
		return errors.New("alias name cannot be empty")
	}
	return nil
}

func fakeAssetResourceAncestorKey(ctx context.Context) *datastore.Key {
	return datastore.MakeKey(ctx, "AssetResource", "key")
}

// Creates the given AssetResourceEntity.
func (e *AssetResourceHandler) Create(ctx context.Context, req *proto.CreateAssetResourceRequest) (*proto.AssetResourceModel, error) {
	id := uuid.New().String()
	entity := &entities.AssetResourceEntity{
		AssetResourceId: id,
		AssetId:         req.GetAssetId(),
		ResourceId:      req.GetResourceId(),
		AliasName:       req.GetAliasName(),
		CreatedAt:       time.Now().UTC(),
		CreatedBy:       auth.CurrentUser(ctx).Email,
	}
	if err := validateAssetResourceEntity(entity); err != nil {
		return nil, err
	}
	if err := datastore.Put(ctx, entity); err != nil {
		return nil, err
	}
	return toAssetResourceModel(entity), nil
}

// Retrieves a AssetResource entity for a given unique value.
func (e *AssetResourceHandler) Get(ctx context.Context, req *proto.GetAssetResourceRequest) (*proto.AssetResourceModel, error) {
	entity, err := getByAssetResourceId(ctx, req.GetAssetResourceId())
	if err == nil {
		return toAssetResourceModel(entity), nil
	}
	return nil, err
}

// Update a single ssetResource entity.
func (e *AssetResourceHandler) Update(ctx context.Context, req *proto.UpdateAssetResourceRequest) (*proto.AssetResourceModel, error) {
	id := req.GetAssetResource().GetAssetResourceId()
	mask := req.GetUpdateMask()
	asset_resource := &entities.AssetResourceEntity{}

	if mask == nil || len(mask.GetPaths()) == 0 || !mask.IsValid(req.GetAssetResource()) {
		return nil, errors.New("Update Mask can't be empty or invalid")
	}
	// In a transaction load asset resource, set fields based on field mask.
	err := datastore.RunInTransaction(ctx, func(ctx context.Context) error {
		asset_resource, err := getByAssetResourceId(ctx, id)
		if err != nil {
			return err
		}
		// Set updated values for fields specified in Update Mask
		for _, field := range mask.GetPaths() {
			newValue := reflect.ValueOf(req.GetAssetResource()).Elem().FieldByName(snakeToPascalCase(field))
			reflect.ValueOf(asset_resource).Elem().FieldByName(snakeToPascalCase(field)).Set(newValue)
		}
		asset_resource.ModifiedBy = auth.CurrentUser(ctx).Email
		asset_resource.ModifiedAt = time.Now().UTC()

		if err := validateAssetResourceEntity(asset_resource); err != nil {
			return err
		}

		err = datastore.Put(ctx, asset_resource)
		return err
	}, nil)

	if err == nil {
		return toAssetResourceModel(asset_resource), nil
	}
	return nil, err
}

// Deletes the given AssetResource entity.
func (e *AssetResourceHandler) Delete(ctx context.Context, req *proto.DeleteAssetResourceRequest) (*emptypb.Empty, error) {
	if err := datastore.Delete(ctx, &entities.AssetResourceEntity{
		AssetResourceId: req.GetAssetResourceId()}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// Lists all AssetResource entities.
func (e *AssetResourceHandler) List(ctx context.Context, in *proto.ListAssetResourcesRequest) (*proto.ListAssetResourcesResponse, error) {
	query := datastore.NewQuery("AssetResourceEntity")
	var assetResourceEntities []*entities.AssetResourceEntity
	res := &proto.ListAssetResourcesResponse{}
	if err := datastore.GetAll(ctx, query, &assetResourceEntities); err != nil {
		return nil, err
	}
	for _, asset_resource := range assetResourceEntities {
		res.AssetResources = append(res.AssetResources, toAssetResourceModel(asset_resource))
	}
	return res, nil
}

func getByAssetResourceId(ctx context.Context, id string) (*entities.AssetResourceEntity, error) {
	asset_resource := &entities.AssetResourceEntity{AssetResourceId: id}
	if err := datastore.Get(ctx, asset_resource); err != nil {
		return nil, err
	}
	return asset_resource, nil
}
