// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsmfg "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/manufacturing"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// HwidDataKind is the datastore entity kind HwidData.
const HwidDataKind string = "HwidData"

// HwidDataEntity is a datastore entity that tracks a HwidData.
type HwidDataEntity struct {
	_kind    string                `gae:"$kind,HwidData"`
	Extra    datastore.PropertyMap `gae:",extra"`
	ID       string                `gae:"$id"`
	HwidData []byte                `gae:",noindex"`
	Updated  time.Time
}

// GetProto returns the unmarshaled HwidData.
func (e *HwidDataEntity) GetProto() (proto.Message, error) {
	p := &ufspb.HwidData{}
	if err := proto.Unmarshal(e.HwidData, p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetDutLabelProto returns the unmarshaled HwidData.
// TODO (b/240771930): Remove this method once datastore data is overwritten.
func (e *HwidDataEntity) GetDutLabelProto() (proto.Message, error) {
	p := &ufspb.DutLabel{}
	if err := proto.Unmarshal(e.HwidData, p); err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateHwidData updates HwidData in datastore.
func UpdateHwidData(ctx context.Context, d *ufspb.HwidData, hwid string) (*HwidDataEntity, error) {
	hwidData, err := proto.Marshal(d)
	if err != nil {
		return nil, errors.Annotate(err, "failed to marshal HwidData %s", d).Err()
	}

	if hwid == "" {
		return nil, status.Errorf(codes.InvalidArgument, "empty hwid")
	}

	entity := &HwidDataEntity{
		ID:       hwid,
		HwidData: hwidData,
		Updated:  time.Now().UTC(),
	}
	if err := datastore.Put(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// UpdateLegacyHwidData updates HwidDataEntity with DutLabel as HwidData instead
// of HwidData proto in datastore. This is also the previous implementation of
// UpdateHwidData.
func UpdateLegacyHwidData(ctx context.Context, d *ufspb.DutLabel, hwid string) (*HwidDataEntity, error) {
	dutLabel, err := proto.Marshal(d)
	if err != nil {
		return nil, errors.Annotate(err, "failed to marshal DutLabel %s", d).Err()
	}

	if hwid == "" {
		return nil, status.Errorf(codes.InvalidArgument, "empty hwid")
	}

	entity := &HwidDataEntity{
		ID:       hwid,
		HwidData: dutLabel,
		Updated:  time.Now().UTC(),
	}
	if err := datastore.Put(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// GetHwidData returns HwidData for the given hwid from datastore.
func GetHwidData(ctx context.Context, hwid string) (*HwidDataEntity, error) {
	entity := &HwidDataEntity{
		ID: hwid,
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

// ParseHwidData returns the HwidData proto based on the datastore entity.
//
// It parses a given HwidDataEntity into the ufspb.HwidData proto containing
// all DutLabels. No error is returned if the entity is nil.
func ParseHwidData(ent *HwidDataEntity) (*ufspb.HwidData, error) {
	if ent == nil {
		return nil, nil
	}

	// Try to get HwidData proto. Try DutLabel proto if fail.
	// TODO (b/240771930): Remove DutLabel conditional once datastore data is
	// overwritten.
	entData, err := ent.GetProto()
	if err != nil {
		return nil, err
	}
	hwidData, ok := entData.(*ufspb.HwidData)
	if !ok {
		return nil, errors.Reason("Failed to cast data to HwidData: %s", entData).Err()
	}

	// Unmarshaling into the wrong type of proto will not fail. So must check if
	// data actually has DutLabel. The correct proto type will always have a
	// DutLabel field after unmarshaling.
	if hwidData.GetDutLabel() == nil {
		data := &ufspb.HwidData{
			Hwid: ent.ID,
		}
		entData, err = ent.GetDutLabelProto()
		if err != nil {
			return nil, err
		}
		dutLabel, ok := entData.(*ufspb.DutLabel)
		if !ok {
			return nil, errors.Reason("Failed to cast data to DutLabel: %s", entData).Err()
		}
		data.DutLabel = dutLabel
		return SetHwidDataWithDutLabels(data), nil
	}
	return hwidData, nil
}

// SetHwidDataWithDutLabels sets the Sku and Variant of HwidData proto.
//
// It parses the DutLabels of a given HwidData entity and fills in the Sku and
// Variant values of the proto.
func SetHwidDataWithDutLabels(hwidData *ufspb.HwidData) *ufspb.HwidData {
	for _, l := range hwidData.GetDutLabel().GetLabels() {
		switch strings.ToLower(l.GetName()) {
		case "sku":
			hwidData.Sku = l.GetValue()
		case "variant":
			hwidData.Variant = l.GetValue()
		case "stylus":
			hwidData.Stylus = true
		case "touchpad":
			hwidData.Touchpad = true
		case "touchscreen":
			hwidData.Touchscreen = true
		case "racc_enabled_status":
			hwidData.RaccEnabledStatus = l.GetValue()
		}
	}
	return hwidData
}

// ParseHwidDataIntoMfgCfg takes HWID data from the HWID server and conforms it
// into the ManufacturingConfig proto format.
//
// Note that this ignores the CR50Phase and CR50KeyEnv fields as those have been
// deprecated in ManufacturingConfig and replaced by StateConfig values.
func ParseHwidDataIntoMfgCfg(hwidData *ufspb.HwidData) (*ufsmfg.ManufacturingConfig, error) {
	if reflect.ValueOf(hwidData).IsNil() {
		return nil, errors.New("HwidData message cannot be empty")
	}
	mfgCfg := &ufsmfg.ManufacturingConfig{
		ManufacturingId: &ufsmfg.ConfigID{
			Value: hwidData.GetHwid(),
		},
	}
	for _, l := range hwidData.GetDutLabel().GetLabels() {
		switch strings.ToLower(l.GetName()) {
		case "hwid_component":
			mfgCfg.HwidComponent = append(mfgCfg.HwidComponent, l.GetValue())
		case "wireless":
			mfgCfg.WifiChip = l.GetValue()
		case "phase":
			mfgCfg.DevicePhase = util.ToUFSDevicePhase(l.GetValue())
		}
	}
	return mfgCfg, nil
}

// ListHwidData lists the HwidData
//
// Does a query over HwidData entities. Returns up to pageSize entities, plus
// non-nil cursor (if there are more results). pageSize must be positive.
func ListHwidData(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]interface{}, keysOnly bool) (res []*ufspb.HwidData, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, HwidDataKind, pageSize, pageToken, nil, keysOnly)
	if err != nil {
		return nil, "", err
	}
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, q, func(ent *HwidDataEntity, cb datastore.CursorCB) error {
		if keysOnly {
			hwidData := &ufspb.HwidData{
				Hwid: ent.ID,
			}
			res = append(res, hwidData)
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to Unmarshal: %s", err)
				return nil
			}
			res = append(res, pm.(*ufspb.HwidData))
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
		logging.Errorf(ctx, "Failed to List HwidData %s", err)
		return nil, "", status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}
