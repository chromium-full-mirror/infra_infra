// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"infra/fleetconsole/internal/database/queryutils"
)

var (
	// Columns
	IdColumn     = queryutils.NewColumn("id").Build()     //nolint:stylecheck
	DutIdColumn  = queryutils.NewColumn("dut_id").Build() //nolint:stylecheck
	HostColumn   = queryutils.NewColumn("host").Build()
	PortColumn   = queryutils.NewColumn("port").Build()
	TypeColumn   = queryutils.NewColumn("type").Build()
	StateColumn  = queryutils.NewColumn("state").Build()
	LabelsColumn = queryutils.NewColumn("labels").WithColumnType(
		queryutils.ColumnTypeJSONB).WithJSONFullPath(func(fields ...string) []string {
		pathComponents := []string{}
		pathComponents = append(pathComponents, fields...)
		pathComponents = append(pathComponents, "Values")
		return pathComponents
	}).Build()

	// Table
	DevicesTable = queryutils.NewTableBuilder("Devices").WithColumns(
		IdColumn,
		DutIdColumn,
		HostColumn,
		PortColumn,
		TypeColumn,
		StateColumn,
		LabelsColumn,
	).Build()
)
