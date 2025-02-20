// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testscenarios

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"google.golang.org/genproto/googleapis/type/money"

	fleetcostModels "go.chromium.org/infra/cros/fleetcost/api/models"
	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/fakeufsdata"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/testsupport"
)

// TestFallbackCostResult tests the flow where we fall back to the cost of a DUT.
func TestFallbackCostResult(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	tf.RegisterGetDeviceDataCall(gomock.Any(), fakeufsdata.FakeOctopusDUTDeviceDataResponse)

	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostModels.CostIndicator{
		Type:     fleetcostModels.IndicatorType_INDICATOR_TYPE_DUT,
		Location: fleetcostModels.Location_LOCATION_ALL,
		Cost: &money.Money{
			CurrencyCode: "USD",
			Units:        1056,
		},
		CostCadence: fleetcostModels.CostCadence_COST_CADENCE_HOURLY,
	})

	result, err := tf.Frontend.GetCostResult(tf.Ctx, &fleetcostAPI.GetCostResultRequest{
		Hostname:              "fake-octopus-dut-1",
		ForgiveMissingEntries: true,
	})

	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if cost := result.GetDedicatedCost(); cost != 1056 {
		t.Errorf("unexpected cost: %f", cost)
	}
}
