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

func Test_convertQueryParameters(t *testing.T) {
	t.Parallel()

	t.Run("empty parameters", func(t *testing.T) {
		t.Parallel()
		params := []any{}
		expected := []bigquery.QueryParameter{}
		actual := convertQueryParameters(params)
		assert.Loosely(t, actual, should.Resemble(expected))
	})

	t.Run("single parameter", func(t *testing.T) {
		t.Parallel()
		params := []any{"test"}
		expected := []bigquery.QueryParameter{
			{
				Value: "test",
			},
		}
		actual := convertQueryParameters(params)
		assert.Loosely(t, actual, should.Resemble(expected))
	})

	t.Run("multiple parameters", func(t *testing.T) {
		t.Parallel()
		params := []any{"'test1'", "test2"}
		expected := []bigquery.QueryParameter{
			{
				Value: "'test1'",
			},
			{
				Value: "test2",
			},
		}
		actual := convertQueryParameters(params)
		assert.Loosely(t, actual, should.Resemble(expected))
	})
}

func Test_buildListResourceRequestsQuery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("empty request", func(t *testing.T) {
		t.Parallel()
		req := &fleetconsolerpc.ListResourceRequestsRequest{}
		query, err := buildListResourceRequestsQuery(ctx, req, 0)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, query.Statement, should.Equal(
			`SELECT rr_id, resource_details, resource_request_actual_delivery_date, resource_request_target_delivery_date, fulfillment_status, material_sourcing_target_delivery_date, build_target_delivery_date, qa_target_delivery_date, config_target_delivery_date
FROM `+
				"`resource_delivery_dev.resource_requests`"+
				`

ORDER BY rr_id
LIMIT 0
OFFSET 0;`,
		))
		assert.Loosely(t, query.Parameters, should.HaveLength(0))
	})

	t.Run("request with filter", func(t *testing.T) {
		t.Parallel()
		req := &fleetconsolerpc.ListResourceRequestsRequest{
			Filter:   "rr_id=RR-001 OR rr_id = RR-002",
			PageSize: 10,
		}
		query, err := buildListResourceRequestsQuery(ctx, req, 0)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, query.Statement, should.Equal(
			`SELECT rr_id, resource_details, resource_request_actual_delivery_date, resource_request_target_delivery_date, fulfillment_status, material_sourcing_target_delivery_date, build_target_delivery_date, qa_target_delivery_date, config_target_delivery_date
FROM `+
				"`resource_delivery_dev.resource_requests`"+
				`
WHERE ((rr_id = ?) OR (rr_id = ?))
ORDER BY rr_id
LIMIT 10
OFFSET 0;`,
		))
		assert.Loosely(t, query.Parameters, should.Match([]any{"RR-001", "RR-002"}))
	})
}
