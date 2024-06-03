// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package entities

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/data/lex64"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/gae/service/datastore"

	fleetcostpb "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/costserver/maskutils"
	"infra/cros/fleetcost/internal/fleetcosterror"
	"infra/cros/fleetcost/internal/utils"
)

// CostIndicatorKind is the datastore kind of a cost indicator entity.
const CostIndicatorKind = "CostIndicatorKind"

// CostIndicatorEntity is a datastore entity storing a cost indicator.
type CostIndicatorEntity struct {
	_kind         string                     `gae:"$kind,CostIndicatorKind"`
	Extra         datastore.PropertyMap      `gae:",extra"`
	CostIndicator *fleetcostpb.CostIndicator `gae:"cost_indicator"`
	// Indexed fields for improved query performance.
	Board    string `gae:"board"`
	Model    string `gae:"model"`
	Sku      string `gae:"sku"`
	Type     string `gae:"type"`
	Location string `gae:"location"`
	Quantity bool   `gae:"quantity"`
}

// Silence staticcheck warning about unused field.
var _ = CostIndicatorEntity{}._kind

// Save saves an entity.
func (indicator *CostIndicatorEntity) Save(withMeta bool) (datastore.PropertyMap, error) {
	// TODO(gregorynisbet): extract normalization logic to helper function.
	indicator.Board = indicator.CostIndicator.GetBoard()
	indicator.Model = indicator.CostIndicator.GetModel()
	indicator.Sku = indicator.CostIndicator.GetSku()
	indicator.Quantity = indicator.CostIndicator.GetQuantity()
	if int(indicator.CostIndicator.GetType()) != 0 {
		indicator.Type = indicator.CostIndicator.GetType().String()
	}
	if int(indicator.CostIndicator.GetLocation()) != 0 {
		indicator.Location = indicator.CostIndicator.GetLocation().String()
	}
	return datastore.GetPLS(indicator).Save(withMeta)
}

// Load loads an entity.
func (indicator *CostIndicatorEntity) Load(propertyMap datastore.PropertyMap) error {
	return datastore.GetPLS(indicator).Load(propertyMap)
}

// Compile-time assertion that CostIndicatorEntity is a PropertyLoadSaver, i.e. it can be converted to and from a
// datastore.PropertyMap.
var _ datastore.PropertyLoadSaver = &CostIndicatorEntity{}

// Compile-time assertion that CostIndicatorEntity is a MetaGetterSetter, i.e. it has the ability to produce meta keys
// (in this case just $id).
var _ datastore.MetaGetterSetter = &CostIndicatorEntity{}

// GetAllMeta transfers control to the default implementation of GetAllMeta.
// We need this function so that we can compute the ID.
func (indicator *CostIndicatorEntity) GetAllMeta() datastore.PropertyMap {
	return datastore.GetPLS(indicator).GetAllMeta()
}

// SetMeta always returns false because we do not allow meta keys to be changed and false communicates this to the LUCI datastore library.
func (indicator *CostIndicatorEntity) SetMeta(key string, value any) bool {
	return false
}

// GetMeta gets meta-values. The id ("$id") is computed. The other things (like $kind, for instance) get their default values.
func (indicator *CostIndicatorEntity) GetMeta(key string) (any, bool) {
	if key == "id" {
		costIndicator := indicator.CostIndicator
		return fmt.Sprintf(
			"v1;%s;%s;%s;%d;%d",
			encode(costIndicator.GetBoard()),
			encode(costIndicator.GetModel()),
			encode(costIndicator.GetSku()),
			costIndicator.GetLocation().Number(),
			costIndicator.GetType().Number(),
		), true
	}
	return datastore.GetPLS(indicator).GetMeta(key)
}

// Function encode encodes a string to a custom base64 encoding that preserves lexicographic comparisons.
func encode(input string) string {
	encoding, _ := lex64.GetEncoding(lex64.V2)
	out, err := lex64.Encode(encoding, []byte(input))
	if err != nil {
		return ""
	}
	return out
}

