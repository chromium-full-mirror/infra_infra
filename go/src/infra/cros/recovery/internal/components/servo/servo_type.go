// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	// Servo components/types used by system.
	SERVO_V2    = "servo_v2"
	SERVO_V3    = "servo_v3"
	SERVO_V4    = "servo_v4"
	SERVO_V4P1  = "servo_v4p1"
	CCD_CR50    = "ccd_cr50"
	CCD_GSC     = "ccd_gsc"
	C2D2        = "c2d2"
	SERVO_MICRO = "servo_micro"
	SWEETBERRY  = "sweetberry"

	// Prefix for CCD components.
	CCD_PREFIX = "ccd_"
)

var (
	// MAP of servos that connect to a debug header on the board.
	FLEX_SERVOS = map[string]bool{
		C2D2:        true,
		SERVO_MICRO: true,
		SERVO_V3:    true,
	}
	// Map of servos that rely on gsc commands for some part of dut control.
	GSC_SERVOS = map[string]bool{
		C2D2:     true,
		CCD_GSC:  true,
		CCD_CR50: true,
	}
)

// ServoType represent structure to allow distinguishe servo components described in servo-type string.
type ServoType struct {
	str string
}

// NewServoType creates new ServoType with provided string representation.
func NewServoType(servoType string) *ServoType {
	return &ServoType{servoType}
}

// IsV2 checks whether the servo has a servo_v2 component.
func (s *ServoType) IsV2() bool {
	return strings.Contains(s.str, SERVO_V2)
}

// IsV3 checks whether the servo has a servo_v3 component.
func (s *ServoType) IsV3() bool {
	return strings.Contains(s.str, SERVO_V3)
}

// IsV4 checks whether the servo has servo_v4 or servo_v4p1 component.
func (s *ServoType) IsV4() bool {
	return strings.Contains(s.str, SERVO_V4)
}

// IsV4p1 returns true if and only if the servo has a servo_v4p1 component.
func (s *ServoType) IsV4p1() bool {
	// TODO(gregorynisbet): Should this be contains or hasPrefix?
	return strings.Contains(s.str, SERVO_V4P1)
}

// IsC2D2 checks whether the servo has a c2d2 component.
func (s *ServoType) IsC2D2() bool {
	return strings.Contains(s.str, C2D2)
}

// IsCCD checks whether the servo has a CCD component.
func (s *ServoType) IsCCD() bool {
	return strings.Contains(s.str, CCD_PREFIX)
}

// IsMainDeviceCCD checks whether the main device of servo is a CCD component.
func (s *ServoType) IsMainDeviceCCD() bool {
	return strings.HasPrefix(s.MainDevice(), CCD_PREFIX)
}

// IsMainDeviceGSC checks whether the main device of servo is a GSC.
func (s *ServoType) IsMainDeviceGSC() bool {
	return s.IsMainDeviceCCD() || s.MainDevice() == C2D2
}

// IsCr50 checks whether the servo has a CCD by CR50 component.
func (s *ServoType) IsCr50() bool {
	return strings.Contains(s.str, CCD_CR50)
}

// IsGSC checks whether the servo has a CCD by GSC component.
func (s *ServoType) IsGSC() bool {
	return strings.Contains(s.str, CCD_GSC)
}

// IsMicro checks whether the servo has a servo_micro component.
func (s *ServoType) IsMicro() bool {
	return strings.Contains(s.str, SERVO_MICRO)
}

// IsDualSetup checks whether the servo has a dual setup.
func (s *ServoType) IsDualSetup() bool {
	return s.IsV4() && (s.IsMicro() || s.IsC2D2()) && s.IsCCD()
}

// IsMultipleServos checks whether the servo has more than one component.
func (s *ServoType) IsMultipleServos() bool {
	return strings.Contains(s.str, "_and_")
}

// String provide ability to use ToString functionality.
func (s *ServoType) String() string {
	return s.str
}

// MainDevice extracts the main servo device.
func (s *ServoType) MainDevice() string {
	s1 := strings.Split(s.str, "_with_")
	if len(s1) < 2 {
		return ""
	}
	s2 := strings.Split(s1[len(s1)-1], "_and_")[0]
	return s2
}

// ExtractComponents extracts components from the name.
func (s *ServoType) ExtractComponents(onlyChild bool) []string {
	s1 := strings.Split(s.str, "_with_")
	switch len(s1) {
	case 0:
		return nil
	case 1:
		if strings.Contains(s.str, "_and_") {
			// that is incorrect servo_type
			return nil
		}
		if onlyChild {
			return nil
		}
		return s1
	}
	s2 := strings.Split(s1[1], "_and_")
	switch len(s2) {
	case 1:
		if onlyChild {
			return []string{s1[1]}
		}
		return s1
	case 2:
		if onlyChild {
			return s2
		}
		return []string{s1[0], s2[0], s2[1]}
	}
	return nil
}

// GetServoType finds and returns the servo type of the DUT's servo.
func GetServoType(ctx context.Context, servod components.Servod) (*ServoType, error) {
	res, err := servod.Get(ctx, "servo_type")
	if err != nil {
		return nil, errors.Annotate(err, "get servo type").Err()
	}
	servoType := res.GetString_()
	if servoType == "" {
		return nil, errors.Reason("get servo type: servo type is empty").Err()
	}
	return NewServoType(servoType), nil
}

// WrappedServoType returns the type of servo device.
//
// This function first looks up the servo type using the servod
// control. If that does not work, it looks up the dut information for
// the servo host.
func WrappedServoType(ctx context.Context, servod components.Servod, servoHost *tlw.ServoHost) (*ServoType, error) {
	servoType, err := GetServoType(ctx, servod)
	if err != nil {
		log.Debugf(ctx, "Wrapped Servo Type: Could not read the servo type from servod.")
		if st := servoHost.GetServodType(); st != "" {
			servoType = NewServoType(st)
		} else {
			return nil, errors.Reason("wrapped servo type: could not determine the servo type from servod control as well DUT Info.").Err()
		}
	}
	return servoType, nil
}
