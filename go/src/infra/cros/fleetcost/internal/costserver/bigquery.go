// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package costserver

import (
	"context"
	"sync/atomic"

	"google.golang.org/protobuf/protoadapt"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	shivasUtil "infra/cmd/shivas/utils"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/costserver/controller"
	"infra/cros/fleetcost/internal/costserver/inventory/ufs"
	"infra/cros/fleetcost/internal/utils"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufsUtil "infra/unifiedfleet/app/util"
)

// PersistToBigquery persists the current cost indicators to BigQuery.
//
// Or rather, it would, if it were implemented, which it is not.
func (f *FleetCostFrontend) PersistToBigquery(ctx context.Context, request *fleetcostAPI.PersistToBigqueryRequest) (*fleetcostAPI.PersistToBigqueryResponse, error) {
	logging.Infof(ctx, "begin PersistToBigquery")
	err := controller.PersistToBigquery(ctx, f.projectID, f.bqClient, request.GetReadonly())
	if err != nil {
		return nil, err
	}
	logging.Infof(ctx, "PersistToBigquery was successful")
	return &fleetcostAPI.PersistToBigqueryResponse{}, nil
}

// RepopulateCache repopulates the datastore cache ahead of persisting to bigquery.
func (f *FleetCostFrontend) RepopulateCache(ctx context.Context, request *fleetcostAPI.RepopulateCacheRequest) (*fleetcostAPI.RepopulateCacheResponse, error) {
	ctx = shivasUtil.SetupContext(ctx, ufsUtil.OSNamespace)

	// The machine channel is used to send machine responses from UFS to
	// worker goroutines that produce updated costs.
	machineChannel := make(chan protoadapt.MessageV1)

	var rErr error
	var tally atomic.Int32

	// UFS reader goroutine.
	go func() {
		// This worker does NOT have the ability to stop doing work once rErr becomes
		// non-nil.
		//
		// Its failure causes our overall RPC to fail, but its success alone is not
		// enough for our RPC to be successful.
		_, err := ufs.GetAllMachineLSEs(ctx, f.fleetClient, false, machineChannel)
		defer close(machineChannel)
		if err != nil {
			rErr = errors.Annotate(err, "reading machine LSEs from UFS").Err()
		}
	}()

	consumer := func(ctx context.Context, item protoadapt.MessageV1) error {
		return processCostResult(ctx, item, f, request.ForgiveMissingEntries, &tally)
	}
	// utils.ConsumeChannel will block until machineChannel is closed. MachineChannel
	err := utils.ConsumeChannel(ctx, machineChannel, consumer)
	if err != nil {
		// Here we swallow up rErr if it is non-nil.
		// I know, I know.
		return nil, errors.Annotate(err, "repopulate cache").Err()
	}

	logging.Debugf(ctx, "processed %d records total", tally.Load())

	if rErr != nil {
		return nil, errors.Annotate(rErr, "repopulate cache").Err()
	}

	return &fleetcostAPI.RepopulateCacheResponse{
		ProcessedRecords: tally.Load(),
	}, nil
}

// processCostResult computes the cost of a single device and puts it in datastore.
func processCostResult(ctx context.Context, machineLSE protoadapt.MessageV1, f *FleetCostFrontend, forgiveMissingEntries bool, tally *atomic.Int32) error {
	m := machineLSE.(*ufspb.MachineLSE)
	hostname := m.GetHostname()
	if hostname == "" {
		logging.Debugf(ctx, "machine %q has empty hostname, skipping. Full record %v", m.GetName(), m)
		return nil
	}
	if _, err := f.GetCostResult(ctx, &fleetcostAPI.GetCostResultRequest{
		Hostname:              hostname,
		ForceUpdate:           true,
		ForgiveMissingEntries: forgiveMissingEntries,
	}); err != nil {
		// TODO(b:321105253): Certain errors, such as
		// a device being a labstation or something else that doesn't have
		// a cost associated with it are expected.
		//
		// Other errors are not. Right now I don't have a good way to distinguish them.
		logging.Errorf(ctx, "error when calculating cost: %s", err)
		return nil
	}
	tally.Add(1)
	return nil
}
