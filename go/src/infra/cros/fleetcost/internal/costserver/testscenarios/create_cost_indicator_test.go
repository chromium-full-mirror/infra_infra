// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testscenarios

import (
	"context"
	"testing"

	"google.golang.org/genproto/googleapis/type/money"

	models "go.chromium.org/infra/cros/fleetcost/api/models"
	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/testsupport"
)

// TestCannotCreateDuplicateCostIndicator tests the behavior of creating a duplicate cost entity.
//
// This must fail. It is a bad user experience if they can replace something without deleting it first.
func TestCannotCreateDuplicateCostIndicator(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	createCostIndicatorRequest1 := &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &models.CostIndicator{
			Primary:   "a",
			Secondary: "b",
			Location:  models.Location_LOCATION_ALL,
			Type:      models.IndicatorType_INDICATOR_TYPE_CLOUD,
			Cost: &money.Money{
				CurrencyCode: "USD",
				Units:        100,
			},
			CostCadence: models.CostCadence_COST_CADENCE_HOURLY,
		},
	}

	createCostIndicatorRequest2 := &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &models.CostIndicator{
			Primary:   "a",
			Secondary: "b",
			Location:  models.Location_LOCATION_ALL,
			Type:      models.IndicatorType_INDICATOR_TYPE_CLOUD,
			Cost: &money.Money{
				CurrencyCode: "USD",
				Units:        200,
			},
			CostCadence: models.CostCadence_COST_CADENCE_HOURLY,
		},
	}

	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, createCostIndicatorRequest1); err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, createCostIndicatorRequest2); err == nil {
		t.Error("second creation attempt MUST fail")
	}
}
