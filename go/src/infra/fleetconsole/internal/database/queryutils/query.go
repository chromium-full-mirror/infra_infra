// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// QueryParameters represents a collection of query parameters.
type QueryParameters struct {
	values        []any
	nextValueName int
}

// QueryBuilder is a helper to create an sql query
type QueryBuilder struct {
	sqlLangType SqlLangType

	table      *Table
	parameters *QueryParameters

	// Clauses
	selectClause  *SelectClause
	fromClause    string
	whereClause   string
	orderByClause string

	// Pagination
	paginationClause string

	// Other filters

	// if provided will query only for the devices with the specified ids
	specificIds []string
}

type Query struct {
	Statement  string
	Parameters []any
}

func NewQueryBuilder(t *Table) *QueryBuilder {
	return &QueryBuilder{
		table:      t,
		parameters: &QueryParameters{nextValueName: 1},
	}
}

type SqlLangType string

const (
	BigQueryLangType SqlLangType = "bigquery"
	PostgresLangType SqlLangType = "postgres"
)

func (q *QueryBuilder) SetSqlLangType(sqlLangType SqlLangType) *QueryBuilder {
	q.sqlLangType = sqlLangType
	return q
}

// WithSelectAllClause adds a select clause with all the columns to the query.
func (q *QueryBuilder) WithSelectAllClause(excludeColumns ...*Column) *QueryBuilder {
	columnsToSelect := []*Column{}
	for _, column := range q.table.Columns {
		if !slices.ContainsFunc(excludeColumns, func(excludeColumn *Column) bool {
			return excludeColumn.name == column.name
		}) {
			columnsToSelect = append(columnsToSelect, column)
		}
	}
	return q.WithSelectClause(false, columnsToSelect...)
}

// WithSelectClause adds a select clause with the specified columns to the query.
func (q *QueryBuilder) WithSelectClause(distinct bool, columns ...*Column) *QueryBuilder {
	var result strings.Builder
	result.WriteString("SELECT ")
	if distinct {
		result.WriteString("DISTINCT ")
	}
	for i, c := range columns {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(c.name)
	}

	q.selectClause = &SelectClause{selectClause: result.String()}
	return q
}

// WithCustomSelectClause adds a select clause object to the query
func (q *QueryBuilder) WithCustomSelectClause(fieldSelectClauses ...*FieldSelectClause) *QueryBuilder {
	return q.withCustomSelectClauseInternal(false, fieldSelectClauses...)
}

// WithCustomSelectDistinctClause adds a select distinct clause object to the query
func (q *QueryBuilder) WithCustomSelectDistinctClause(fieldSelectClauses ...*FieldSelectClause) *QueryBuilder {
	return q.withCustomSelectClauseInternal(true, fieldSelectClauses...)
}

func (q *QueryBuilder) withCustomSelectClauseInternal(distinct bool, fieldSelectClauses ...*FieldSelectClause) *QueryBuilder {
	var selectClauses []string
	var bindings = map[string]*any{}
	for i, builder := range fieldSelectClauses {
		bindingAlias := "v" + strconv.Itoa(i)
		boundClause := builder.fieldSelectClause + " AS " + bindingAlias

		bindings[bindingAlias] = builder.binding
		selectClauses = append(selectClauses, boundClause)
	}

	selectClause := "SELECT "
	if distinct {
		selectClause += "DISTINCT "
	}

	q.selectClause = &SelectClause{
		selectClause: selectClause + strings.Join(selectClauses, ", "),
		bindings:     bindings,
	}

	return q
}

// WithRawSelectClause adds a raw SQL select clause to the query.
func (q *QueryBuilder) WithRawSelectClause(selectClause string) *QueryBuilder {
	q.selectClause = &SelectClause{selectClause: selectClause}
	return q
}

// WithFromClause currently does nothing and was left for backwards compatibility. TODO: remove it
func (q *QueryBuilder) WithFromClause() *QueryBuilder {
	return q
}

// WithOffsetPagination adds necessary clauses to get the specific page.
func (q *QueryBuilder) WithOffsetPagination(offset int, pageSize int) *QueryBuilder {
	q.paginationClause = fmt.Sprintf("LIMIT %d\nOFFSET %d", pageSize, offset)
	return q
}

func (q *QueryBuilder) WithSpecificIdsFilter(ids []string) *QueryBuilder {
	q.specificIds = ids
	return q
}

func (q *QueryBuilder) Build(realms []string) (*Query, error) {
	if q.specificIds != nil {
		if err := q.addInFilter("id", q.specificIds, false); err != nil {
			return nil, err
		}
	}
	if realms != nil {
		if err := q.addInFilter("realm", realms, true); err != nil {
			return nil, err
		}
	}

	if q.sqlLangType == BigQueryLangType {
		q.fromClause = fmt.Sprintf("FROM `%s`", q.table.name)
	} else {
		q.fromClause = fmt.Sprintf("FROM \"%s\"", q.table.name)
	}

	return &Query{
		Statement:  fmt.Sprintf("%s\n%s\n%s\n%s\n%s;", q.selectClause.selectClause, q.fromClause, q.whereClause, q.orderByClause, q.paginationClause),
		Parameters: q.parameters.values,
	}, nil
}

func (q *QueryBuilder) addInFilter(column string, values []string, allowNull bool) error {
	if len(values) == 0 && !allowNull {
		return nil
	}

	if !q.table.ColumnExists(column) {
		return fmt.Errorf("column `%s` doesn't exist", column)
	}

	var result strings.Builder
	if q.whereClause != "" {
		result.WriteString(" AND")
	} else {
		result.WriteString("WHERE")
	}
	result.WriteString(" (")

	params := make([]string, len(values))
	for i, value := range values {
		params[i] = q.bind(value)
	}

	if len(params) > 0 {
		result.WriteString(fmt.Sprintf("%s IN (%s)", column, strings.Join(params, ",")))
	}

	if allowNull {
		if len(params) > 0 {
			result.WriteString(" OR ")
		}
		result.WriteString(fmt.Sprintf("%s IS NULL", column))
	}
	result.WriteString(")")

	q.whereClause = fmt.Sprintf("%s%s", q.whereClause, result.String())

	return nil
}

// bind binds a new query parameter with the given value, and returns
// the name of the parameter.
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) bind(value string) string {
	var name string
	if q.sqlLangType == BigQueryLangType {
		name = "?"
	} else {
		name = fmt.Sprintf("$%d", q.parameters.nextValueName)
	}
	q.parameters.nextValueName += 1
	q.parameters.values = append(q.parameters.values, value)
	return name
}
