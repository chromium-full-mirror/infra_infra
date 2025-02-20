// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/components/cros/power"
	"go.chromium.org/infra/cros/recovery/internal/components/servo"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	// The percentage of the battery that is considered to be not
	// enough.
	MinimumBatteryLevel = 70
)

// RecoveryModeRequiredPDOff examines whether servo_pd_role has to be set to
// `snk` before booting into recovery mode.
func RecoveryModeRequiredPDOff(ctx context.Context, run components.Runner, servod components.Servod, dut *tlw.Dut) (bool, error) {
	expectBattery := dut.GetChromeos().GetBattery() != nil
	if !expectBattery {
		log.Debugf(ctx, "DUT is not expected to have the battery, so recovery mode does not required PD:snk!")
		return false, nil
	}
	// Verify that device is SSH-able to avoid an unnecessary attempt to read power-info.
	if sshErr := IsSSHable(ctx, run, DefaultSSHTimeout); sshErr != nil {
		log.Debugf(ctx, "Skipping check battery present on the devices as DUT is not SSH-able!")
	} else if p, err := power.ReadPowerInfo(ctx, run); err == nil {
		expectBattery, _ = p.HasBattery()
	}
	if !expectBattery {
		log.Debugf(ctx, "The DUT does not have a battery, so recovery mode does not required PD:snk!")
		return false, nil
	}
	if sType, err := servo.WrappedServoType(ctx, servod, dut.GetChromeos().GetServo()); err != nil {
		return false, errors.Annotate(err, "require sink mode in recovery").Err()
	} else if sType.IsMicro() {
		log.Debugf(ctx, "DUT uses servo_micro, so recovery mode does not required PD:snk!")
		return false, nil
	}
	if pdControlSupported, err := servo.ServoSupportsBuiltInPDControl(ctx, servod); err != nil {
		log.Debugf(ctx, "Servo does not support PD, so recovery mode does not required PD:snk!")
		return false, errors.Annotate(err, "require sink mode in recovery").Err()
	} else if !pdControlSupported {
		log.Debugf(ctx, "Require Sink Mode in Recovery: power delivery is no tsupported on this servo, snk mode is not needed for recovery.")
		return false, nil
	}
	return true, nil
}

// Uptime returns uptime of resource.
func Uptime(ctx context.Context, run components.Runner) (*time.Duration, error) {
	// Received value represent two parts where the first value represents the total number
	// of seconds the system has been up and the second value is the sum of how much time
	// each core has spent idle, in seconds. We are looking
	//  E.g.: 683503.88 1003324.85
	// Consequently, the second value may be greater than the overall system uptime on systems with multiple cores.
	out, err := run(ctx, time.Minute, "cat /proc/uptime")
	if err != nil {
		return nil, errors.Annotate(err, "uptime").Err()
	}
	log.Debugf(ctx, "Uptime value read: %q.", out)
	dur, err := ProcessUptime(out)
	if err != nil {
		return nil, errors.Annotate(err, "get uptime").Err()
	}
	return dur, nil
}

func ProcessUptime(uptimeVal string) (*time.Duration, error) {
	// uptimePattern is a decimal number, possibly containing a decimal point.
	uptimePattern := regexp.MustCompile(`^\s*(\d+\.?\d*)\s+(\d+\.?\d*)\s*$`)
	parts := uptimePattern.FindStringSubmatch(uptimeVal)
	if len(parts) != 3 {
		// 'parts' consists of the complete match, as well as the two
		// captured groups for the two numerical quantities returned
		// upon reading /proc/uptime. Hence, in a correct read, the
		// array should have exactly three elements.
		return nil, errors.Reason("process uptime: fail to read value from %s", uptimeVal).Err()
	}
	dur, err := time.ParseDuration(fmt.Sprintf("%ss", parts[1]))
	if err != nil {
		return nil, errors.Annotate(err, "process uptime").Err()
	}
	return &dur, nil
}
