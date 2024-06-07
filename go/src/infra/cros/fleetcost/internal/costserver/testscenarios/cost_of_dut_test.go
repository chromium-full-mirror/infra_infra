// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testscenarios

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp/cmpopts"

	"go.chromium.org/luci/common/testing/typed"

	fleetcostpb "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/costserver"
	"infra/cros/fleetcost/internal/costserver/fakeufsdata"
	"infra/cros/fleetcost/internal/costserver/testsupport"
	"infra/cros/fleetcost/internal/utils"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

// TestTotalCostOfDUT tests the cost of a DUT.
func TestTotalCostOfDUT(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	// acquisition costs
	const dutCost = 6177.61
	const labstationCost = 8808.21
	const servoV4P1 = 583.17
	const rackNetworking = 20.14
	const droneServer = 22.903
	const rackSetup = 588.541
	const usbHub = 1918.86
	// annual costs
	const cloudCosts = 447.634

	// Normalize one-time costs to 60 years.
	const amortizationTimeYears = 60.0

	// All numbers are quoted in cost per hour
	const hoursPerYear = 365 * 24

	expected := &fleetcostAPI.GetCostResultResponse{
		Result: &fleetcostpb.CostResult{
			DedicatedCost:    (dutCost + servoV4P1) / amortizationTimeYears / hoursPerYear,
			SharedCost:       (rackNetworking + droneServer + rackSetup + labstationCost + usbHub) / amortizationTimeYears / hoursPerYear,
			CloudServiceCost: cloudCosts / hoursPerYear,
		},
	}

	// Load the data
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Cost:                utils.FloatToMoney(dutCost),
		Type:                fleetcostpb.IndicatorType_INDICATOR_TYPE_DUT,
		Location:            fleetcostpb.Location_LOCATION_ALL,
		Board:               "build-target",
		CostCadence:         fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME,
		AmortizationInYears: amortizationTimeYears,
	})
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Cost:                utils.FloatToMoney(labstationCost),
		Type:                fleetcostpb.IndicatorType_INDICATOR_TYPE_LABSTATION,
		Location:            fleetcostpb.Location_LOCATION_ALL,
		CostCadence:         fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME,
		AmortizationInYears: amortizationTimeYears,
	})
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Cost:                utils.FloatToMoney(servoV4P1),
		Type:                fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVO,
		Location:            fleetcostpb.Location_LOCATION_ALL,
		CostCadence:         fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME,
		AmortizationInYears: amortizationTimeYears,
	})
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Cost:                utils.FloatToMoney(rackNetworking),
		Type:                fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:               "rack-networking",
		Location:            fleetcostpb.Location_LOCATION_ALL,
		CostCadence:         fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME,
		AmortizationInYears: amortizationTimeYears,
	})
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Cost:                utils.FloatToMoney(droneServer),
		Type:                fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:               "drone-server",
		Location:            fleetcostpb.Location_LOCATION_ALL,
		CostCadence:         fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME,
		AmortizationInYears: amortizationTimeYears,
	})
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Cost:                utils.FloatToMoney(rackSetup),
		Type:                fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:               "rack-setup",
		Location:            fleetcostpb.Location_LOCATION_ALL,
		CostCadence:         fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME,
		AmortizationInYears: amortizationTimeYears,
	})
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Cost:                utils.FloatToMoney(usbHub),
		Type:                fleetcostpb.IndicatorType_INDICATOR_TYPE_USBHUB,
		Location:            fleetcostpb.Location_LOCATION_ALL,
		CostCadence:         fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME,
		AmortizationInYears: amortizationTimeYears,
	})

	// Annual Costs
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Cost:        utils.FloatToMoney(cloudCosts),
		Type:        fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
		Location:    fleetcostpb.Location_LOCATION_ALL,
		CostCadence: fleetcostpb.CostCadence_COST_CADENCE_ANNUALLY,
	})

	fakeOctopusDut1Matcher := testsupport.NewMatcher("matcher", func(item any) bool {
		req, ok := item.(*ufsAPI.GetDeviceDataRequest)
		if !ok {
			panic("item has wrong type")
		}
		return req.GetHostname() == fakeufsdata.FakeOctopusDUTHostname
	})

	tf.RegisterGetDeviceDataCall(fakeOctopusDut1Matcher, fakeufsdata.FakeOctopusDUTWithServoDeviceDataResponse)
	tf.RegisterGetChromeOSDeviceDataCall(gomock.Any(), &ufspb.ChromeOSDeviceData{})
	tf.RegisterDUTsForLabstation(gomock.Any(), &ufsAPI.GetDUTsForLabstationResponse{
		Items: []*ufsAPI.GetDUTsForLabstationResponse_LabstationMapping{{
			Hostname: "fake-labstation",
			DutName: []string{
				fakeufsdata.FakeOctopusDUTHostname,
			},
		}},
	})

	resp, err := tf.Frontend.GetCostResult(tf.Ctx, &fleetcostAPI.GetCostResultRequest{
		Hostname: fakeufsdata.FakeOctopusDUTHostname,
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if diff := typed.Got(resp).Want(expected).Options(cmpopts.EquateApprox(0, 0.000001)).Diff(); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}
