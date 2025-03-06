// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
)

// MaintenanceConfigKind is the datastore entity kind MaintenanceConfig.
const BotMaintenanceConfigKind string = "BotMaintenanceConfig"

type BotMaintenanceConfigEntity struct {
	_kind                string                `gae:"$kind,BotMaintenanceConfig"`
	Extra                datastore.PropertyMap `gae:",extra"`
	Id                   string                `gae:"$id"`
	AssetType            string                `gae:"AssetType"`
	BotMaintenanceConfig []byte                `gae:",noindex"`
}

// GetProto returns the unmarshaled BotMaintenanceConfig.
func (e *BotMaintenanceConfigEntity) GetProto() (proto.Message, error) {
	var p ufspb.BotMaintenanceConfig
	if err := proto.Unmarshal(e.BotMaintenanceConfig, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate returns whether an BotMaintenanceConfigEntity is valid.
func (e *BotMaintenanceConfigEntity) Validate() error {
	return nil
}

// PutBotMaintenanceConfig adds a bot name and its corresponding maintenance config in the datastore.
func PutBotMaintenanceConfig(ctx context.Context, maintenanceConfig *ufspb.BotMaintenanceConfig, id string, assetType string) (*BotMaintenanceConfigEntity, error) {
	if id == "" {
		return nil, status.Errorf(codes.Internal, "Empty bot name")
	}
	maintenanceConfigBytes, err := proto.Marshal(maintenanceConfig)
	if err != nil {
		return nil, errors.Annotate(err, "fail to marshal bot maintenance config: %s", maintenanceConfig).Err()
	}
	entity := &BotMaintenanceConfigEntity{
		Id:                   id,
		AssetType:            assetType,
		BotMaintenanceConfig: maintenanceConfigBytes,
	}
	if err := datastore.Put(ctx, entity); err != nil {
		logging.Errorf(ctx, "Failed to put bot maintenance config in datastore : %s - %s", id, err)
		return nil, err
	}
	return entity, nil
}

// GetBotMaintenanceConfig returns BotMaintenanceConfigEntity for the given bot name from datastore.
func GetBotMaintenanceConfig(ctx context.Context, id string) (*BotMaintenanceConfigEntity, error) {
	entity := &BotMaintenanceConfigEntity{
		Id: id,
	}
	if err := datastore.Get(ctx, entity); err != nil {
		if datastore.IsErrNoSuchEntity(err) {
			return nil, status.Errorf(codes.NotFound, "Entity not found %+v", entity)
		}
		return nil, err
	}
	return entity, nil
}

// ListBotMaintenanceConfigs lists the maintenance configs
// Does a query over BotMaintenanceConfig entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListBotMaintenanceConfigs(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]interface{}, keysOnly bool) (res []BotMaintenanceConfigEntity, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, BotMaintenanceConfigKind, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}
	return runListBotConfigQuery(ctx, q, pageSize, pageToken, keysOnly)
}

// Runs the query to list maintenance config entities and returns results.
func runListBotConfigQuery(ctx context.Context, query *datastore.Query, pageSize int32, pageToken string, keysOnly bool) (res []BotMaintenanceConfigEntity, nextPageToken string, err error) {
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, query, func(ent *BotMaintenanceConfigEntity, cb datastore.CursorCB) error {
		res = append(res, *ent)
		if len(res) >= int(pageSize) {
			if nextCur, err = cb(); err != nil {
				return err
			}
			return datastore.Stop
		}
		return nil
	})
	if err != nil {
		logging.Errorf(ctx, "Failed to List BotMaintenanceConfigs %s", err)
		return nil, "", status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

// ListHostsByBotIdPrefixSearch lists the hosts
//
// Does a query over BotMaintenanceConfig entities using ID prefix.
// Returns up to pageSize entities, plus non-nil cursor (
// if there are more results).
// PageSize must be positive.
func ListHostsByBotIdPrefixSearch(ctx context.Context, pageSize int32, pageToken string, prefix string, keysOnly bool) (res []BotMaintenanceConfigEntity, nextPageToken string, err error) {
	q, err := ufsds.ListQueryIdPrefixSearch(ctx, BotMaintenanceConfigKind, pageSize, pageToken, prefix, keysOnly)
	if err != nil {
		return nil, "", err
	}
	return runListBotConfigQuery(ctx, q, pageSize, pageToken, keysOnly)
}

// BatchDeleteBotConfigs deletes a batch of entities with the given ids
func BatchDeleteBotMaintenanceConfig(ctx context.Context, ids []string) error {
	entities := make([]ufsds.FleetEntity, len(ids))
	for i, id := range ids {
		entities[i] = &BotMaintenanceConfigEntity{Id: id}
	}
	if err := datastore.Delete(ctx, entities); err != nil {
		logging.Errorf(ctx, "Failed to delete entities from datastore: %s", err)
		return status.Errorf(codes.Internal, fmt.Sprintf("%s: %s", "Internal Error", err.Error()))
	}
	return nil
}
