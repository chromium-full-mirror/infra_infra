// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"

	fleetcostpb "go.chromium.org/infra/cros/fleetcost/api/models"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/entities"
)

// indicatorAttribute is the information that's necessary to look up a datastore record.
type indicatorAttribute struct {
	// ErrorHint is a description of what you were looking for.
	// It gets inserted into the error message.
	ErrorHint string
	// IndicatorType is the type of the indicator.
	IndicatorType fleetcostpb.IndicatorType
	// Primary is the primary field (e.g. board).
	Primary string
	// Secondary is the secondary field (e.g. model).
	Secondary string
	// Tertiary is the tertiary field (e.g. sku).
	Tertiary string
	// Location is the lab location of the asset with the price.
	Location fleetcostpb.Location
}

// newIndicatorAttribute creates a new indicator attribute.
//
// TODO(gregorynisbet): Rethink the API for this function, maybe move it to utils.
func newIndicatorAttribute(errorHint string, typ fleetcostpb.IndicatorType, primary string, secondary string, tertiary string, location fleetcostpb.Location) *indicatorAttribute {
	return &indicatorAttribute{
		ErrorHint:     errorHint,
		IndicatorType: typ,
		Primary:       primary,
		Secondary:     secondary,
		Tertiary:      tertiary,
		Location:      location,
	}
}

// String produces a human-readable string for error messages.
//
// This string is NOT RELATED to how IndicatorAttributes or CostIndicatorEntities are actually stored
// in the database.
func (attribute *indicatorAttribute) String() string {
	if attribute == nil {
		return "<nil>"
	}
	message := fmt.Sprintf("type=%s primary=%s secondary=%s tertiary=%s loc=%s", attribute.IndicatorType.String(), attribute.Primary, attribute.Secondary, attribute.Tertiary, attribute.Location.String())
	return message
}

// asEntity converts an IndicatorAttribute to a datastore Entity.
func (attribute *indicatorAttribute) asEntity() *entities.CostIndicatorEntity {
	if attribute == nil {
		return nil
	}
	return &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Type:      attribute.IndicatorType,
			Primary:   attribute.Primary,
			Secondary: attribute.Secondary,
			Tertiary:  attribute.Tertiary,
			Location:  attribute.Location,
		},
	}
}
