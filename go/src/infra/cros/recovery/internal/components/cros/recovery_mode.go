// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/components/cros/android"
	"go.chromium.org/infra/cros/recovery/internal/components/cros/storage"
	"go.chromium.org/infra/cros/recovery/internal/components/servo"
	"go.chromium.org/infra/cros/recovery/internal/retry"
	"go.chromium.org/infra/cros/recovery/logger"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// BootInRecoveryRequest holds info to boot device in recovery mode.
type BootInRecoveryRequest struct {
	DUT *tlw.Dut
	// Booting time value to verify when device booted and available for SSH.
	BootRetry    int
	BootTimeout  time.Duration
	BootInterval time.Duration
	// Prevent PD switch to snk before boot.
	PreventPowerSnk bool
	// Call function to cal after device booted in recovery mode.
	Callback       func(context.Context) error
	AddObservation func(*metrics.Observation)
	// Options to ignore errors happened during restoring stage.
	IgnoreServoRestoreFailure bool
	IgnoreRebootFailure       bool
	// After reboot params specified to check if device booted or not.
	AfterRebootVerify             bool
	AfterRebootTimeout            time.Duration
	AfterRebootAllowUseServoReset bool
	// The image on the USB stick is a Light-Provision image used for booting.
	UseLightProvisionImage bool
}

