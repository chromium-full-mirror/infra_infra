// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	fleetcostpb "go.chromium.org/infra/cros/fleetcost/api/models"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/entities"
	"go.chromium.org/infra/cros/fleetcost/internal/utils"
)

func normalizeToHourlyCost(ent *entities.CostIndicatorEntity, forgiveMissingEntries bool) (float64, error) {
	if ent == nil {
		if forgiveMissingEntries {
			return 0, nil
		}
		return 0, errors.New("entity cannot be nil")
	}
	const dayToHour = 1.0 / 24.0
	const monthToHour = 1.0 / float64(30*24)
	const hourToYear = float64(365 * 24)
	const yearToHour = 1.0 / hourToYear
	cadence := ent.CostIndicator.GetCostCadence()
	annualBurnoutRate := ent.CostIndicator.GetBurnoutRate()
	amortizationTimeInYears := ent.CostIndicator.GetAmortizationInYears()
	rawCost := utils.MoneyToFloat(ent.CostIndicator.GetCost())
	switch cadence {
	case fleetcostpb.CostCadence_COST_CADENCE_UNKNOWN:
		return 0, errors.New("unkown cost cadence")
	case fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME:
		costPerHour, err := utils.SafeDivide(rawCost, amortizationTimeInYears*hourToYear)
		if err != nil {
			return 0, fmt.Errorf("bad cost per hour for %w", err)
		}
		return BurnoutRateLinearPenalty(costPerHour, annualBurnoutRate, amortizationTimeInYears)
	case fleetcostpb.CostCadence_COST_CADENCE_ANNUALLY:
		costPerHour := rawCost * yearToHour
		return BurnoutRateLinearPenalty(costPerHour, annualBurnoutRate, amortizationTimeInYears)
	case fleetcostpb.CostCadence_COST_CADENCE_MONTHLY:
		costPerHour := rawCost * monthToHour
		return BurnoutRateLinearPenalty(costPerHour, annualBurnoutRate, amortizationTimeInYears)
	case fleetcostpb.CostCadence_COST_CADENCE_DAILY:
		costPerHour := rawCost * dayToHour
		return BurnoutRateLinearPenalty(costPerHour, annualBurnoutRate, amortizationTimeInYears)
	case fleetcostpb.CostCadence_COST_CADENCE_HOURLY:
		return BurnoutRateLinearPenalty(rawCost, annualBurnoutRate, amortizationTimeInYears)
	}
	return 0, fmt.Errorf("tag not handled yet: %s", cadence.String())
}

// BurnoutRateLinearPenalty divides the annual burnout rate by the number of hours in a year and uses that to compute additional cost.
//
// This is the simplest burnout model that can possibly work. Other alternatives include an exponential model.
//
// TODO(gregorynisbet): investigate exponential models for burnout rate.
// TODO(gregorynisbet): extend the RPC interface to make the burnout penalty its own line item.
//
// Basically, we assume that if the burnout rate is 0.2, then that 0.2 will be spread out evenly per every hour of the year.
// This is not a realistic assumption, but it is an understandable one.
//
// Next we assume that *if* the device burns out, then we are metaphorically hit with the cost of acquiring it again.
// We get the cost of acquiring the device again by multiplying the hourly cost by the amortization period. This isn't ideal.
//
// TODO(gregorynisbet): Look into giving EVERY cost indicator a separate reacquisition cost (for dealing with burnout)
func BurnoutRateLinearPenalty(costPerHour float64, annualBurnoutRate float64, amortizationTimeYears float64) (float64, error) {
	if annualBurnoutRate == 0 {
		return costPerHour, nil
	}
	if amortizationTimeYears <= 0 {
		return 0, fmt.Errorf("amortization time %f must be positive when burnout rate is provided", amortizationTimeYears)
	}
	const hourToYear = float64(365 * 24)
	switch {
	case annualBurnoutRate < 0:
		return 0, fmt.Errorf("burnout rate %f must be non-negative", annualBurnoutRate)
	case annualBurnoutRate > 1:
		return 0, fmt.Errorf("burnout rate %f cannot exceed one", annualBurnoutRate)
	}
	hourlyBurnoutRate := annualBurnoutRate / hourToYear
	reacquisitionCost := costPerHour * hourToYear * amortizationTimeYears
	return costPerHour + hourlyBurnoutRate*reacquisitionCost, nil
}

