// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/consoleserver/dimensions"
	"infra/fleetconsole/internal/devicemanagerclient"
)

// GetDeviceDimensions returns dimensions of all devices
func (frontend *FleetConsoleFrontend) GetDeviceDimensions(ctx context.Context, req *emptypb.Empty) (*fleetconsolerpc.GetDeviceDimensionsResponse, error) {
	deviceManagerClient, err := frontend.deviceManagerClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "get device dimensions").Err()
	}

	d, err := deviceManagerClient.Leaser.ListDevices(ctx, &api.ListDevicesRequest{})

	if err != nil {
		return nil, err
	}

	return getDimensions(devicemanagerclient.MapDevices(d.Devices)), nil
}

func getDimensions(devices []*fleetconsolerpc.Device) *fleetconsolerpc.GetDeviceDimensionsResponse {
	baseDimensions := map[string]map[string]bool{}
	labels := map[string]map[string]bool{}

	addDimension := func(dimensionMap map[string]map[string]bool, key string, value string) {
		if _, ok := dimensionMap[key]; !ok {
			dimensionMap[key] = map[string]bool{}
		}
		dimensionMap[key][value] = true
	}

	for _, device := range devices {
		for _, dimensionDescriptor := range dimensions.GetDimensionDescriptors() {
			addDimension(baseDimensions, dimensionDescriptor.ID, dimensionDescriptor.StringValueGetter(device))
		}

		for labelKey, value := range device.DeviceSpec.Labels {
			for _, labelValue := range value.Values {
				addDimension(labels, labelKey, labelValue)
			}
		}
	}

	return &fleetconsolerpc.GetDeviceDimensionsResponse{
		BaseDimensions: mapDimensionMapToLabelValues(baseDimensions),
		Labels:         mapDimensionMapToLabelValues(labels),
	}
}

func mapDimensionMapToLabelValues(dimensionMap map[string]map[string]bool) map[string]*fleetconsolerpc.LabelValues {
	dimensionMapResponse := make(map[string]*fleetconsolerpc.LabelValues, len(dimensionMap))

	for key, values := range dimensionMap {
		dimensionMapResponse[key] = &fleetconsolerpc.LabelValues{}
		for dimensionValue := range values {
			dimensionMapResponse[key].Values = append(dimensionMapResponse[key].Values, dimensionValue)
		}
	}

	return dimensionMapResponse
}