// BootInRecoveryMode perform boot device in recovery mode.
//
// Boot in recovery mode performed by RO firmware and in some cases required stopPD negotiation.
// Please specify callback function to perform needed actions when device booted in recovery mode.
func BootInRecoveryMode(ctx context.Context, req *BootInRecoveryRequest, dutRun, dutBackgroundRun components.Runner, dutPing components.Pinger, ha components.HostAccess, servod components.Servod, log logger.Logger) (rErr error) {
	if req.BootRetry < 1 {
		// We retry at least once when method called.
		req.BootRetry = 1
	}
	// If observation is not provided then we create fake to print to logs
	if req.AddObservation == nil {
		req.AddObservation = func(observation *metrics.Observation) {
			if observation != nil {
				log.Debugf("Observation created kind:%q with %v", observation.MetricKind, observation.Value)
			}
		}
	}
	// Flag specified if we need set PD to `snk` before boot in recovery mode.
	var needSink bool
	if req.PreventPowerSnk {
		log.Infof("Recovery boot will be performed without PD:snk by request.")
		needSink = false
	} else {
		var err error
		needSink, err = RecoveryModeRequiredPDOff(ctx, dutRun, servod, req.DUT)
		if err != nil {
			return errors.Annotate(err, "boot in recovery mode").Err()
		}
	}
	defer func() {
		// Record the label at the end as it can be changed.
		req.AddObservation(metrics.NewStringObservation("need_snk_power", fmt.Sprintf("%v", needSink)))
	}()
	req.AddObservation(metrics.NewStringObservation("need_snk_expected", fmt.Sprintf("%v", needSink)))
	if needSink {
		if batteryLevel, err := servo.BatteryChargePercent(ctx, servod); err != nil {
			req.AddObservation(metrics.NewInt64Observation("battery_level", -1))
			log.Debugf("Fail to read battery level from device %s.", err)
			log.Debugf("We will not set PD to snk mode when boot in recovery mode.")
			needSink = false
		} else {
			req.AddObservation(metrics.NewInt64Observation("battery_level", int64(batteryLevel)))
			// If device has less 30% of battery then we will not try to recover it.
			// If device lost power in middle of install it damage the disk.
			const minBatterLevel = int32(30)
			if batteryLevel < minBatterLevel {
				log.Debugf("Battery level %d%% is lower minimum expectation of %d%%.", batteryLevel, minBatterLevel)
				log.Debugf("We will not set PD to snk mode when boot in recovery mode.")
				needSink = false
			}
		}
	}
	log.Debugf("Servo OS Install Repair: needSink :%t", needSink)
	restoreStates := func() error {
		log.Debugf("Boot in recovery mode: recover servo states...")
		// Turn on the DUT at the end in case it was not.
		// All errors just logging as the action to clean up the state.
		if err := servo.SetPDRole(ctx, servod, servo.PD_ON); err != nil {
			log.Debugf("Restore PD for DUT failed: %s", err)
		}
		// Waiting 10 seconds for USB re-enumerate after PD role switch.
		time.Sleep(10 * time.Second)
		if err := servo.SetPowerState(ctx, servod, servo.PowerStateValueOFF); err != nil {
			log.Debugf("Turn off DUT failed: %s", err)
		}
		if err := servo.UpdateUSBVisibility(ctx, servo.USBVisibleOff, servod); err != nil {
			log.Debugf("Turn off USB drive on servo failed: %s", err)
		}
		// Waiting 10 seconds before turn it on as the device can be still in transition to off.
		time.Sleep(10 * time.Second)
		if err := servo.SetPowerState(ctx, servod, servo.PowerStateValueON); err != nil {
			return errors.Annotate(err, "restore DUT state").Err()
		}
		// Waiting 3 seconds before allowed followeing commands to try something else.
		time.Sleep(3 * time.Second)
		log.Debugf("Boot in recovery mode: DUT booted.")
		return nil
	}
	// Always restore servo state by the end!
	defer func() {
		if err := restoreStates(); err != nil {
			log.Debugf("Boot in recovery mode: %s", err)
			// Don't override the original error.
			if !req.IgnoreServoRestoreFailure && rErr == nil {
				// We cannot return it, so we set it.
				// If we fail when restored the states then we have issues.
				rErr = err
				return
			}
		}
		// Verify the boot only if pass the execution or restore states.
		if rErr == nil && req.AfterRebootVerify {
			log.Debugf("Boot in recovery mode: starting verification of the boot...")
			accessCheck := func() error {
				return retry.WithTimeout(ctx, req.BootInterval, req.AfterRebootTimeout, func() error {
					if err := IsPingable(ctx, DefaultPingCount, dutPing); err != nil {
						return err
					}
					if req.DUT.GetChromeos().GetIsAndroidBased() {
						forceReconnect := false
						if err := android.ADBConnect(ctx, 3, time.Second, forceReconnect, 3*time.Second, req.DUT); err != nil {
							return err
						}
					}
					return IsSSHable(ctx, dutRun, DefaultSSHTimeout)
				}, "wait until accessible")
			}
			for {
				if err := accessCheck(); err != nil {
					if req.AfterRebootAllowUseServoReset {
						req.AfterRebootAllowUseServoReset = false
						if err := servo.SetPowerState(ctx, servod, servo.PowerStateValueReset); err != nil {
							log.Infof("Fail to reset by servo: %s", err)
						}
						continue
					}
					log.Debugf("Device is not accessible after reboot!")
					rErr = err
				} else {
					log.Debugf("Device is accessible!")
				}
				break
			}
		}
	}()
	retryBootFunc := func() error {
		// On Android everything tries to use ADB, so switch to Chrome OS to be able to use SSH access.
		cacheIsAndroid := req.DUT.GetChromeos().GetIsAndroidBased()
		req.DUT.GetChromeos().IsAndroidBased = false
		defer func() {
			req.DUT.GetChromeos().IsAndroidBased = cacheIsAndroid
		}()

		log.Infof("Boot in Recovery Mode: starting retry...")
		// Turn power off.
		if err := servo.SetPowerState(ctx, servod, servo.PowerStateValueOFF); err != nil {
			return errors.Annotate(err, "retry boot").Err()
		}
		// Next:Boot in recovery mode. The steps are:
		// Step 1. Switch the USB to DUT on the servo multiplexer
		if err := servo.UpdateUSBVisibility(ctx, servo.USBVisibleDUT, servod); err != nil {
			return errors.Annotate(err, "retry boot").Err()
		}
		// Step 2. For servo V4, switch power delivery to sink mode. c.f.:
		// crbug.com/1129165.
		if needSink {
			if err := servo.SetPDRole(ctx, servod, servo.PD_OFF); err != nil {
				return errors.Annotate(err, "retry boot").Err()
			}
		}
		// If c2d2, then set cold_reset_select:gsc_ec_reset
		sType, err := servo.WrappedServoType(ctx, servod, req.DUT.GetChromeos().GetServo())
		if err != nil {
			log.Infof("Check for C2D2: could not determine the servo type.")
			return errors.Annotate(err, "check for C2D2").Err()
		}
		if sType.IsC2D2() {
			if err := servod.Set(ctx, "cold_reset_select", "gsc_ec_reset"); err != nil {
				return errors.Annotate(err, "set cold_reset_select").Err()
			}
		}
		// Sleep a few seconds to allowed apply all previous states before boot in recovery mode.
		time.Sleep(1 * time.Second)
		log.Infof("Boot in Recovery Mode: Started try to boot in recovery mode by power_state:rec.")
		if err := servo.SetPowerState(ctx, servod, servo.PowerStateValueRecoveryMode); err != nil {
			log.Debugf("Boot in Recovery Mode: Failure when trying to set power_state:rec with error: %s", err)
		}
		log.Debugf("Boot in Recovery Mode: Waiting to device to be SSH-able.")
		if err := WaitUntilAccessible(ctx, req.BootTimeout, req.BootInterval, dutRun, dutPing); err != nil {
			return errors.Annotate(err, "retry boot").Err()
		}
		if cacheIsAndroid {
			// With Android we can SSH only to the provision image.
			// The provision image is very limited and does not have many tools.
		} else if req.UseLightProvisionImage {
			if _, err := dutRun(ctx, 10*time.Second, "which al-install"); err != nil {
				return errors.Annotate(err, "retry boot: device is not booted from light-provision image").Err()
			}
		} else {
			if err := storage.IsBootedFromExternalStorage(ctx, dutRun); err != nil {
				log.Infof("Device booted from internal storage.")
				return errors.Annotate(err, "retry boot").Err()
			}
			// List information about block devices.
			// This informcation helps to understand which devices present and visible on the DUT.
			if _, err := dutRun(ctx, 10*time.Second, "lsblk"); err != nil {
				log.Infof("Fail to list device of the DUT: %s", err)
			}
		}
		log.Infof("Device successfully booted in recovery mode from USB-drive.")
		return nil
	}
	if retryErr := retry.LimitCount(ctx, req.BootRetry, req.BootInterval, retryBootFunc, "boot in recovery mode"); retryErr != nil {
		return errors.Annotate(retryErr, "boot in recovery mode").Err()
	}
	if req.Callback != nil {
		log.Infof("Boot in recovery mode: passing control to call back.")
		if err := req.Callback(ctx); err != nil {
			return errors.Annotate(err, "boot in recovery mode: callback").Err()
		}
		log.Infof("Boot in recovery mode: control returned.")
	}
	return nil
}
