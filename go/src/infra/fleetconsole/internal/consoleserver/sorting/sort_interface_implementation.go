// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sorting

import (
	"sort"

	"infra/fleetconsole/api/fleetconsolerpc"
)

var _ sort.Interface = &sortImpl{}

// sortImpl is an implementation of sort.Interface required for sorting devices
type sortImpl struct {
	Devices         []*fleetconsolerpc.Device
	sortDescriptors []*SortDescriptor
}

type SortDescriptor struct {
	sortComparator sortComparator
	isDesc         bool
}

type sortComparator = func(a, b *fleetconsolerpc.Device) bool

func newSortImpl(devices []*fleetconsolerpc.Device, sortDescriptors []*SortDescriptor) *sortImpl {
	return &sortImpl{Devices: devices, sortDescriptors: sortDescriptors}
}

func (s *sortImpl) Len() int {
	return len(s.Devices)
}

func (s *sortImpl) Less(i int, j int) bool {
	for _, descriptor := range s.sortDescriptors {
		isLess := descriptor.sortComparator(s.Devices[i], s.Devices[j])
		isMore := descriptor.sortComparator(s.Devices[j], s.Devices[i])
		if !isLess && !isMore {
			continue
		}
		if descriptor.isDesc {
			return isMore
		} else {
			return isLess
		}
	}

	return false
}

func (s *sortImpl) Swap(i int, j int) {
	dummy := s.Devices[i]
	s.Devices[i] = s.Devices[j]
	s.Devices[j] = dummy
}
