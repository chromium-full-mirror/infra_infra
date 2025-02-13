// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"

	"infra/fleetconsole/api/fleetconsolerpc"
)

// DeviceDAO represents a device as saved in AlloyDB
type DeviceDAO struct {
	Id           string //nolint:stylecheck
	DutId        string //nolint:stylecheck
	Address      *DeviceAddressDAO
	Type         string
	State        string
	HardwareReqs *HardwareRequirementsDAO
}

type DeviceAddressDAO struct {
	Host string
	Port int32
}

type HardwareRequirementsDAO struct {
	SchedulableLabels map[string]*HardwareRequirements_LabelValues `json:"labels,omitempty"`
}

type HardwareRequirements_LabelValues struct { //nolint:stylecheck
	Values []string
}

func FromDeviceManagerDevice(device *api.Device) *DeviceDAO {
	hardwareReqs := &HardwareRequirementsDAO{
		SchedulableLabels: map[string]*HardwareRequirements_LabelValues{},
	}
	for k, v := range device.HardwareReqs.SchedulableLabels {
		hardwareReqs.SchedulableLabels[k] = &HardwareRequirements_LabelValues{
			Values: v.Values,
		}
	}

	return &DeviceDAO{
		Id:    device.Id,
		DutId: device.DutId,
		Address: &DeviceAddressDAO{
			Host: device.Address.Host,
			Port: device.Address.Port,
		},
		Type:         device.Type.String(),
		State:        device.State.String(),
		HardwareReqs: hardwareReqs,
	}
}

func ToListDevicesDevice(device *DeviceDAO) *fleetconsolerpc.Device {
	labels := make(map[string]*fleetconsolerpc.LabelValues)
	for k, v := range device.HardwareReqs.SchedulableLabels {
		labels[k] = &fleetconsolerpc.LabelValues{
			Values: v.Values,
		}
	}
	return &fleetconsolerpc.Device{
		Id:    device.Id,
		DutId: device.DutId,
		Address: &fleetconsolerpc.DeviceAddress{
			Host: device.Address.Host,
			Port: device.Address.Port,
		},
		Type:  fleetconsolerpc.DeviceType(fleetconsolerpc.DeviceType_value[device.Type]),
		State: fleetconsolerpc.DeviceState(fleetconsolerpc.DeviceState_value[device.State]),
		DeviceSpec: &fleetconsolerpc.DeviceSpec{
			Labels: labels,
		},
	}
}

func (device *DeviceDAO) DeviceAsDBArguments() []any {
	var port string
	if device.Address != nil {
		port = fmt.Sprintf("%d", device.Address.Port)
	} else {
		port = ""
	}

	return []any{
		device.Id,
		device.DutId,
		device.Address.Host,
		port,
		device.Type,
		device.State,
		device.HardwareReqs.SchedulableLabels,
	}

}