// Clone produces a deep copy of a cost indicator.
//
// This method intentionally takes a non-pointer receiver to perform a
// shallow copy, and then replaces a field to perform a deep copy.
//
// I don't actually know whether I also need to copy the datastore.PropertyMap.
func (indicator CostIndicatorEntity) Clone() *CostIndicatorEntity {
	indicator.CostIndicator = proto.Clone(indicator.CostIndicator).(*fleetcostpb.CostIndicator)
	return &indicator
}

// NewCostIndicatorEntity makes a cost indicator entity from an object extracted from a request.
func NewCostIndicatorEntity(costIndicator *fleetcostpb.CostIndicator) *CostIndicatorEntity {
	return &CostIndicatorEntity{
		CostIndicator: costIndicator,
	}
}

// GetCostIndicatorEntity extracts a cost indicator from the database.
func GetCostIndicatorEntity(ctx context.Context, entity *CostIndicatorEntity) (*CostIndicatorEntity, error) {
	if err := datastore.Get(ctx, entity); err != nil {
		return nil, errors.Annotate(err, "get cost indicator").Err()
	}
	return entity, nil
}

// ListCostIndicators lists the cost indicators in the database, up to a limit (not yet implemented).
func ListCostIndicators(ctx context.Context, limit int, filter *fleetcostAPI.ListCostIndicatorsFilter) ([]*fleetcostpb.CostIndicator, error) {
	var out []*fleetcostpb.CostIndicator
	query, err := ApplyFilter(datastore.NewQuery(CostIndicatorKind), filter)
	if err != nil {
		return nil, errors.Annotate(err, "list cost indicators").Err()
	}
	if err := datastore.Run(ctx, query, func(entity *CostIndicatorEntity) {
		out = append(out, entity.CostIndicator)
	}); err != nil {
		return nil, errors.Annotate(err, "list cost indicators").Err()
	}
	return out, nil
}

// ApplyFilter applies filters to a datastore query and returns the original query unmodified
func ApplyFilter(query *datastore.Query, filter *fleetcostAPI.ListCostIndicatorsFilter) (*datastore.Query, error) {
	if filter == nil {
		return query, nil
	}
	if filter.GetBoard() != "" {
		query = query.Eq("board", filter.GetBoard())
	}
	if filter.GetModel() != "" {
		query = query.Eq("model", filter.GetModel())
	}
	if filter.GetSku() != "" {
		query = query.Eq("sku", filter.GetSku())
	}
	if filter.GetLocation() != "" {
		location, err := utils.ToLocation(filter.GetLocation())
		if err != nil {
			return nil, err
		}
		query = query.Eq("location", location.String())
	}
	if filter.GetType() != "" {
		typ, err := utils.ToIndicatorType(filter.GetType())
		if err != nil {
			return nil, err
		}
		query = query.Eq("type", typ.String())
	}
	return query, nil
}

// UpdateCostIndicatorEntity extracts a cost indicator entity from the database and updates it.
func UpdateCostIndicatorEntity(ctx context.Context, entity *CostIndicatorEntity, fields []string) (*CostIndicatorEntity, error) {
	oldEntity := entity.Clone()
	newEntity := entity
	if err := datastore.Get(ctx, oldEntity); err != nil {
		if datastore.IsErrNoSuchEntity(err) {
			return nil, fleetcosterror.WithDefaultCode(codes.NotFound, errors.Annotate(err, "update cost indicator").Err())
		}
		return nil, errors.Annotate(err, "update cost indicator").Err()
	}
	maskutils.UpdateCostIndicatorProto(oldEntity.CostIndicator, newEntity.CostIndicator, fields)
	if err := datastore.Put(ctx, oldEntity); err != nil {
		return nil, fleetcosterror.WithDefaultCode(codes.Aborted, errors.Annotate(err, "update cost indicator proto").Err())
	}
	return oldEntity, nil
}

// DeleteCostIndicatorEntity deletes a cost indicator entity.
func DeleteCostIndicatorEntity(ctx context.Context, entity *CostIndicatorEntity) error {
	return errors.Annotate(datastore.Delete(ctx, entity), "delete cost indicator").Err()
}
