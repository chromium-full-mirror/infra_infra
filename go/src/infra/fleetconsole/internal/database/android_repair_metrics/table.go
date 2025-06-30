// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidrepairmetricsdb

import (
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

var (
	// Columns
	Id              = queryutils.NewColumn("id").Build()
	LabName         = queryutils.NewColumn("lab_name").Build()
	HostGroupColumn = queryutils.NewColumn("host_group").Build()
	RunTargetColumn = queryutils.NewColumn("run_target").Build()
	state           = queryutils.NewColumn("state").Build()

	AndroidDevicesTable = queryutils.NewTableBuilder("android_devices").WithColumns(
		Id,
		LabName,
		HostGroupColumn,
		RunTargetColumn,
		state,
	).Build()
)
