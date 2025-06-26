// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"fmt"

	proto "github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/chromiumos/infra/proto/go/lab"
	"go.chromium.org/chromiumos/infra/proto/go/manufacturing"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/grpcutil"

	api "go.chromium.org/infra/appengine/cros/lab_inventory/api/v1"
	"go.chromium.org/infra/cros/lab_inventory/datastore"
	"go.chromium.org/infra/cros/lab_inventory/deviceconfig"
)

// InventoryServerImpl implements service interfaces.
type InventoryServerImpl struct {
}

var (
	getDeviceConfigFunc    = deviceconfig.GetCachedConfig
	getAllDeviceConfigFunc = deviceconfig.GetAllCachedConfig
)

// GetCrosDevices retrieves requested Chrome OS devices from the inventory.
func (is *InventoryServerImpl) GetCrosDevices(ctx context.Context, req *api.GetCrosDevicesRequest) (resp *api.GetCrosDevicesResponse, err error) {
	return nil, grpcutil.GRPCifyAndLogErr(ctx, errors.New(`
	GetCrosDevices is deprecated.
	Please contact fleet infra oncall (go/peep-fleet-oncall) if using this API.
	`))
}

// UpdateDutsStatus updates selected Duts' status labels, metas related to testing.
func (is *InventoryServerImpl) UpdateDutsStatus(ctx context.Context, req *api.UpdateDutsStatusRequest) (resp *api.UpdateDutsStatusResponse, err error) {
	return nil, grpcutil.GRPCifyAndLogErr(ctx, errors.New(`
	UpdateDutsStatus is deprecated.
	Please contact fleet infra oncall (go/peep-fleet-oncall) if using this API.
	`))
}

// GetHwidData retrieves requested Chrome OS device Hwid Data from the inventory.
func (is *InventoryServerImpl) GetHwidData(ctx context.Context, req *api.GetHwidDataRequest) (resp *api.HwidData, err error) {
	return nil, grpcutil.GRPCifyAndLogErr(ctx, errors.New(`
	GetHwidData is deprecated.
	Please contact fleet infra oncall (go/peep-fleet-oncall) if using this API.
	`))
}

// GetManufacturingConfig retrieves requested Chrome OS device manufacturing config from the inventory.
func (is *InventoryServerImpl) GetManufacturingConfig(ctx context.Context, req *api.GetManufacturingConfigRequest) (resp *manufacturing.Config, err error) {
	return nil, grpcutil.GRPCifyAndLogErr(ctx, errors.New(`
	GetManufacturingConfig is deprecated.
	Please contact fleet infra oncall (go/peep-fleet-oncall) if using this API.
	`))
}

func getFallbackDeviceConfigID(oldConfigID *device.ConfigId) *device.ConfigId {
	if oldConfigID.GetVariantId().GetValue() != "" {
		fallbackID := proto.Clone(oldConfigID).(*device.ConfigId)
		fallbackID.VariantId = nil
		return fallbackID
	}
	return oldConfigID
}

// ListCrosDevicesLabConfig retrieves all lab configs
func (is *InventoryServerImpl) ListCrosDevicesLabConfig(ctx context.Context, req *api.ListCrosDevicesLabConfigRequest) (response *api.ListCrosDevicesLabConfigResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	allDevices, err := datastore.GetAllDevices(ctx)
	logging.Debugf(ctx, "got devices (%d)", len(allDevices))
	if err != nil {
		return nil, errors.Fmt("get all devices: %w", err)
	}
	labConfigs := make([]*api.ListCrosDevicesLabConfigResponse_LabConfig, 0, len(allDevices))
	for _, d := range allDevices {
		if d.Entity == nil || d.Err != nil || d.Entity.ID == "" {
			continue
		}
		dev := &lab.ChromeOSDevice{}
		if err := d.Entity.GetCrosDeviceProto(dev); err != nil {
			logging.Debugf(ctx, "fail to get lab config proto for %s (%s)", d.Entity.ID, d.Entity.Hostname)
		}
		dutState := &lab.DutState{}
		if err := d.Entity.GetDutStateProto(dutState); err != nil {
			logging.Debugf(ctx, "fail to get dut state proto for %s (%s)", d.Entity.ID, d.Entity.Hostname)
		}
		utime, _ := ptypes.TimestampProto(d.Entity.Updated)
		labConfigs = append(labConfigs, &api.ListCrosDevicesLabConfigResponse_LabConfig{
			Config:      dev,
			State:       dutState,
			UpdatedTime: utime,
		})
	}
	return &api.ListCrosDevicesLabConfigResponse{
		LabConfigs: labConfigs,
	}, nil
}

// DeviceConfigsExists checks if the device_configs for the given configIds exists in the datastore
func (is *InventoryServerImpl) DeviceConfigsExists(ctx context.Context, req *api.DeviceConfigsExistsRequest) (rsp *api.DeviceConfigsExistsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	devCfgIds := make([]*device.ConfigId, 0, len(req.ConfigIds))
	for _, d := range req.ConfigIds {
		convertedID := deviceconfig.ConvertValidDeviceConfigID(d)
		devCfgIds = append(devCfgIds, convertedID)
	}
	res, err := deviceconfig.DeviceConfigsExists(ctx, devCfgIds)
	if err != nil {
		return nil, err
	}
	response := &api.DeviceConfigsExistsResponse{
		Exists: res,
	}
	return response, err
}

// GetDeviceManualRepairRecord checks and returns a manual repair record for
// a given device hostname if it exists.
func (is *InventoryServerImpl) GetDeviceManualRepairRecord(ctx context.Context, req *api.GetDeviceManualRepairRecordRequest) (rsp *api.GetDeviceManualRepairRecordResponse, err error) {
	return nil, grpcutil.GRPCifyAndLogErr(ctx, errors.New(`
	GetDeviceManualRepairRecord is deprecated.
	Please contact fleet infra oncall (go/peep-fleet-oncall) if using this API.
	`))
}

