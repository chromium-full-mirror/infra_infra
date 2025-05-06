// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package entities_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/genproto/googleapis/type/money"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	fleetcostpb "go.chromium.org/infra/cros/fleetcost/api/models"
	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/entities"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/testsupport"
	"go.chromium.org/infra/cros/fleetcost/internal/utils"
)

// TestCostIndicatorSimple tests putting a cost indicator into database and retrieving it.
func TestCostIndicatorSimple(t *testing.T) {
	t.Parallel()
	tf := testsupport.NewFixture(context.Background(), t)

	if err := datastore.Put(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:     "e",
			BurnoutRate: 12.0,
		},
	}); err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if err := datastore.Get(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:     "e",
			BurnoutRate: 12.0,
		},
	}); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

// TestCostIndicatorSimple tests that writing a cost indicator to the database populates the correct fields.
func TestCostIndicatorIndexedFields(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	oldIndicator := &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   "e",
			Secondary: "w",
		},
	}

	err := datastore.Put(tf.Ctx, oldIndicator)
	if err != nil {
		t.Fatal(err)
	}

	item := &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   "e",
			Secondary: "w",
		},
	}
	if err := datastore.Get(tf.Ctx, item); err != nil {
		t.Error(err)
	}

	assert.That(t, item, should.Match(&entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   "e",
			Secondary: "w",
		},
		Board: "e",
		Model: "w",
	}, cmp.AllowUnexported(entities.CostIndicatorEntity{})))
}

// TestCostIndicatorClone tests cloning a cost indicator
func TestCostIndicatorClone(t *testing.T) {
	t.Parallel()

	oldIndicator := &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Name:    "a",
			Primary: "e",
		},
	}

	newIndicator := oldIndicator.Clone()

	assert.That(t, newIndicator, should.Match(oldIndicator, cmp.AllowUnexported(*oldIndicator)))
}

