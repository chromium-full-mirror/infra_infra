// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"regexp"

	"infra/fleetconsole/internal/database/queryutils"
	"infra/fleetconsole/internal/utils"
)

func buildListDevicesQuery(offset int, pageSize int, filter string, orderby string) (*queryutils.Query, error) {
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
