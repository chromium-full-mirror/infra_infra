// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestQueryBuilder(t *testing.T) {
	ftt.Run("QueryBuilder", t, func(t *ftt.Test) {
		dutStateColumn := NewColumn("dut_state").Build()
		table := NewTableBuilder("Devices").WithColumns(
			NewColumn("id").Build(),
			dutStateColumn,
			NewColumn("dut_name").Build(),
			NewColumn("labels").WithColumnType(ColumnTypeJSONB).WithJSONFullPath(func(fields ...string) []string {
				pathComponents := []string{"labels"}
				pathComponents = append(pathComponents, fields...)
				pathComponents = append(pathComponents, "values")
				return pathComponents
			}).Build(),
			NewColumn("realm").Build(),
		).Build()

		t.Run("query with all the clauses", func(t *ftt.Test) {
			builder := NewQueryBuilder(table).WithSelectAllClause().WithFromClause().WithOffsetPagination(20, 10)
			builder, err := builder.WithWhereClause("dut_state = available", nil)
			assert.Loosely(t, err, should.BeNil)
			builder, err = builder.WithOrderByClause("id desc", "")
			assert.Loosely(t, err, should.BeNil)
			q, err := builder.Build([]string{"my-realm"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.Parameters, should.Match([]any{
				"available",
				"my-realm",
			}))
			assert.Loosely(t, q.Statement, should.Equal("SELECT id, dut_state, dut_name, labels, realm\nFROM \"Devices\"\nWHERE (dut_state = $1) AND (realm IN ($2) OR realm IS NULL)\nORDER BY id DESC\nLIMIT 10\nOFFSET 20;"))
		})

		t.Run("query with all the clauses (bigquery)", func(t *ftt.Test) {
			builder := NewQueryBuilder(table)
			builder.sqlLangType = BigQueryLangType
			assert.Loosely(t, builder.sqlLangType, should.Equal(BigQueryLangType))
			builder = builder.WithSelectAllClause().WithFromClause().WithOffsetPagination(20, 10)
			builder, err := builder.WithWhereClause("dut_state = available", nil)
			assert.Loosely(t, err, should.BeNil)
			builder, err = builder.WithOrderByClause("id desc", "")
			assert.Loosely(t, err, should.BeNil)
			q, err := builder.Build([]string{"my-realm"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.Parameters, should.Match([]any{
				"available",
				"my-realm",
			}))
			assert.Loosely(t, q.Statement, should.Equal("SELECT id, dut_state, dut_name, labels, realm\nFROM `Devices`\nWHERE (dut_state = ?) AND (realm IN (?) OR realm IS NULL)\nORDER BY id DESC\nLIMIT 10\nOFFSET 20;"))
		})

		t.Run("query for a specific column's distinct values", func(t *ftt.Test) {
			q, err := NewQueryBuilder(table).WithSelectClause(true, dutStateColumn).WithFromClause().Build(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.Parameters, should.BeEmpty)
			assert.Loosely(t, q.Statement, should.Equal("SELECT DISTINCT dut_state\nFROM \"Devices\"\n\n\n;"))
		})

		t.Run("query for specific ids and with empty realms", func(t *ftt.Test) {
			q, err := NewQueryBuilder(table).WithSelectAllClause().WithFromClause().WithSpecificIdsFilter([]string{"device1", "device2"}).Build([]string{})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.Parameters, should.Match([]any{
				"device1",
				"device2",
			}))
			assert.Loosely(t, q.Statement, should.Equal("SELECT id, dut_state, dut_name, labels, realm\nFROM \"Devices\"\nWHERE (id IN ($1,$2)) AND (realm IS NULL)\n\n;"))
		})

		t.Run("query for all columns except dut_state", func(t *ftt.Test) {
			q, err := NewQueryBuilder(table).WithSelectAllClause(dutStateColumn).WithFromClause().Build([]string{})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.Parameters, should.BeEmpty)
			assert.Loosely(t, q.Statement, should.Equal("SELECT id, dut_name, labels, realm\nFROM \"Devices\"\nWHERE (realm IS NULL)\n\n;"))
		})
	})
}
