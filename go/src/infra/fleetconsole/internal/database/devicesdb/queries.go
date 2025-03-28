// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"context"
	"regexp"

	"go.chromium.org/luci/server/auth"

	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
	"go.chromium.org/infra/fleetconsole/internal/utils"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

func buildListDevicesQuery(ctx context.Context, offset, pageSize int, filter, orderby string, ids []string, realms []string) (*queryutils.Query, error) {
	q, err := queryutils.NewQueryBuilder(DevicesTable).WithSelectAllClause().WithFromClause().WithSpecificIdsFilter(ids).WithWhereClause(filter)

	if pageSize > 0 {
		q = q.WithOffsetPagination(offset, pageSize)

	}
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

	return q.Build(realms)
}

func buildGetColumnQuery(ctx context.Context, distinct bool, column *queryutils.Column, realms []string) (*queryutils.Query, error) {
	q, _ := queryutils.NewQueryBuilder(DevicesTable).WithSelectClause(distinct, column).WithFromClause().Build(realms)
	return q, nil
}

func buildCountDevicesQuery(ctx context.Context, filter string, realms []string) (*queryutils.Query, error) {
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

	return q.Build(realms)
}

func GetUserRealms(ctx context.Context, cloudProject string) ([]string, error) {
	// When running locally auth.QueryRealms is not implemented,
	// this defaults to showing all devices
	if cloudProject == "" {
		return nil, nil
	}

	realms, err := auth.QueryRealms(ctx, ufsUtil.InventoriesList, "", nil)
	if err != nil {
		return nil, err
	}
	realms = append(realms, "") // Devices with no realms are visible to everyone
	return realms, nil
}
