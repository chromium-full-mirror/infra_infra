// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/api"
	"go.chromium.org/chromiumos/config/go/payload"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
)

// ConfigBundleKind is the datastore entity kind ConfigBundle.
const ConfigBundleKind string = "ConfigBundle"

// ConfigBundleEntity is a datastore entity that tracks a ConfigBundle.
type ConfigBundleEntity struct {
	_kind      string                `gae:"$kind,ConfigBundle"`
	Extra      datastore.PropertyMap `gae:",extra"`
	ID         string                `gae:"$id"`
	ConfigData []byte                `gae:",noindex"`
	Updated    time.Time
}

// GetProto returns the unmarshaled ConfigBundle.
func (e *ConfigBundleEntity) GetProto() (proto.Message, error) {
	p := &payload.ConfigBundle{}
	if err := proto.Unmarshal(e.ConfigData, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Validate returns whether a ConfigBundleEntity is valid.
func (e *ConfigBundleEntity) Validate() error {
	return nil
}

func GenerateCBEntityId(cb *payload.ConfigBundle) (string, error) {
	if len(cb.GetDesignList()) == 0 {
		return "", errors.New("empty ConfigBundle DesignList")
	}
	program := cb.GetDesignList()[0].GetProgramId().GetValue()
	design := cb.GetDesignList()[0].GetId().GetValue()

	if program == "" {
		return "", errors.New("empty ConfigBundle ProgramId")
	}
	if design == "" {
		return "", errors.New("empty ConfigBundle DesignId")
	}

	return fmt.Sprintf("%s-%s", program, design), nil
}

func newConfigBundleEntity(ctx context.Context, pm proto.Message) (cbEntity ufsds.FleetEntity, err error) {
	p, ok := pm.(*payload.ConfigBundle)
	if !ok {
		return nil, fmt.Errorf("failed to create ConfigBundleEntity: %s", pm)
	}

	id, err := GenerateCBEntityId(p)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ConfigBundle entity id: %w", err)
	}

	configData, err := proto.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ConfigBundle %s: %w", p, err)
	}

	return &ConfigBundleEntity{
		ID:         id,
		ConfigData: configData,
		Updated:    time.Now().UTC(),
	}, nil
}

// UpdateConfigBundle updates ConfigBundle in datastore.
func UpdateConfigBundle(ctx context.Context, cb *payload.ConfigBundle) (*payload.ConfigBundle, error) {
	pm, err := ufsds.PutSingle(ctx, cb, newConfigBundleEntity)
	if err != nil {
		return nil, err
	}
	return pm.(*payload.ConfigBundle), nil
}

// GetConfigBundle returns ConfigBundle for the given id
// (${programId}-${designId}) from datastore.
func GetConfigBundle(ctx context.Context, id string) (rsp *payload.ConfigBundle, err error) {
	ids, err := extractCBIds(ctx, id)
	if err != nil {
		return nil, err
	}

	cb := &payload.ConfigBundle{
		DesignList: []*api.Design{
			{
				Id: &api.DesignId{
					Value: ids[1],
				},
				ProgramId: &api.ProgramId{
					Value: ids[0],
				},
			},
		},
	}
	pm, err := ufsds.Get(ctx, cb, newConfigBundleEntity)
	if err != nil {
		return nil, err
	}

	p, ok := pm.(*payload.ConfigBundle)
	if !ok {
		return nil, fmt.Errorf("failed to create ConfigBundleEntity: %s", pm)
	}
	return p, nil
}

func extractCBIds(ctx context.Context, id string) ([]string, error) {
	ids := strings.Split(id, "-")
	if len(ids) != 2 {
		return nil, status.Errorf(codes.InvalidArgument, "faulty id value; please make sure the format is ${programId}-${designId}")
	}
	return ids, nil
}

// FlatConfigKind is the datastore entity kind FlatConfig.
const FlatConfigKind string = "FlatConfig"

// FlatConfigEntity is a datastore entity that tracks a FlatConfig.
type FlatConfigEntity struct {
	_kind      string                `gae:"$kind,FlatConfig"`
	Extra      datastore.PropertyMap `gae:",extra"`
	ID         string                `gae:"$id"`
	ConfigData []byte                `gae:",noindex"`
	Updated    time.Time
}

