// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"regexp"

	"infra/fleetconsole/internal/database/queryutils"
	"infra/fleetconsole/internal/utils"
)

func buildListDevicesQuery(offset, pageSize int, filter, orderby string) (*queryutils.Query, error) {
	q, err := queryutils.NewQueryBuilder(DevicesTable).WithSelectAllClause().WithFromClause().WithOffsetPagination(offset, pageSize).WithWhereClause(filter)
	if err != nil {
		return nil, utils.InvalidFilterError(err)
	}

	// As aip132 parser doesn't allow "-" in the identifiers
	// we need to add quotation marks for the dynamic labels as they may have it.
	re := regexp.MustCompile(`labels\.([^\s]+)`)
	orderby = re.ReplaceAllString(orderby, "labels.`$1`")

	q, err = q.WithOrderByClause(orderby, "id")
	if err != nil {
		return nil, utils.InvalidOrderByError(err)
	}

	return q.Build(), nil
}

func buildGetColumnQuery(distinct bool, column *queryutils.Column) *queryutils.Query {
	return queryutils.NewQueryBuilder(DevicesTable).WithSelectClause(distinct, column).WithFromClause().Build()
}

func buildCountDevicesQuery(filter string) (*queryutils.Query, error) {
	q, err := queryutils.NewQueryBuilder(DevicesTable).WithCustomSelectClause(`SELECT
		COUNT(*) AS total,
		COUNT(CASE WHEN state = 'DEVICE_STATE_LEASED' THEN 1 ELSE NULL END) AS leased,
		COUNT(CASE WHEN state = 'DEVICE_STATE_AVAILABLE' THEN 1 ELSE NULL END) AS available,
		COUNT(CASE WHEN labels -> 'dut_state' -> 'Values' ? 'ready' THEN 1 ELSE NULL END) AS ready,
		COUNT(CASE WHEN labels -> 'dut_state' -> 'Values' ? 'needs_manual_repair' THEN 1 ELSE NULL END) AS needs_manual_repair,
		COUNT(CASE WHEN labels -> 'dut_state' -> 'Values' ? 'needs_repair' THEN 1 ELSE NULL END) AS needs_repair,
		COUNT(CASE WHEN labels -> 'dut_state' -> 'Values' ? 'repair_failed' THEN 1 ELSE NULL END) AS repair_failed`).WithFromClause().WithWhereClause(filter)
	if err != nil {
		return nil, utils.InvalidFilterError(err)
	}

	return q.Build(), nil
}
