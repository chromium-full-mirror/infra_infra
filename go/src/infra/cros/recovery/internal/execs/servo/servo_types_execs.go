// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"regexp"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/servo"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// servoVerifyV3Exec verifies whether the servo attached to the servo host is servo_v3.
func servoVerifyV3Exec(ctx context.Context, info *execs.ExecInfo) error {
	// The "-servo" suffix will exist only when the setup is for type V3.
	args := info.GetActionArgs(ctx)
	reverse := args.AsBool(ctx, "reverse", false)
	if strings.HasSuffix(info.GetChromeos().GetServo().GetName(), "-servo") {
		// That is servo_v3.
		if reverse {
			return errors.Reason("servo verify v3: that is servo_v3 based on host name").Err()
		}
		return nil
	}
	// That is not servo_v3.
	if reverse {
		return nil
	}
	return errors.Reason("servo verify v3: that is not servo_v3 based on host name").Err()
}

// servoVerifyV4Exec verifies whether the servo attached to the servo
// host is of type V4 or V4p1.
func servoVerifyV4Exec(ctx context.Context, info *execs.ExecInfo) error {
	sType, err := servo.WrappedServoType(ctx, info.NewServod(), info.GetChromeos().GetServo())
	if err != nil {
		log.Debugf(ctx, "Servo Verify V4: could not determine the servo type")
		return errors.Annotate(err, "servo verify v4").Err()
	}
	if !sType.IsV4() {
		log.Debugf(ctx, "Servo Verify V4: servo type is neither V4, or V4P1.")
		return errors.Reason("servo verify v4: servo type %q is not V4.", sType).Err()
	}
	return nil
}

// servoVerifyV4Exec verifies whether the servo attached to the servo
// host is of type V4p1.
func servoVerifyV4p1Exec(ctx context.Context, info *execs.ExecInfo) error {
	sType, err := servo.WrappedServoType(ctx, info.NewServod(), info.GetChromeos().GetServo())
	if err != nil {
		log.Debugf(ctx, "Servo Verify V4p1: could not determine the servo type")
		return errors.Annotate(err, "servo verify v4p1").Err()
	}
	if !sType.IsV4() {
		log.Debugf(ctx, "Servo Verify V4: servo type is not V4P1.")
		return errors.Reason("servo verify v4p1: servo type %q is not V4p1.", sType).Err()
	}
	return nil
}

// servoVerifyV4p1BySerialNumberExec verifies whether the servo attached to the servo
// host is of type v4p1 based on its serial number.
func servoVerifyV4p1BySerialNumberExec(ctx context.Context, info *execs.ExecInfo) error {
	const servoV4p1SerialPrefix = "SERVOV4P1"
	args := info.GetActionArgs(ctx)
	reverse := args.AsBool(ctx, "reverse", false)
	sn := info.GetChromeos().GetServo().GetSerialNumber()
	isServoV4p1 := strings.HasPrefix(sn, servoV4p1SerialPrefix)
	if isServoV4p1 && reverse {
		return errors.Reason("servo verify v4p1 by serial number (reverse): the serial number have expected prefix %s.", servoV4p1SerialPrefix).Err()
	}
	if !reverse && !isServoV4p1 {
		return errors.Reason("servo verify v4p1 by serial number: the serial number %s does not have expected prefix %s.", sn, servoV4p1SerialPrefix).Err()
	}
	return nil
}

// servoVerifyServoMicroExec verifies whether the servo attached to
// the servo host is of type servo micro.
func servoVerifyServoMicroExec(ctx context.Context, info *execs.ExecInfo) error {
	args := info.GetActionArgs(ctx)
	reverse := args.AsBool(ctx, "reverse", false)
	sType, err := servo.WrappedServoType(ctx, info.NewServod(), info.GetChromeos().GetServo())
	if err != nil {
		log.Debugf(ctx, "Servo Verify Servo Micro: could not determine the servo type")
		return errors.Annotate(err, "servo verify servo micro").Err()
	}
	if sType.IsMicro() {
		if reverse {
			return errors.Reason("servo verify servo micro: servo type is servo micro").Err()
		} else {
			log.Debugf(ctx, "Servo Verify: servo type is servo micro!")
			return nil
		}
	} else if reverse {
		log.Debugf(ctx, "Servo Verify: servo type is not servo micro!")
		return nil
	}
	return errors.Reason("servo verify servo micro: servo type %q is not servo micro", sType).Err()
}

