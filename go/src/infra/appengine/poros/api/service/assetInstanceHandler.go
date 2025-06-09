// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/tq"

	"go.chromium.org/infra/appengine/poros/api/entities"
	proto "go.chromium.org/infra/appengine/poros/api/proto"
	"go.chromium.org/infra/appengine/poros/taskspb"
)

type AssetInstanceHandler struct {
	proto.UnimplementedAssetInstanceServer
}

func toAssetIntanceModel(entity *entities.AssetInstanceEntity) *proto.AssetInstanceModel {
	if entity != nil {
		return &proto.AssetInstanceModel{
			AssetInstanceId: entity.AssetInstanceId,
			AssetId:         entity.AssetId,
			Status:          entity.Status,
			ProjectId:       entity.ProjectId,
			CreatedAt:       timestamppb.New(entity.CreatedAt),
			CreatedBy:       entity.CreatedBy,
			ModifiedAt:      timestamppb.New(entity.ModifiedAt),
			ModifiedBy:      entity.ModifiedBy,
			DeleteAt:        timestamppb.New(entity.DeleteAt),
		}
	}
	return nil
}

func validateAssetInstanceEntity(entity *entities.AssetInstanceEntity) error {
	// validate AssetId and Status
	if entity.AssetId == "" {
		return errors.New("assetId cannot be empty")
	}
	if _, validStatus := proto.DeploymentStatus_value[entity.Status]; !validStatus {
		return errors.New("status is invalid")
	}
	return nil
}

// Creates the given AssetInstance.
func (e *AssetInstanceHandler) Create(ctx context.Context, req *proto.CreateAssetInstanceRequest) (*proto.AssetInstanceModel, error) {
	id := uuid.New().String()
	timestamp := time.Now().UTC()
	entity := &entities.AssetInstanceEntity{
		AssetInstanceId: id,
		AssetId:         req.GetAssetId(),
		Status:          req.GetStatus(),
		CreatedBy:       auth.CurrentUser(ctx).Email,
		CreatedAt:       timestamp,
		DeleteAt:        timestamp.Add(time.Hour * 24 * 7),
	}
	if err := validateAssetInstanceEntity(entity); err != nil {
		return nil, err
	}
	if err := datastore.Put(ctx, entity); err != nil {
		return nil, err
	}
	assetInstance := toAssetIntanceModel(entity)
	err := EnqueueAssetAdditionOrDeletion(ctx, assetInstance.AssetInstanceId, "deploy", 100)
	if err != nil {
		logging.Errorf(ctx, "Error adding the task to the queue: %s", err.Error())
	}

	return assetInstance, nil
}

// Returns a gcp project which will be used for deployment of resources by cel_ctl
func deploymentProject(ctx context.Context) (string, error) {
	projectList := gcpProjectList()

	// get all projects that are already deployed
	activeInstanceStatuses := []string{"STATUS_RUNNING", "STATUS_COMPLETED"}
	var activeProjects []string
	for _, status := range activeInstanceStatuses {
		query := datastore.NewQuery("AssetInstanceEntity").Eq("Status", status)
		var asset_instances []*entities.AssetInstanceEntity
		if err := datastore.GetAll(ctx, query, &asset_instances); err != nil {
			return "", err
		}
		for _, asset_instance := range asset_instances {
			activeProjects = append(activeProjects, asset_instance.ProjectId)
		}
	}
	activeProjects = uniqueStrings(activeProjects)

	// exclude the deployed projects from the list of
	// projects that are available for being chosen
	availableProjects := []string{}
	for _, project := range projectList {
		if !valueInSlice(project, activeProjects) {
			availableProjects = append(availableProjects, project)
		}
	}

	if len(availableProjects) == 0 { // no project is available for deployment
		return "", errors.New("No Projects available at the moment")
	}

	// return the first available project
	return availableProjects[0], nil
}

