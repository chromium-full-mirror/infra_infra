// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller_test

import (
	"context"
	"testing"

	"google.golang.org/genproto/googleapis/type/money"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/testing/typed"

	fleetcostpb "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/costserver/controller"
	"infra/cros/fleetcost/internal/costserver/testsupport"
	ufspb "infra/unifiedfleet/api/v1/models"
	lab "infra/unifiedfleet/api/v1/models/chromeos/lab"
)

// TestCalculateCostForSingleChromeosDut tests the happy path for getting the cost estimate for a ChromeOS device.
//
// Here we look up the cost for a device with only a board and a model.
func TestCalculateCostForSingleChromeosDut(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	req := &ufspb.ChromeOSDeviceData{
		LabConfig: &ufspb.MachineLSE{
			Lse: &ufspb.MachineLSE_ChromeosMachineLse{
				ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
					ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
						DeviceLse: &ufspb.ChromeOSDeviceLSE{
							Device: &ufspb.ChromeOSDeviceLSE_Dut{
								Dut: &lab.DeviceUnderTest{
									Hostname: "a",
								},
							},
						},
					},
				},
			},
		},
		Machine: &ufspb.Machine{
			Device: &ufspb.Machine_ChromeosMachine{
				ChromeosMachine: &ufspb.ChromeOSMachine{
					BuildTarget: "build-target",
					Model:       "model",
				},
			},
		},
	}

	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   "build-target",
			Secondary: "model",
			Tertiary:  "",
			Location:  fleetcostpb.Location_LOCATION_ALL,
			Type:      fleetcostpb.IndicatorType_INDICATOR_TYPE_DUT,
			Cost: &money.Money{
				CurrencyCode: "USD",
				Units:        12,
			},
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}); err != nil {
		panic(err)
	}

	resp, _, err := controller.CalculateCostForSingleChromeosDut(tf.Ctx, tf.MockUFS, req, true, false)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if diff := typed.Got(resp).Want(&fleetcostpb.CostResult{
		DedicatedCost:    12.0,
		SharedCost:       0.0,
		CloudServiceCost: 0.0,
	}).Diff(); diff != "" {
		t.Errorf("unexpected error (-want +got): %s", diff)
	}
}

func TestCalculateCostForOsResource_NoUfs(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	req := &fleetcostAPI.GetCostResultRequest{
		Hostname:              "arbitrary string",
		ForgiveMissingEntries: true,
		NoUfs:                 true,
		AnalysisHint:          []string{"chromeos"},
	}

	result, report, err := controller.CalculateCostForOsResource(tf.Ctx, nil, nil, req)
	assert.Loosely(t, err, should.BeNil)
	assert.Loosely(t, result, should.NotBeNil)
	assert.Loosely(t, report, should.NotBeNil)
}
