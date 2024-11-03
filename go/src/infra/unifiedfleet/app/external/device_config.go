// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package external

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	deviceconfig "go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	invV2Api "infra/appengine/cros/lab_inventory/api/v1"
	"infra/unifiedfleet/app/model/configuration"
)

// DeviceConfigClient handles read operations for DeviceConfigs
// These functions match the functions in the configuration package, and if
// datastore is the only source for DeviceConfigs, consider deleting this code
type DeviceConfigClient interface {
	GetDeviceConfig(ctx context.Context, cfgID *deviceconfig.ConfigId) (*deviceconfig.Config, error)
	DeviceConfigsExists(ctx context.Context, cfgIDs []*deviceconfig.ConfigId) ([]bool, error)
}

// InventoryDeviceConfigClient exposes methods needed to read from inventory.
// This is used when we dual read from inventory and UFS sources.
type InventoryDeviceConfigClient interface {
	DeviceConfigsExists(ctx context.Context, in *invV2Api.DeviceConfigsExistsRequest, opts ...grpc.CallOption) (*invV2Api.DeviceConfigsExistsResponse, error)
	GetDeviceConfig(ctx context.Context, in *invV2Api.GetDeviceConfigRequest, opts ...grpc.CallOption) (*deviceconfig.Config, error)
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
func (c *DualDeviceConfigClient) GetDeviceConfig(ctx context.Context, cfgID *deviceconfig.ConfigId) (*deviceconfig.Config, error) {
	if cfgID.GetPlatformId().GetValue() == "" && cfgID.GetModelId().GetValue() == "" {
		return nil, fmt.Errorf("cannot fetch device config for empty platform and model")
	}
	resp, err := configuration.GetDeviceConfigACL(ctx, cfgID)
	if err == nil {
		return resp, nil
	}
	// Try fallback cfgID which ignores variant
	if cfgID.GetVariantId().GetValue() != "" {
		fallbackID := proto.Clone(cfgID).(*deviceconfig.ConfigId)
		fallbackID.VariantId = nil
		resp, err = configuration.GetDeviceConfigACL(ctx, fallbackID)
		if err == nil {
			return resp, nil
		}
		logging.Debugf(ctx, "GetDeviceConfig: device config IDs %v, %v not found in UFS with error: %s. falling back to inventoryv2", cfgID, fallbackID, err)
	} else {
		logging.Debugf(ctx, "GetDeviceConfig: device config ID %v not found in UFS with error: %s. falling back to inventoryv2", cfgID, err)
	}

	// if we cannot fetch from UFS, fall back to inventoryv2
	dc, err := c.inventoryClient.GetDeviceConfig(ctx, &invV2Api.GetDeviceConfigRequest{
		ConfigId: cfgID,
	})
	if err != nil || dc == nil {
		logging.Debugf(ctx, "device config ID %v was not found in inventoryv2 with error: %s.", cfgID, err)
	}
	return dc, err
}

// DeviceConfigsExists detects whether any number of configs exist.
//
// The return is an array of booleans, where the ith boolean represents the
// existence of the ith config.
// It queries UFS first, if no response, fallback to call inventoryv2.
func (c *DualDeviceConfigClient) DeviceConfigsExists(ctx context.Context, cfgIDs []*deviceconfig.ConfigId) ([]bool, error) {
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

	resp, err := c.inventoryClient.DeviceConfigsExists(ctx, &invV2Api.DeviceConfigsExistsRequest{
		ConfigIds: cfgIDs,
	})
	if err != nil || resp == nil {
		return ufsResultsArr, nil
	}
	inventoryResultsArr := mapToSlice(len(cfgIDs), resp.Exists)
	if len(ufsResultsArr) != len(inventoryResultsArr) {
		return nil, errors.New("unexpected diff in return lengths between UFS and inventory device config exists")
	}
	return mergeOr(inventoryResultsArr, ufsResultsArr), nil
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
