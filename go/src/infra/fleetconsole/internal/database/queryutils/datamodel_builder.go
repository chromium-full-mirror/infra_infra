// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

type ColumnBuilder struct {
	column Column
}

// NewColumn starts building a new column.
func NewColumn(name string) *ColumnBuilder {
	return &ColumnBuilder{Column{
		name:         name,
		externalName: name,
		columnType:   ColumnTypeString,
		jsonFullPath: func(fields ...string) []string {
			return fields
		}}}
}

func (c *ColumnBuilder) WithExternalName(externalName string) *ColumnBuilder {
	c.column.externalName = externalName
	return c
}

func (c *ColumnBuilder) WithColumnType(columnType ColumnType) *ColumnBuilder {
	c.column.columnType = columnType
	return c
}

func (c *ColumnBuilder) WithJSONFullPath(f func(fields ...string) []string) *ColumnBuilder {
	c.column.jsonFullPath = f
	return c
}

// Build returns the built column.
func (c *ColumnBuilder) Build() *Column {
	result := &c.column
	return result
}

type TableBuilder struct {
	columns []*Column
}

// NewTable starts building a new table.
func NewTable() *TableBuilder {
	return &TableBuilder{}
}

// WithColumns specifies the columns in the table.
func (t *TableBuilder) WithColumns(columns ...*Column) *TableBuilder {
	t.columns = columns
	return t
}

// Build returns the built table.
func (t *TableBuilder) Build() *Table {
	columnByExternalName := make(map[string]*Column)
	for _, c := range t.columns {
		if _, ok := columnByExternalName[c.externalName]; ok {
			panic("multiple columns with the same field path: " + c.externalName)
		}
		columnByExternalName[c.externalName] = c
	}

	return &Table{
		columns:              t.columns,
		columnByExternalName: columnByExternalName,
	}
}
