// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidrepairmetricsdb

import (
	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

var (
	// Columns
	Id             = queryutils.NewColumn("id").Build()
	Priority       = queryutils.NewColumn("priority").Build()
	LabName        = queryutils.NewColumn("lab_name").Build()
	HostGroup      = queryutils.NewColumn("host_group").Build()
	RunTarget      = queryutils.NewColumn("run_target").Build()
	MinimumRepairs = queryutils.NewColumn("minimum_repairs").Build()
	DevicesOffline = queryutils.NewColumn("devices_offline").Build()
	TotalDevices   = queryutils.NewColumn("total_devices").Build()
	Hostname       = queryutils.NewColumn("hostname").Build()
	State          = queryutils.NewColumn("state").Build()

	// Table
	AndroidRepairMetricsTable = queryutils.NewTableBuilder("android_repair_metrics").WithColumns(
		Priority,
		LabName,
		HostGroup,
		RunTarget,
		MinimumRepairs,
		DevicesOffline,
		TotalDevices,
	).Build()

	AndroidDevicesTable = queryutils.NewTableBuilder("android_devices").WithColumns(
		Id,
		LabName,
		HostGroup,
		RunTarget,
		State,
	).Build()

	// AndroidHostsTable
	AndroidHostsTable = queryutils.NewTableBuilder("android_hosts").WithColumns(
		Hostname,
		HostGroup,
		State,
	).Build()
)
