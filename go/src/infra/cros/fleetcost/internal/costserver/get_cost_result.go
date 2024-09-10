// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package costserver

import (
	"context"

	"google.golang.org/grpc/codes"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	// TODO, move shared util to a standalone directory.
	shivasUtil "infra/cmd/shivas/utils"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/costserver/controller"
	"infra/cros/fleetcost/internal/costserver/entities"
	"infra/cros/fleetcost/internal/fleetcosterror"
	ufsUtil "infra/unifiedfleet/app/util"
)

// GetCostResult gets cost result of a fleet resource(DUT, scheduling unit).
//
// TODO(gregorynisbet):
//
//		 Include "missing entries forgiveness" disposition in the cache.
//	         We don't want strict and lax cache entries interfering with each other.
func (f *FleetCostFrontend) GetCostResult(ctx context.Context, req *fleetcostAPI.GetCostResultRequest) (*fleetcostAPI.GetCostResultResponse, error) {
	logging.Infof(ctx, "Begin GetCostResult for hostname=%q", req.GetHostname())
	if req.GetForceUpdate() {
		return f.getCostResultImpl(ctx, req)
	}
	ent, readErr := controller.ReadValidCachedCostResult(ctx, req.GetHostname())
	if entHasCostResult(readErr, ent) {
		logging.Infof(ctx, "Return GetCostResult result from cache for hostname=%q", req.GetHostname())
		return &fleetcostAPI.GetCostResultResponse{
			Result: ent.CostResult,
			Report: ent.CostReport,
		}, nil
	}
	if readErr != nil && !datastore.IsErrNoSuchEntity(readErr) {
		logging.Errorf(ctx, "Unexpected error while reading from cache for hostname=%q: %s", req.GetHostname(), readErr)
		return nil, errors.Annotate(readErr, "get cost result").Err()
	}

	logging.Infof(ctx, "really compute cost for hostname=%q", req.GetHostname())
	return f.getCostResultImpl(ctx, req)
}

// Function getCostResultImpl calculates a cost result and saves it to the database.
//
// We assume that either GetForceUpdate has been applied or that there's no cache entry that's recent enough to use instead.
func (f *FleetCostFrontend) getCostResultImpl(ctx context.Context, req *fleetcostAPI.GetCostResultRequest) (*fleetcostAPI.GetCostResultResponse, error) {
	// Handling OS namespace request only at MVP.
	logging.Infof(ctx, "begin cost result request for dut %q Id %q", req.GetHostname(), req.GetDeviceId())
	ctx = shivasUtil.SetupContext(ctx, ufsUtil.OSNamespace)
	if f.fleetClient == nil {
		return nil, fleetcosterror.WithDefaultCode(codes.Internal, errors.New("fleet client must exist"))
	}
	res, rep, err := controller.CalculateCostForOsResource(ctx, f.fleetClient, req)
	if err != nil {
		return nil, fleetcosterror.WithDefaultCode(codes.Aborted, errors.Annotate(err, "get cost result").Err())
	}
	if err := controller.StoreCachedCostResult(ctx, req.GetHostname(), res, rep); err != nil {
		logging.Errorf(ctx, "%s\n", errors.Annotate(err, "caching get cost result").Err())
	}
	if rep == nil {
		logging.Infof(ctx, "cost result request for dut %q produced an empty report\n")
	}
	return &fleetcostAPI.GetCostResultResponse{
		Result: res,
		Report: rep,
	}, nil
}

// entHasCostResult returns true if and only if we read a valid entity out of datastore.
func entHasCostResult(readErr error, ent *entities.CachedCostResultEntity) bool {
	if readErr != nil {
		return false
	}
	if ent == nil {
		return false
	}
	if ent.CostResult == nil {
		return false
	}
	return true
}
