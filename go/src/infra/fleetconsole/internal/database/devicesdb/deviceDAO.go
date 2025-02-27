// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"context"
	"fmt"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	ufsmodel "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

// DeviceDAO represents a device as saved in AlloyDB
type DeviceDAO struct {
	Id         string //nolint:stylecheck
	DutId      string //nolint:stylecheck
	Address    *DeviceAddressDAO
	Type       string
	State      string
	DeviceSpec *DeviceSpecDAO
}

type DeviceAddressDAO struct {
	Host string
	Port int32
}

type DeviceSpecDAO struct {
	Labels map[string]*LabelValuesDAO
}

type LabelValuesDAO struct {
	Values []string
}

func FromDeviceManagerDevice(device *api.Device) *DeviceDAO {
	deviceSpec := &DeviceSpecDAO{
		Labels: map[string]*LabelValuesDAO{},
	}
	for k, v := range device.HardwareReqs.SchedulableLabels {
		if k == "dut_id" {
			continue
		}

		deviceSpec.Labels[k] = &LabelValuesDAO{
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
		Type:       device.Type.String(),
		State:      device.State.String(),
		DeviceSpec: deviceSpec,
	}
}

func ToListDevicesDevice(device *DeviceDAO) *fleetconsolerpc.Device {
	labels := make(map[string]*fleetconsolerpc.LabelValues)
	for k, v := range device.DeviceSpec.Labels {
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
	var host string
	if device.Address != nil {
		port = fmt.Sprintf("%d", device.Address.Port)
		host = device.Address.Host
	} else {
		port = ""
		host = ""
	}

	return []any{
		device.Id,
		device.DutId,
		host,
		port,
		device.Type,
		device.State,
		device.DeviceSpec.Labels,
	}
}

func FromUfsDevice(device *ufsmodel.DeviceLabels, state string) *DeviceDAO {
	hardwareReqs := &DeviceSpecDAO{
		Labels: map[string]*LabelValuesDAO{},
	}
	d := &DeviceDAO{
		Id:      IdFromUfsName(device.GetName()),
		Address: nil, // At the moment we are not getting any address (not even from device manager)
		// Type: deprecated
		DeviceSpec: hardwareReqs,
		State:      state,
	}

	for k, v := range device.GetLabels() {
		if k == "dut_id" && len(v.GetLabelValues()) == 1 {
			d.DutId = v.GetLabelValues()[0]
			continue
		}

		hardwareReqs.Labels[k] = &LabelValuesDAO{
			Values: v.GetLabelValues(),
		}
	}

	if d.Id == "" {
		logging.Warningf(context.TODO(), "Missing dutId for device %s", d.Id)
	}

	return d
}

func IdFromUfsName(name string) string {
	split := strings.Split(name, "/")
	return split[len(split)-1]
}

func typeFromUFSType(resourceType ufsmodel.ResourceType) string {
	switch resourceType {
	case ufsmodel.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE:
		return "DEVICE_TYPE_PHYSICAL"
	default:
		return "DEVICE_TYPE_UNSPECIFIED"

	}
}

func FromSwarmingBots(bot *swarmingv2.BotInfo) (*DeviceDAO, error) {
	d := &DeviceDAO{
		Address: nil, // At the moment we are not getting any address (not even from device manager)
		// Type: deprecated
		DeviceSpec: &DeviceSpecDAO{
			Labels: map[string]*LabelValuesDAO{},
		},
	}

	for _, dimension := range bot.Dimensions {
		key, values := dimension.Key, dimension.Value
		if key == "dut_name" && len(values) == 1 {
			d.Id = values[0]
		}
		if key == "dut_id" && len(values) == 1 {
			d.DutId = values[0]
			continue
		}

		d.DeviceSpec.Labels[key] = &LabelValuesDAO{
			Values: values,
		}
	}

	if d.Id == "" {
		return nil, fmt.Errorf("Missing dut_name")
	}

	return d, nil
}
