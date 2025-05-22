// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dataframe

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewFlatTable(t *testing.T) {
	t.Parallel()
	columns := []string{"col1", "col2"}
	table := NewFlatTable(columns)

	if diff := cmp.Diff(table.GetColumns(), columns); diff != "" {
		t.Errorf("NewFlatTable: Columns not initialized correctly. Expected %v, got %v, diff (-want +got):\n%s", columns, table.GetColumns(), diff)
	}

	if len(table.GetRows()) != 0 {
		t.Errorf("NewFlatTable: Rows not initialized correctly. Expected empty slice, got %v", table.GetRows())
	}
}

func TestFlatTable_AddRow(t *testing.T) {
	t.Parallel()
	table := NewFlatTable([]string{"col1", "col2"})
	row := NewRowFromMap(map[string]any{"col1": 1, "col2": "a"})

	err := table.AddRow(row)
	if err != nil {
		t.Errorf("AddRow: Error adding row: %v", err)
	}

	if len(table.GetRows()) != 1 {
		t.Errorf("AddRow: Row not added. Expected 1 row, got %d", len(table.GetRows()))
	}

	addedRow, _ := table.GetRow(0)
	if diff := cmp.Diff(addedRow, row); diff != "" {
		t.Errorf("AddRow: Row content incorrect. Expected %v, got %v, diff (-want +got):\n%s", row, addedRow, diff)
	}

	// Test adding a row with an invalid column
	invalidRow := NewRowFromMap(map[string]any{"col1": 1, "col3": "b"})
	err = table.AddRow(invalidRow)
	if err == nil {
		t.Errorf("AddRow: Expected error when adding row with invalid column, got nil")
	}
}

func TestFlatTable_GetRow(t *testing.T) {
	t.Parallel()
	table := NewFlatTable([]string{"col1", "col2"})
	row1 := NewRowFromMap(map[string]any{"col1": 1, "col2": "a"})
	row2 := NewRowFromMap(map[string]any{"col1": 2, "col2": "b"})
	table.AddRow(row1)
	table.AddRow(row2)

	row, err := table.GetRow(1)
	if err != nil {
		t.Errorf("GetRow: Error getting row: %v", err)
	}

	if diff := cmp.Diff(row, row2); diff != "" {
		t.Errorf("GetRow: Incorrect row returned. Expected %v, got %v, diff (-want +got):\n%s", row2, row, diff)
	}

	// Test out of range index
	_, err = table.GetRow(2)
	if err == nil {
		t.Errorf("GetRow: Expected error for out of range index, got nil")
	}

	_, err = table.GetRow(-1)
	if err == nil {
		t.Errorf("GetRow: Expected error for negative index, got nil")
	}
}

func TestFlatTable_GetValue(t *testing.T) {
	t.Parallel()
	table := NewFlatTable([]string{"col1", "col2"})
	row := NewRowFromMap(map[string]any{"col1": 1, "col2": "a"})
	table.AddRow(row)

	value, err := table.GetValue(0, "col1")
	if err != nil {
		t.Errorf("GetValue: Error getting value: %v", err)
	}

	if value != 1 {
		t.Errorf("GetValue: Incorrect value returned. Expected 1, got %v", value)
	}

	// Test invalid row index
	_, err = table.GetValue(1, "col1")
	if err == nil {
		t.Errorf("GetValue: Expected error for invalid row index, got nil")
	}

	// Test invalid column name
	_, err = table.GetValue(0, "col3")
	if err == nil {
		t.Errorf("GetValue: Expected error for invalid column name, got nil")
	}
}

