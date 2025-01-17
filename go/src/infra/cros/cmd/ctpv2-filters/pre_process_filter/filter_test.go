// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"infra/cros/cmd/ctpv2-filters/pre_process_filter/structs"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// compareSlices compares two slices element by element
func compareSlices(slice1, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	for i := range slice1 {
		if slice1[i] != slice2[i] {
			return false
		}
	}

	return true
}

func TestFilter_testEligible(t *testing.T) {
	tests := []struct {
		name             string
		filter           *Filter
		value            string
		expectedResult   bool
		expectedRemoved  []string
		expectedNotFound []string
	}{
		{
			name: "Test found and eligible",
			filter: &Filter{
				data: map[string]structs.SignalFormat{
					"test1": {Signal: true},
				},
				removed:  []string{},
				notFound: []string{},
				req:      &api.FilterFlakyRequest{DefaultEnabled: false},
			},
			value:            "test1",
			expectedResult:   true,
			expectedRemoved:  []string{},
			expectedNotFound: []string{},
		},
		{
			name: "Test found but not eligible",
			filter: &Filter{
				data: map[string]structs.SignalFormat{
					"test2": {Signal: false},
				},
				removed:  []string{},
				notFound: []string{},
				req:      &api.FilterFlakyRequest{DefaultEnabled: false},
			},
			value:            "test2",
			expectedResult:   false,
			expectedRemoved:  []string{"test2"},
			expectedNotFound: []string{},
		},
		{
			name: "Test not found, default enabled",
			filter: &Filter{
				data:     map[string]structs.SignalFormat{},
				removed:  []string{},
				notFound: []string{},
				req:      &api.FilterFlakyRequest{DefaultEnabled: true},
			},
			value:            "test3",
			expectedResult:   true,
			expectedRemoved:  []string{},
			expectedNotFound: []string{"test3"},
		},
		{
			name: "Test not found, default disabled",
			filter: &Filter{
				data:     map[string]structs.SignalFormat{},
				removed:  []string{},
				notFound: []string{},
				req:      &api.FilterFlakyRequest{DefaultEnabled: false},
			},
			value:            "test4",
			expectedResult:   false,
			expectedRemoved:  []string{},
			expectedNotFound: []string{"test4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.testEligible(tt.value)
			if tt.expectedResult != result {
				t.Errorf("Expected result not same")
			}
			if !compareSlices(tt.expectedNotFound, tt.filter.notFound) {
				t.Errorf("Expected notFound not same")
			}
			if !compareSlices(tt.expectedRemoved, tt.filter.removed) {
				t.Errorf("Expected removed not same")
			}
		})
	}
}
