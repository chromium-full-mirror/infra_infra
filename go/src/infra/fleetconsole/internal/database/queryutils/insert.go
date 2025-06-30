// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import (
	"fmt"
	"slices"
	"strings"

	"go.chromium.org/luci/common/errors"
)

type ConflictBuilder struct {
	constrainColumns []*Column
	replace          []*Column
}

type InsertBuilder struct {
	table    *Table
	columns  []*Column
	values   []string
	conflict *ConflictBuilder
}

func NewInsertBuilder(table *Table) *InsertBuilder {
	return &InsertBuilder{
		table: table,
	}
}

func (builder *InsertBuilder) Columns(columns ...*Column) *InsertBuilder {
	builder.columns = append(builder.columns, columns...)
	return builder
}

func (builder *InsertBuilder) AllColumns() *InsertBuilder {
	builder.columns = append(builder.columns, builder.table.Columns...)
	return builder
}

func (builder *InsertBuilder) Values(values ...string) *InsertBuilder {
	builder.values = append(builder.values, values...)
	return builder
}

func (builder *InsertBuilder) OnConflict(conflict *ConflictBuilder) *InsertBuilder {
	builder.conflict = conflict
	return builder
}

func ConflictOn(constrainColumns ...*Column) *ConflictBuilder {
	return &ConflictBuilder{
		constrainColumns: constrainColumns,
	}
}

func (conflictBuilder *ConflictBuilder) Replace(c ...*Column) *ConflictBuilder {
	conflictBuilder.replace = slices.Concat(conflictBuilder.replace, c)
	return conflictBuilder
}

func (builder *InsertBuilder) Build() (*Query, error) {
	if len(builder.columns) == 0 {
		builder.AllColumns()
	}

	if len(builder.columns) != len(builder.values) {
		return nil, errors.Reason("number of columns (%d) does not match number of values (%d)", len(builder.columns), len(builder.values)).Err()
	}

	var sb strings.Builder
	sb.WriteString("INSERT INTO \"")
	sb.WriteString(builder.table.name)
	sb.WriteString("\" ( ")

	for i, c := range builder.columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(c.name)
	}
	sb.WriteString(" ) VALUES ")

	sb.WriteString(ValuesString(len(builder.columns), len(builder.columns)))

	if builder.conflict != nil && len(builder.conflict.constrainColumns) > 0 {
		sb.WriteString(" ON CONFLICT (")
		for i, c := range builder.conflict.constrainColumns {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(c.name)
		}
		sb.WriteString(")")

		if len(builder.conflict.replace) > 0 {
			sb.WriteString(" DO UPDATE SET ")
			for i, c := range builder.conflict.replace {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%s = EXCLUDED.%s", c.name, c.name))
			}
		} else {
			sb.WriteString(" DO NOTHING")
		}
	}

	sb.WriteString(";")

	return &Query{
		Statement:  sb.String(),
		Parameters: ToAnySlice(builder.values),
	}, nil
}