// servoVerifyC2D2Exec verifies whether the servo attached to the servo host is C2D2.
func servoVerifyC2D2Exec(ctx context.Context, info *execs.ExecInfo) error {
	args := info.GetActionArgs(ctx)
	reverse := args.AsBool(ctx, "reverse", false)
	sType, err := servo.WrappedServoType(ctx, info.NewServod(), info.GetChromeos().GetServo())
	if err != nil {
		log.Debugf(ctx, "Servo Verify C2D2: could not determine the servo type.")
		return errors.Annotate(err, "servo verify C2D2").Err()
	}
	if sType.IsC2D2() {
		if reverse {
			return errors.Reason("servo verify C2D2: servo type is C2D2").Err()
		} else {
			log.Debugf(ctx, "Servo Verify: servo type is C2D2!")
			return nil
		}
	} else if reverse {
		log.Debugf(ctx, "Servo Verify: servo type is not C2D2!")
		return nil
	}
	return errors.Reason("servo verify servo micro: servo type %q is not C2D2", sType).Err()
}

// servoIsDualSetupConfiguredExec checks whether the servo device has
// been setup in dual mode.
func servoIsDualSetupConfiguredExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetDut() != nil && info.GetDut().ExtraAttributes != nil {
		if attrs, ok := info.GetDut().ExtraAttributes[tlw.ExtraAttributeServoSetup]; ok {
			for _, a := range attrs {
				if a == tlw.ExtraAttributeServoSetupDual {
					log.Debugf(ctx, "Servo Is Dual Setup Configured: servo device is configured to be in dual-setup mode.")
					return nil
				}
			}
		}
	}
	return errors.Reason("servo is dual setup configured: servo device is not configured to be in dual-setup mode").Err()
}

// servoVerifyDualSetupExec verifies whether the servo attached to the
// servo host actually exhibits dual setup.
func servoVerifyDualSetupExec(ctx context.Context, info *execs.ExecInfo) error {
	sType, err := servo.WrappedServoType(ctx, info.NewServod(), info.GetChromeos().GetServo())
	if err != nil {
		return errors.Annotate(err, "servo verify dual setup").Err()
	}
	if !sType.IsDualSetup() {
		return errors.Reason("servo verify dual setup: servo type %q is not dual setup.", sType).Err()
	}
	return nil
}

// servoVerifyServoCCDExec verifies whether the servo attached to
// the servo host is of type servo ccd.
func servoVerifyServoCCDExec(ctx context.Context, info *execs.ExecInfo) error {
	actionMap := info.GetActionArgs(ctx)
	if actionMap.AsBool(ctx, "check_info", true) {
		if st := info.GetChromeos().GetServo().GetServodType(); st != "" {
			if servo.NewServoType(st).IsCCD() {
				log.Debugf(ctx, "Servo Verify servo CCD: established from DUT info: %q", st)
				return nil
			}
		}
	}
	if actionMap.AsBool(ctx, "read_servod", true) {
		if sType, err := servo.WrappedServoType(ctx, info.NewServod(), info.GetChromeos().GetServo()); err != nil {
			return errors.Annotate(err, "servo verify servo ccd").Err()
		} else if sType.IsCCD() {
			log.Debugf(ctx, "Servo Verify servo CCD: established from servod response: %q", sType.String())
			return nil
		}
	}
	return errors.Reason("servo verify servo ccd: does not match expectations").Err()
}

// mainDeviceIsGSCExec checks whether or not the servo device is CR50 or TI50.
func mainDeviceIsGSCExec(ctx context.Context, info *execs.ExecInfo) error {
	sType, err := servo.WrappedServoType(ctx, info.NewServod(), info.GetChromeos().GetServo())
	if err != nil {
		return errors.Annotate(err, "main devices is gsc").Err()
	}
	md := sType.MainDevice()
	if sType.IsMainDeviceGSC() {
		log.Debugf(ctx, "Found main device: %q", md)
		return nil
	}
	return errors.Reason("main devices is gsc: found %q does not match expectations", md).Err()
}

// mainDeviceIsCCDExec checks whether or not the servo device is CCD.
func mainDeviceIsCCDExec(ctx context.Context, info *execs.ExecInfo) error {
	actionMap := info.GetActionArgs(ctx)
	if actionMap.AsBool(ctx, "check_info", true) {
		if st := info.GetChromeos().GetServo().GetServodType(); st != "" {
			if servo.NewServoType(st).IsMainDeviceCCD() {
				log.Debugf(ctx, "Main CCD device established from DUT info: %q", st)
				return nil
			}
		}
	}
	if actionMap.AsBool(ctx, "read_servod", true) {
		if sType, err := servo.WrappedServoType(ctx, info.NewServod(), info.GetChromeos().GetServo()); err != nil {
			return errors.Annotate(err, "main devices is ccd").Err()
		} else if sType.IsMainDeviceCCD() {
			log.Debugf(ctx, "Main CCD device established from servod response: %q", sType.String())
			return nil
		}
	}
	return errors.Reason("main devices is ccd: does not match expectations").Err()
}

