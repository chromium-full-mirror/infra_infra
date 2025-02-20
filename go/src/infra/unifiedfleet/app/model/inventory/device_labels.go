// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"context"
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

// DeviceLabelsKind is the datastore entity kind for labels.
const DeviceLabelsKind string = "DeviceLabels"

// DeviceLabelsEntity is a datastore entity that tracks DeviceLabels.
type DeviceLabelsEntity struct {
	_kind        string                `gae:"$kind,DeviceLabels"`
	Extra        datastore.PropertyMap `gae:",extra"`
	ID           string                `gae:"$id"`
	ResourceType string                `gae:"resource_type"`
	// ufspb.DeviceLabels cannot be directly used as it contains pointer.
	DeviceLabels []byte `gae:",noindex"`
}

// GetProto returns the unmarshaled DeviceLabels.
func (e *DeviceLabelsEntity) GetProto() (proto.Message, error) {
	var p ufspb.DeviceLabels
	if err := proto.Unmarshal(e.DeviceLabels, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate returns whether a DeviceLabelsEntity is valid
func (e *DeviceLabelsEntity) Validate() error {
	return nil
}

func newDeviceLabelsEntity(ctx context.Context, pm proto.Message) (ufsds.FleetEntity, error) {
	p := pm.(*ufspb.DeviceLabels)
	if p.GetName() == "" {
		return nil, errors.Reason("Empty DeviceLabels ID").Err()
	}
	labels, err := proto.Marshal(p)
	if err != nil {
		return nil, errors.Annotate(err, "fail to marshal DeviceLabels %s", p).Err()
	}

	return &DeviceLabelsEntity{
		ID:           p.GetName(),
		ResourceType: p.GetResourceType().String(),
		DeviceLabels: labels,
	}, nil
}

// QueryDeviceLabelsByPropertyName queries DeviceLabels Entity in the datastore
// If keysOnly is true, then only key field is populated in returned machinelses
func QueryDeviceLabelsByPropertyName(ctx context.Context, propertyName, id string, keysOnly bool) ([]*ufspb.DeviceLabels, error) {
	q := datastore.NewQuery(DeviceLabelsKind).KeysOnly(keysOnly).FirestoreMode(true)
	var entities []*DeviceLabelsEntity
	if err := datastore.GetAll(ctx, q.Eq(propertyName, id), &entities); err != nil {
		logging.Errorf(ctx, "Failed to query from datastore: %s", err)
		return nil, status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if len(entities) == 0 {
		logging.Infof(ctx, "No labels found for the query: %s=%s", propertyName, id)
		return nil, nil
	}
	labelsList := make([]*ufspb.DeviceLabels, 0, len(entities))
	for _, entity := range entities {
		if keysOnly {
			labels := &ufspb.DeviceLabels{
				Name: entity.ID,
			}
			labelsList = append(labelsList, labels)
		} else {
			pm, perr := entity.GetProto()
			if perr != nil {
				logging.Errorf(ctx, "Failed to unmarshal proto: %s", perr)
				continue
			}
			labelsList = append(labelsList, pm.(*ufspb.DeviceLabels))
		}
	}
	return labelsList, nil
}

func queryAllDeviceLabels(ctx context.Context) ([]ufsds.FleetEntity, error) {
	var entities []*DeviceLabelsEntity
	q := datastore.NewQuery(DeviceLabelsKind)
	if err := datastore.GetAll(ctx, q, &entities); err != nil {
		return nil, err
	}
	fe := make([]ufsds.FleetEntity, len(entities))
	for i, e := range entities {
		fe[i] = e
	}
	return fe, nil
}

// GetAllDeviceLabels returns all labels in datastore.
func GetAllDeviceLabels(ctx context.Context) (*ufsds.OpResults, error) {
	return ufsds.GetAll(ctx, queryAllDeviceLabels)
}

// DeleteDeviceLabels deletes the labels in datastore
func DeleteDeviceLabels(ctx context.Context, id string) error {
	return ufsds.Delete(ctx, &ufspb.DeviceLabels{Name: id}, newDeviceLabelsEntity)
}

// DeleteDeviceLabelsList deletes a batch of labels
//
// Can be used in a transaction
func DeleteDeviceLabelsList(ctx context.Context, resourceNames []string) *ufsds.OpResults {
	protos := make([]proto.Message, len(resourceNames))
	for i, m := range resourceNames {
		protos[i] = &ufspb.DeviceLabels{
			Name: m,
		}
	}
	return ufsds.DeleteAll(ctx, protos, newDeviceLabelsEntity)
}

// BatchDeleteDeviceLabels deletes labels in datastore.
//
// This is a non-atomic operation. Must be used within a transaction.
// Will lead to partial deletes if not used in a transaction.
func BatchDeleteDeviceLabels(ctx context.Context, ids []string) error {
	protos := make([]proto.Message, len(ids))
	for i, id := range ids {
		protos[i] = &ufspb.DeviceLabels{Name: id}
	}
	return ufsds.BatchDelete(ctx, protos, newDeviceLabelsEntity)
}

// GetDeviceLabels returns labels for the given id from datastore.
func GetDeviceLabels(ctx context.Context, id string) (*ufspb.DeviceLabels, error) {
	pm, err := ufsds.Get(ctx, &ufspb.DeviceLabels{Name: id}, newDeviceLabelsEntity)
	if err == nil {
		return pm.(*ufspb.DeviceLabels), err
	}
	return nil, err
}

func getDeviceLabelsID(pm proto.Message) string {
	p := pm.(*ufspb.DeviceLabels)
	return p.GetName()
}

// BatchGetDeviceLabels returns a batch of labels from datastore.
func BatchGetDeviceLabels(ctx context.Context, ids []string) ([]*ufspb.DeviceLabels, error) {
	protos := make([]proto.Message, len(ids))
	for i, n := range ids {
		protos[i] = &ufspb.DeviceLabels{Name: n}
	}
	pms, err := ufsds.BatchGet(ctx, protos, newDeviceLabelsEntity, getDeviceLabelsID)
	if err != nil {
		return nil, err
	}
	res := make([]*ufspb.DeviceLabels, len(pms))
	for i, pm := range pms {
		res[i] = pm.(*ufspb.DeviceLabels)
	}
	return res, nil
}

// BatchUpdateDeviceLabels updates labels in datastore.
//
// This is a non-atomic operation and doesnt check if the object already exists before
// update. Must be used within a Transaction where objects are checked before update.
// Will lead to partial updates if not used in a transaction.
func BatchUpdateDeviceLabels(ctx context.Context, labelsList []*ufspb.DeviceLabels) ([]*ufspb.DeviceLabels, error) {
	protos := make([]proto.Message, len(labelsList))
	for i, l := range labelsList {
		protos[i] = l
	}
	_, err := ufsds.PutAll(ctx, protos, newDeviceLabelsEntity, true)
	if err == nil {
		return labelsList, err
	}
	return nil, err
}

// ListDeviceLabels lists the labels
//
// Does a query over labels entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListDeviceLabels(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]interface{}, keysOnly bool) (res []*ufspb.DeviceLabels, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, DeviceLabelsKind, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, q, func(ent *DeviceLabelsEntity, cb datastore.CursorCB) error {
		if keysOnly {
			labels := &ufspb.DeviceLabels{
				Name: ent.ID,
			}
			res = append(res, labels)
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil
			}
			res = append(res, pm.(*ufspb.DeviceLabels))
		}
		if len(res) >= int(pageSize) {
			if nextCur, err = cb(); err != nil {
				return err
			}
			return datastore.Stop
		}
		return nil
	})
	if err != nil {
		logging.Errorf(ctx, "Failed to List DeviceLabels: %s", err)
		return nil, "", status.Errorf(codes.Internal, "%s", err.Error())
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

// GetDeviceLabelsIndexedFieldName returns the index name
func GetDeviceLabelsIndexedFieldName(input string) (string, error) {
	var field string
	input = strings.TrimSpace(input)
	switch strings.ToLower(input) {
	case util.ResourceTypeFilterName:
		field = "resource_type"
	default:
		return "", status.Errorf(codes.InvalidArgument, "Invalid field name %s - field name for labels are resource_type", input)
	}
	return field, nil
}
