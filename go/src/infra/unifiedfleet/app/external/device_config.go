// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package external

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	deviceconfig "go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/common/logging"

	"infra/unifiedfleet/app/model/configuration"
)

// DeviceConfigClient handles read operations for DeviceConfigs
// These functions match the functions in the configuration package, and if
// datastore is the only source for DeviceConfigs, consider deleting this code
type DeviceConfigClient interface {
	GetDeviceConfig(ctx context.Context, cfgID *deviceconfig.ConfigId) (*deviceconfig.Config, error)
	DeviceConfigsExists(ctx context.Context, cfgIDs []*deviceconfig.ConfigId) ([]bool, error)
}

// DualDeviceConfigClient can use multiple sources to fetch device configs.
type DualDeviceConfigClient struct {
}

// GetDeviceConfig fetches a specific device config in UFS.
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
		logging.Debugf(ctx, "GetDeviceConfig: device config IDs %v, %v not found in UFS with error: %s. Please check if this ID exist in go/cros_device_configs.", cfgID, fallbackID, err)
	} else {
		logging.Debugf(ctx, "GetDeviceConfig: device config ID %v not found in UFS with error: %s. Please check if this ID exist in go/cros_device_configs.", cfgID, err)
	}
	return resp, err
}

// DeviceConfigsExists detects whether any number of configs exist.
//
// The return is an array of booleans, where the ith boolean represents the
// existence of the ith config.
func (c *DualDeviceConfigClient) DeviceConfigsExists(ctx context.Context, cfgIDs []*deviceconfig.ConfigId) ([]bool, error) {
	ufsResultsArr, err := configuration.DeviceConfigsExistACL(ctx, cfgIDs)
	if err != nil {
		return nil, err
	}
	for i, r := range ufsResultsArr {
		if !r {
			logging.Debugf(ctx, "device config ID %v not found in UFS. Please check if this ID exist in go/cros_device_configs", cfgIDs[i])
		}
	}
	return ufsResultsArr, err
}
