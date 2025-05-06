// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testscenarios

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp/cmpopts"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	fleetcostpb "go.chromium.org/infra/cros/fleetcost/api/models"
	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/fakeufsdata"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/testsupport"
	"go.chromium.org/infra/cros/fleetcost/internal/utils"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
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

	dedicatedCost := (dutCost + servoV4P1) / amortizationTimeYears / hoursPerYear
	sharedCost := (rackNetworking + droneServer + rackSetup + labstationCost + usbHub) / amortizationTimeYears / hoursPerYear
	cloudServiceCost := cloudCosts / hoursPerYear
	totalCost := dedicatedCost + sharedCost + cloudServiceCost
	expected := &fleetcostpb.CostResult{
		DedicatedCost:    dedicatedCost,
		SharedCost:       sharedCost,
		CloudServiceCost: cloudServiceCost,
		Report: &fleetcostpb.CostReport{
			Total: totalCost,
			Expr: &fleetcostpb.CostReportExpr{
				Label: &fleetcostpb.CostReportLabel{Name: "total"},
				Item: []*fleetcostpb.CostReportItem{
					{
						SubtotalName:    "dut cost",
						RawFigure:       dutCost / amortizationTimeYears / hoursPerYear,
						PerDeviceFigure: dutCost / amortizationTimeYears / hoursPerYear,
						Category:        "dedicated",
					},
					{
						SubtotalName:    "servo cost",
						RawFigure:       servoV4P1 / amortizationTimeYears / hoursPerYear,
						PerDeviceFigure: servoV4P1 / amortizationTimeYears / hoursPerYear,
						Category:        "dedicated",
					},
					{
						SubtotalName:    "testbed cost",
						RawFigure:       0,
						PerDeviceFigure: 0,
						Category:        "dedicated",
					},
					{
						SubtotalName:    "server acquisition",
						RawFigure:       droneServer / amortizationTimeYears / hoursPerYear,
						PerDeviceFigure: droneServer / amortizationTimeYears / hoursPerYear,
						Category:        "shared",
					},
					{
						SubtotalName:    "server maintenance",
						RawFigure:       0,
						PerDeviceFigure: 0,
						Category:        "shared",
					},
					{
						SubtotalName:    "network infra acquisition",
						RawFigure:       0,
						PerDeviceFigure: 0,
						Category:        "shared",
					},
					{
						SubtotalName:    "network infra maintenance",
						RawFigure:       0,
						PerDeviceFigure: 0,
						Category:        "shared",
					},
					{
						SubtotalName:    "quota faft opex",
						RawFigure:       0,
						PerDeviceFigure: 0,
						Category:        "shared",
					},
					{
						SubtotalName:    "connectivity and misc testbeds",
						RawFigure:       0,
						PerDeviceFigure: 0,
						Category:        "shared",
					},
					{
						SubtotalName:    "control network racks",
						RawFigure:       rackNetworking / amortizationTimeYears / hoursPerYear,
						PerDeviceFigure: rackNetworking / amortizationTimeYears / hoursPerYear,
						Category:        "shared",
					},
					{
						SubtotalName:    "rack setup",
						RawFigure:       rackSetup / amortizationTimeYears / hoursPerYear,
						PerDeviceFigure: rackSetup / amortizationTimeYears / hoursPerYear,
						Category:        "shared",
					},
					{
						SubtotalName:    "phase deployments",
						RawFigure:       0,
						PerDeviceFigure: 0,
						Category:        "shared",
					},
					{
						SubtotalName:    "annual maintenance",
						RawFigure:       0,
						PerDeviceFigure: 0,
						Category:        "shared",
					},
					{
						SubtotalName:    "labstation",
						RawFigure:       (labstationCost + usbHub) / amortizationTimeYears / hoursPerYear,
						PerDeviceFigure: (labstationCost + usbHub) / amortizationTimeYears / hoursPerYear,
						Category:        "shared",
					},
					{
						SubtotalName:    "cloud",
						RawFigure:       cloudServiceCost,
						PerDeviceFigure: cloudServiceCost,
						Category:        "cloud",
					},
				},
			},
		},
	}

	// Load the data
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Cost:                utils.FloatToMoney(dutCost),
		Type:                fleetcostpb.IndicatorType_INDICATOR_TYPE_DUT,
		Location:            fleetcostpb.Location_LOCATION_ALL,
		Primary:             "build-target",
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
		Primary:             "control-network-racks",
		Location:            fleetcostpb.Location_LOCATION_ALL,
		CostCadence:         fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME,
		AmortizationInYears: amortizationTimeYears,
	})
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Cost:                utils.FloatToMoney(droneServer),
		Type:                fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Primary:             "server-acquisition",
		Location:            fleetcostpb.Location_LOCATION_ALL,
		CostCadence:         fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME,
		AmortizationInYears: amortizationTimeYears,
	})
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Cost:                utils.FloatToMoney(rackSetup),
		Type:                fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Primary:             "rack-setup",
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
		Hostname:              fakeufsdata.FakeOctopusDUTHostname,
		ForgiveMissingEntries: true,
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	assert.That(t, resp, should.Match(expected, cmpopts.EquateApprox(0, 0.000001)))
}
