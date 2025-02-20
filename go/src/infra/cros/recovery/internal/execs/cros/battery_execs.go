// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/execs/cros/battery"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// auditBatteryExec confirms that it is able to audit battery info
// and mark the DUT if it needs replacement.
func auditBatteryExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	batteryInfo := info.GetChromeos().GetBattery()
	if batteryInfo == nil {
		return errors.Reason("audit battery: data is not present in dut info").Err()
	}
	b, err := battery.ReadBatteryInfo(ctx, r)
	if err != nil {
		batteryInfo.State = tlw.HardwareState_HARDWARE_UNSPECIFIED
		return errors.Annotate(err, "audit battery: dut battery state cannot extracted").Err()
	}
	hardwareState := battery.DetermineHardwareStatus(ctx, b.FullChargeCapacity, b.FullChargeCapacityDesigned)
	log.Infof(ctx, "Battery hardware state: %s", hardwareState)
	if hardwareState == tlw.HardwareState_HARDWARE_UNSPECIFIED {
		return errors.Reason("audit battery: dut battery did not detected or state cannot extracted").Err()
	}
	batteryInfo.State = hardwareState
	if hardwareState == tlw.HardwareState_HARDWARE_NEED_REPLACEMENT {
		log.Infof(ctx, "Detected issue with battery on the DUT.")
		log.Debugf(ctx, "Audit Battery Exec: setting dut state to %s", string(dutstate.NeedsReplacement))
		info.GetDut().State = dutstate.NeedsReplacement
		info.GetDut().DutStateReason = tlw.DutStateReasonBatteryCapacityTooLow
		return errors.Reason("audit battery: bad battery deteceted").Err()
	}
	return nil
}

func init() {
	execs.Register("cros_audit_battery", auditBatteryExec)
}
