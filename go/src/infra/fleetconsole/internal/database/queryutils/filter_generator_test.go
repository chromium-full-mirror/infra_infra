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

func TestWhereClause(t *testing.T) {
	ftt.Run("WhereClause", t, func(t *ftt.Test) {
		table := NewTableBuilder("Devices").WithColumns(
			NewColumn("dut_state").Build(),
			NewColumn("dut_name").Build(),
			NewColumn("test_date").Build(),
			NewColumn("labels").WithColumnType(ColumnTypeJSONB).WithJSONFullPath(func(fields ...string) []string {
				pathComponents := []string{"labels"}
				pathComponents = append(pathComponents, fields...)
				pathComponents = append(pathComponents, "values")
				return pathComponents
			}).Build(),
		).Build()

		t.Run("Empty filter", func(t *ftt.Test) {
			q, err := NewQueryBuilder(table).WithWhereClause("", nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, q.parameters.values, should.HaveLength(0))
			assert.Loosely(t, q.whereClause, should.Equal(""))
		})
		t.Run("Simple filter", func(t *ftt.Test) {
			t.Run("has operator", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("dut_name:chromeos", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "dut_name", Value: "%chromeos%"},
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE (dut_name LIKE $1)"))
			})
			t.Run("equals operator", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("dut_state = available", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "dut_state", Value: "available"},
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE (dut_state = $1)"))
			})
			t.Run("not equals operator", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("dut_state != available", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "dut_state", Value: "available"},
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE (dut_state <> $1)"))
			})
			t.Run("greater than operator", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("test_date > 2020-01-01", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "test_date", Value: "2020-01-01"},
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE (test_date > $1)"))
			})
			t.Run("lesser than operator", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("test_date < 2020-01-01", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "test_date", Value: "2020-01-01"},
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE (test_date < $1)"))
			})
			t.Run("greater or equal operator", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("test_date >= 2020-01-01", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "test_date", Value: "2020-01-01"},
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE (test_date >= $1)"))
			})
			t.Run("lesser or equal operator", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("test_date <= 2020-01-01", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "test_date", Value: "2020-01-01"},
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE (test_date <= $1)"))
			})
			t.Run("composite to LIKE", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("dut_state:(something)", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "dut_state", Value: "%something%"},
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE (dut_state LIKE $1)"))
			})
			t.Run("composite to equals", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("dut_state=(something)", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "dut_state", Value: "something"},
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE (dut_state = $1)"))
			})
			t.Run("unsupported field LHS", func(t *ftt.Test) {
				_, err := NewQueryBuilder(table).WithWhereClause("dut_state.something=available", nil)
				assert.Loosely(t, err, should.ErrLike("fields are only supported for json columns"))
			})
			t.Run("unsupported field RHS", func(t *ftt.Test) {
				_, err := NewQueryBuilder(table).WithWhereClause("dut_name=chromeos.host4", nil)
				assert.Loosely(t, err, should.ErrLike("fields not implemented yet"))
			})
			t.Run("not existing column", func(t *ftt.Test) {
				_, err := NewQueryBuilder(table).WithWhereClause("suspicious_column=suspicious_value", nil)
				assert.Loosely(t, err, should.ErrLike("column `suspicious_column` doesn't exist"))
			})
			t.Run("json array has filter", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("labels.labels-os_type=OS_TYPE_CROS", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "json_path_variable_0:labels", Value: "labels"},
					{Name: "json_path_variable_1:labels", Value: "labels-os_type"},
					{Name: "json_path_variable_2:labels", Value: string("values")},
					{
						Name:  "labels",
						Value: "OS_TYPE_CROS",
					},
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE (labels -> $1 -> $2 -> $3 ? $4)"))
			})

			t.Run("complex filter", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("labels.label-os_type = (OS_TYPE_CROS AND OS_TYPE_LABSTATION) dut_state = (DEVICE_STATE_LEASED AND DEVICE_STATE_AVAILABLE)", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "json_path_variable_0:labels", Value: "labels"},
					{Name: "json_path_variable_1:labels", Value: "label-os_type"},
					{Name: "json_path_variable_2:labels", Value: string("values")},
					{
						Name:  "labels",
						Value: "OS_TYPE_CROS",
					},
					{Name: "json_path_variable_0:labels", Value: "labels"},
					{Name: "json_path_variable_1:labels", Value: "label-os_type"},
					{Name: "json_path_variable_2:labels", Value: string("values")},
					{
						Name:  "labels",
						Value: "OS_TYPE_LABSTATION",
					},
					{Name: "dut_state", Value: "DEVICE_STATE_LEASED"},
					{Name: "dut_state", Value: "DEVICE_STATE_AVAILABLE"},
					// "labels", "label-os_type", "values", "OS_TYPE_CROS", "labels", "label-os_type", "values", "OS_TYPE_LABSTATION", "DEVICE_STATE_LEASED", "DEVICE_STATE_AVAILABLE",
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE (((labels -> $1 -> $2 -> $3 ? $4) AND (labels -> $5 -> $6 -> $7 ? $8)) AND ((dut_state = $9) AND (dut_state = $10)))"))
			})

			t.Run("filter with AND and OR", func(t *ftt.Test) {
				q, err := NewQueryBuilder(table).WithWhereClause("dut_state = (a AND b OR c)", nil)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, q.parameters.values, should.Match([]QueryParameter{
					{Name: "dut_state", Value: "a"},
					{Name: "dut_state", Value: "b"},
					{Name: "dut_state", Value: "c"},
				}))
				assert.Loosely(t, q.whereClause, should.Equal("WHERE ((dut_state = $1) AND ((dut_state = $2) OR (dut_state = $3)))"))
			})
		})
	})
}