func TestFlatTable_IterateRows(t *testing.T) {
	t.Parallel()
	table := NewFlatTable([]string{"col1", "col2"})
	row1 := NewRowFromMap(map[string]any{"col1": 1, "col2": "a"})
	row2 := NewRowFromMap(map[string]any{"col1": 2, "col2": "b"})
	table.AddRow(row1)
	table.AddRow(row2)

	var iteratedRows []Row
	callback := func(row Row) {
		iteratedRows = append(iteratedRows, row)
	}

	table.IterateRows(callback)

	if len(iteratedRows) != 2 {
		t.Errorf("IterateRows: Incorrect number of rows iterated. Expected 2, got %d", len(iteratedRows))
	}

	if diff := cmp.Diff(iteratedRows[0], row1); diff != "" {
		t.Errorf("IterateRows: Incorrect row iterated. Expected %v, got %v, diff (-want +got):\n%s", row1, iteratedRows[0], diff)
	}

	if diff := cmp.Diff(iteratedRows[1], row2); diff != "" {
		t.Errorf("IterateRows: Incorrect row iterated. Expected %v, got %v, diff (-want +got):\n%s", row2, iteratedRows[1], diff)
	}
}

func TestFlatTable_DropColumn(t *testing.T) {
	t.Parallel()
	table := NewFlatTable([]string{"col1", "col2", "col3"})
	row1 := NewRowFromMap(map[string]any{"col1": 1, "col2": "a", "col3": true})
	row2 := NewRowFromMap(map[string]any{"col1": 2, "col2": "b", "col3": false})
	table.AddRow(row1)
	table.AddRow(row2)

	cases := []struct {
		droppingCol     string
		expectedColumns []string
	}{
		{"col2", []string{"col1", "col3"}},
		{"col3", []string{"col1"}},
	}
	for _, tc := range cases {

		table.DropColumn(tc.droppingCol)

		if diff := cmp.Diff(table.GetColumns(), tc.expectedColumns); diff != "" {
			t.Errorf("DropColumn: Columns not updated correctly. Expected %v, got %v, diff (-want +got):\n%s", tc.expectedColumns, table.GetColumns(), diff)
		}

		for _, row := range table.GetRows() {
			_, err := row.GetValue(tc.droppingCol)
			if err == nil {
				t.Errorf("DropColumn: Column %q should be removed from rows", tc.droppingCol)
			}
		}
	}

	// Test dropping a non-existent column
	originalColumns := table.GetColumns()
	table.DropColumn("col4")
	if diff := cmp.Diff(table.GetColumns(), originalColumns); diff != "" {
		t.Errorf("DropColumn: Dropping non-existent column should not change Columns, diff (-want +got):\n%s", diff)
	}
}

func TestSimpleRow_GetColumns(t *testing.T) {
	t.Parallel()
	row := NewRowFromMap(map[string]any{"col1": 1, "col2": "a"})
	columns := row.GetColumns()

	expectedColumns := []string{"col1", "col2"} // Order is not guaranteed
	if len(columns) != len(expectedColumns) {
		t.Errorf("GetColumns: Incorrect number of columns. Expected %d, got %d", len(expectedColumns), len(columns))
	}

	colMap := make(map[string]bool)
	for _, col := range columns {
		colMap[col] = true
	}

	for _, expectedCol := range expectedColumns {
		if _, ok := colMap[expectedCol]; !ok {
			t.Errorf("GetColumns: Missing column %s", expectedCol)
		}
	}
}

func TestSimpleRow_GetValue(t *testing.T) {
	t.Parallel()
	row := NewRowFromMap(map[string]any{"col1": 1, "col2": "a"})

	value, err := row.GetValue("col1")
	if err != nil {
		t.Errorf("GetValue: Error getting value: %v", err)
	}

	if value != 1 {
		t.Errorf("GetValue: Incorrect value returned. Expected 1, got %v", value)
	}

	// Test invalid column name
	_, err = row.GetValue("col3")
	if err == nil {
		t.Errorf("GetValue: Expected error for invalid column name, got nil")
	}
}

func TestSimpleRow_DeleteColumn(t *testing.T) {
	t.Parallel()
	row := NewRowFromMap(map[string]any{"col1": 1, "col2": "a"})

	row.DeleteColumn("col1")

	_, err := row.GetValue("col1")
	if err == nil {
		t.Errorf("DeleteColumn: Column 'col1' should be deleted")
	}
}
