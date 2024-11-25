// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/consoleserver/sorting"
)

const maxPageSize int = 50

// ListDevices lists devices provided via DeviceManager.
func (frontend *FleetConsoleFrontend) ListDevices(ctx context.Context, req *fleetconsolerpc.ListDevicesRequest) (*fleetconsolerpc.ListDevicesResponse, error) {
	deviceManagerClient, err := frontend.deviceManagerClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "list devices").Err()
	}
	afterDeviceID := pageTokenToDeviceID(req.PageToken)

	d, err := deviceManagerClient.Leaser.ListDevices(ctx, &api.ListDevicesRequest{})

	if err != nil {
		return nil, err
	}

	devices, err := sorting.SortDevices(mapDevices(d.Devices), req.OrderBy)

	if err != nil {
		return nil, err
	}

	pageSize := maxPageSize
	if req.PageSize != 0 {
		pageSize = min(int(req.PageSize), maxPageSize)
	}

	devicesPage, err := getPage(devices, afterDeviceID, pageSize)

	if err != nil {
		return nil, err
	}

	nextPageToken := ""
	if len(devicesPage) > 0 && devicesPage[len(devicesPage)-1].Id != devices[len(devices)-1].Id { // not reached the end of the collection yet
		nextPageToken = devicesPage[len(devicesPage)-1].Id // TODO: b/378633906 - obfuscate the page token
	}

	return &fleetconsolerpc.ListDevicesResponse{
		Devices:       devicesPage,
		NextPageToken: nextPageToken,
	}, nil
}

// TODO: b/378633906 - this is a stub method, which in future will unpack an obfuscated page token
// Uses last device ID from a previous page as a cursor
func pageTokenToDeviceID(pageToken string) string {
	return pageToken
}

func getPage(devices []*fleetconsolerpc.Device, afterDeviceID string, pageSize int) ([]*fleetconsolerpc.Device, error) {
	if afterDeviceID == "" {
		return devices[0:min(pageSize, len(devices))], nil
	}

	for i, v := range devices {
		if v.Id == afterDeviceID {
			return devices[i+1 : min(i+1+pageSize, len(devices))], nil
		}
	}
	return nil, fmt.Errorf("couldn't find device id: %s", afterDeviceID)
}

func mapDevices(devices []*api.Device) []*fleetconsolerpc.Device {
	var mappedDevices []*fleetconsolerpc.Device
	for _, device := range devices {
		mappedDevices = append(mappedDevices, mapDevice(device))
	}
	return mappedDevices
}

func mapDevice(device *api.Device) *fleetconsolerpc.Device {
	return &fleetconsolerpc.Device{
		Id:    device.Id,
		DutId: device.DutId,
		Address: &fleetconsolerpc.DeviceAddress{
			Host: device.Address.Host,
			Port: device.Address.Port,
		},
		Type:  fleetconsolerpc.DeviceType(device.Type),
		State: fleetconsolerpc.DeviceState(device.State),
		DeviceSpec: &fleetconsolerpc.DeviceSpec{
			Labels: mapLabels(device.HardwareReqs.SchedulableLabels),
		},
	}
}

func mapLabels(labels map[string]*api.HardwareRequirements_LabelValues) map[string]*fleetconsolerpc.DeviceSpec_LabelValues {
	mappedLabels := make(map[string]*fleetconsolerpc.DeviceSpec_LabelValues)
	for k, v := range labels {
		mappedLabels[k] = &fleetconsolerpc.DeviceSpec_LabelValues{
			Values: v.Values,
		}
	}
	return mappedLabels
}
