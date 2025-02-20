// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"fmt"

	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/libs/skylab/inventory/swarming"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufslab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
)

// GetDUTBotDims gets all Swarming dimensions of a DUT bot.
func GetDUTBotDims(ctx context.Context, r swarming.ReportFunc, ds dutstate.Info, deviceData *ufspb.ChromeOSDeviceData) swarming.Dimensions {
	c := deviceData.GetDutV1().GetCommon()
	dims := swarming.Convert(c.GetLabels())
	dims["dut_id"] = []string{c.GetId()}
	dims["dut_name"] = []string{c.GetHostname()}
	dims["label-multiduts"] = []string{"False"}
	if v := c.GetHwid(); v != "" {
		dims["hwid"] = []string{v}
	}
	if v := c.GetSerialNumber(); v != "" {
		dims["serial_number"] = []string{v}
	}
	if v := c.GetLocation(); v != nil {
		location := fmt.Sprintf("%s-row%d-rack%d-host%d",
			v.GetLab().GetName(), v.GetRow(), v.GetRack(), v.GetHost())
		dims["location"] = []string{location}
	}
	dims["dut_state"] = []string{string(ds.State)}
	dims["ufs_zone"] = []string{deviceData.GetLabConfig().GetZone()}
	processVersionInfo(dims, deviceData.GetDutState().GetVersionInfo())

	lc := deviceData.GetLabConfig()
	dut := lc.GetChromeosMachineLse().GetDeviceLse().GetDut()
	p := dut.GetPeripherals()
	if p.GetDolos() != nil {
		dims["dolos_state"] = []string{deviceData.GetDutState().GetDolosState().String()}
	}

	// Only expose the label when there is a valid value for logical zone.
	if lc.GetLogicalZone() != ufspb.LogicalZone_LOGICAL_ZONE_UNSPECIFIED {
		dims["logical_zone"] = []string{lc.GetLogicalZone().String()}
	}
	swarming.Sanitize(dims, r)
	return dims
}

func processVersionInfo(dims swarming.Dimensions, v *ufslab.VersionInfo) {
	if v == nil || v.GetOsType() == ufslab.VersionInfo_UNKNOWN {
		return
	}
	dims["version_info_os_type"] = []string{v.GetOsType().String()}
	if v.GetOs() != "" {
		dims["version_info_os"] = []string{v.GetOs()}
	}
	if v.GetRwFirmware() != "" {
		dims["version_info_rw_firmware"] = []string{v.GetRwFirmware()}
	}
	if v.GetRoFirmware() != "" {
		dims["version_info_ro_firmware"] = []string{v.GetRoFirmware()}
	}
}
