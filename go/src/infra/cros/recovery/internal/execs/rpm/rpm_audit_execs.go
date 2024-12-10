// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rpm

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components/cros"
	"infra/cros/recovery/internal/components/cros/rpm"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

// rpmAuditWithoutBatteryExec verifies that the RPM configuration is correct.
//
// The function targets devices that do not have a battery.
// If servo present then we will try to set `servo_pd_role:src` after set power on.
func rpmAuditWithoutBatteryExec(ctx context.Context, info *execs.ExecInfo) error {
	outlet := info.GetChromeos().GetRpmOutlet()
	if outlet == nil {
		return errors.Reason("rpm audit: outlet not present").Err()
	}
	argsMap := info.GetActionArgs(ctx)
	downTimeout := argsMap.AsDuration(ctx, "down_timeout", 120, time.Second)
	bootTimeout := argsMap.AsDuration(ctx, "boot_timeout", 150, time.Second)
	waitInterval := argsMap.AsDuration(ctx, "wait_interval", 5, time.Second)
	ping := info.DefaultPinger()
	run := info.DefaultRunner()

	// Set the state to wrong initially.
	// If everything is working as expected, it will be updated to working.
	outlet.State = tlw.RPMOutlet_WRONG_CONFIG

	log.Debugf(ctx, "Set RPM off ...")
	if err := rpmPowerOffExec(ctx, info); err != nil {
		return errors.Annotate(err, "rpm audit").Err()
	}
	// RPM service is single thread, perform the action OFF can take up-to 60 seconds.
	log.Debugf(ctx, "Start waiting until the device goes down...")
	if waitDownErr := cros.WaitUntilNotPingable(ctx, downTimeout, waitInterval, 2, ping); waitDownErr != nil {
		log.Debugf(ctx, "Failed to power down the host: restoring RPM to ON state.")
		if err := rpmPowerOnExec(ctx, info); err != nil {
			log.Debugf(ctx, "Failed to recover RPM state to ON: %s", err)
		} else {
			log.Debugf(ctx, "Successfully recovered RPM state to ON.")
		}
		return errors.Annotate(waitDownErr, "rpm audit: resource still pingable").Err()
	}
	log.Debugf(ctx, "Set RPM on ...")
	if err := rpmPowerOnExec(ctx, info); err != nil {
		return errors.Annotate(err, "rpm audit").Err()
	}
	if s := info.GetChromeos().GetServo(); s != nil {
		log.Debugf(ctx, "Servo detected. try to set `servo_pd_role` to `src` state.")
		if err := info.NewServod().Set(ctx, "servo_pd_role", "src"); err != nil {
			log.Debugf(ctx, "Fail to set `servo_pd_role:src` due error: %s", err)
		}
	}
	log.Debugf(ctx, "Start waiting until the device goes up...")
	if err := cros.WaitUntilSSHable(ctx, bootTimeout, waitInterval, run); err != nil {
		return errors.Annotate(err, "rpm audit: resource did not booted").Err()
	}
	log.Debugf(ctx, "Verification finished.")
	outlet.State = tlw.RPMOutlet_WORKING
	return nil
}

// rpmAuditWithBatteryExec verifies that the RPM configuration is correct.
//
// The function targets devices that use batteries.
// If servo present then we will try to set `servo_pd_role:src` after set power on.
func rpmAuditWithBatteryExec(ctx context.Context, info *execs.ExecInfo) error {
	outlet := info.GetChromeos().GetRpmOutlet()
	if outlet == nil {
		return errors.Reason("rpm audit: outlet not present").Err()
	}
	run := info.DefaultRunner()
	ping := info.NewPinger(info.GetDut().Name)
	argsMap := info.GetActionArgs(ctx)
	waitTimeout := argsMap.AsDuration(ctx, "timeout", 150, time.Second)
	waitInterval := argsMap.AsDuration(ctx, "wait_interval", 5, time.Second)

	// Set the state to wrong initially.
	// If everything is working as expected, it will be updated to working.
	outlet.State = tlw.RPMOutlet_WRONG_CONFIG

	log.Debugf(ctx, "Set RPM off ...")
	if err := rpmPowerOffExec(ctx, info); err != nil {
		return errors.Annotate(err, "rpm audit").Err()
	}
	log.Debugf(ctx, "Start waiting until the device stops detecting power...")
	if err := rpm.ValidatePowerState(ctx, run, ping, false, waitTimeout, waitInterval); err != nil {
		log.Debugf(ctx, "Failed: device still detecting power: restoring RPM to ON state.")
		if err := rpmPowerOnExec(ctx, info); err != nil {
			log.Debugf(ctx, "Failed to recover RPM state to ON: %s", err)
		} else {
			log.Debugf(ctx, "Successfully recovered RPM state to ON.")
		}
		return errors.Annotate(err, "rpm audit with battery").Err()
	}
	log.Debugf(ctx, "Set RPM on ...")
	if err := rpmPowerOnExec(ctx, info); err != nil {
		return errors.Annotate(err, "rpm audit").Err()
	}
	if s := info.GetChromeos().GetServo(); s != nil {
		log.Debugf(ctx, "Servo detected. try to set `servo_pd_role` to `src` state.")
		if err := info.NewServod().Set(ctx, "servo_pd_role", "src"); err != nil {
			log.Debugf(ctx, "Fail to set `servo_pd_role:src` due error: %s", err)
		}
	}
	log.Debugf(ctx, "Start waiting until the device detects power...")
	if err := rpm.ValidatePowerState(ctx, run, ping, true, waitTimeout, waitInterval); err != nil {
		return errors.Annotate(err, "rpm audit with battery").Err()
	}
	log.Debugf(ctx, "Verification finished.")
	outlet.State = tlw.RPMOutlet_WORKING
	return nil
}

func init() {
	execs.Register("rpm_audit_without_battery", rpmAuditWithoutBatteryExec)
	execs.Register("rpm_audit_with_battery", rpmAuditWithBatteryExec)
}
