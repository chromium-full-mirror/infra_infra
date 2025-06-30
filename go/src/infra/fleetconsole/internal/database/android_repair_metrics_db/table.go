// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidrepairmetricsdb

import (
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

var (
	// Columns
	PriorityColumn       = queryutils.NewColumn("priority").Build()
	LabName              = queryutils.NewColumn("lab_name").Build()
	HostGroupColumn      = queryutils.NewColumn("host_group").Build()
	RunTargetColumn      = queryutils.NewColumn("run_target").Build()
	MinimumRepairsColumn = queryutils.NewColumn("minimum_repairs").Build()
	DevicesOfflineColumn = queryutils.NewColumn("devices_offline").Build()
	TotalDevicesColumn   = queryutils.NewColumn("total_devices").Build()

	// Table
	AndroidRepairMetricsTable = queryutils.NewTableBuilder("android_repair_metrics").WithColumns(
		PriorityColumn,
		LabName,
		HostGroupColumn,
		RunTargetColumn,
		MinimumRepairsColumn,
		DevicesOfflineColumn,
		TotalDevicesColumn,
	).Build()
)
