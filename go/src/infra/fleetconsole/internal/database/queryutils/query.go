// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import (
	"fmt"
	"strings"
)

// QueryParameters represents a collection of query parameters.
type QueryParameters struct {
	values        []any
	nextValueName int
}

// QueryBuilder is a helper to create an sql query
type QueryBuilder struct {
	table      *Table
	parameters *QueryParameters

	// Clauses
	selectClause  string
	fromClause    string
	whereClause   string
	orderByClause string

	// Pagination
	paginationClause string
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

// WithSelectAllClause adds a select clause with all the columns to the query.
func (q *QueryBuilder) WithSelectAllClause() *QueryBuilder {
	return q.WithSelectClause(false, q.table.Columns...)
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

	q.selectClause = result.String()
	return q
}

// WithFromClause adds a from clause to the query.
func (q *QueryBuilder) WithFromClause() *QueryBuilder {
	q.fromClause = fmt.Sprintf("FROM \"%s\"", q.table.name)
	return q
}

// WithOffsetPagination adds necessary clauses to get the specific page.
func (q *QueryBuilder) WithOffsetPagination(offset int, pageSize int) *QueryBuilder {
	q.paginationClause = fmt.Sprintf("LIMIT %d\nOFFSET %d", pageSize, offset)
	return q
}

func (q *QueryBuilder) Build() *Query {
	return &Query{
		Statement:  fmt.Sprintf("%s\n%s\n%s\n%s\n%s;", q.selectClause, q.fromClause, q.whereClause, q.orderByClause, q.paginationClause),
		Parameters: q.parameters.values,
	}
}

// bind binds a new query parameter with the given value, and returns
// the name of the parameter.
// The returned string is an injection-safe SQL expression.
func (q *QueryBuilder) bind(value string) string {
	name := fmt.Sprintf("$%d", q.parameters.nextValueName)
	q.parameters.nextValueName += 1
	q.parameters.values = append(q.parameters.values, value)
	return name
}
