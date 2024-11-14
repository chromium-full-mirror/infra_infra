// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"fmt"
	"strconv"

	"infra/fleetconsole/api/fleetconsolerpc"
)

const maxPageSize int = 50
const mockedDevicesCount int = 60

// ListDevices lists devices provided via DeviceManager.
func (frontend *FleetConsoleFrontend) ListDevices(ctx context.Context, req *fleetconsolerpc.ListDevicesRequest) (*fleetconsolerpc.ListDevicesResponse, error) {
	afterDeviceID := pageTokenToDeviceID(req.PageToken)

	devices := getMockDevices()

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

func getMockDevices() []*fleetconsolerpc.Device {
	var devices []*fleetconsolerpc.Device

	for i := 1; i <= mockedDevicesCount; i++ {
		id := strconv.Itoa(i)
		devices = append(devices, &fleetconsolerpc.Device{
			Id:    id,
			DutId: "dut_id_" + id,
			Address: &fleetconsolerpc.DeviceAddress{
				Host: "host" + id,
				Port: 1234,
			},
			Type:  fleetconsolerpc.DeviceType_DEVICE_TYPE_UNSPECIFIED,
			State: fleetconsolerpc.DeviceState_DEVICE_STATE_AVAILABLE,
			DeviceSpec: &fleetconsolerpc.DeviceSpec{
				SchedulableLabels: map[string]*fleetconsolerpc.DeviceSpec_LabelValues{
					"label1": {
						Values: []string{"value1_" + id, "value2_" + id},
					},
					"label2": {
						Values: []string{"value3_" + id, "value4_" + id},
					},
					"label3": {
						Values: []string{"value5_" + id},
					},
				},
			},
		})
	}

	return devices
}