// CreateDeviceManualRepairRecord adds a new submitted manual repair record for
// a given device.
func (is *InventoryServerImpl) CreateDeviceManualRepairRecord(ctx context.Context, req *api.CreateDeviceManualRepairRecordRequest) (rsp *api.CreateDeviceManualRepairRecordResponse, err error) {
	return nil, grpcutil.GRPCifyAndLogErr(ctx, errors.New(`
	CreateDeviceManualRepairRecord is deprecated.
	Please contact fleet infra oncall (go/peep-fleet-oncall) if using this API.
	`))
}

// UpdateDeviceManualRepairRecord updates an existing manual repair record with
// new submitted info for a given device.
func (is *InventoryServerImpl) UpdateDeviceManualRepairRecord(ctx context.Context, req *api.UpdateDeviceManualRepairRecordRequest) (rsp *api.UpdateDeviceManualRepairRecordResponse, err error) {
	return nil, grpcutil.GRPCifyAndLogErr(ctx, errors.New(`
	UpdateDeviceManualRepairRecord is deprecated.
	Please contact fleet infra oncall (go/peep-fleet-oncall) if using this API.
	`))
}

// ListManualRepairRecords takes filtering parameters and returns a list of
// repair records that match the filters.
//
// Currently supports filtering on:
// - hostname
// - asset tag
// - user ldap
// - repair state
// - limit (number of records)
// - offset - used for pagination
func (is *InventoryServerImpl) ListManualRepairRecords(ctx context.Context, req *api.ListManualRepairRecordsRequest) (rsp *api.ListManualRepairRecordsResponse, err error) {
	return nil, grpcutil.GRPCifyAndLogErr(ctx, errors.New(`
	ListManualRepairRecords is deprecated.
	Please contact fleet infra oncall (go/peep-fleet-oncall) if using this API.
	`))
}

// GetDeviceConfig retrieves requested Chrome OS device device config from the inventory.
func (is *InventoryServerImpl) GetDeviceConfig(ctx context.Context, req *api.GetDeviceConfigRequest) (resp *device.Config, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	if err = req.Validate(); err != nil {
		return nil, err
	}
	convertedID := deviceconfig.ConvertValidDeviceConfigID(req.GetConfigId())
	fallbackID := getFallbackDeviceConfigID(convertedID)
	logging.Debugf(ctx, "before convert: %s", req.GetConfigId().String())
	logging.Debugf(ctx, "real device config ID: %s", convertedID.String())
	logging.Debugf(ctx, "fallback device config ID: %s", fallbackID.String())

	devCfgIds := make([]*device.ConfigId, 0, 1)
	idToDevCfg := map[string]*device.Config{}
	for _, cID := range []*device.ConfigId{convertedID, fallbackID} {
		if _, found := idToDevCfg[cID.String()]; found {
			continue
		}
		devCfgIds = append(devCfgIds, cID)
		idToDevCfg[cID.String()] = nil
	}

	devCfgs, err := getDeviceConfigFunc(ctx, devCfgIds)
	for i := range devCfgs {
		if err == nil || err.(errors.MultiError)[i] == nil {
			idToDevCfg[devCfgIds[i].String()] = devCfgs[i].(*device.Config)
		} else {
			logging.Warningf(ctx, "Ignored error: cannot get device config for %v: %v", devCfgIds[i], err.(errors.MultiError)[i])
		}
	}

	res := idToDevCfg[convertedID.String()]
	if res == nil || res.GetId() == nil {
		res = idToDevCfg[fallbackID.String()]
		if res == nil || res.GetId() == nil {
			return nil, status.Errorf(codes.NotFound, fmt.Sprintf("device config not found for %+v", req.GetConfigId()))
		}
	}
	return res, nil
}

// BatchGetManualRepairRecords gets the open record corresponding to each host
// in the list of given hostnames. If no open record is found, an empty object
// will be returned for that hostname.
func (is *InventoryServerImpl) BatchGetManualRepairRecords(ctx context.Context, req *api.BatchGetManualRepairRecordsRequest) (rsp *api.BatchGetManualRepairRecordsResponse, err error) {
	return nil, grpcutil.GRPCifyAndLogErr(ctx, errors.New(`
	BatchGetManualRepairRecords is deprecated.
	Please contact fleet infra oncall (go/peep-fleet-oncall) if using this API.
	`))
}

// BatchCreateManualRepairRecords creates new submitted manual repair records
// for a batch of given devices. All records will have the same CreatedTime.
func (is *InventoryServerImpl) BatchCreateManualRepairRecords(ctx context.Context, req *api.BatchCreateManualRepairRecordsRequest) (rsp *api.BatchCreateManualRepairRecordsResponse, err error) {
	return nil, grpcutil.GRPCifyAndLogErr(ctx, errors.New(`
	BatchCreateManualRepairRecords is deprecated.
	Please contact fleet infra oncall (go/peep-fleet-oncall) if using this API.
	`))
}

// ListDeviceConfigs lists all device configs inventory has in datastore.
func (is *InventoryServerImpl) ListDeviceConfigs(ctx context.Context, req *api.ListDeviceConfigsRequest) (resp *api.ListDeviceConfigsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()

	cfgMap, err := getAllDeviceConfigFunc(ctx)

	cfgs := make([]*device.Config, len(cfgMap))
	i := 0
	for k := range cfgMap {
		cfgs[i] = k
		i++
	}

	return &api.ListDeviceConfigsResponse{
		DeviceConfigs: cfgs,
	}, nil
}
