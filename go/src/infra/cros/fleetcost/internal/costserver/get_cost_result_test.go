// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package costserver_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/type/money"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	fleetcostModels "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/costserver"
	"infra/cros/fleetcost/internal/costserver/controller"
	"infra/cros/fleetcost/internal/costserver/entities"
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
		Type:        fleetcostModels.IndicatorType_INDICATOR_TYPE_DUT,
		Primary:     "build-target",
		Secondary:   "model",
		Tertiary:    "",
		Location:    fleetcostModels.Location_LOCATION_ALL,
		CostCadence: fleetcostModels.CostCadence_COST_CADENCE_HOURLY,
		Cost: &money.Money{
			CurrencyCode: "USD",
			Units:        134,
		},
	})

	_, err := tf.Frontend.GetCostResult(tf.Ctx, &fleetcostAPI.GetCostResultRequest{
		Hostname:              "fake-octopus-dut-1",
		ForgiveMissingEntries: true,
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

// TestGetCostResultWithStaleCacheEntry tests that reading an old cache entry that is
// beyond its time horizon still causes us to go back and recompute the cost.
func TestGetCostResultWithStaleCacheEntry(t *testing.T) {
	t.Parallel()
	tf := testsupport.NewFixture(context.Background(), t)

	if err := controller.StoreCachedCostResult(tf.Ctx, "fake-hostname", &fleetcostModels.CostResult{
		DedicatedCost: 30,
	}); err != nil {
		t.Errorf("unexpected error when filling cache: %s", err)
	}
	tf.AdvanceClock(5 * time.Hour)

	fakeOctopusDut1Matcher := testsupport.NewMatcher("matcher", func(item any) bool {
		req, ok := item.(*ufsAPI.GetDeviceDataRequest)
		if !ok {
			panic("item has wrong type")
		}
		return req.GetHostname() == "fake-octopus-dut-1"
	})
	tf.RegisterGetDeviceDataCall(fakeOctopusDut1Matcher, fakeufsdata.FakeOctopusDUTDeviceDataResponse)

	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostModels.CostIndicator{
		Type:        fleetcostModels.IndicatorType_INDICATOR_TYPE_DUT,
		Primary:     "build-target",
		Secondary:   "model",
		Tertiary:    "",
		Location:    fleetcostModels.Location_LOCATION_ALL,
		CostCadence: fleetcostModels.CostCadence_COST_CADENCE_HOURLY,
		Cost: &money.Money{
			CurrencyCode: "USD",
			Units:        134,
		},
	})

	_, err := tf.Frontend.GetCostResult(tf.Ctx, &fleetcostAPI.GetCostResultRequest{
		Hostname:              "fake-octopus-dut-1",
		ForgiveMissingEntries: true,
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

// TestEntHasCostResult tests the helper function entHasCostResult to make sure that it doesn't panic.
// That is the root cause behind https://b.corp.google.com/issues/362793808.
func TestEntHasCostResult(t *testing.T) {
	t.Parallel()

	assert.That(t, costserver.EntHasCostResult(nil, nil), should.BeFalse)
	assert.That(t, costserver.EntHasCostResult(errors.New("hi"), nil), should.BeFalse)
	assert.That(t, costserver.EntHasCostResult(errors.New("hi"), &entities.CachedCostResultEntity{}), should.BeFalse)
	assert.That(t, costserver.EntHasCostResult(errors.New("hi"), &entities.CachedCostResultEntity{
		CostResult: &fleetcostModels.CostResult{},
	}), should.BeFalse)
	assert.That(t, costserver.EntHasCostResult(nil, &entities.CachedCostResultEntity{}), should.BeFalse)
	assert.That(t, costserver.EntHasCostResult(nil, &entities.CachedCostResultEntity{
		CostResult: &fleetcostModels.CostResult{},
	}), should.BeTrue)
}