// servoTypeRegexMatchExec checks if servo_type match to provided regex.
func servoTypeRegexMatchExec(ctx context.Context, info *execs.ExecInfo) error {
	actionMap := info.GetActionArgs(ctx)
	regex := actionMap.AsString(ctx, "regex", "")
	if regex == "" {
		return errors.Reason("servo-type regex match: regex is empty").Err()
	}
	regexMatch := func(servoType string) error {
		m, err := regexp.MatchString(regex, servoType)
		if err != nil {
			return errors.Annotate(err, "regex match").Err()
		}
		if !m {
			return errors.Reason("regex match: not match").Err()
		}
		return nil
	}
	if actionMap.AsBool(ctx, "check_info", true) {
		if st := info.GetChromeos().GetServo().GetServodType(); st != "" {
			if err := regexMatch(st); err == nil {
				log.Debugf(ctx, "Servo type matches by %q inventory data", regex)
				return nil
			} else {
				log.Debugf(ctx, "Matching servo type from inventory failed: %q", err)
			}
		}
	}
	if actionMap.AsBool(ctx, "read_servod", true) {
		if sType, err := servo.WrappedServoType(ctx, info.NewServod(), info.GetChromeos().GetServo()); err != nil {
			return errors.Annotate(err, "servo verify servo ccd").Err()
		} else {
			if err := regexMatch(sType.String()); err == nil {
				log.Debugf(ctx, "Servo type matches by %q servod data", regex)
				return nil
			} else {
				log.Debugf(ctx, "Matching servo type from inventory failed: %q", err)
			}
		}
	}
	return errors.Reason("servo-type regex match: not match").Err()
}

// servoHasDebugHeaderExec checks if any of servo component is not ccd.
func servoHasDebugHeaderExec(ctx context.Context, info *execs.ExecInfo) error {
	actionMap := info.GetActionArgs(ctx)
	hasTargetComponent := func(components []string) bool {
		for _, c := range components {
			if strings.HasPrefix(c, "ccd_") {
				continue
			}
			log.Debugf(ctx, "Found debug header servo: %q", c)
			return true
		}
		return false
	}
	if actionMap.AsBool(ctx, "check_info", true) {
		if st := info.GetChromeos().GetServo().GetServodType(); st != "" {
			components := servo.NewServoType(st).ExtractComponents(true)
			if len(components) > 0 && hasTargetComponent(components) {
				log.Debugf(ctx, "Servo has debug header component: found header child: %q", st)
				return nil
			}
		}
	}
	if actionMap.AsBool(ctx, "read_servod", true) {
		if sType, err := servo.WrappedServoType(ctx, info.NewServod(), info.GetChromeos().GetServo()); err != nil {
			return errors.Annotate(err, "servo verify servo ccd").Err()
		} else {
			components := sType.ExtractComponents(true)
			if len(components) > 0 && hasTargetComponent(components) {
				log.Debugf(ctx, "Servo has debug header component: found header child: %q", sType.String())
				return nil
			}
		}
	}
	return errors.Reason("servo has debug header servo: child not found").Err()
}

func init() {
	execs.Register("is_servo_v3", servoVerifyV3Exec)
	execs.Register("is_servo_v4", servoVerifyV4Exec)
	execs.Register("is_servo_v4p1", servoVerifyV4p1Exec)
	execs.Register("is_servo_v4p1_by_serial_number", servoVerifyV4p1BySerialNumberExec)
	execs.Register("is_servo_micro", servoVerifyServoMicroExec)
	execs.Register("is_servo_c2d2", servoVerifyC2D2Exec)
	execs.Register("is_dual_setup_configured", servoIsDualSetupConfiguredExec)
	execs.Register("is_dual_setup", servoVerifyDualSetupExec)
	execs.Register("is_servo_type_ccd", servoVerifyServoCCDExec)
	execs.Register("servo_main_device_is_gsc", mainDeviceIsGSCExec)
	execs.Register("servo_main_device_is_ccd", mainDeviceIsCCDExec)
	execs.Register("servo_type_regex_match", servoTypeRegexMatchExec)
	execs.Register("servo_has_debug_header", servoHasDebugHeaderExec)
}
