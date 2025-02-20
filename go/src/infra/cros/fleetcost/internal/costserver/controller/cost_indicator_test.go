// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/genproto/googleapis/type/money"

	"go.chromium.org/luci/common/testing/typed"

	models "go.chromium.org/infra/cros/fleetcost/api/models"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/controller"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/entities"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/testsupport"
	"go.chromium.org/infra/cros/fleetcost/internal/utils"
)

// TestGetCostIndicatorValue is a simple smoke test that checks whether we can get a cost indicator.
func TestGetCostIndicatorValue(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)
	costserver.MustCreateCostIndicator(tf.Ctx, tf.Frontend, &models.CostIndicator{
		Type:     models.IndicatorType_INDICATOR_TYPE_POWER,
		Location: models.Location_LOCATION_ALL,
		Cost: &money.Money{
			CurrencyCode: "USD",
			Units:        47,
		},
		CostCadence: models.CostCadence_COST_CADENCE_HOURLY,
	})

	ent, err := controller.GetCostIndicatorValue(tf.Ctx, &controller.IndicatorAttribute{
		IndicatorType: models.IndicatorType_INDICATOR_TYPE_POWER,
		Location:      models.Location_LOCATION_ALL,
	}, true, true)
	cost := utils.MoneyToFloat(ent.CostIndicator.GetCost())

	if cost != 47.0 {
		t.Errorf("unexpected cost %f", cost)
	}
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

// TestGetIndicatorFallbacks tests getting the indicator fallbacks.
func TestGetIndicatorFallbacks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  *controller.IndicatorAttribute
		output []*controller.IndicatorAttribute
		ok     bool
	}{
		{
			name:   "empty",
			input:  &controller.IndicatorAttribute{},
			output: nil,
			ok:     false,
		},
		{
			name: "type and location only",
			input: &controller.IndicatorAttribute{
				IndicatorType: models.IndicatorType_INDICATOR_TYPE_CLOUD,
				Location:      models.Location_LOCATION_SFO36,
			},
			output: []*controller.IndicatorAttribute{
				controller.NewIndicatorAttribute("", models.IndicatorType_INDICATOR_TYPE_CLOUD, "", "", "", models.Location_LOCATION_SFO36),
				controller.NewIndicatorAttribute("", models.IndicatorType_INDICATOR_TYPE_CLOUD, "", "", "", models.Location_LOCATION_ALL),
			},
			ok: true,
		},
		{
			name: "full example",
			input: &controller.IndicatorAttribute{
				IndicatorType: models.IndicatorType_INDICATOR_TYPE_CLOUD,
				Primary:       "board",
				Secondary:     "model",
				Tertiary:      "sku",
				Location:      models.Location_LOCATION_SFO36,
			},
			output: []*controller.IndicatorAttribute{
				controller.NewIndicatorAttribute("", models.IndicatorType_INDICATOR_TYPE_CLOUD, "board", "model", "sku", models.Location_LOCATION_SFO36),
				controller.NewIndicatorAttribute("", models.IndicatorType_INDICATOR_TYPE_CLOUD, "board", "model", "sku", models.Location_LOCATION_ALL),
				controller.NewIndicatorAttribute("", models.IndicatorType_INDICATOR_TYPE_CLOUD, "board", "model", "", models.Location_LOCATION_SFO36),
				controller.NewIndicatorAttribute("", models.IndicatorType_INDICATOR_TYPE_CLOUD, "board", "model", "", models.Location_LOCATION_ALL),
				controller.NewIndicatorAttribute("", models.IndicatorType_INDICATOR_TYPE_CLOUD, "board", "", "", models.Location_LOCATION_SFO36),
				controller.NewIndicatorAttribute("", models.IndicatorType_INDICATOR_TYPE_CLOUD, "board", "", "", models.Location_LOCATION_ALL),
				controller.NewIndicatorAttribute("", models.IndicatorType_INDICATOR_TYPE_CLOUD, "", "", "", models.Location_LOCATION_SFO36),
				controller.NewIndicatorAttribute("", models.IndicatorType_INDICATOR_TYPE_CLOUD, "", "", "", models.Location_LOCATION_ALL),
			},
			ok: true,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := controller.GetIndicatorFallbacks(tt.input)
			if diff := typed.Got(actual).Want(tt.output).Diff(); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
			switch {
			case tt.ok && err != nil:
				t.Errorf("unexpected error: %s", err)
			case !tt.ok && err == nil:
				t.Error("error is unexpectedly nil")
			}
		})
	}
}

func TestNormalizeToHourlyCost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ent  *entities.CostIndicatorEntity
		out  float64
		ok   bool
	}{
		{
			name: "simple hourly cost",
			ent: &entities.CostIndicatorEntity{
				CostIndicator: &models.CostIndicator{
					Cost:        utils.FloatToMoney(56.27),
					CostCadence: models.CostCadence_COST_CADENCE_HOURLY,
				},
			},
			out: 56.27,
			ok:  true,
		},
		{
			name: "simple annual cost",
			ent: &entities.CostIndicatorEntity{
				CostIndicator: &models.CostIndicator{
					Cost:        utils.FloatToMoney(25.57),
					CostCadence: models.CostCadence_COST_CADENCE_ANNUALLY,
				},
			},
			out: 25.57 / 24 / 365,
			ok:  true,
		},
		{
			name: "hourly cost with burnout rate",
			ent: &entities.CostIndicatorEntity{
				CostIndicator: &models.CostIndicator{
					Cost:                utils.FloatToMoney(67.12),
					BurnoutRate:         0.218,
					AmortizationInYears: 1000,
					CostCadence:         models.CostCadence_COST_CADENCE_HOURLY,
				},
			},
			out: 67.12 + (0.218*1000)*67.12,
			ok:  true,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := controller.NormalizeToHourlyCost(tt.ent, false)
			switch {
			case tt.ok && err != nil:
				t.Errorf("unexpected error: %s", err)
			case !tt.ok && err == nil:
				t.Error("err is unexpectedly nil")
			}
			if diff := typed.Got(actual).Want(tt.out).Options(cmpopts.EquateApprox(0, 0.000001)).Diff(); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}