func TestPutCostIndicator(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	err := utils.InsertOneWithoutReplacement(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:     "e",
			BurnoutRate: 12.0,
		},
	}, nil)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	result, err := entities.GetCostIndicatorEntity(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary: "e",
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if result.CostIndicator.GetBurnoutRate() != 12.0 {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestGetCostIndicator(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixtureWithData(context.Background(), t)

	costIndicator, err := entities.GetCostIndicatorEntity(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary: "e",
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	want := &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:     "e",
			BurnoutRate: 44.0,
		},
		Board: "e",
	}

	assert.That(t, costIndicator, should.Match(want, cmp.AllowUnexported(entities.CostIndicatorEntity{})))
}

// TestListCostIndicator tests listing all cost indicators in a scenario where this is only
// one cost indicator.
func TestListCostIndicator(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixtureWithData(context.Background(), t)

	costIndicators, err := entities.ListCostIndicators(tf.Ctx, 1, nil)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	want := []*fleetcostpb.CostIndicator{
		{
			Primary:     "e",
			BurnoutRate: 44.0,
		},
	}

	assert.That(t, costIndicators, should.Match(want, cmp.AllowUnexported(entities.CostIndicatorEntity{})))
}

// TestListCostIndicatorWithModelFilter tests listing devices with a model filter.
func TestListCostIndicatorWithModelFilter(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)
	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   "fake-board-1",
			Secondary: "fake-model",
			Location:  fleetcostpb.Location_LOCATION_ACS,
			Type:      fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
			Cost: &money.Money{
				CurrencyCode: "USD",
				Units:        100,
			},
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}); err != nil {
		panic(err)
	}
	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   "fake-board-2",
			Secondary: "fake-model",
			Location:  fleetcostpb.Location_LOCATION_ACS,
			Type:      fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
			Cost: &money.Money{
				CurrencyCode: "USD",
				Units:        200,
			},
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}); err != nil {
		panic(err)
	}
	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   "fake-board-2",
			Secondary: "a-different-model",
			Location:  fleetcostpb.Location_LOCATION_ACS,
			Type:      fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
			Cost: &money.Money{
				CurrencyCode: "USD",
				Units:        200,
			},
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}); err != nil {
		panic(err)
	}

	resp, err := tf.Frontend.ListCostIndicators(tf.Ctx, &fleetcostAPI.ListCostIndicatorsRequest{
		PageSize: 1000,
		Filter: &fleetcostAPI.ListCostIndicatorsFilter{
			Secondary: "fake-model",
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	assert.That(t, len(resp.GetCostIndicator()), should.Equal(2))
}

// TestListCostIndicatorWithSkuFilter tests listing devices with a SKU filter.
func TestListCostIndicatorWithSkuFilter(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)
	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   "fake-board-1",
			Secondary: "fake-model-1",
			Tertiary:  "fake-sku",
			Location:  fleetcostpb.Location_LOCATION_ACS,
			Type:      fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
			Cost: &money.Money{
				CurrencyCode: "USD",
				Units:        100,
			},
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}); err != nil {
		panic(err)
	}
	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   "fake-board-2",
			Secondary: "fake-model-2",
			Tertiary:  "fake-sku",
			Location:  fleetcostpb.Location_LOCATION_ACS,
			Type:      fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
			Cost: &money.Money{
				CurrencyCode: "USD",
				Units:        200,
			},
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}); err != nil {
		panic(err)
	}
	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:   "fake-board-3",
			Secondary: "fake-model-3",
			Tertiary:  "different-sku",
			Location:  fleetcostpb.Location_LOCATION_ACS,
			Type:      fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
			Cost: &money.Money{
				CurrencyCode: "USD",
				Units:        200,
			},
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}); err != nil {
		panic(err)
	}

	resp, err := tf.Frontend.ListCostIndicators(tf.Ctx, &fleetcostAPI.ListCostIndicatorsRequest{
		PageSize: 1000,
		Filter: &fleetcostAPI.ListCostIndicatorsFilter{
			Tertiary: "fake-sku",
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	assert.That(t, len(resp.GetCostIndicator()), should.Equal(2))
}

// TestUpdateCostIndicatorHappyPath tests updating a cost indicator that already exists.
//
// Note that when updating the record, we provide an argument that
func TestUpdateCostIndicatorHappyPath(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	if err := utils.InsertOneWithoutReplacement(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:     "fake-board",
			BurnoutRate: 12.0,
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}, nil); err != nil {
		t.Fatalf("failed to insert cost indicator: %s", err)
	}

	got, err := entities.UpdateCostIndicatorEntity(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:     "fake-board",
			BurnoutRate: 14.0,
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
		Board: "fake-board",
	}, []string{"burnout_rate"})
	if err != nil {
		t.Errorf("unexpected error: %q", err)
	}

	assert.That(t, got, should.Match(&entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:     "fake-board",
			BurnoutRate: 14.0,
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
		Board: "fake-board",
	}, cmp.AllowUnexported(*got)))
}

func TestDeleteCostIndicatorEntity(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:     "fake-board",
			BurnoutRate: 14.0,
			Location:    fleetcostpb.Location_LOCATION_ACS,
			Type:        fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
			Cost: &money.Money{
				CurrencyCode: "USD",
				Units:        200,
			},
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}); err != nil {
		panic(err)
	}

	if _, err := tf.Frontend.DeleteCostIndicator(tf.Ctx, &fleetcostAPI.DeleteCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Primary:     "fake-board",
			Location:    fleetcostpb.Location_LOCATION_ACS,
			Type:        fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}); err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	_, err := entities.GetCostIndicatorEntity(tf.Ctx, &entities.CostIndicatorEntity{})
	if !datastore.IsErrNoSuchEntity(err) {
		t.Errorf("unexpected error: %s", err)
	}
}

// TestApplyFilter tests searching for a record using the default values for location and type.
//
// Using the default values for location and type should *not* result in the exclusion of any records.
func TestApplyFilter(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &fleetcostpb.CostIndicator{
		Type:     fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
		Location: fleetcostpb.Location_LOCATION_SFO36,
		Cost: &money.Money{
			CurrencyCode: "USD",
			Units:        100,
		},
		CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
	})

	query, err := entities.ApplyFilter(datastore.NewQuery(entities.CostIndicatorKind), &fleetcostAPI.ListCostIndicatorsFilter{
		Location: "",
		Type:     "",
	})
	if err != nil {
		panic(err)
	}

	n, err := datastore.Count(tf.Ctx, query)
	if err != nil {
		t.Errorf("unexpected error when counting matches: %s", err)
	}
	if n != 1 {
		t.Errorf("unexpected count %d", n)
	}
}
