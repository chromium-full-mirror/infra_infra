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
		).Build()

		t.Run("query with all the clauses", func(t *ftt.Test) {
			builder := NewQueryBuilder(table).WithSelectAllClause().WithFromClause().WithOffsetPagination(20, 10)
			builder, err := builder.WithWhereClause("dut_state = available")
			assert.Loosely(t, err, should.BeNil)
			builder, err = builder.WithOrderByClause("id desc", "")
			assert.Loosely(t, err, should.BeNil)
			q := builder.Build()
			assert.Loosely(t, q.Parameters, should.Match([]any{
				"available",
			}))
			assert.Loosely(t, q.Statement, should.Equal("SELECT id, dut_state, dut_name, labels\nFROM \"Devices\"\nWHERE (dut_state = $1)\nORDER BY id DESC\nLIMIT 10\nOFFSET 20;"))
		})

		t.Run("query for a specific column's distinct values", func(t *ftt.Test) {
			q := NewQueryBuilder(table).WithSelectClause(true, dutStateColumn).WithFromClause().Build()
			assert.Loosely(t, q.Parameters, should.BeEmpty)
			assert.Loosely(t, q.Statement, should.Equal("SELECT DISTINCT dut_state\nFROM \"Devices\"\n\n\n;"))
		})
	})
}