// GetProto returns the unmarshaled FlatConfig.
func (e *FlatConfigEntity) GetProto() (proto.Message, error) {
	p := &payload.FlatConfig{}
	if err := proto.Unmarshal(e.ConfigData, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Validate returns whether a FlatConfigEntity is valid.
func (e *FlatConfigEntity) Validate() error {
	return nil
}

func GenerateFCEntityId(fc *payload.FlatConfig) (string, error) {
	program := fc.GetHwDesign().GetProgramId().GetValue()
	design := fc.GetHwDesign().GetId().GetValue()
	designConfig := fc.GetHwDesignConfig().GetId().GetValue()

	if program == "" {
		return "", errors.New("empty FlatConfig ProgramId")
	}
	if design == "" {
		return "", errors.New("empty FlatConfig DesignId")
	}
	if designConfig == "" {
		return strings.ToLower(fmt.Sprintf("%s-%s", program, design)), nil
	}

	return strings.ToLower(fmt.Sprintf("%s-%s-%s", program, design, designConfig)), nil
}

func newFlatConfigEntity(ctx context.Context, pm proto.Message) (fcEntity ufsds.FleetEntity, err error) {
	p, ok := pm.(*payload.FlatConfig)
	if !ok {
		return nil, fmt.Errorf("failed to create FlatConfigEntity: %s", pm)
	}

	id, err := GenerateFCEntityId(p)
	if err != nil {
		return nil, fmt.Errorf("failed to generate FlatConfig entity id: %w", err)
	}

	configData, err := proto.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal FlatConfig %s: %w", p, err)
	}

	return &FlatConfigEntity{
		ID:         id,
		ConfigData: configData,
		Updated:    time.Now().UTC(),
	}, nil
}

// UpdateFlatConfig updates FlatConfig in datastore.
func UpdateFlatConfig(ctx context.Context, fc *payload.FlatConfig) (*payload.FlatConfig, error) {
	pm, err := ufsds.PutSingle(ctx, fc, newFlatConfigEntity)
	if err != nil {
		return nil, err
	}
	return pm.(*payload.FlatConfig), nil
}

// GetFlatConfig returns FlatConfig for the given id
// (${programId}-${designId} or ${programId}-${designId}-${designConfigId}) from datastore.
func GetFlatConfig(ctx context.Context, id string) (rsp *payload.FlatConfig, err error) {
	ids, err := extractFCIds(ctx, id)
	if err != nil {
		return nil, err
	}

	fc := &payload.FlatConfig{
		HwDesign: &api.Design{
			Id: &api.DesignId{
				Value: ids[1],
			},
			ProgramId: &api.ProgramId{
				Value: ids[0],
			},
		},
	}

	if len(ids) == 3 {
		fc.HwDesignConfig = &api.Design_Config{
			Id: &api.DesignConfigId{
				Value: ids[2],
			},
		}
	}

	pm, err := ufsds.Get(ctx, fc, newFlatConfigEntity)
	if err != nil {
		return nil, err
	}

	p, ok := pm.(*payload.FlatConfig)
	if !ok {
		return nil, fmt.Errorf("failed to create FlatConfigEntity: %s", pm)
	}
	return p, nil
}

func extractFCIds(ctx context.Context, id string) ([]string, error) {
	ids := strings.Split(id, "-")
	if len(ids) < 2 || len(ids) > 3 {
		return nil, status.Errorf(codes.InvalidArgument, "faulty id value; please make sure the format is ${programId}-${designId} or ${programId}-${designId}-${designConfigId}")
	}
	return ids, nil
}

// GenerateFCIdFromCrosMachine generates a FlatConfig ID from a Cros Machine.
//
// The ID generated is based on the build target (board), model, and sku of a
// ChromeOS Machine. The result is ${board}-${model}-${model:sku}.
func GenerateFCIdFromCrosMachine(machine *ufspb.Machine) (string, error) {
	crosMachine := machine.GetChromeosMachine()
	if crosMachine == nil {
		return "", errors.New("invalid machine type. Not a chrome OS machine")
	}

	board := crosMachine.GetBuildTarget()
	if board == "" {
		return "", errors.New("empty board value")
	}

	model := crosMachine.GetModel()
	if model == "" {
		return "", errors.New("empty model value")
	}

	sku := crosMachine.GetSku()
	if sku == "" {
		return fmt.Sprintf("%s-%s", board, model), nil
	}
	return fmt.Sprintf("%s-%s-%s:%s", board, model, model, sku), nil
}
