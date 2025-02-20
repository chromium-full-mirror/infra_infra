// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package costserver

import (
	"context"

	"google.golang.org/grpc"

	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/cron"

	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
	"go.chromium.org/infra/libs/bqwrapper"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// NewFleetCostFrontend returns a new fleet cost frontend.
func NewFleetCostFrontend() fleetcostAPI.FleetCostServer {
	return &FleetCostFrontend{}
}

// FleetCostFrontend is the fleet cost frontend.
type FleetCostFrontend struct {
	// Clients.
	fleetClient ufspb.FleetClient
	// Debugging information exposed through admin RPCs.
	ufsHostname string
	// bqClient is a BigQuery client.
	bqClient bqwrapper.BQIf
	// projectID is our own projectID
	projectID string
}

// InstallServices installs services (such as the prpc server) into the frontend.
func InstallServices(costFrontend fleetcostAPI.FleetCostServer, srv grpc.ServiceRegistrar) {
	fleetcostAPI.RegisterFleetCostServer(srv, costFrontend)
}

// SetUFSClient sets the UFS client on a frontend.
func SetUFSClient(costFrontend *FleetCostFrontend, client ufspb.FleetClient) {
	if costFrontend == nil {
		panic("SetUFSClient: cost frontend cannot be nil")
	}
	if client == nil {
		panic("SetUFSClient: ufs client cannot be nil")
	}
	costFrontend.fleetClient = client
}

// SetUFSHostname sets the UFS hostname on the frontend.
//
// This is used to populate debugging info in the PingUFS RPC.
func SetUFSHostname(costFrontend *FleetCostFrontend, ufsHostname string) {
	costFrontend.ufsHostname = ufsHostname
}

// SetBQClient sets the bigquery client.
func SetBQClient(costFrontend *FleetCostFrontend, client bqwrapper.BQIf) {
	costFrontend.bqClient = client
}

// SetProjectID records the projectID, needed for writing to BigQuery.
func SetProjectID(costFrontend *FleetCostFrontend, projectID string) {
	costFrontend.projectID = projectID
}

// InstallCron installs cron jobs into the frontend.
func InstallCron(costFrontend *FleetCostFrontend) {
	cron.RegisterHandler(
		"persist-to-bq",
		func(ctx context.Context) error {
			_, err := costFrontend.PersistToBigquery(
				ctx,
				&fleetcostAPI.PersistToBigqueryRequest{},
			)
			return grpcutil.WrapIfTransient(err)
		},
	)
	cron.RegisterHandler(
		"repopulate-cache",
		func(ctx context.Context) error {
			_, err := costFrontend.RepopulateCache(
				ctx,
				&fleetcostAPI.RepopulateCacheRequest{},
			)
			return grpcutil.WrapIfTransient(err)
		},
	)
}
