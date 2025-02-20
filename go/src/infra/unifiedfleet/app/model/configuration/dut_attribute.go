// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
)

// DutAttributeKind is the datastore entity kind DutAttribute.
const DutAttributeKind string = "DutAttribute"

// DutAttributeEntity is a datastore entity that tracks a DutAttribute.
type DutAttributeEntity struct {
	_kind         string                `gae:"$kind,DutAttribute"`
	Extra         datastore.PropertyMap `gae:",extra"`
	ID            string                `gae:"$id"`
	AttributeData []byte                `gae:",noindex"`
	Updated       time.Time
}

var DutAttributeRegex = regexp.MustCompile(`^[a-z0-9]+(?:[\-_][a-z0-9]+)*$`)

// GetProto returns the unmarshaled DutAttribute.
func (e *DutAttributeEntity) GetProto() (proto.Message, error) {
	p := &api.DutAttribute{}
	if err := proto.Unmarshal(e.AttributeData, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Validate returns whether a DutAttributeEntity is valid.
func (e *DutAttributeEntity) Validate() error {
	return nil
}

func newDutAttributeEntity(ctx context.Context, pm proto.Message) (attrEntity ufsds.FleetEntity, err error) {
	p, ok := pm.(*api.DutAttribute)
	if !ok {
		return nil, errors.Reason("Failed to create DutAttributeEntity: %s", pm).Err()
	}

	attrData, err := proto.Marshal(p)
	if err != nil {
		return nil, errors.Annotate(err, "failed to marshal DutAttribute %s", p).Err()
	}

	id := p.GetId().GetValue()
	if id == "" {
		return nil, errors.Reason("Empty DutAttribute ID").Err()
	}

	if err := validateDutAttributeId(id); err != nil {
		return nil, err
	}

	return &DutAttributeEntity{
		ID:            id,
		AttributeData: attrData,
		Updated:       time.Now().UTC(),
	}, nil
}

// UpdateDutAttribute updates DutAttribute in datastore.
func UpdateDutAttribute(ctx context.Context, attr *api.DutAttribute) (*api.DutAttribute, error) {
	pm, err := ufsds.PutSingle(ctx, attr, newDutAttributeEntity)
	if err != nil {
		return nil, err
	}
	return pm.(*api.DutAttribute), nil
}

// GetDutAttribute returns DutAttribute for the given id from datastore.
func GetDutAttribute(ctx context.Context, id string) (rsp *api.DutAttribute, err error) {
	attr := &api.DutAttribute{
		Id: &api.DutAttribute_Id{
			Value: id,
		},
	}
	pm, err := ufsds.Get(ctx, attr, newDutAttributeEntity)
	if err != nil {
		return nil, err
	}

	p, ok := pm.(*api.DutAttribute)
	if !ok {
		return nil, errors.Reason("Failed to create DutAttributeEntity: %s", pm).Err()
	}
	return p, nil
}

// ListDutAttributes lists the DutAttributes from datastore.
func ListDutAttributes(ctx context.Context, keysOnly bool) (rsp []*api.DutAttribute, err error) {
	var entities []*DutAttributeEntity
	q := datastore.NewQuery(DutAttributeKind).KeysOnly(keysOnly).FirestoreMode(true)
	if err = datastore.GetAll(ctx, q, &entities); err != nil {
		return nil, errors.Annotate(err, "ListDutAttributes error: failed to get DutAttributes").Err()
	}
	for _, ent := range entities {
		if keysOnly {
			rsp = append(rsp, &api.DutAttribute{
				Id: &api.DutAttribute_Id{
					Value: ent.ID,
				},
			})
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Warningf(ctx, "ListDutAttributes: failed to unmarshal: %s", err)
				continue
			}
			rsp = append(rsp, pm.(*api.DutAttribute))
		}
	}
	return rsp, nil
}

// validateDutAttributeId checks whether DutAttribute is snake/kebab case.
func validateDutAttributeId(id string) error {
	if !DutAttributeRegex.MatchString(id) {
		return status.Errorf(codes.InvalidArgument, "Invalid input - ID must be snake/kebab case.")
	}
	return nil
}
