// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// vidPidGSCChipMap maps the VID:PID to the GSC chip type.
var vidPidGSCChipMap = map[string]tlw.ChromeOS_GscChip{
	"18d1:5014": tlw.ChromeOS_GSC_CHIP_H1,
	"18d1:504a": tlw.ChromeOS_GSC_CHIP_DT,
	"18d1:5066": tlw.ChromeOS_GSC_CHIP_NT,
}

// crosReadGSCChipExec reads the GSC chip from the servo topology and updates the inventory.
func crosReadGSCChipExec(ctx context.Context, info *execs.ExecInfo) error {
	servo := info.GetChromeos().GetServo()
	if servo == nil || servo.GetSerialNumber() == "" {
		return errors.New("cros read gsc chip: servo not found")
	}
	for _, device := range servo.GetServoTopology().GetChildren() {
		if gscChip, ok := vidPidGSCChipMap[device.GetVidPid()]; ok {
			log.Infof(ctx, "Found GSC chip %q with VID:PID %q", gscChip, device.GetVidPid())
			info.GetChromeos().GscChip = gscChip
			return nil
		}
	}
	return errors.New("cros read gsc chip: no GSC chip found")
}

func init() {
	execs.Register("cros_read_gsc_chip", crosReadGSCChipExec)
}
