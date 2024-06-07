// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package entities_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/genproto/googleapis/type/money"

	"go.chromium.org/luci/common/testing/typed"
	"go.chromium.org/luci/gae/service/datastore"

	fleetcostpb "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/costserver"
	"infra/cros/fleetcost/internal/costserver/entities"
	"infra/cros/fleetcost/internal/costserver/testsupport"
	"infra/cros/fleetcost/internal/utils"
)

// TestCostIndicatorSimple tests putting a cost indicator into database and retrieving it.
func TestCostIndicatorSimple(t *testing.T) {
	t.Parallel()
	tf := testsupport.NewFixture(context.Background(), t)

	if err := datastore.Put(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board:       "e",
			BurnoutRate: 12.0,
		},
	}); err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if err := datastore.Get(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board:       "e",
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
			Board: "e",
			Model: "w",
		},
	}

	err := datastore.Put(tf.Ctx, oldIndicator)
	if err != nil {
		t.Fatal(err)
	}

	item := &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board: "e",
			Model: "w",
		},
	}
	if err := datastore.Get(tf.Ctx, item); err != nil {
		t.Error(err)
	}

	if diff := typed.Got(item).Want(&entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board: "e",
			Model: "w",
		},
		Board: "e",
		Model: "w",
	}).Options(cmp.AllowUnexported(entities.CostIndicatorEntity{})).Diff(); diff != "" {
		t.Errorf("unexpected error (-want +got): %s", diff)
	}
}

// TestCostIndicatorClone tests cloning a cost indicator
func TestCostIndicatorClone(t *testing.T) {
	t.Parallel()

	oldIndicator := &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Name:  "a",
			Board: "e",
		},
	}

	newIndicator := oldIndicator.Clone()

	if diff := typed.Got(newIndicator).Want(oldIndicator).Options(cmp.AllowUnexported(*oldIndicator)).Diff(); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

func TestPutCostIndicator(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	err := utils.InsertOneWithoutReplacement(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board:       "e",
			BurnoutRate: 12.0,
		},
	}, nil)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	result, err := entities.GetCostIndicatorEntity(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board: "e",
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
			Board: "e",
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	want := &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board:       "e",
			BurnoutRate: 44.0,
		},
		Board: "e",
	}

	if diff := typed.Got(costIndicator).Want(want).Options(cmp.AllowUnexported(entities.CostIndicatorEntity{})).Diff(); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
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
			Board:       "e",
			BurnoutRate: 44.0,
		},
	}

	if diff := typed.Got(costIndicators).Want(want).Options(cmp.AllowUnexported(entities.CostIndicatorEntity{})).Diff(); diff != "" {
		t.Errorf("unexpected diff: %s", diff)
	}
}

// TestListCostIndicatorWithModelFilter tests listing devices with a model filter.
func TestListCostIndicatorWithModelFilter(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)
	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board:    "fake-board-1",
			Model:    "fake-model",
			Location: fleetcostpb.Location_LOCATION_ACS,
			Type:     fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
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
			Board:    "fake-board-2",
			Model:    "fake-model",
			Location: fleetcostpb.Location_LOCATION_ACS,
			Type:     fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
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
			Board:    "fake-board-2",
			Model:    "a-different-model",
			Location: fleetcostpb.Location_LOCATION_ACS,
			Type:     fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
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
			Model: "fake-model",
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if diff := typed.Got(len(resp.GetCostIndicator())).Want(2).Diff(); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestListCostIndicatorWithSkuFilter tests listing devices with a SKU filter.
func TestListCostIndicatorWithSkuFilter(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)
	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board:    "fake-board-1",
			Model:    "fake-model-1",
			Sku:      "fake-sku",
			Location: fleetcostpb.Location_LOCATION_ACS,
			Type:     fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
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
			Board:    "fake-board-2",
			Model:    "fake-model-2",
			Sku:      "fake-sku",
			Location: fleetcostpb.Location_LOCATION_ACS,
			Type:     fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
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
			Board:    "fake-board-3",
			Model:    "fake-model-3",
			Sku:      "different-sku",
			Location: fleetcostpb.Location_LOCATION_ACS,
			Type:     fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
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
			Sku: "fake-sku",
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if diff := typed.Got(len(resp.GetCostIndicator())).Want(2).Diff(); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestUpdateCostIndicatorHappyPath tests updating a cost indicator that already exists.
//
// Note that when updating the record, we provide an argument that
func TestUpdateCostIndicatorHappyPath(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	if err := utils.InsertOneWithoutReplacement(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board:       "fake-board",
			BurnoutRate: 12.0,
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
	}, nil); err != nil {
		t.Fatalf("failed to insert cost indicator: %s", err)
	}

	got, err := entities.UpdateCostIndicatorEntity(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board:       "fake-board",
			BurnoutRate: 14.0,
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
		Board: "fake-board",
	}, []string{"burnout_rate"})
	if err != nil {
		t.Errorf("unexpected error: %q", err)
	}

	if diff := typed.Got(got).Want(&entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board:       "fake-board",
			BurnoutRate: 14.0,
			CostCadence: fleetcostpb.CostCadence_COST_CADENCE_HOURLY,
		},
		Board: "fake-board",
	}).Options(cmp.AllowUnexported(*got)).Diff(); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

func TestDeleteCostIndicatorEntity(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	if _, err := tf.Frontend.CreateCostIndicator(tf.Ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board:       "fake-board",
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
			Board:       "fake-board",
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
