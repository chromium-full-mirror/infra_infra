// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fake

import (
	"context"

	deviceconfig "go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/common/errors"
)

// DeviceConfigClient is a fake impl for testing
type DeviceConfigClient struct {
}

// GetDeviceConfig fetches a specific device config.
func (c *DeviceConfigClient) GetDeviceConfig(ctx context.Context, cfgID *deviceconfig.ConfigId) (*deviceconfig.Config, error) {
	if cfgID.GetPlatformId().GetValue() == "test" && cfgID.GetModelId().GetValue() == "test" {
		return &deviceconfig.Config{
			Id: &deviceconfig.ConfigId{
				PlatformId: &deviceconfig.PlatformId{Value: "test"},
				ModelId:    &deviceconfig.ModelId{Value: "test"},
			},
		}, nil
	}
	return nil, errors.New("No device config found")
}

// DeviceConfigsExists detects whether any number of configs exist. The return
// is an array of booleans, where the ith boolean represents the existence of
// the ith config.
func (c *DeviceConfigClient) DeviceConfigsExists(ctx context.Context, cfgIDs []*deviceconfig.ConfigId) ([]bool, error) {
	resp := make([]bool, len(cfgIDs))
	for idx, config := range cfgIDs {
		if pid := config.GetPlatformId(); pid != nil && pid.GetValue() == "test" {
			if mid := config.GetModelId(); mid != nil && mid.GetValue() == "test" {
				resp[idx] = true
			}
		} else {
			resp[idx] = false
		}
	}
	return resp, nil
}
