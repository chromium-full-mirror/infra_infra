// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// OwnershipDataKind is the datastore entity kind OwnershipData.
const OwnershipDataKind string = "Ownership"

// AssetTypeMachine is the Machine asset type.
const AssetTypeMachine string = "Machine"

// AssetTypeMachineLSE is the MachineLSE asset type
const AssetTypeMachineLSE string = "MachineLSE"

// AssetTypeVM is the VM asset type.
const AssetTypeVM string = "VM"

// CommitSh is the commit at which we need to pull the configs.
const CommitSh string = "CommitSh"

// OwnershipDataEntity is a datastore entity that tracks a OwnershipData.
type OwnershipDataEntity struct {
	_kind         string                `gae:"$kind,Ownership"`
	Extra         datastore.PropertyMap `gae:",extra"`
	Name          string                `gae:"$id"`
	AssetType     string                `gae:"AssetType"`
	OwnershipData []byte                `gae:",noindex"`
}

// GetProto returns the unmarshaled OwnershipData.
func (e *OwnershipDataEntity) GetProto() (proto.Message, error) {
	var p ufspb.OwnershipData
	if err := proto.Unmarshal(e.OwnershipData, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate returns whether an OwnershipDataEntity is valid.
func (e *OwnershipDataEntity) Validate() error {
	return nil
}

// PutOwnershipData adds a bot name and its corresponding ownership in the datastore.
func PutOwnershipData(ctx context.Context, ownership *ufspb.OwnershipData, name string, assetType string) (*OwnershipDataEntity, error) {
	if name == "" {
		return nil, status.Errorf(codes.Internal, "Empty bot name")
	}
	ownershipBytes, err := proto.Marshal(ownership)
	if err != nil {
		return nil, errors.Annotate(err, "fail to marshal Ownership data %s", ownership).Err()
	}
	entity := &OwnershipDataEntity{
		Name:          name,
		OwnershipData: ownershipBytes,
		AssetType:     assetType,
	}
	if err := datastore.Put(ctx, entity); err != nil {
		logging.Errorf(ctx, "Failed to put ownership in datastore : %s - %s", name, err)
		return nil, err
	}
	return entity, nil
}

// GetOwnershipData returns OwnershipData for the given bot name from datastore.
func GetOwnershipData(ctx context.Context, name string) (*OwnershipDataEntity, error) {
	entity := &OwnershipDataEntity{
		Name: name,
	}
	if err := datastore.Get(ctx, entity); err != nil {
		if datastore.IsErrNoSuchEntity(err) {
			errorMsg := fmt.Sprintf("Entity not found %+v", entity)
			return nil, status.Errorf(codes.NotFound, errorMsg)
		}
		return nil, err
	}
	return entity, nil
}

// ListOwnerships lists the owmerships
// Does a query over Ownership entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListOwnerships(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]interface{}, keysOnly bool) (res []OwnershipDataEntity, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, OwnershipDataKind, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}
	return runListOwnershipQuery(ctx, q, pageSize, pageToken, keysOnly)
}

// ListHostsByIdPrefixSearch lists the hosts
//
// Does a query over OwnershipData entities using ID prefix.
// Returns up to pageSize entities, plus non-nil cursor (
// if there are more results).
// PageSize must be positive.
func ListHostsByIdPrefixSearch(ctx context.Context, pageSize int32, pageToken string, prefix string, keysOnly bool) (res []OwnershipDataEntity, nextPageToken string, err error) {
	q, err := ufsds.ListQueryIdPrefixSearch(ctx, OwnershipDataKind, pageSize, pageToken, prefix, keysOnly)
	if err != nil {
		return nil, "", err
	}
	return runListOwnershipQuery(ctx, q, pageSize, pageToken, keysOnly)
}

// Runs the query to list ownership entities and returns results.
func runListOwnershipQuery(ctx context.Context, query *datastore.Query, pageSize int32, pageToken string, keysOnly bool) (res []OwnershipDataEntity, nextPageToken string, err error) {
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, query, func(ent *OwnershipDataEntity, cb datastore.CursorCB) error {
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
		logging.Errorf(ctx, "Failed to List OwnershipData %s", err)
		return nil, "", status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

// GetOwnershipIndexedFieldName returns the index name
func GetOwnershipIndexedFieldName(input string) (string, error) {
	var field string
	input = strings.TrimSpace(input)
	switch strings.ToLower(input) {
	case util.AssetType:
		field = "AssetType"
	// CommitSh is not stored in the datastore but can be part of the filter map and will be used to directly fetch the configs at that particular commit
	case util.CommittishFilterName:
		field = CommitSh
	default:
		return "", status.Errorf(codes.InvalidArgument, "Invalid field name %s - field name for host are assettype", input)
	}
	return field, nil
}

// BatchDeleteOwnerships deletes a batch of entities with the given ids
func BatchDeleteOwnerships(ctx context.Context, ids []string) error {
	entities := make([]ufsds.FleetEntity, len(ids))
	for i, id := range ids {
		entities[i] = &OwnershipDataEntity{Name: id}
	}
	if err := datastore.Delete(ctx, entities); err != nil {
		logging.Errorf(ctx, "Failed to delete entities from datastore: %s", err)
		return status.Errorf(codes.Internal, fmt.Sprintf("%s: %s", "Internal Error", err.Error()))
	}
	return nil
}
