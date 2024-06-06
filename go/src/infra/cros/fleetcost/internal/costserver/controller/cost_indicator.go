// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"math"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	fleetcostpb "infra/cros/fleetcost/api/models"
	"infra/cros/fleetcost/internal/costserver/entities"
	"infra/cros/fleetcost/internal/utils"
)

func normalizeToHourlyCost(rawCost float64, cadence fleetcostpb.CostCadence) (float64, error) {
	const dayToHour = 1.0 / 24.0
	const monthToHour = 1.0 / float64(30*24)
	const yearToHour = 1.0 / float64(365*24)
	switch cadence {
	case fleetcostpb.CostCadence_COST_CADENCE_UNKNOWN:
		return math.NaN(), errors.New("unkown cost cadence")
	case fleetcostpb.CostCadence_COST_CADENCE_ONE_TIME:
		return math.NaN(), errors.New("conversion from one-time cost to time-bound cost not yet supported")
	case fleetcostpb.CostCadence_COST_CADENCE_ANNUALLY:
		return rawCost * yearToHour, nil
	case fleetcostpb.CostCadence_COST_CADENCE_MONTHLY:
		return rawCost * monthToHour, nil
	case fleetcostpb.CostCadence_COST_CADENCE_DAILY:
		return rawCost * dayToHour, nil
	case fleetcostpb.CostCadence_COST_CADENCE_HOURLY:
		return rawCost, nil
	}
	return math.NaN(), fmt.Errorf("tag not handled yet: %s", cadence.String())
}

// GetCostIndicatorValue gets the value of a cost indicator, potentially falling back.
//
// GetCostIndicatorValue normalizes all values to hourly.
func GetCostIndicatorValue(ctx context.Context, attribute *indicatorAttribute, usefallbacks bool, forgiveMissingEntries bool) (float64, error) {
	if !usefallbacks {
		v, c, err := GetCostIndicatorValueDirectly(ctx, attribute)
		if err != nil {
			return 0, err
		}
		return normalizeToHourlyCost(v, c)
	}
	sequence, err := GetIndicatorFallbacks(attribute)
	if err != nil {
		return math.NaN(), err
	}
	for _, attribute := range sequence {
		result, cadence, err := GetCostIndicatorValueDirectly(ctx, attribute)
		switch {
		case err == nil:
			return normalizeToHourlyCost(result, cadence)
		case datastore.IsErrNoSuchEntity(err):
			continue
		default:
			return math.NaN(), err
		}

	}

	if forgiveMissingEntries {
		logging.Debugf(ctx, "forgiving missing attribute: %q", attribute.FriendlyString())
		return 0.0, nil
	}

	return math.NaN(), datastore.ErrNoSuchEntity
}

// GetCostIndicatorValueDirectly gets the value of a cost indicator.
func GetCostIndicatorValueDirectly(ctx context.Context, attribute *indicatorAttribute) (float64, fleetcostpb.CostCadence, error) {
	entity := attribute.AsEntity()
	if _, err := entities.GetCostIndicatorEntity(ctx, entity); err != nil {
		return 0, fleetcostpb.CostCadence_COST_CADENCE_UNKNOWN, errors.Annotate(err, "get cost indicator value").Err()
	}
	return utils.MoneyToFloat(entity.CostIndicator.GetCost()), entity.CostIndicator.GetCostCadence(), nil
}

// GetIndicatorFallbacks takes an indicatorAttribute and returns the list of fallback indicator attributes.
func GetIndicatorFallbacks(attribute *indicatorAttribute) ([]*indicatorAttribute, error) {
	typ := attribute.IndicatorType
	board := attribute.Board
	model := attribute.Model
	sku := attribute.Sku
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
	if sku != "" {
		output = append(output, NewIndicatorAttribute(typ, board, model, sku, location))
	}
	if sku != "" && !hasLocationAll {
		output = append(output, NewIndicatorAttribute(typ, board, model, sku, fleetcostpb.Location_LOCATION_ALL))
	}
	if model != "" {
		output = append(output, NewIndicatorAttribute(typ, board, model, "", location))
	}
	if model != "" && !hasLocationAll {
		output = append(output, NewIndicatorAttribute(typ, board, model, "", fleetcostpb.Location_LOCATION_ALL))
	}
	if board != "" {
		output = append(output, NewIndicatorAttribute(typ, board, "", "", location))
	}
	if board != "" && !hasLocationAll {
		output = append(output, NewIndicatorAttribute(typ, board, "", "", fleetcostpb.Location_LOCATION_ALL))
	}
	if typ != fleetcostpb.IndicatorType_INDICATOR_TYPE_UNKNOWN {
		output = append(output, NewIndicatorAttribute(typ, "", "", "", location))
	}
	if typ != fleetcostpb.IndicatorType_INDICATOR_TYPE_UNKNOWN && !hasLocationAll {
		output = append(output, NewIndicatorAttribute(typ, "", "", "", fleetcostpb.Location_LOCATION_ALL))
	}

	return output, nil
}
