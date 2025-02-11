// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import (
	"fmt"
	"strings"

	"go.chromium.org/luci/common/data/aip132"
)

// WithOrderByClause adds Standard SQL Order by clause, to the query including
// "ORDER BY" and trailing new line (if an order is specified).
// If no order is specified, returns "".
//
// The returned order clause is safe against SQL injection; only
// strings appearing from Table appear in the output.
func (q *QueryBuilder) WithOrderByClause(order string) (*QueryBuilder, error) {
	orderby, err := aip132.ParseOrderBy(order)
	if err != nil {
		return q, err
	}

	if len(orderby) == 0 {
		return q, nil
	}
	var result strings.Builder
	result.WriteString("ORDER BY ")
	for i, o := range orderby {
		if i > 0 {
			result.WriteString(", ")
		}

		if o.FieldPath.String() == "" {
			return q, fmt.Errorf("column name cannot be empty string")
		}

		segments := o.FieldPath.GetSegments()

		// Treat the first segment as a column name and the rest as a path
		// assuming that the column is a json
		columnName := segments[0]
		fields := segments[1:]
		column, ok := q.table.columnByExternalName[columnName]
		if !ok {
			return q, fmt.Errorf("column `%s` doesn't exist", columnName)
		}

		if len(fields) > 0 {
			if column.columnType != ColumnTypeJSONB {
				return q, fmt.Errorf("subfields are only supported for json columns. Try removing the '.' from after your column named %q", column.externalName)
			}

			fullPath := column.jsonFullPath(fields...)
			params := make([]string, len(fullPath))
			for i, field := range fullPath {
				params[i] = q.bind(field)
			}

			// In case of arrays in json it will order by based on the first element
			result.WriteString(fmt.Sprintf("%s -> %s", column.name, strings.Join(params, " -> ")))
		} else {
			result.WriteString(column.name)
		}

		if o.Descending {
			result.WriteString(" DESC")
		}
	}
	result.WriteString("\n")

	q.orderByClause = result.String()
	return q, nil
}
