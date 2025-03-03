// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package servo

import (
	"reflect"
	"strings"
	"testing"
)

func TestServoType(t *testing.T) {
	SERVO_C2D2 := "c2d2"
	SERVO_CCD_CR50 := "ccd_cr50"
	SERVO_CCD_TI50 := "ccd_ti50"
	SERVO_CCD_GSC := "ccd_gsc"
	SERVO_MICRO := "servo_micro"
	SERVO_V2 := "servo_v2"
	SERVO_V4_C2D2 := "servo_v4_with_c2d2"
	SERVO_V4_CCD := "servo_v4_with_ccd_something"
	SERVO_V4_CCD_CR50 := "servo_v4_with_ccd_cr50"
	SERVO_V4_CCD_TI50 := "servo_v4_with_ccd_ti50"
	SERVO_V4_CCD_GSC := "servo_v4_with_ccd_gsc"
	SERVO_V4_MICRO := "servo_v4_with_servo_micro"
	SERVO_V4P1_C2D2 := "servo_v4p1_with_c2d2"
	SERVO_V4P1_CCD := "servo_v4p1_with_ccd_something"
	SERVO_V4P1_CCD_CR50 := "servo_v4p1_with_ccd_cr50"
	SERVO_V4P1_CCD_TI50 := "servo_v4p1_with_ccd_ti50"
	SERVO_V4P1_CCD_GSC := "servo_v4p1_with_ccd_gsc"
	SERVO_V4P1_MICRO := "servo_v4p1_with_servo_micro"

	VALID_SERVOS := []string{
		SERVO_C2D2,
		SERVO_CCD_CR50,
		SERVO_CCD_TI50,
		SERVO_CCD_GSC,
		SERVO_MICRO,
		SERVO_V2,
		SERVO_V4_C2D2,
		SERVO_V4_CCD,
		SERVO_V4_CCD_CR50,
		SERVO_V4_CCD_TI50,
		SERVO_V4_CCD_GSC,
		SERVO_V4_MICRO,
		SERVO_V4P1_C2D2,
		SERVO_V4P1_CCD,
		SERVO_V4P1_CCD_CR50,
		SERVO_V4P1_CCD_TI50,
		SERVO_V4P1_CCD_GSC,
		SERVO_V4P1_MICRO,
	}
	CCD_SERVOS := []string{
		SERVO_CCD_CR50,
		SERVO_CCD_TI50,
		SERVO_CCD_GSC,
		SERVO_V4_CCD,
		SERVO_V4_CCD_CR50,
		SERVO_V4_CCD_TI50,
		SERVO_V4_CCD_GSC,
		SERVO_V4P1_CCD,
		SERVO_V4P1_CCD_CR50,
		SERVO_V4P1_CCD_TI50,
		SERVO_V4P1_CCD_GSC,
	}
	GSC_SERVOS := []string{
		SERVO_CCD_GSC,
		SERVO_V4_CCD_GSC,
		SERVO_V4P1_CCD_GSC,
	}
	CR50_SERVOS := []string{
		SERVO_CCD_CR50,
		SERVO_V4_CCD_CR50,
		SERVO_V4P1_CCD_CR50,
	}
	MICRO_SERVOS := []string{SERVO_MICRO, SERVO_V4_MICRO, SERVO_V4P1_MICRO}
	V2_SERVOS := []string{SERVO_V2}
	V4_SERVOS := []string{SERVO_V4_C2D2, SERVO_V4_CCD, SERVO_V4_CCD_CR50,
		SERVO_V4_CCD_TI50, SERVO_V4_CCD_GSC, SERVO_V4_MICRO,
		SERVO_V4P1_C2D2, SERVO_V4P1_CCD, SERVO_V4P1_CCD_CR50, SERVO_V4P1_CCD_TI50,
		SERVO_V4P1_CCD_GSC, SERVO_V4P1_MICRO}
	C2D2_SERVOS := []string{SERVO_C2D2, SERVO_V4_C2D2, SERVO_V4P1_C2D2}

	listContains := func(list []string, str string) bool {
		for i := range len(list) {
			if list[i] == str {
				return true
			}
		}
		return false
	}

	for i := range len(VALID_SERVOS) {
		servoStr := VALID_SERVOS[i]
		servo := NewServoType(servoStr)
		if servo.IsV2() != listContains(V2_SERVOS, servoStr) {
			t.Errorf("servo %v: expected IsV2() to return %v", servoStr, !servo.IsV2())
		}
		if servo.IsV4() != listContains(V4_SERVOS, servoStr) {
			t.Errorf("servo %v: expected IsV4() to return %v", servoStr, !servo.IsV4())
		}
		if servo.IsCCD() != listContains(CCD_SERVOS, servoStr) {
			t.Errorf("servo %v: expected IsCCD() to return %v", servoStr, !servo.IsCCD())
		}
		if servo.IsMainDeviceCCD() != strings.HasPrefix(servo.MainDevice(), "ccd_") {
			t.Errorf("servo %v: expected IsMainDeviceCCD() to return %v", servoStr, !servo.IsMainDeviceCCD())
		}
		if servo.IsCr50() != listContains(CR50_SERVOS, servoStr) {
			t.Errorf("servo %v: expected IsCr50() to return %v", servoStr, !servo.IsCr50())
		}
		if servo.IsGSC() != listContains(GSC_SERVOS, servoStr) {
			t.Errorf("servo %v: expected IsGSC() to return %v", servoStr, !servo.IsGSC())
		}
		if servo.IsC2D2() != listContains(C2D2_SERVOS, servoStr) {
			t.Errorf("servo %v: expected IsC2D2() to return %v", servoStr, !servo.IsC2D2())
		}
		if servo.IsMicro() != listContains(MICRO_SERVOS, servoStr) {
			t.Errorf("servo %v: expected IsMicro() to return %v", servoStr, !servo.IsMicro())
		}
	}
}

