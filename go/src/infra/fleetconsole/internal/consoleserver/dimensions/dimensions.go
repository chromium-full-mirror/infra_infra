// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dimensions

import (
	"strconv"

	"infra/fleetconsole/api/fleetconsolerpc"
)

type DimensionDescriptor struct {
	ID                string // id used as client-server contract for sorting and filtering
	Comparator        func(a, b *fleetconsolerpc.Device) bool
	StringValueGetter func(d *fleetconsolerpc.Device) string // used for filter available values
}

func GetDimensionDescriptors() []*DimensionDescriptor {
	return []*DimensionDescriptor{
		{
			ID: "id",
			Comparator: func(a, b *fleetconsolerpc.Device) bool {
				return a.Id < b.Id
			},
			StringValueGetter: func(d *fleetconsolerpc.Device) string {
				return d.Id
			},
		},
		{
			ID: "dut_id",
			Comparator: func(a, b *fleetconsolerpc.Device) bool {
				return a.DutId < b.DutId
			},
			StringValueGetter: func(d *fleetconsolerpc.Device) string {
				return d.DutId
			},
		},
		{
			ID: "address.host",
			Comparator: func(a, b *fleetconsolerpc.Device) bool {
				return a.Address.Host < b.Address.Host
			},
			StringValueGetter: func(d *fleetconsolerpc.Device) string {
				return d.Address.Host
			},
		},
		{
			ID: "address.port",
			Comparator: func(a, b *fleetconsolerpc.Device) bool {
				return a.Address.Port < b.Address.Port
			},
			StringValueGetter: func(d *fleetconsolerpc.Device) string {
				return strconv.Itoa(int(d.Address.Port))
			},
		},
		{
			ID: "state",
			Comparator: func(a, b *fleetconsolerpc.Device) bool {
				return a.State < b.State
			},
			StringValueGetter: func(d *fleetconsolerpc.Device) string {
				return d.State.String()
			},
		},
		{
			ID: "type",
			Comparator: func(a, b *fleetconsolerpc.Device) bool {
				return a.Type < b.Type
			},
			StringValueGetter: func(d *fleetconsolerpc.Device) string {
				return d.Type.String()
			},
		},
	}
}

func GetDimensionDescriptorsMap() map[string]*DimensionDescriptor {
	dimensionDescriptors := GetDimensionDescriptors()
	dimensionDescriptorsMap := map[string]*DimensionDescriptor{}

	for _, dimensionDescriptor := range dimensionDescriptors {
		dimensionDescriptorsMap[dimensionDescriptor.ID] = dimensionDescriptor
	}

	return dimensionDescriptorsMap
}
