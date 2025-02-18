// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

import (
	"infra/fleetconsole/internal/database/queryutils"
)

var DevicesTable = queryutils.NewTableBuilder("Devices").WithColumns(
	queryutils.NewColumn("id").Build(),
	queryutils.NewColumn("dut_id").Build(),
	queryutils.NewColumn("host").Build(),
	queryutils.NewColumn("port").Build(),
	queryutils.NewColumn("type").Build(),
	queryutils.NewColumn("state").Build(),
	queryutils.NewColumn("labels").WithColumnType(
		queryutils.ColumnTypeJSONB).WithJSONFullPath(func(fields ...string) []string {
		pathComponents := []string{}
		pathComponents = append(pathComponents, fields...)
		pathComponents = append(pathComponents, "values")
		return pathComponents
	}).Build(),
).Build()
