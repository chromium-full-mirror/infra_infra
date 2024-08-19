// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package maskutils

import (
	fleetcostpb "infra/cros/fleetcost/api/models"
)

// UpdateCostIndicatorProto updates the cost indicator proto.
//
// The fieldmask argument is probably an expression of the form request.GetFieldMask().GetPaths().
//
// Between the source and destinations the names must match OR one or both must be empty.
func UpdateCostIndicatorProto(dst *fleetcostpb.CostIndicator, src *fleetcostpb.CostIndicator, fieldmask []string) {
	if dst == nil {
		return
	}
	if !compatibleNames(dst.GetName(), src.GetName()) {
		return
	}
	for _, field := range fieldmask {
		updateCostIndicatorField(dst, src, field)
	}
}

// updateCostIndicatorField updates a single field in a cost indicator proto.
func updateCostIndicatorField(dst *fleetcostpb.CostIndicator, src *fleetcostpb.CostIndicator, field string) {
	switch field {
	// The field "name" is specifically prohibited from being updated using a mask.
	case "name":
		return
	case "type":
		dst.Type = src.GetType()
	case "primary":
		dst.Primary = src.GetPrimary()
	case "secondary":
		dst.Secondary = src.GetSecondary()
	case "tertiary":
		dst.Tertiary = src.GetTertiary()
	case "cost":
		dst.Cost = src.GetCost()
	case "cost_cadence":
		dst.CostCadence = src.GetCostCadence()
	case "burnout_rate":
		dst.BurnoutRate = src.GetBurnoutRate()
	case "location":
		dst.Location = src.GetLocation()
	case "description":
		dst.Description = src.GetDescription()
	case "amortization_in_years":
		dst.AmortizationInYears = src.GetAmortizationInYears()
	}
}

// compatibleNames checks to see that the right name is an acceptable candidate to assign to the left.
//
// This condition fails precisely when the left and right name are nonempty and also not equal to each other.
func compatibleNames(leftName string, rightName string) bool {
	if leftName == "" || rightName == "" {
		return true
	}
	return leftName == rightName
}