// Retrieves an AssetInstance for a given unique value.
func (e *AssetInstanceHandler) Get(ctx context.Context, req *proto.GetAssetInstanceRequest) (*proto.AssetInstanceModel, error) {
	entity, err := getAssetInstanceById(ctx, req.GetAssetInstanceId())
	if err == nil {
		return toAssetIntanceModel(entity), nil
	}
	return nil, err
}

// Updates a single AssetInstance.
func (e *AssetInstanceHandler) Update(ctx context.Context, req *proto.UpdateAssetInstanceRequest) (*proto.AssetInstanceModel, error) {
	id := req.GetAssetInstance().GetAssetInstanceId()
	mask := req.GetUpdateMask()
	asset_instance, err := getAssetInstanceById(ctx, id)
	if err != nil {
		return nil, err
	}

	if mask == nil || len(mask.GetPaths()) == 0 || !mask.IsValid(req.GetAssetInstance()) {
		return nil, errors.New("Update Mask can't be empty or invalid")
	}
	// In a transaction load AssetInstance, set fields based on field mask.
	err = datastore.RunInTransaction(ctx, func(ctx context.Context) error {
		// Set updated values for fields specified in Update Mask
		for _, field := range mask.GetPaths() {
			if field == "delete_at" {
				asset_instance.DeleteAt = req.GetAssetInstance().GetDeleteAt().AsTime()
				continue
			}
			newValue := reflect.ValueOf(req.GetAssetInstance()).Elem().FieldByName(snakeToPascalCase(field))
			reflect.ValueOf(asset_instance).Elem().FieldByName(snakeToPascalCase(field)).Set(newValue)
		}
		asset_instance.ModifiedBy = auth.CurrentUser(ctx).Email
		asset_instance.ModifiedAt = time.Now().UTC()

		if err := validateAssetInstanceEntity(asset_instance); err != nil {
			return err
		}

		err = datastore.Put(ctx, asset_instance)
		return err
	}, nil)

	if err == nil {
		return toAssetIntanceModel(asset_instance), nil
	}
	return nil, err
}

