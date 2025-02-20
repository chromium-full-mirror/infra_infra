// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package attacheddevice

import (
	"context"
	"fmt"

	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/libs/skylab/inventory/swarming"
	"go.chromium.org/infra/libs/skylab/inventory/swarming/attacheddevice"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// GetAttachedDeviceBotDims gets all Swarming dimensions of an AttachedDevice bot.
func GetAttachedDeviceBotDims(ctx context.Context, r swarming.ReportFunc, ds dutstate.Info, deviceData *ufsAPI.AttachedDeviceData) swarming.Dimensions {
	dims := attacheddevice.Convert(deviceData)
	if v := deviceData.GetMachine().GetLocation(); v != nil {
		location := fmt.Sprintf("%s-aisle%s-row%s-rack%s-racknumber%s-shelf%s-position%s",
			v.GetAisle(), v.GetZone(), v.GetRow(), v.GetRack(), v.GetRackNumber(), v.GetShelf(), v.GetPosition())
		dims["location"] = []string{location}
	}
	dims["dut_state"] = []string{string(ds.State)}
	dims["ufs_zone"] = []string{deviceData.GetLabConfig().GetZone()}
	swarming.Sanitize(dims, r)
	return dims
}
