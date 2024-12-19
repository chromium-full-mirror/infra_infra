// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"

	"infra/fleetconsole/api/fleetconsolerpc"
)

func (frontend *FleetConsoleFrontend) CountDevices(ctx context.Context, req *fleetconsolerpc.CountDevicesRequest) (*fleetconsolerpc.CountDevicesResponse, error) {
	// For now I am mocking the data, in a future cl we will grab this data
	// with a call to UFS
	return &fleetconsolerpc.CountDevicesResponse{
		Total: 440,
		TaskState: &fleetconsolerpc.TaskStateCounts{
			Busy: 401,
			Idle: 40,
		},
		DeviceState: &fleetconsolerpc.DeviceStateCounts{
			Ready:            40,
			NeedManualRepair: 40,
			NeedRepair:       40,
			RepairFailed:     40,
		},
	}, nil

}
