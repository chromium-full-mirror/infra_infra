// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"time"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/bigqueryclient"
)

// PingBigQuery pings the mock BQ database with a random query and logs the results.
func (frontend *FleetConsoleFrontend) PingBigQuery(ctx context.Context, req *fleetconsolerpc.PingBigQueryRequest) (_ *fleetconsolerpc.PingBigQueryResponse, err error) {
	logging.Infof(ctx, "beginning of ping big query call")
	client, err := bigqueryclient.NewBQClient(ctx, "fleet-console-dev")
	if err != nil {
		return nil, err
	}
	q := client.Query("select * from fleet_console_bq.resource_requests where procurement_end_date < '2025-05-01' order by rr_id limit 1")
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
	defer cancel()
	it, err := q.Read(ctx)
	if err != nil {
		logging.Errorf(ctx, "ping big query call failed: %s", err)
		return nil, err
	}
	for {
		var val map[string]bigquery.Value
		err := it.Next(&val)
		if err == nil {
			logging.Infof(ctx, "Read request: %s", val["rr_id"])
		} else {
			break
		}
	}
	logging.Infof(ctx, "successful end of ping big query call")
	return &fleetconsolerpc.PingBigQueryResponse{}, nil
}
