// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package validation contains validation for requests.
package validation_test

import (
	"testing"

	"google.golang.org/genproto/googleapis/type/money"

	models "go.chromium.org/infra/cros/fleetcost/api/models"
	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
	"go.chromium.org/infra/cros/fleetcost/internal/validation"
)

// TestValidateCreateCostIndicatorRequest tests incoming indicator creation requests.
func TestValidateCreateCostIndicatorRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   *fleetcostAPI.CreateCostIndicatorRequest
		ok   bool
	}{
		{
			name: "request with user-defined name",
			in: &fleetcostAPI.CreateCostIndicatorRequest{
				CostIndicator: &models.CostIndicator{
					Name:        "aaa",
					Location:    models.Location_LOCATION_IAD65,
					Type:        models.IndicatorType_INDICATOR_TYPE_CLOUD,
					CostCadence: models.CostCadence_COST_CADENCE_HOURLY,
				},
			},
			ok: false,
		},
		{
			name: "request with unknown location",
			in: &fleetcostAPI.CreateCostIndicatorRequest{
				CostIndicator: &models.CostIndicator{
					Name:        "",
					Location:    models.Location_LOCATION_UNKNOWN,
					Type:        models.IndicatorType_INDICATOR_TYPE_CLOUD,
					CostCadence: models.CostCadence_COST_CADENCE_HOURLY,
				},
			},
			ok: false,
		},
		{
			name: "request with unknown type",
			in: &fleetcostAPI.CreateCostIndicatorRequest{
				CostIndicator: &models.CostIndicator{
					Name:        "",
					Location:    models.Location_LOCATION_ALL,
					Type:        models.IndicatorType_INDICATOR_TYPE_UNKNOWN,
					CostCadence: models.CostCadence_COST_CADENCE_HOURLY,
				},
			},
			ok: false,
		},
		{
			name: "record with unknown location",
			in: &fleetcostAPI.CreateCostIndicatorRequest{
				CostIndicator: &models.CostIndicator{
					Name:     "",
					Location: models.Location_LOCATION_UNKNOWN,
					Type:     models.IndicatorType_INDICATOR_TYPE_CLOUD,
					Cost: &money.Money{
						CurrencyCode: "USD",
						Units:        123.0,
					},
					CostCadence: models.CostCadence_COST_CADENCE_HOURLY,
				},
			},
			ok: false,
		},
		{
			name: "good record",
			in: &fleetcostAPI.CreateCostIndicatorRequest{
				CostIndicator: &models.CostIndicator{
					Name:     "",
					Location: models.Location_LOCATION_ALL,
					Type:     models.IndicatorType_INDICATOR_TYPE_CLOUD,
					Cost: &money.Money{
						CurrencyCode: "USD",
						Units:        123.0,
					},
					CostCadence: models.CostCadence_COST_CADENCE_HOURLY,
				},
			},
			ok: true,
		},
		{
			name: "good record with amortization",
			in: &fleetcostAPI.CreateCostIndicatorRequest{
				CostIndicator: &models.CostIndicator{
					Name:     "",
					Location: models.Location_LOCATION_ALL,
					Type:     models.IndicatorType_INDICATOR_TYPE_CLOUD,
					Cost: &money.Money{
						CurrencyCode: "USD",
						Units:        123.0,
					},
					CostCadence:         models.CostCadence_COST_CADENCE_ONE_TIME,
					AmortizationInYears: 0.380765,
				},
			},
			ok: true,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validation.ValidateCreateCostIndicatorRequest(tt.in)
			ok := (err == nil)

			switch {
			case ok && !tt.ok:
				t.Error("in is unexpectedly ok")
			case !ok && tt.ok:
				t.Errorf("in is unexpectedly not ok: %s", err)
			}
		})
	}
}
