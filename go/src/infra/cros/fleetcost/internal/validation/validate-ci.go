// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package validation contains validation for requests.
package validation

import (
	"go.chromium.org/luci/common/errors"

	fleetcostpb "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
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
	if indicator.GetLocation() == fleetcostpb.Location_LOCATION_UNKNOWN {
		errs = append(errs, errors.New("must provide valid location"))
	}
	if indicator.GetType() == fleetcostpb.IndicatorType_INDICATOR_TYPE_UNKNOWN {
		errs = append(errs, errors.New("must provide valid type"))
	}
	if indicator.GetCostCadence() == fleetcostpb.CostCadence_COST_CADENCE_UNKNOWN {
		errs = append(errs, errors.New("must provide valid type"))
	}
	return errors.Append(errs...)
}