var mainDeviceTestCases = []struct {
	servoType string
	expected  string
}{
	{"servo_v4_with_ccd_cr50", "ccd_cr50"},
	{"servo_v4_with_c2d2_and_ccd_cr50", "c2d2"},
	{"servo_v4_with_servo_micro", "servo_micro"},
	{"servo_v4_and_servo_micro", ""},
	{"servo_v4", ""},
	{"servo_v3", ""},
	{"c2d2", ""},
	{"servo_micro", ""},
	{"servo_v4_with_servo_micro_and_ccd_cr50", "servo_micro"},
}

func TestMainDevice(t *testing.T) {
	t.Parallel()
	for _, tt := range mainDeviceTestCases {
		t.Run(tt.servoType, func(t *testing.T) {
			t.Parallel()
			servo := NewServoType(tt.servoType)
			main := servo.MainDevice()
			if main != tt.expected {
				t.Errorf("%q -> expected %q, but got %q", tt.servoType, tt.expected, main)
			}
		})
	}
}

var extractComponentsTestCases = []struct {
	servoType string
	onlyChild bool
	expected  []string
}{
	{"empty", true, nil},
	{"servo_v4_with_ccd_cr50", true, []string{"ccd_cr50"}},
	{"servo_v4_with_c2d2_and_ccd_cr50", true, []string{"c2d2", "ccd_cr50"}},
	{"servo_v4_with_ccd_gsc_and_c2d2", true, []string{"ccd_gsc", "c2d2"}},
	{"servo_v4_with_servo_micro", true, []string{"servo_micro"}},
	{"servo_v4_with_ccd_cr50_and_servo_micro", true, []string{"ccd_cr50", "servo_micro"}},
	{"servo_v4_with_servo_micro_and_ccd_cr50", true, []string{"servo_micro", "ccd_cr50"}},
	{"servo_v4_and_servo_micro", true, nil},
	{"servo_v4", true, nil},
	{"servo_v3", true, nil},
	{"c2d2", true, nil},
	{"servo_micro", true, nil},

	{"empty", false, []string{"empty"}},
	{"servo_v4_with_ccd_cr50", false, []string{"servo_v4", "ccd_cr50"}},
	{"servo_v4_with_c2d2_and_ccd_cr50", false, []string{"servo_v4", "c2d2", "ccd_cr50"}},
	{"servo_v4_with_ccd_gsc_and_c2d2", false, []string{"servo_v4", "ccd_gsc", "c2d2"}},
	{"servo_v4_with_servo_micro", false, []string{"servo_v4", "servo_micro"}},
	{"servo_v4_with_ccd_cr50_and_servo_micro", false, []string{"servo_v4", "ccd_cr50", "servo_micro"}},
	{"servo_v4_with_servo_micro_and_ccd_cr50", false, []string{"servo_v4", "servo_micro", "ccd_cr50"}},
	{"servo_v4_and_servo_micro", false, nil},
	{"servo_v4", false, []string{"servo_v4"}},
	{"servo_v3", false, []string{"servo_v3"}},
	{"c2d2", false, []string{"c2d2"}},
	{"servo_micro", false, []string{"servo_micro"}},
}

func TestExtractComponents(t *testing.T) {
	t.Parallel()
	for _, tt := range extractComponentsTestCases {
		t.Run(tt.servoType, func(t *testing.T) {
			t.Parallel()
			servo := NewServoType(tt.servoType)
			components := servo.ExtractComponents(tt.onlyChild)
			if !reflect.DeepEqual(components, tt.expected) {
				t.Errorf("%q -> expected %v, but got %v", tt.servoType, tt.expected, components)
			}
		})
	}
}
