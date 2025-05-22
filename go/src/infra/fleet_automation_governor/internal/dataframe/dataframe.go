// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dataframe provides a data structure and methods for representing
// and manipulating tabular data.
package dataframe

import (
	"fmt"
)

// DataFrame represents a table with arbitrary columns.
// It allows for storing and retrieving rows of data with
// arbitrary columns. It's designed to be used as an internal data
// representation within a larger data processing system.
type DataFrame interface {
	// AddRow adds a Row to the data frame.
	AddRow(Row) error
	// GetRows returns all the rows of the data frame.
	GetRows() []Row
	// GetRow retrieves a row from the DataFrame by its index.
	GetRow(index int) (Row, error)
	// IterateRows iterates over each row in the DataFrame, applying the provided
	// callback function.
	IterateRows(callback func(row Row))
	// GetColumns returns the column names.
	GetColumns() []string
	// DropColumn removes a column from the DataFrame.
	DropColumn(columnName string)
	// GetValue retrieves a value from a specific cell in the DataFrame.
	GetValue(rowIndex int, columnName string) (any, error)
}

type Row interface {
	// GetColumns returns the column names of the Row.
	GetColumns() []string
	// DeleteColumn deletes a column from the Row.
	DeleteColumn(col string)
	// GetValue gets the value associated with the given column name.
	GetValue(columnName string) (any, error)
}

type flatTable struct {
	columns []string
	rows    []Row
}

// NewFlatTable creates a new flat tble with specified column names.
func NewFlatTable(columns []string) DataFrame {
	return &flatTable{
		columns: columns,
		rows:    []Row{},
	}
}

// AddRow adds a new row to the DataFrame.
func (t *flatTable) AddRow(row Row) error {
	// Validation: Ensure the row's keys match the table's columns.
	for _, key := range row.GetColumns() {
		found := false
		for _, col := range t.columns {
			if key == col {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("column '%s' not found in table schema", key)
		}
	}
	t.rows = append(t.rows, row)
	return nil
}

func (t *flatTable) GetRows() []Row {
	return t.rows
}

// GetRow retrieves a row by index.
func (t *flatTable) GetRow(index int) (Row, error) {
	if index < 0 || index >= len(t.rows) {
		return nil, fmt.Errorf("index out of range")
	}
	return t.rows[index], nil
}

func (t *flatTable) GetColumns() []string {
	return t.columns
}

// GetValue retrieves a value from a row and column.
func (t *flatTable) GetValue(rowIndex int, columnName string) (any, error) {
	row, err := t.GetRow(rowIndex)
	if err != nil {
		return nil, err
	}
	value, err := row.GetValue(columnName)
	if err != nil {
		return nil, fmt.Errorf("get value in row %d, column %q: %w", rowIndex, columnName, err)
	}
	return value, nil
}

// IterateRows iterates over all rows in the DataFrame.
func (t *flatTable) IterateRows(callback func(row Row)) {
	for _, row := range t.rows {
		callback(row)
	}
}

// DropColumn removes a specified column from the flatTable.
func (t *flatTable) DropColumn(columnName string) {
	// Find the index of the column to drop
	columnIndex := -1
	for i, col := range t.columns {
		if col == columnName {
			columnIndex = i
			break
		}
	}
	// If the column doesn't exist, do nothing
	if columnIndex == -1 {
		return
	}
	// Remove the column from the Columns slice
	t.columns = append(t.columns[:columnIndex], t.columns[columnIndex+1:]...)
	// Remove the column from each row
	for _, row := range t.rows {
		row.DeleteColumn(columnName)
	}
}

// simpleRow represents a single row in a table
type simpleRow map[string]any

// NewRowFromMap creates a Row from a map.
// The key of the map is the column name.
func NewRowFromMap(r map[string]any) Row {
	return simpleRow(r)
}

// GetColumns returns the column names of the simpleRow.
func (row simpleRow) GetColumns() []string {
	cols := make([]string, 0, len(row))
	for col := range row {
		cols = append(cols, col)
	}
	return cols
}

// GetValue retrieves a value from a Row by column name.
func (row simpleRow) GetValue(columnName string) (any, error) {
	value, ok := row[columnName]
	if !ok {
		return nil, fmt.Errorf("column '%s' not found in row", columnName)
	}
	return value, nil
}

// DeleteColumn deletes a column from the simpleRow.
func (row simpleRow) DeleteColumn(col string) {
	delete(row, col)
}
