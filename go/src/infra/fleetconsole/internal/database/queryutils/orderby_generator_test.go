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

func TestOrderByClause(t *testing.T) {
	ftt.Run("OrderByClause", t, func(t *ftt.Test) {
		table := NewTableBuilder("Devices").WithColumns(
			NewColumn("id").Build(),
			NewColumn("dut_state").Build(),
			NewColumn("dut_name").Build(),
			NewColumn("labels").WithColumnType(ColumnTypeJSONB).WithJSONFullPath(func(fields ...string) []string {
				pathComponents := []string{"labels"}
				pathComponents = append(pathComponents, fields...)
				pathComponents = append(pathComponents, "values")
				return pathComponents
			}).Build(),
		).Build()

		t.Run("empty order by", func(t *ftt.Test) {
			q, err := NewQueryBuilder(table).WithOrderByClause("", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.orderByClause, should.BeEmpty)
		})
		t.Run("single order by", func(t *ftt.Test) {
			q, err := NewQueryBuilder(table).WithOrderByClause("dut_state", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.orderByClause, should.Equal("ORDER BY dut_state"))
		})
		t.Run("multiple order by", func(t *ftt.Test) {
			q, err := NewQueryBuilder(table).WithOrderByClause("dut_state desc, dut_name", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.orderByClause, should.Equal("ORDER BY dut_state DESC, dut_name"))
		})
		t.Run("order by based on specific label from labels", func(t *ftt.Test) {
			q, err := NewQueryBuilder(table).WithOrderByClause("labels.`dut-state` desc", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.parameters.values, should.Match([]any{
				"labels", "dut-state", "values",
			}))
			assert.Loosely(t, q.orderByClause, should.Equal("ORDER BY labels -> $1 -> $2 -> $3 DESC"))
		})
		t.Run("not existing column", func(t *ftt.Test) {
			_, err := NewQueryBuilder(table).WithOrderByClause("suspicious_column", "")
			assert.Loosely(t, err, should.ErrLike("column `suspicious_column` doesn't exist"))
		})
		t.Run("repeated field in order by", func(t *ftt.Test) {
			_, err := NewQueryBuilder(table).WithOrderByClause("dut_state,dut_state", "")
			assert.Loosely(t, err, should.ErrLike(`field appears multiple times: "dut_state"`))
		})
		t.Run("add deterministic field in order by", func(t *ftt.Test) {
			q, err := NewQueryBuilder(table).WithOrderByClause("dut_state", "id")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.orderByClause, should.Equal("ORDER BY dut_state, id"))
		})
		t.Run("don't add deterministic field in order by if it is already in use", func(t *ftt.Test) {
			q, err := NewQueryBuilder(table).WithOrderByClause("id desc, dut_state", "id")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.orderByClause, should.Equal("ORDER BY id DESC, dut_state"))
		})
	})
}
