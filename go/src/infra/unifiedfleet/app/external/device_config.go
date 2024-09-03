// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package external

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	invV2Api "infra/appengine/cros/lab_inventory/api/v1"
	ufsdevice "infra/unifiedfleet/api/v1/models/chromeos/device"
	"infra/unifiedfleet/app/model/configuration"
)

// DeviceConfigClient handles read operations for DeviceConfigs
// These functions match the functions in the configuration package, and if
// datastore is the only source for DeviceConfigs, consider deleting this code
type DeviceConfigClient interface {
	GetDeviceConfig(ctx context.Context, cfgID *ufsdevice.ConfigId) (*ufsdevice.Config, error)
	DeviceConfigsExists(ctx context.Context, cfgIDs []*ufsdevice.ConfigId) ([]bool, error)
}

// InventoryDeviceConfigClient exposes methods needed to read from inventory.
// This is used when we dual read from inventory and UFS sources.
type InventoryDeviceConfigClient interface {
	DeviceConfigsExists(ctx context.Context, in *invV2Api.DeviceConfigsExistsRequest, opts ...grpc.CallOption) (*invV2Api.DeviceConfigsExistsResponse, error)
	GetDeviceConfig(ctx context.Context, in *invV2Api.GetDeviceConfigRequest, opts ...grpc.CallOption) (*device.Config, error)
}

// DualDeviceConfigClient uses both inventory and UFS data sources to fetch
// device configs. If it is able to detect a device config in either data
// source, it treat it as existing.
type DualDeviceConfigClient struct {
	inventoryClient InventoryDeviceConfigClient
}

// GetDeviceConfig fetches a specific device config.
//
// Query UFS first, if no response, fallback to call inventoryv2.
func (c *DualDeviceConfigClient) GetDeviceConfig(ctx context.Context, cfgID *ufsdevice.ConfigId) (*ufsdevice.Config, error) {
	resp, err := configuration.GetDeviceConfigACL(ctx, cfgID)
	if err == nil {
		return resp, nil
	}
	logging.Debugf(ctx, "device config ID %v was not found in UFS with error: %s. falling back to inventoryv2", cfgID, err)

	// if we cannot fetch from UFS, fall back to inventoryv2
	crosCfgID, err := ufsToCrosCfgIDProto(cfgID)
	if err != nil {
		return nil, errors.Annotate(err, "failed to convert between ufs and inventory proto, likely proto versions are out of sync").Err()
	}
	dc, err := c.inventoryClient.GetDeviceConfig(ctx, &invV2Api.GetDeviceConfigRequest{
		ConfigId: crosCfgID,
	})
	if err == nil && dc != nil {
		return crosToUFSDeviceConfigProto(dc)
	}
	logging.Debugf(ctx, "device config ID %v was not found in inventoryv2 with error: %s.", cfgID, err)
	return nil, err
}

// DeviceConfigsExists detects whether any number of configs exist.

// The return is an array of booleans, where the ith boolean represents the
// existence of the ith config.
// It queries UFS first, if no response, fallback to call inventoryv2.
func (c *DualDeviceConfigClient) DeviceConfigsExists(ctx context.Context, cfgIDs []*ufsdevice.ConfigId) ([]bool, error) {
	ufsResultsArr, err := configuration.DeviceConfigsExistACL(ctx, cfgIDs)
	if err == nil && allTrue(ufsResultsArr) {
		return ufsResultsArr, nil
	}
	if err != nil {
		ufsResultsArr = make([]bool, len(cfgIDs))
		logging.Debugf(ctx, "fail to query device config IDs %v in UFS. falling back to inventoryv2", cfgIDs)
	} else {
		for i, r := range ufsResultsArr {
			if !r {
				// This is for checking if there's any missing device config ID in UFS.
				// If not, inventoryv2 call will be deleted.
				logging.Debugf(ctx, "device config ID %v not found in UFS. falling back to inventoryv2", cfgIDs[i])
			}
		}
	}

	crosCfgIDs := make([]*device.ConfigId, len(cfgIDs))
	for i, cfgID := range cfgIDs {
		crosCfgID, err := ufsToCrosCfgIDProto(cfgID)
		if err != nil {
			return nil, errors.Annotate(err, "failed to convert between ufs and inventory proto, likely proto versions are out of sync").Err()
		}
		crosCfgIDs[i] = crosCfgID
	}

	resp, err := c.inventoryClient.DeviceConfigsExists(ctx, &invV2Api.DeviceConfigsExistsRequest{
		ConfigIds: crosCfgIDs,
	})
	if err != nil || resp == nil {
		return ufsResultsArr, nil
	}
	inventoryResultsArr := mapToSlice(len(crosCfgIDs), resp.Exists)
	if len(ufsResultsArr) != len(inventoryResultsArr) {
		return nil, errors.New("unexpected diff in return lengths between UFS and inventory device config exists")
	}
	return mergeOr(inventoryResultsArr, ufsResultsArr), nil
}

// ufsToCrosCfgIDProto naively marshalls then unmarshalls proto to convert
// between the format cros inventory expects and the format UFS expects. This
// is dangerous and can break when the protos are out of sync.
func ufsToCrosCfgIDProto(ufsCfgID *ufsdevice.ConfigId) (*device.ConfigId, error) {
	s, err := proto.Marshal(ufsCfgID)
	if err != nil {
		return nil, err
	}
	var crosCfgID device.ConfigId
	err = proto.Unmarshal(s, &crosCfgID)

	return &crosCfgID, err
}

// crosToUFSDeviceConfigProto naively marshalls then unmarshalls proto to convert
// between the format cros inventory expects and the format UFS expects. This
// is dangerous and can break when the protos are out of sync.
func crosToUFSDeviceConfigProto(crosCfgID *device.Config) (*ufsdevice.Config, error) {
	s, err := proto.Marshal(crosCfgID)
	if err != nil {
		return nil, err
	}
	var ufsCfgID ufsdevice.Config
	err = proto.Unmarshal(s, &ufsCfgID)

	return &ufsCfgID, err
}

// mapToSlice converts a map of bools to an array of bools.
func mapToSlice(numCfgs int, existsMap map[int32]bool) []bool {
	existsArr := make([]bool, numCfgs)

	for i := range existsMap {
		existsArr[i] = existsMap[i]
	}

	return existsArr
}

// allTrue returns whether or not the entire array is true.
func allTrue(a []bool) bool {
	for _, e := range a {
		if !e {
			return false
		}
	}

	return true
}

// mergeOr returns the result of ORing each index in two arrays which are the
// same size.
func mergeOr(x []bool, y []bool) []bool {
	newArr := make([]bool, len(x))

	for i := range newArr {
		newArr[i] = x[i] || y[i]
	}

	return newArr
}
