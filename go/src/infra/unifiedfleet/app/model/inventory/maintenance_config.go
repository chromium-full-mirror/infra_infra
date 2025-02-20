// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"context"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "infra/unifiedfleet/api/v1/models"
	ufsds "infra/unifiedfleet/app/model/datastore"
)

// MaintenanceConfigKind is the datastore entity kind MaintenanceConfig.
const MaintenanceConfigKind string = "MaintenanceConfig"

type MaintenanceConfigEntity struct {
	_kind             string                `gae:"$kind,MaintenanceConfig"`
	Extra             datastore.PropertyMap `gae:",extra"`
	Name              string                `gae:"$id"`
	MaintenanceConfig []byte                `gae:",noindex"`
}

// GetProto returns the unmarshalled MaintenanceConfig
func (e *MaintenanceConfigEntity) GetProto() (proto.Message, error) {
	var p ufspb.MaintenanceConfig
	if err := proto.Unmarshal(e.MaintenanceConfig, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate returns whether a MaintenanceConfigEntity is valid
func (e *MaintenanceConfigEntity) Validate() error {
	return nil
}

func newMaintenanceConfigEntity(ctx context.Context, pm proto.Message) (ufsds.FleetEntity, error) {
	p := pm.(*ufspb.MaintenanceConfig)
	if p.GetName() == "" {
		return nil, status.Errorf(codes.Internal, "Empty maintenance configuration name")
	}
	maintenanceConfig, err := proto.Marshal(p)
	if err != nil {
		return nil, errors.Annotate(err, "fail to marshal maintenance config %s", p).Err()
	}

	return &MaintenanceConfigEntity{
		Name:              p.GetName(),
		MaintenanceConfig: maintenanceConfig,
	}, nil
}

func AddMaintenanceConfig(ctx context.Context, name string) (*ufspb.MaintenanceConfig, error) {
	pm, err := ufsds.Get(ctx, &ufspb.MaintenanceConfig{Name: name}, newMaintenanceConfigEntity)
	if err == nil {
		return pm.(*ufspb.MaintenanceConfig), err
	}
	return nil, err
}

func getMaintenanceConfigName(pm proto.Message) string {
	p := pm.(*ufspb.MaintenanceConfig)
	return p.GetName()
}

// GetMaintenanceConfig retrieves a maintenance configuration from the datastore by its name if it exists.
func GetMaintenanceConfig(ctx context.Context, name string) (*ufspb.MaintenanceConfig, error) {
	pm, err := ufsds.Get(ctx, &ufspb.MaintenanceConfig{Name: name}, newMaintenanceConfigEntity)
	if err == nil {
		return pm.(*ufspb.MaintenanceConfig), err
	}
	return nil, err
}

func queryAllMaintenanceConfigs(ctx context.Context) ([]ufsds.FleetEntity, error) {
	var entities []*MaintenanceConfigEntity
	q := datastore.NewQuery(MaintenanceConfigKind)
	if err := datastore.GetAll(ctx, q, &entities); err != nil {
		return nil, err
	}
	fe := make([]ufsds.FleetEntity, len(entities))
	for i, e := range entities {
		fe[i] = e
	}
	return fe, nil
}

// GetAllMaintenanceConfigs returns all maintenance configurations in datastore.
func GetAllMaintenanceConfigs(ctx context.Context) (*ufsds.OpResults, error) {
	return ufsds.GetAll(ctx, queryAllMaintenanceConfigs)
}

// BatchGetMaintenanceConfigs returns a batch of maintenance configurations from datastore.
func BatchGetMaintenanceConfigs(ctx context.Context, names []string) ([]*ufspb.MaintenanceConfig, error) {
	protos := make([]proto.Message, len(names))
	for i, name := range names {
		protos[i] = &ufspb.MaintenanceConfig{Name: name}
	}
	pms, err := ufsds.BatchGet(ctx, protos, newMaintenanceConfigEntity, getMaintenanceConfigName)
	if err != nil {
		return nil, err
	}
	res := make([]*ufspb.MaintenanceConfig, len(pms))
	for i, pm := range pms {
		res[i] = pm.(*ufspb.MaintenanceConfig)
	}
	return res, nil
}

// UpdateMaintenanceConfigs updates the Maintenance Configuration in datastore.
func UpdateMaintenanceConfigs(ctx context.Context, configs []*ufspb.MaintenanceConfig) ([]*ufspb.MaintenanceConfig, error) {
	protos := make([]proto.Message, len(configs))
	for i, config := range configs {
		protos[i] = config
	}
	_, err := ufsds.PutAll(ctx, protos, newMaintenanceConfigEntity, true)
	if err == nil {
		return configs, err
	}
	return nil, err
}

// DeleteMaintenanceConfig removes a maintenance configuration using its unique name.
func DeleteMaintenanceConfig(ctx context.Context, name string) error {
	return ufsds.Delete(ctx, &ufspb.MaintenanceConfig{Name: name}, newMaintenanceConfigEntity)
}
