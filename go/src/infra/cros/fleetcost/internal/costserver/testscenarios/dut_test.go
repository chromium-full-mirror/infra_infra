// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testscenarios

import (
	"context"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"google.golang.org/genproto/googleapis/type/money"

	fleetcostpb "go.chromium.org/infra/cros/fleetcost/api/models"
	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/fakeufsdata"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/testsupport"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// TestNonexistentDUT tests trying to get the cost of a DUT that doesn't exist.
func TestNonexistentDUT(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	response := &ufsAPI.GetDeviceDataResponse{}

	tf.MockUFS.EXPECT().GetDeviceData(gomock.Any(), gomock.Any()).Return(response, nil)

	_, err := tf.Frontend.GetCostResult(tf.Ctx, &fleetcostAPI.GetCostResultRequest{
		Hostname: "fake-octopus-dut-1",
	})
	// TODO(gregorynisbet): Make this test a little smarter and look at the
	//                      GCP-reported error status too.
	if ok := (err != nil) && strings.Contains(err.Error(), "find a valid resource type"); !ok {
		t.Errorf("unexpected error: %s", err)
	}
}

// TestDUTWithNoPeripherals tests a device with no peripherals.
func TestDUTWithNoPeripherals(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   "build-target",
			Secondary: "model",
			Type:      fleetcostpb.IndicatorType_INDICATOR_TYPE_DUT,
			Location:  fleetcostpb.Location_LOCATION_ALL,
			Cost: &money.Money{
				Units: 100,
			},
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}); err != nil {
		panic(err)
	}

	tf.RegisterGetDeviceDataCall(gomock.Any(), fakeufsdata.FakeOctopusDUTDeviceDataResponse)

	_, err := tf.Frontend.GetCostResult(tf.Ctx, &fleetcostAPI.GetCostResultRequest{
		Hostname:              "fake-octopus-dut-1",
		ForgiveMissingEntries: true,
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}
