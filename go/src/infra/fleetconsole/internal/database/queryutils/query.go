// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import "fmt"

// QueryParameters represents a collection of query parameters.
type QueryParameters struct {
	values        []any
	nextValueName int
}

// QueryBuilder is a helper to create an sql query
type QueryBuilder struct {
	table      *Table
	parameters *QueryParameters

	//Clauses
	whereClause   string
	orderByClause string
}

func NewQueryBuilder(t *Table) *QueryBuilder {
	return &QueryBuilder{
		table:      t,
		parameters: &QueryParameters{nextValueName: 1},
	}
}

func (q *QueryBuilder) Build() (string, []any) {
	// TODO: construct the query

	return "should have been a query", q.parameters.values
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
