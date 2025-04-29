// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/servo"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/execs/servo/topology"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// servoPowerCycleRootServoExec resets(power-cycle) the servo via smart usbhub.
func servoPowerCycleRootServoExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	// Timeout for resetting the servo. Default to be 30s.
	resetTimeout := argsMap.AsDuration(ctx, "reset_timeout", 30, time.Second)
	// Timeout to wait after resetting the servo. Default to be 20s.
	waitTimeout := argsMap.AsDuration(ctx, "wait_timeout", 20, time.Second)
	resetAuthorizedFlag := argsMap.AsBool(ctx, "reset_authorized", false)
	run := info.DefaultRunner()
	servoInfo := info.GetChromeos().GetServo()
	var smartUsbhubPresent = false
	defer func() {
		servoInfo.SmartUsbhubPresent = smartUsbhubPresent
	}()
	servoSerial := servoInfo.SerialNumber
	// Get the usb devnum before the reset.
	preResetDevnum, err := topology.GetServoUsbDevnum(ctx, run, servoSerial)
	if err != nil {
		return errors.Annotate(err, "servo power cycle root servo: find the servo").Err()
	}
	log.Infof(ctx, "Servo usb devnum before reset: %s", preResetDevnum)
	// Resetting servo. Call powercycle-servo-usbhub-port
	log.Infof(ctx, "Try to reset servo through usbhub.")
	if _, err = run(ctx, resetTimeout, "powercycle-servo-usbhub-port --servo_serial ", servoSerial, " --downtime_sec 10"); err != nil {
		log.Warningf(ctx, `Failed to reset servo with serial: %s. Please ignore this error if the DUT is not connected to a supported USB hub`, servoSerial)
		return errors.Annotate(err, "servo power cycle root servo").Err()
	}

	// Since we are able to run the power cycle servodtool command
	// It implies the smartUsb is present.
	smartUsbhubPresent = true
	log.Debugf(ctx, "Wait %v for servo to come back from reset.", waitTimeout)
	time.Sleep(waitTimeout)
	if resetAuthorizedFlag {
		// Reset authorized flag fror servo-hub for servo v4p1 only.
		if err := ResetUsbkeyAuthorized(ctx, run, servoSerial, info.GetChromeos().GetServo().ServodType); err != nil {
			return errors.Annotate(err, "servo power cycle root servo").Err()
		}
	}
	// Get the usb devnum after the reset.
	postResetDevnum, err := topology.GetServoUsbDevnum(ctx, run, servoSerial)
	if err != nil {
		return errors.Annotate(err, "servo power cycle root servo: after rest").Err()
	}
	log.Infof(ctx, "Servo usb devnum after reset: %s", postResetDevnum)
	if preResetDevnum == "" || postResetDevnum == "" {
		log.Infof(ctx, "Servo reset completed but unable to verify devnum change!")
	} else if preResetDevnum != postResetDevnum {
		log.Infof(ctx, "Reset servo with serial %s completed successfully!", servoSerial)
	} else {
		log.Infof(ctx, "Servo reset completed but devnum is still not changed!")
	}
	return nil
}

// servoV4P1NetResetExec reset servo_v4p1 network controller.
func servoV4P1NetResetExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	// Timeout between off/on reset steps.
	resetTimeout := argsMap.AsDuration(ctx, "reset_timeout", 1, time.Second)
	servod := info.NewServod()
	logger := info.NewLogger()
	err := servo.ResetServoV4p1EthernetController(ctx, servod, logger, resetTimeout)
	return errors.Annotate(err, "servo_v4p1 net reset").Err()
}

// servoRebootExec use servodtool to reboot servo device.
func servoRebootExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	rebootTimeout := argsMap.AsDuration(ctx, "reboot_timeout", 30, time.Second)
	waitTimeout := argsMap.AsDuration(ctx, "wait_timeout", 30, time.Second)
	run := info.DefaultRunner()
	servoInfo := info.GetChromeos().GetServo()
	servoSerial := servoInfo.SerialNumber
	if _, err := run(ctx, rebootTimeout, "servodtool", "device", "-s", servoSerial, "reboot"); err != nil {
		log.Warningf(ctx, "Failed to reboot servo with serial: %s.", servoSerial)
		return errors.Annotate(err, "servo reboot").Err()
	}
	log.Debugf(ctx, "Wait %v for servo to re-enumerate after reboot.", waitTimeout)
	time.Sleep(waitTimeout)
	return nil
}

func init() {
	execs.Register("servo_power_cycle_root_servo", servoPowerCycleRootServoExec)
	execs.Register("servo_v4p1_network_reset", servoV4P1NetResetExec)
	execs.Register("servo_reboot", servoRebootExec)
}
