// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/bigqueryclient"
	"go.chromium.org/infra/fleetconsole/internal/utils"
)

// ListResourceRequests lists resource requests.
func (frontend *FleetConsoleFrontend) ListResourceRequests(ctx context.Context, req *fleetconsolerpc.ListResourceRequestsRequest) (*fleetconsolerpc.ListResourceRequestsResponse, error) {
	logging.Infof(ctx, "ListResourceRequests called")

	bqClient, err := bigqueryclient.NewBQClient(ctx, "fleet-console-dev")

	if err != nil {
		logging.Infof(ctx, "Error instantiating a new BigQuery client")
		return nil, err
	}

	q := bqClient.Query("select * from fleet_console_bq.resource_requests")

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
	defer cancel()
	it, err := q.Read(ctx)
	if err != nil {
		logging.Errorf(ctx, "fetching resource requests from big query failed: %s", err)
		return nil, err
	}

	resourceRequests := []*fleetconsolerpc.ResourceRequest{}

	for {
		var row map[string]bigquery.Value
		err := it.Next(&row)
		if err == nil {
			rrID := row["rr_id"].(string)

			resourceRequests = append(resourceRequests, &fleetconsolerpc.ResourceRequest{
				RrId:               rrID,
				Name:               "resourceRequests/" + rrID,
				ResourceDetails:    row["resource_details"].(string),
				ProcurementEndDate: utils.FromCivilDate(row["procurement_end_date"].(civil.Date)),
				BuildEndDate:       utils.FromCivilDate(row["build_end_date"].(civil.Date)),
				QaEndDate:          utils.FromCivilDate(row["qa_end_date"].(civil.Date)),
				ConfigEndDate:      utils.FromCivilDate(row["config_end_date"].(civil.Date)),
			})
		} else {
			break
		}
	}

	return &fleetconsolerpc.ListResourceRequestsResponse{
		ResourceRequests: resourceRequests,
	}, nil
}
