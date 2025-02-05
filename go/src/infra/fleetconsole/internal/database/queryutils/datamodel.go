// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

// ColumnType is an enum for the type of a column.  Valid values are in the const block above.
type ColumnType int32

const (
	ColumnTypeString ColumnType = iota
	ColumnTypeJSONB
)

// Column represents the schema of a Database column.
type Column struct {
	// Actual column name.
	name string

	// Externally visible name. Defaults to name if not specified.
	externalName string

	// Defaults to ColumnTypeString.
	columnType ColumnType

	// Used only if columnType is ColumnTypeJSONB. Returns actual path to the value.
	// Defaults to [<key1>, <key2>, ..., <keyn>]
	jsonFullPath func(fields ...string) []string
}

// Table represents the schema of a Database table.
type Table struct {
	// The columns in the database table.
	columns []*Column

	// A mapping from externally-visible name to the column
	columnByExternalName map[string]*Column
}