func getAmortizedCostIndicatorValue(ctx context.Context, attribute *indicatorAttribute, usefallbacks bool, forgiveMissingEntries bool) (float64, error) {
	ent, err := getCostIndicatorValue(ctx, attribute, usefallbacks, forgiveMissingEntries)
	if err != nil {
		return 0, err
	}
	v, err := normalizeToHourlyCost(ent, forgiveMissingEntries)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// GetCostIndicatorValue gets the value of a cost indicator, potentially falling back.
//
// GetCostIndicatorValue normalizes all values to hourly.
func getCostIndicatorValue(ctx context.Context, attribute *indicatorAttribute, usefallbacks bool, forgiveMissingEntries bool) (*entities.CostIndicatorEntity, error) {
	if !usefallbacks {
		ent, err := getCostIndicatorValueDirectly(ctx, attribute)
		if err != nil {
			return nil, errors.Annotate(err, "error looking up %q", attribute.ErrorHint).Err()
		}
		return ent, nil
	}
	sequence, err := getIndicatorFallbacks(attribute)
	if err != nil {
		return nil, errors.Annotate(err, "error looking up %q", attribute.ErrorHint).Err()
	}
	for _, attribute := range sequence {
		ent, err := getCostIndicatorValueDirectly(ctx, attribute)
		switch {
		case err == nil:
			return ent, nil
		case datastore.IsErrNoSuchEntity(err):
			continue
		default:
			return nil, errors.Annotate(err, "error looking up %q", attribute.ErrorHint).Err()
		}
	}

	if forgiveMissingEntries {
		logging.Debugf(ctx, "forgiving missing attribute: %q", attribute.String())
		return nil, nil
	}

	return nil, errors.Fmt("error looking up %q: %w", attribute.ErrorHint, datastore.ErrNoSuchEntity)
}

// getCostIndicatorValueDirectly gets the value of a cost indicator.
func getCostIndicatorValueDirectly(ctx context.Context, attribute *indicatorAttribute) (*entities.CostIndicatorEntity, error) {
	entity := attribute.asEntity()
	if _, err := entities.GetCostIndicatorEntity(ctx, entity); err != nil {
		return nil, errors.Annotate(err, "get cost indicator value").Err()
	}
	return entity, nil
}

// getIndicatorFallbacks takes an indicatorAttribute and returns the list of fallback indicator attributes.
func getIndicatorFallbacks(attribute *indicatorAttribute) ([]*indicatorAttribute, error) {
	errorHint := attribute.ErrorHint
	typ := attribute.IndicatorType
	primary := attribute.Primary
	secondary := attribute.Secondary
	tertiary := attribute.Tertiary
	location := attribute.Location

	if location == fleetcostpb.Location_LOCATION_UNKNOWN {
		return nil, errors.New("location cannot be unknown")
	}
	if typ == fleetcostpb.IndicatorType_INDICATOR_TYPE_UNKNOWN {
		return nil, errors.New("type cannot be unknown")
	}

	hasLocationAll := location == fleetcostpb.Location_LOCATION_ALL

	var output []*indicatorAttribute

	// TODO(gregorynisbet): rework this logic so that it isn't hardcoded.
	if tertiary != "" {
		output = append(output, newIndicatorAttribute(errorHint, typ, primary, secondary, tertiary, location))
	}
	if tertiary != "" && !hasLocationAll {
		output = append(output, newIndicatorAttribute(errorHint, typ, primary, secondary, tertiary, fleetcostpb.Location_LOCATION_ALL))
	}
	if secondary != "" {
		output = append(output, newIndicatorAttribute(errorHint, typ, primary, secondary, "", location))
	}
	if secondary != "" && !hasLocationAll {
		output = append(output, newIndicatorAttribute(errorHint, typ, primary, secondary, "", fleetcostpb.Location_LOCATION_ALL))
	}
	if primary != "" {
		output = append(output, newIndicatorAttribute(errorHint, typ, primary, "", "", location))
	}
	if primary != "" && !hasLocationAll {
		output = append(output, newIndicatorAttribute(errorHint, typ, primary, "", "", fleetcostpb.Location_LOCATION_ALL))
	}
	if typ != fleetcostpb.IndicatorType_INDICATOR_TYPE_UNKNOWN {
		output = append(output, newIndicatorAttribute(errorHint, typ, "", "", "", location))
	}
	if typ != fleetcostpb.IndicatorType_INDICATOR_TYPE_UNKNOWN && !hasLocationAll {
		output = append(output, newIndicatorAttribute(errorHint, typ, "", "", "", fleetcostpb.Location_LOCATION_ALL))
	}

	return output, nil
}
