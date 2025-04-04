// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testutils

import (
	"database/sql/driver"
	"encoding/csv"
	"os"
	"slices"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"go.chromium.org/infra/fleetconsole/internal/utils"
)

// MockSQLRowsFromCSV reads a CSV file and creates sqlmock.Rows, optionally filtering by 'id'.
func MockSQLRowsFromCSV(t *testing.T, path string, ids []string) *sqlmock.Rows {
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	cols, err := reader.Read()
	if err != nil {
		t.Fatalf("failed to read columns from the CSV: %v", err)
	}

	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read rows from the CSV: %v", err)
	}

	if ids != nil {
		idIndex := findColIndex(t, cols, "id")
		rows = utils.Filter(rows, func(row []string) bool { return slices.Contains(ids, row[idIndex]) })
	}

	return sqlmock.NewRows(cols).AddRows(utils.Map(rows, func(row []string) []driver.Value {
		return utils.Map(row, func(value string) driver.Value { return driver.Value(value) })
	})...)
}

// MockSQLRowsFromCSVForCol reads a CSV file and creates sqlmock.Rows for the specified column.
func MockSQLRowsFromCSVForCol(t *testing.T, path string, distinct bool, column string) *sqlmock.Rows {
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	cols, err := reader.Read()
	if err != nil {
		t.Fatalf("failed to read columns from the CSV: %v", err)
	}

	columnIndex := findColIndex(t, cols, column)

	rowsData, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read rows from the CSV: %v", err)
	}

	mockRows := sqlmock.NewRows([]string{column})

	seen := utils.Set[string]{}
	for _, row := range rowsData {
		if columnIndex < len(row) {
			value := row[columnIndex]
			if !distinct || distinct && !seen.Contains(value) {
				mockRows.AddRow(driver.Value(row[columnIndex]))
			}
			seen.Add(value)
		} else {
			t.Fatalf("row does not have enough columns, expected at least column '%s' (index %d), got %d", column, columnIndex, len(row))
		}
	}

	return mockRows
}

func findColIndex(t *testing.T, cols []string, column string) int {
	for i, col := range cols {
		if col == column {
			return i
		}
	}

	t.Fatalf("column '%s' not found in the CSV", column)
	return -1
}