// Deletes the given AssetInstance.
func (e *AssetInstanceHandler) Delete(ctx context.Context, req *proto.DeleteAssetInstanceRequest) (*emptypb.Empty, error) {
	if err := datastore.Delete(ctx, &entities.AssetInstanceEntity{
		AssetInstanceId: req.GetAssetInstanceId()}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// Lists all AssetInstance.
func (e *AssetInstanceHandler) List(ctx context.Context, in *proto.ListAssetInstancesRequest) (*proto.ListAssetInstancesResponse, error) {
	query := datastore.NewQuery("AssetInstanceEntity")
	var asset_instances []*entities.AssetInstanceEntity
	res := &proto.ListAssetInstancesResponse{}
	if err := datastore.GetAll(ctx, query, &asset_instances); err != nil {
		return nil, err
	}
	for _, asset_instance := range asset_instances {
		res.AssetInstances = append(res.AssetInstances, toAssetIntanceModel(asset_instance))
	}
	return res, nil
}

// Based on type, TriggerDeployment either creates or updates an asset instance
// with an available project to carry out a deployment on. Returns the project
// information like projectId, projectPrefix, etc.
func (e *AssetInstanceHandler) TriggerDeployment(ctx context.Context, in *proto.TriggerDeploymentRequest) (*proto.TriggerDeploymentResponse, error) {
	entityType := in.GetEntityType()
	entityId := in.GetEntityId()
	projectPrefix := "celab-poros"

	var entity *entities.AssetInstanceEntity
	var err error
	if entityType == "Asset" {
		project, err := deploymentProject(ctx)
		if err != nil {
			return nil, err
		}

		id := uuid.New().String()
		timestamp := time.Now().UTC()
		entity = &entities.AssetInstanceEntity{
			AssetInstanceId: id,
			AssetId:         entityId,
			Status:          "STATUS_RUNNING",
			ProjectId:       project,
			CreatedBy:       auth.CurrentUser(ctx).Email,
			CreatedAt:       timestamp,
			DeleteAt:        timestamp.Add(time.Hour * 24 * 7),
		}
	} else if entityType == "AssetInstance" {
		entity, err = getAssetInstanceById(ctx, entityId)
		if err != nil {
			return nil, err
		}

		if entity.Status == "STATUS_PENDING" {
			project, err := deploymentProject(ctx)
			if err != nil {
				return nil, err
			}
			entity.Status = "STATUS_RUNNING"
			entity.ProjectId = project
		}
	}

	if err := validateAssetInstanceEntity(entity); err != nil {
		return nil, err
	}
	if err := datastore.Put(ctx, entity); err != nil {
		return nil, err
	}

	response := &proto.TriggerDeploymentResponse{
		AssetId:         entity.AssetId,
		AssetInstanceId: entity.AssetInstanceId,
		ProjectId:       entity.ProjectId,
		ProjectPrefix:   projectPrefix,
	}

	return response, nil
}

// Fetch the Deployment Logs associated with Asset Instance Id
func (e *AssetInstanceHandler) FetchLogs(ctx context.Context, req *proto.FetchLogsRequest) (*proto.FetchLogsResponse, error) {
	entity, err := getAssetInstanceById(ctx, req.GetAssetInstanceId())
	if err != nil {
		return nil, err
	}
	response := &proto.FetchLogsResponse{
		Logs: entity.Logs,
	}
	return response, nil
}

func gcpProjectList() []string {
	projectList := []string{}
	// Increase the number once more GCP project created and configured
	for i := 1; i <= 60; i++ {
		projectList = append(projectList, fmt.Sprintf("celab-poros-%03d", i))
	}
	return projectList
}

func getAssetInstanceById(ctx context.Context, id string) (*entities.AssetInstanceEntity, error) {
	asset_instance := &entities.AssetInstanceEntity{AssetInstanceId: id}
	if err := datastore.Get(ctx, asset_instance); err != nil {
		return nil, err
	}
	return asset_instance, nil
}

// Collect the records which are ready for deletion and add those to cloud task
func (e *AssetInstanceHandler) TriggerAssetDeletion(ctx context.Context, in *emptypb.Empty) (*emptypb.Empty, error) {
	currentTime := time.Now().UTC()
	query := datastore.NewQuery("AssetInstanceEntity").Eq("Status", "STATUS_COMPLETED").Lte("DeleteAt", currentTime)
	var asset_instances []*entities.AssetInstanceEntity
	if err := datastore.GetAll(ctx, query, &asset_instances); err != nil {
		return nil, err
	}

	for _, asset_instance := range asset_instances {
		assetInstanceId := asset_instance.AssetInstanceId
		err := EnqueueAssetAdditionOrDeletion(ctx, assetInstanceId, "purge", 100)
		if err != nil {
			logging.Errorf(ctx, "Error adding the deletion task to the queue for AssetInstance %s: %s", assetInstanceId, err.Error())
		}
	}

	return &emptypb.Empty{}, nil
}

// EnqueueAssetInstance enqueues a asset instance creattion/deletion task.
func EnqueueAssetAdditionOrDeletion(ctx context.Context, assetInstanceId string, operation string, delay int64) error {
	return tq.AddTask(ctx, &tq.Task{
		// The body of the task. Also identifies what TaskClass to use.
		Payload: &taskspb.AssetAdditionOrDeletionTask{AssetInstanceId: assetInstanceId, Operation: operation},
		// Title appears in logs and URLs, useful for debugging.
		Title: fmt.Sprintf("AssetInstanceId-%v--Operation-%v", assetInstanceId, operation),
		// How long to wait before executing this task. Not super precise.
		Delay: time.Duration(delay) * time.Millisecond,
	})
}
