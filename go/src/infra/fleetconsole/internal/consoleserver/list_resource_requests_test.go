// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"testing"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
)

func Test_buildListResourceRequestsQuery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("request with filter", func(t *testing.T) {
		t.Parallel()
		req := &fleetconsolerpc.ListResourceRequestsRequest{
			Filter:   "rr_id=RR-001 OR rr_id = RR-002",
			PageSize: 10,
		}
		query, err := buildListResourceRequestsQuery(ctx, &bigquery.Client{}, req, 0, false)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, query.Q, should.Equal(
			`SELECT rr_id, resource_details, resource_request_actual_delivery_date, resource_request_target_delivery_date, fulfillment_status, material_sourcing_target_start_date, material_sourcing_actual_start_date, material_sourcing_target_delivery_date, material_sourcing_actual_delivery_date, build_target_start_date, build_actual_start_date, build_target_delivery_date, build_actual_delivery_date, qa_target_start_date, qa_actual_start_date, qa_target_delivery_date, qa_actual_delivery_date, config_target_start_date, config_actual_start_date, config_target_delivery_date, config_actual_delivery_date
FROM `+
				"`resource_delivery_dev.resource_requests`"+
				`
WHERE ((rr_id = ?) OR (rr_id = ?))
ORDER BY rr_id
LIMIT 11
OFFSET 0;`,
		))
		assert.Loosely(t, query.Parameters[0].Value, should.Equal("RR-001"))
		assert.Loosely(t, query.Parameters[1].Value, should.Equal("RR-002"))
	})
}
