// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package maskutils

import (
	"testing"

	"go.chromium.org/luci/common/testing/typed"

	fleetcostpb "go.chromium.org/infra/cros/fleetcost/api/models"
)

// TestUpdateCostIndicatorProtoHappyPath tests the happy path where the two protos are compatible
// and some-but-not-all of the fields are capable of being updated.
func TestUpdateCostIndicatorProtoHappyPath(t *testing.T) {
	t.Parallel()

	dst := &fleetcostpb.CostIndicator{
		Name:      "wombat",
		Primary:   "woof",
		Secondary: "aaaaa",
	}
	src := &fleetcostpb.CostIndicator{
		Name:      "wombat",
		Primary:   "the noise that wombats make",
		Secondary: "bbbbb",
	}

	UpdateCostIndicatorProto(dst, src, []string{"primary"})

	if dst.GetPrimary() != "the noise that wombats make" {
		t.Error("update cost failed to update board")
	}
	if dst.GetSecondary() != "aaaaa" {
		t.Errorf("model is unexpectedly %q", dst.GetSecondary())
	}
}

// TestUpdateCostIndicatorProto is a table-driven test for testing UpdateCostIndicatorProto edge cases.
func TestUpdateCostIndicatorProto(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		dst       *fleetcostpb.CostIndicator
		src       *fleetcostpb.CostIndicator
		fieldmask []string
		output    *fleetcostpb.CostIndicator
	}{
		{
			name:      "empty",
			dst:       nil,
			src:       nil,
			fieldmask: nil,
			output:    nil,
		},
		{
			name:      "empty with name fieldmask",
			dst:       nil,
			src:       nil,
			fieldmask: []string{"name"},
			output:    nil,
		},
		{
			name: "compatible name happy path",
			dst: &fleetcostpb.CostIndicator{
				Name:    "platypus",
				Primary: "old-board",
			},
			src: &fleetcostpb.CostIndicator{
				Name:    "platypus",
				Primary: "new-board",
			},
			fieldmask: []string{"primary"},
			output: &fleetcostpb.CostIndicator{
				Name:    "platypus",
				Primary: "new-board",
			},
		},
		{
			name: "compatible name wildcard name",
			dst: &fleetcostpb.CostIndicator{
				Name:    "platypus",
				Primary: "old-board",
			},
			src: &fleetcostpb.CostIndicator{
				Name:    "platypus",
				Primary: "new-board",
			},
			fieldmask: []string{"primary"},
			output: &fleetcostpb.CostIndicator{
				Name:    "platypus",
				Primary: "new-board",
			},
		},
		{
			name: "amortization in years",
			dst: &fleetcostpb.CostIndicator{
				Name:                "platypus",
				AmortizationInYears: 400,
			},
			src: &fleetcostpb.CostIndicator{
				Name:                "platypus",
				AmortizationInYears: 2,
			},
			fieldmask: []string{"amortization_in_years"},
			output: &fleetcostpb.CostIndicator{
				Name:                "platypus",
				AmortizationInYears: 2,
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			UpdateCostIndicatorProto(tt.dst, tt.src, tt.fieldmask)
			got := tt.dst
			want := tt.output
			if diff := typed.Got(got).Want(want).Diff(); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}
