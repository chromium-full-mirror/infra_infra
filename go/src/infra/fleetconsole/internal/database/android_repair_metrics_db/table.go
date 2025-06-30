// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidrepairmetricsdb

import (
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

var (
	// Columns
	Id                   = queryutils.NewColumn("id").Build()
	PriorityColumn       = queryutils.NewColumn("priority").Build()
	LabName              = queryutils.NewColumn("lab_name").Build()
	HostGroupColumn      = queryutils.NewColumn("host_group").Build()
	RunTargetColumn      = queryutils.NewColumn("run_target").Build()
	MinimumRepairsColumn = queryutils.NewColumn("minimum_repairs").Build()
	DevicesOfflineColumn = queryutils.NewColumn("devices_offline").Build()
	TotalDevicesColumn   = queryutils.NewColumn("total_devices").Build()
	Hostname             = queryutils.NewColumn("hostname").Build()
	State                = queryutils.NewColumn("state").Build()

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

	// AndroidDevicesTable
	AndroidDevicesTable = queryutils.NewTableBuilder("android_devices").WithColumns(
		Id,
		LabName,
		HostGroupColumn,
		RunTargetColumn,
		State,
	).Build()

	// AndroidHostsTable
	AndroidHostsTable = queryutils.NewTableBuilder("android_hosts").WithColumns(
		Hostname,
		HostGroupColumn,
		State,
	).Build()
)
