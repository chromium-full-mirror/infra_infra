// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package costserver

import (
	"context"

	"google.golang.org/grpc/codes"

	"go.chromium.org/luci/common/errors"

	fleetcostModels "go.chromium.org/infra/cros/fleetcost/api/models"
	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
	"go.chromium.org/infra/cros/fleetcost/internal/costserver/entities"
	"go.chromium.org/infra/cros/fleetcost/internal/fleetcosterror"
	"go.chromium.org/infra/cros/fleetcost/internal/site"
	"go.chromium.org/infra/cros/fleetcost/internal/utils"
	"go.chromium.org/infra/cros/fleetcost/internal/validation"
)

// MustCreateCostIndicator is a helper function for tests that ergonomically creates a
// CostIndicator so that the database can be brought to a known state.
func MustCreateCostIndicator(ctx context.Context, f *FleetCostFrontend, costIndicator *fleetcostModels.CostIndicator) {
	_, err := f.CreateCostIndicator(ctx, &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: costIndicator,
	})
	if err != nil {
		panic(err)
	}
}

// CreateCostIndicator creates a cost indicator.
func (f *FleetCostFrontend) CreateCostIndicator(ctx context.Context, request *fleetcostAPI.CreateCostIndicatorRequest) (*fleetcostAPI.CreateCostIndicatorResponse, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, site.ShortRPCDuration, site.ErrShortRPCEnded)
	defer cancel()
	if err := validation.ValidateCreateCostIndicatorRequest(request); err != nil {
		return nil, fleetcosterror.WithDefaultCode(codes.FailedPrecondition, err)
	}
	costIndicator := request.GetCostIndicator()
	entity := entities.NewCostIndicatorEntity(costIndicator)
	if err := utils.InsertOneWithoutReplacement(ctx, entity, nil); err != nil {
		return nil, fleetcosterror.WithDefaultCode(codes.Aborted, errors.Annotate(err, "create cost indicator").Err())
	}
	return &fleetcostAPI.CreateCostIndicatorResponse{
		CostIndicator: costIndicator,
	}, nil
}

// ListCostIndicators lists the cost indicators in the database satisfying the request.
func (f *FleetCostFrontend) ListCostIndicators(ctx context.Context, request *fleetcostAPI.ListCostIndicatorsRequest) (*fleetcostAPI.ListCostIndicatorsResponse, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, site.MediumRPCDuration, site.ErrMediumRPCEnded)
	defer cancel()
	out, err := entities.ListCostIndicators(ctx, 0, request.GetFilter())
	if err != nil {
		return nil, fleetcosterror.WithDefaultCode(codes.Aborted, errors.Annotate(err, "list cost indicators").Err())
	}
	return &fleetcostAPI.ListCostIndicatorsResponse{
		CostIndicator: out,
	}, nil
}

// UpdateCostIndicator updates a CostIndicator.
func (f *FleetCostFrontend) UpdateCostIndicator(ctx context.Context, request *fleetcostAPI.UpdateCostIndicatorRequest) (*fleetcostAPI.UpdateCostIndicatorResponse, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, site.ShortRPCDuration, site.ErrShortRPCEnded)
	defer cancel()
	entity := entities.NewCostIndicatorEntity(request.GetCostIndicator())
	out, err := entities.UpdateCostIndicatorEntity(ctx, entity, request.GetUpdateMask().GetPaths())
	if err != nil {
		return nil, fleetcosterror.WithDefaultCode(codes.Aborted, errors.Annotate(err, "update cost indicator").Err())
	}
	return &fleetcostAPI.UpdateCostIndicatorResponse{
		CostIndicator: out.CostIndicator,
	}, nil
}

// DeleteCostIndicator deletes a CostIndicator.
func (f *FleetCostFrontend) DeleteCostIndicator(ctx context.Context, request *fleetcostAPI.DeleteCostIndicatorRequest) (*fleetcostAPI.DeleteCostIndicatorResponse, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, site.ShortRPCDuration, site.ErrShortRPCEnded)
	defer cancel()
	entity := entities.NewCostIndicatorEntity(request.GetCostIndicator())
	if err := utils.DeleteOneIfExists(ctx, entity, nil); err != nil {
		return nil, fleetcosterror.WithDefaultCode(codes.Aborted, errors.Annotate(err, "delete cost indicator").Err())
	}
	return &fleetcostAPI.DeleteCostIndicatorResponse{
		CostIndicator: entity.CostIndicator,
	}, nil
}
