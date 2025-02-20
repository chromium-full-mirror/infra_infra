// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"

	"golang.org/x/exp/slices"

	"go.chromium.org/luci/common/errors"

	components_servo "go.chromium.org/infra/cros/recovery/internal/components/servo"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	// This represents the number of times we will attempt to collect
	// SBU voltage to calculate the average value.
	sbuVoltageTotalCheckCount = 10
)

// servoCR50LowSBUExec verifies whether SBU voltage is below a
// threshold (2500 mv) blocking enumeration of CR50 component.
//
// This verifier is conditioned on whether the value of servod control
// 'dut_sbu_voltage_float_fault' is on or not.
func servoCR50LowSBUExec(ctx context.Context, info *execs.ExecInfo) error {
	sbuValue, err := MaximalAvgSbuValue(ctx, info.NewServod(), sbuVoltageTotalCheckCount)
	if err != nil {
		return errors.Annotate(err, "servo CR50 low sbu exec").Err()
	}
	log.Debugf(ctx, "Servo CR50 Low Sbu Exec: avg SBU value is %f", sbuValue)
	if sbuValue <= sbuThreshold {
		return errors.Reason("servo CR50 low sbu exec: CR50 not detected due to low SBU voltage").Err()
	}
	return nil
}

// servoCR50EnumeratedExec verifies whether CR50 cannot be enumerated
// despite the voltage being higher than a threshold (2500 mV). This
// can happen when CR50 is in deep sleep.
//
// Please use condition to verify that 'dut_sbu_voltage_float_fault'
// has the value 'on'.
func servoCR50EnumeratedExec(ctx context.Context, info *execs.ExecInfo) error {
	sbuValue, err := MaximalAvgSbuValue(ctx, info.NewServod(), sbuVoltageTotalCheckCount)
	if err != nil {
		return errors.Annotate(err, "servo CR50 enumerated exec").Err()
	}
	log.Debugf(ctx, "Servo CR50 Enumerated Exec: avg SBU value is %f", sbuValue)
	if sbuValue > sbuThreshold {
		return errors.Reason("servo CR50 enumerated exec: CR50 SBU voltage is greater than the threshold").Err()
	}
	return nil
}

// servoCCDExpectedHaveFactoryResetExec verifies is this devices should have CCD open
// and reset to factory settings.
func servoCCDExpectedHaveFactoryResetExec(ctx context.Context, info *execs.ExecInfo) error {
	// TODO(b/300287654): Remove this work around once we have implemented a testlab open procedure
	// that works reliably for devices in faft-cr50 pool
	pools := info.GetDut().ExtraAttributes[tlw.ExtraAttributePools]
	if slices.Contains(pools, "faft-cr50") {
		return errors.Reason("device in faft-cr50 pool not expected to have ccd open (b/300287654)").Err()
	}
	// If device is Ti50 (not cr50). We always want CCD to be open and reset
	err := info.NewServod().Has(ctx, "ti50_version")
	if err == nil {
		log.Debugf(ctx, "Found ti50 device")
		return nil
	}
	// For Cr50 device, we want CCD to be the main servo device for CCD to be open
	sType, err := components_servo.WrappedServoType(ctx, info.NewServod(), info.GetChromeos().GetServo())
	if err != nil {
		return errors.Annotate(err, "servo ccd expect have factory reset").Err()
	}
	if sType.IsMainDeviceGSC() {
		log.Debugf(ctx, "Found main device is cr50")
		return nil
	}
	return errors.Reason("servo ccd expect have factory reset: Not Ti50 and not Cr50 with CCD as main device").Err()
}

func init() {
	execs.Register("servo_cr50_low_sbu", servoCR50LowSBUExec)
	execs.Register("servo_cr50_enumerated", servoCR50EnumeratedExec)
	execs.Register("servo_ccd_expect_have_factory_reset", servoCCDExpectedHaveFactoryResetExec)
}
