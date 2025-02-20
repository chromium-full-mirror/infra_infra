// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicelabel

import (
	"go.chromium.org/infra/libs/fleet"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

// ConvertChromeOS converts a UFS ChromeOS entity to a device representation with labels stored as key-value pairs.
func ConvertChromeOS(data *ufspb.ChromeOSDeviceData) (*fleet.Device, error) {
	d := &fleet.Device{
		DeviceLabels: []*fleet.DeviceLabel{},
		ResourceType: fleet.ResourceType_RESOURCETYPE_CROS,
	}
	for _, reg := range labelRegs {
		v, err := reg.getValue(data)
		if err != nil {
			return nil, err
		}
		d.DeviceLabels = append(d.DeviceLabels, &fleet.DeviceLabel{
			Key:              reg.name,
			SchedulableId:    reg.schedulableID,
			SchedulableValue: v,
			Source:           reg.source,
			ReasonToAdd:      reg.reasonToAdd,
		})
	}
	return d, nil
}
