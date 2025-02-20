// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package validation contains validation for requests.
package validation

import (
	"fmt"

	"go.chromium.org/luci/common/errors"

	fleetcostpb "go.chromium.org/infra/cros/fleetcost/api/models"
	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
	"go.chromium.org/infra/cros/fleetcost/internal/utils"
)

// ValidateCreateCostIndicatorRequest performs a shallow validation of cost indicator request fields.
func ValidateCreateCostIndicatorRequest(request *fleetcostAPI.CreateCostIndicatorRequest) error {
	var errs []error
	indicator := request.GetCostIndicator()
	if indicator.GetName() != "" {
		errs = append(errs, errors.New("indicator name must be empty"))
	}
	// No requirements imposed on Board field.
	// No requirements imposed on Model field.
	if indicator.GetCost() == nil {
		errs = append(errs, errors.New("cost must be provided"))
	}
	if request.GetCostIndicator().GetCostCadence() == fleetcostpb.CostCadence_COST_CADENCE_UNKNOWN {
		return errors.New("must provide cost cadence")
	}
	if indicator.GetLocation() == fleetcostpb.Location_LOCATION_UNKNOWN {
		errs = append(errs, errors.New("must provide valid location"))
	}
	if indicator.GetType() == fleetcostpb.IndicatorType_INDICATOR_TYPE_UNKNOWN {
		errs = append(errs, errors.New("must provide valid type"))
	}
	errs = append(errs, validateCostIndicatorAmortizationTime(request))
	return errors.Append(errs...)
}

// validateCostIndicatorAmortizationTime checks that that cost indicators with a one-time cost have
// a valid amortization time.
func validateCostIndicatorAmortizationTime(request *fleetcostAPI.CreateCostIndicatorRequest) error {
	if request.GetCostIndicator().GetCostCadence() != fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME {
		return nil
	}
	if request.GetCostIndicator().GetAmortizationInYears() > 0 {
		return nil
	}
	return fmt.Errorf(
		"cost indicator %q has cadence %s and amortization %f, which is invalid",
		utils.IndicatorToString(request.GetCostIndicator()),
		request.GetCostIndicator().GetCostCadence().String(),
		request.CostIndicator.GetAmortizationInYears(),
	)
}
