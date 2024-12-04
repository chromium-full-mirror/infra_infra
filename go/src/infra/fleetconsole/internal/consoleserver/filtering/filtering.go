// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filtering

import (
	"fmt"
	"regexp"

	"go.chromium.org/luci/common/data/aip160"

	"infra/fleetconsole/api/fleetconsolerpc"
	"infra/fleetconsole/internal/consoleserver/dimensions"
)

func FilterDevices(devices []*fleetconsolerpc.Device, filter string) ([]*fleetconsolerpc.Device, error) {
	ast, err := aip160.ParseFilter(filter)
	if err != nil {
		return nil, err
	}

	if ast == nil {
		return devices, nil
	}

	filteredDevices := []*fleetconsolerpc.Device{}
	for _, device := range devices {
		satisfies, err := deviceSatisfiesFilters(ast, device)
		if err != nil {
			return nil, err
		}

		if satisfies {
			filteredDevices = append(filteredDevices, device)
		}
	}
	return filteredDevices, nil
}

func getDeviceProperty(path string, device *fleetconsolerpc.Device) (bool, []string) {
	re := regexp.MustCompile(`labels\.(.*)`)
	match := re.FindStringSubmatch(path)

	if len(match) > 1 {
		labelName := match[1]
		label, ok := device.DeviceSpec.Labels[labelName]

		if !ok {
			// since labels are dynamic, we treat all labels as existing, as it is not an error
			return true, []string{""}
		}
		return true, label.Values
	}

	dimensionDescriptor, ok := dimensions.GetDimensionDescriptorsMap()[path]

	if !ok {
		return false, []string{""}
	}

	return true, []string{dimensionDescriptor.StringValueGetter(device)}
}

func deviceSatisfiesFilters(filter *aip160.Filter, device *fleetconsolerpc.Device) (bool, error) {
	propertyGetter := func(s string) (bool, []string) {
		return getDeviceProperty(s, device)
	}

	deviceSatisfiesGlobalProperty := func(s string) (bool, error) {
		return false, fmt.Errorf("global properties are not supported. Got: %s", s)
	}

	return SatisfiesFilters(filter, propertyGetter, deviceSatisfiesGlobalProperty)
}
