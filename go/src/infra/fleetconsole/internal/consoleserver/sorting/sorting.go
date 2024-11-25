// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sorting

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"infra/fleetconsole/api/fleetconsolerpc"
)

// SortDevices sorts devices based on order, following google.aip.dev/132#ordering
func SortDevices(devices []*fleetconsolerpc.Device, orderBy string) ([]*fleetconsolerpc.Device, error) {
	sortImpl, err := getSortImpl(devices, orderBy)

	if err != nil {
		return nil, err
	}

	sort.Sort(sortImpl)

	return sortImpl.Devices, nil
}

// getSortComparator translates a string path to a dedicated comparator for a corresponding field
func getSortComparator(path string) (func(a, b *fleetconsolerpc.Device) bool, error) {
	regex := regexp.MustCompile(`labels\.(.*)`)
	match := regex.MatchString(path)

	if match {
		labelName := regex.ReplaceAllString(path, "$1")
		res := createComparator(func(d *fleetconsolerpc.Device) string {
			value, exists := d.DeviceSpec.Labels[labelName]
			if !exists {
				return ""
			}
			// TODO: b/379079889 - currently we are taking the longest value for sorting, mimicking Swarming UI's default display option. We should rethink it when implementing b/379079889
			return slices.MaxFunc(value.Values, func(a, b string) int { return len(a) - len(b) })
		})
		return res, nil
	}

	switch path {
	case "id":
		return createComparator(func(d *fleetconsolerpc.Device) string { return d.Id }), nil
	case "dut_id":
		return createComparator(func(d *fleetconsolerpc.Device) string { return d.DutId }), nil
	case "address.host":
		return createComparator(func(d *fleetconsolerpc.Device) string { return d.Address.Host }), nil
	case "address.port":
		return createComparator(func(d *fleetconsolerpc.Device) int { return int(d.Address.Port) }), nil
	case "state":
		return createComparator(func(d *fleetconsolerpc.Device) int { return int(d.State) }), nil
	case "type":
		return createComparator(func(d *fleetconsolerpc.Device) int { return int(d.Type) }), nil
	default:
		return nil, fmt.Errorf("path not found: %s", path)
	}
}

func getSortImpl(devices []*fleetconsolerpc.Device, orderBy string) (*sortImpl, error) {
	fieldsToSort := slices.DeleteFunc(strings.Split(orderBy, ","), func(s string) bool { return strings.TrimSpace(s) == "" })

	descriptors := []*SortDescriptor{}

	for _, field := range fieldsToSort {
		isDesc := false
		parts := strings.Fields(field) //should be either "<field>" or "<field> desc"
		if len(parts) > 2 {
			return nil, fmt.Errorf("unexpected field sorting descriptor. Expected 1 or 2 parts. %s", field)
		}
		if len(parts) > 1 {
			if !strings.EqualFold(parts[1], "desc") {
				return nil, fmt.Errorf("unexpected sorting direction. Expected \"desc\". Got: %s", parts[1])
			}
			isDesc = true
		}

		sortComparator, err := getSortComparator(parts[0])

		if err != nil {
			return nil, err
		}

		descriptors = append(descriptors, &SortDescriptor{sortComparator: sortComparator, isDesc: isDesc})
	}

	return newSortImpl(devices, descriptors), nil
}

func createComparator[T cmp.Ordered](f func(d *fleetconsolerpc.Device) T) func(a, b *fleetconsolerpc.Device) bool {
	return func(a, b *fleetconsolerpc.Device) bool { return f(a) < f(b) }
}
