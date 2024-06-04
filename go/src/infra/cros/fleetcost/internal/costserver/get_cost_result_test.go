// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package costserver_test

import (
	"context"
	"testing"

	"google.golang.org/genproto/googleapis/type/money"

	"go.chromium.org/luci/common/errors"

	fleetcostModels "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/costserver"
	"infra/cros/fleetcost/internal/costserver/fakeufsdata"
	"infra/cros/fleetcost/internal/costserver/testsupport"
	"infra/cros/fleetcost/internal/utils"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

// TestGetCostResult tests the last mile of the cost result API.
//
// More specifically, it tests that looking up a DUT that exists works.
// It also checks that looking up a DUT that does not exist doesn't work.
func TestGetCostResult(t *testing.T) {
	t.Parallel()
	tf := testsupport.NewFixture(context.Background(), t)

	fakeOctopusDut1Matcher := testsupport.NewMatcher("matcher", func(item any) bool {
		req, ok := item.(*ufsAPI.GetDeviceDataRequest)
		if !ok {
			panic("item has wrong type")
		}
		return req.GetHostname() == "fake-octopus-dut-1"
	})
	tf.RegisterGetDeviceDataCall(fakeOctopusDut1Matcher, fakeufsdata.FakeOctopusDUTDeviceDataResponse)

	fakeOctopusDut2Matcher := testsupport.NewMatcher("matcher", func(item any) bool {
		req, ok := item.(*ufsAPI.GetDeviceDataRequest)
		if !ok {
			panic("item has wrong type")
		}
		return req.GetHostname() == "fake-octopus-dut-2"
	})
	tf.RegisterGetDeviceDataFailure(fakeOctopusDut2Matcher, errors.New("a wild error appears"))

	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostModels.CostIndicator{
		Type:     fleetcostModels.IndicatorType_INDICATOR_TYPE_DUT,
		Board:    "build-target",
		Model:    "model",
		Sku:      "",
		Location: fleetcostModels.Location_LOCATION_ALL,
		Cost: &money.Money{
			CurrencyCode: "USD",
			Units:        134,
		},
		CostCadence: fleetcostModels.CostCadence_COST_CADENCE_HOURLY,
	})

	_, err := tf.Frontend.GetCostResult(tf.Ctx, &fleetcostAPI.GetCostResultRequest{
		Hostname: "fake-octopus-dut-1",
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	_, err = tf.Frontend.GetCostResult(tf.Ctx, &fleetcostAPI.GetCostResultRequest{
		Hostname: "fake-octopus-dut-2",
	})
	if err == nil {
		t.Errorf("error should not have been nil")
	}
	if !utils.ErrorStringContains(err, "a wild error appears") {
		t.Errorf("non-nil error %s is unexpected", err)
	}
}
