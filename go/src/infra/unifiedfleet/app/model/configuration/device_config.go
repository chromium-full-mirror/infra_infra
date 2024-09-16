// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	deviceconfig "go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufsds "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/util"
)

// DeviceConfigKind is the name of the device config entity kind in datastore.
const DeviceConfigKind string = "DeviceConfig"

// DeviceConfigEntity is a datastore entity that tracks a DeviceConfig.
type DeviceConfigEntity struct {
	_kind        string                `gae:"$kind,DeviceConfig"`
	Extra        datastore.PropertyMap `gae:",extra"`
	ID           string                `gae:"$id"`
	DeviceConfig []byte                `gae:",noindex"`
	Updated      time.Time
	Realm        string `gae:"realm"` // Realm for this entity
}

// GetProto returns the unmarshaled DeviceConfig.
func (e *DeviceConfigEntity) GetProto() (proto.Message, error) {
	var p deviceconfig.Config
	if err := proto.Unmarshal(e.DeviceConfig, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate returns whether a DeviceConfigEntity is valid.
func (e *DeviceConfigEntity) Validate() error {
	return nil
}

// GetRealm returns the realm of the device config.
func (e *DeviceConfigEntity) GetRealm() string {
	return e.Realm
}

// RealmAssignerFunc holds logic for associating a `DeviceConfig` with a realm.
type RealmAssignerFunc func(*deviceconfig.Config) string

// BlankRealmAssigner is a RealmAssignerFunc for situations where associating a
// realm is not import, ex. fetching an entity.
func BlankRealmAssigner(d *deviceconfig.Config) string {
	return ""
}

// BoardModelRealmAssigner constructs the realm of deviceconfig based on the board and model
func BoardModelRealmAssigner(d *deviceconfig.Config) string {
	return fmt.Sprintf("chromeos:%s-%s", strings.ToLower(d.Id.PlatformId.Value), strings.ToLower(d.Id.ModelId.Value))
}

// CrOSRealmAssigner constructs the realm
func CrOSRealmAssigner(d *deviceconfig.Config) string {
	return util.AtlLabAdminRealm
}

// newDeviceConfigEntityFunc generates a `datastore.NewFunc` that adds a realm
// to the entity based on the `realmAssigner` passed in
//
// This pattern is necessary as the upstream DeviceConfig proto won't have a
// `realm` field, but we need it in the entity. Because each namespace may
// desire a separate realm mapping, we can't hardcode this logic.
func newDeviceConfigEntityFunc(realmAssigner RealmAssignerFunc) ufsds.NewFunc {
	return func(ctx context.Context, pm proto.Message) (ufsds.FleetEntity, error) {
		return newDeviceConfig(ctx, pm, realmAssigner)
	}
}

// newDeviceConfigEntityFunc generates a `datastore.NewFunc` that adds a realm
// to the entity based on the `realmAssigner` passed in
//
// This pattern is necessary as the upstream DeviceConfig proto won't have a
// `realm` field, but we need it in the entity. Because each namespace may
// desire a separate realm mapping, we can't hardcode this logic.
func newDeviceConfigRealmEntityFunc(realmAssigner RealmAssignerFunc) ufsds.NewRealmEntityFunc {
	return func(ctx context.Context, pm proto.Message) (ufsds.RealmEntity, error) {
		return newDeviceConfig(ctx, pm, realmAssigner)
	}
}

// newDeviceConfig creates a DeviceConfig entity, with realms assigned via
// realmAssigner.
func newDeviceConfig(ctx context.Context, pm proto.Message, realmAssigner RealmAssignerFunc) (*DeviceConfigEntity, error) {
	p := pm.(*deviceconfig.Config)

	// configs must have model and platform
	if p.Id.GetModelId().GetValue() == "" || p.Id.GetPlatformId().GetValue() == "" {
		return nil, errors.Reason("invalid device config, platform id and model id must be populated").Err()
	}

	idString := GetDeviceConfigIDStr(p.GetId())

	dc, err := proto.Marshal(p)
	if err != nil {
		return nil, errors.Annotate(err, "failed to marshal proto: %s", p).Err()
	}
	return &DeviceConfigEntity{
		ID:           idString,
		DeviceConfig: dc,
		Realm:        realmAssigner(p),
		Updated:      time.Now().UTC(),
	}, nil
}

// GetDeviceConfigACL fetches a single device config if it is visible to the
// user.
func GetDeviceConfigACL(ctx context.Context, cfgID *deviceconfig.ConfigId) (*deviceconfig.Config, error) {
	pm, err := ufsds.GetACL(ctx, &deviceconfig.Config{Id: cfgID}, newDeviceConfigRealmEntityFunc(BlankRealmAssigner), util.ConfigurationsGet)
	if err == nil {
		return pm.(*deviceconfig.Config), err
	}
	return nil, err
}

// DeviceConfigsExistACL returns an array of bools. The ith value in this array
// represents whether the ith entry in cfgIDs exists and is visible to the user
func DeviceConfigsExistACL(ctx context.Context, cfgIDs []*deviceconfig.ConfigId) ([]bool, error) {
	entities := make([]ufsds.RealmEntity, len(cfgIDs))
	for i, id := range cfgIDs {
		idString := GetDeviceConfigIDStr(id)
		entities[i] = &DeviceConfigEntity{ID: idString}
	}

	return ufsds.ExistsACL(ctx, entities, util.ConfigurationsGet)
}

// BatchUpdateDeviceConfigs upserts all configs. The `realmAssigner` determines
// the logic for adding realms to these configs, and can be different in
// different namespaces.
func BatchUpdateDeviceConfigs(ctx context.Context, configs []*deviceconfig.Config, realmAssigner RealmAssignerFunc) ([]*deviceconfig.Config, error) {
	protos := make([]proto.Message, len(configs))
	for i, cfg := range configs {
		protos[i] = cfg
	}
	_, err := ufsds.PutAll(ctx, protos, newDeviceConfigEntityFunc(realmAssigner), true)
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// ListDeviceConfigs lists all device configs.
//
// Does a query over device config entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListDeviceConfigs(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]interface{}, keysOnly bool) (res []*deviceconfig.Config, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, DeviceConfigKind, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, q, func(ent *DeviceConfigEntity, cb datastore.CursorCB) error {
		pm, err := ent.GetProto()
		if err != nil {
			logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
			return nil
		}
		dc := pm.(*deviceconfig.Config)
		if keysOnly {
			res = append(res, &deviceconfig.Config{
				Id: dc.Id,
			})
		} else {
			res = append(res, dc)
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
		logging.Errorf(ctx, "Failed to list device configs %s", err)
		return nil, "", status.Errorf(codes.Internal, fmt.Sprintf("%s: %s", ufsds.InternalError, err.Error()))
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

// BatchDeleteDeviceConfigsACL deletes a batch of device configs and check ACLs
//
// This is a non-atomic operation. Must be used within a transaction.
// Will lead to partial deletes if not used in a transaction.
func BatchDeleteDeviceConfigsACL(ctx context.Context, cfgIDs []*deviceconfig.ConfigId) error {
	protos := make([]proto.Message, len(cfgIDs))
	for i, id := range cfgIDs {
		protos[i] = &deviceconfig.Config{Id: id}
	}
	return ufsds.BatchDeleteACL(ctx, protos, newDeviceConfigRealmEntityFunc(BlankRealmAssigner), util.ConfigurationsDelete)
}

// GetDeviceConfigIDStr returns a string as device config short name.
func GetDeviceConfigIDStr(cfgid *deviceconfig.ConfigId) string {
	var platformID, modelID, variantID string
	if v := cfgid.GetPlatformId(); v != nil {
		platformID = strings.ToLower(v.GetValue())
	}
	if v := cfgid.GetModelId(); v != nil {
		modelID = strings.ToLower(v.GetValue())
	}
	if v := cfgid.GetVariantId(); v != nil {
		variantID = strings.ToLower(v.GetValue())
	}
	return strings.Join([]string{platformID, modelID, variantID}, ".")
}

// GetConfigID creates ConfigId from the board/model/variant strings.
func GetConfigID(board, model, variant string) *deviceconfig.ConfigId {
	return &deviceconfig.ConfigId{
		PlatformId: &deviceconfig.PlatformId{Value: board},
		ModelId:    &deviceconfig.ModelId{Value: model},
		VariantId:  &deviceconfig.VariantId{Value: variant},
	}
}
