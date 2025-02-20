// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"strings"
	"testing"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/tlw"
)

var setServoStateExecTestCases = []struct {
	testName           string
	actionArgs         []string
	expectedServoState tlw.ServoHost_State
	servoHost          *tlw.ServoHost
	expectedErr        error
}{
	{
		"success: set servo state to SBU_LOW_VOLTAGE",
		[]string{
			"state:SBU_LOW_VOLTAGE",
		},
		tlw.ServoHost_SBU_LOW_VOLTAGE,
		&tlw.ServoHost{
			State: tlw.ServoHost_STATE_UNSPECIFIED,
		},
		nil,
	},
	{
		"fail: missing state info found",
		[]string{
			"test:SBU_LOW_VOLTAGE",
		},
		tlw.ServoHost_STATE_UNSPECIFIED,
		&tlw.ServoHost{
			State: tlw.ServoHost_STATE_UNSPECIFIED,
		},
		errors.Reason("set servo state: state is not provided").Err(),
	},
	{
		"fail: state info is empty",
		[]string{
			"state:",
		},
		tlw.ServoHost_STATE_UNSPECIFIED,
		&tlw.ServoHost{
			State: tlw.ServoHost_STATE_UNSPECIFIED,
		},
		errors.Reason("set servo state: state is not provided").Err(),
	},
	{
		"fail: state info in wrong format",
		[]string{
			"state:sbu_LOW_VOLTAGE",
		},
		tlw.ServoHost_SBU_LOW_VOLTAGE,
		&tlw.ServoHost{
			State: tlw.ServoHost_STATE_UNSPECIFIED,
		},
		nil,
	},
	{
		"fail: do not update if servo is not supported in structure",
		[]string{
			"state:sbu_LOW_VOLTAGE",
		},
		tlw.ServoHost_SBU_LOW_VOLTAGE,
		nil,
		errors.Reason("set servo state: servo is not supported").Err(),
	},
	{
		"Good case with CR50_NOT_ENUMERATED",
		[]string{
			"state:CR50_NOT_ENUMERATED",
		},
		tlw.ServoHost_CR50_NOT_ENUMERATED,
		&tlw.ServoHost{
			State: tlw.ServoHost_CR50_NOT_ENUMERATED,
		},
		nil,
	},
	{
		"Good set state SERVOD_DUT_CONTROLLER_MISSING",
		[]string{
			"state:SERVOD_DUT_CONTROLLER_MISSING",
		},
		tlw.ServoHost_SERVOD_DUT_CONTROLLER_MISSING,
		&tlw.ServoHost{
			State: tlw.ServoHost_SERVOD_DUT_CONTROLLER_MISSING,
		},
		nil,
	},
}

func TestSetServoStateExec(t *testing.T) {
	t.Parallel()
	for _, tt := range setServoStateExecTestCases {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			args := &execs.RunArgs{
				DUT: &tlw.Dut{
					Chromeos: &tlw.ChromeOS{
						Servo: tt.servoHost,
					},
				},
			}
			info := execs.NewExecInfo(args, "name", tt.actionArgs, 0, nil)
			actualErr := setServoStateExec(ctx, info)
			if actualErr != nil && tt.expectedErr != nil {
				if !strings.Contains(actualErr.Error(), tt.expectedErr.Error()) {
					t.Errorf("Expected error %q, but got %q", tt.expectedErr, actualErr)
				}
			}
			if (actualErr == nil && tt.expectedErr != nil) || (actualErr != nil && tt.expectedErr == nil) {
				t.Errorf("Expected error %q, but got %q", tt.expectedErr, actualErr)
			}
			if tt.servoHost != nil {
				actualServoState := tt.servoHost.State
				if actualServoState != tt.expectedServoState {
					t.Errorf("Expected servo state %q, but got %q", tt.expectedServoState, actualServoState)
				}
			}
		})
	}
}

var matchServoStateExecTestCases = []struct {
	testName    string
	actionArg   string
	servoHost   *tlw.ServoHost
	expectedErr error
}{
	{
		"states are matching",
		"state:SBU_LOW_VOLTAGE",
		&tlw.ServoHost{
			State: tlw.ServoHost_SBU_LOW_VOLTAGE,
		},
		nil,
	},
	{
		"not matching",
		"state:SBU_LOW_VOLTAGE",
		&tlw.ServoHost{
			State: tlw.ServoHost_STATE_UNSPECIFIED,
		},
		errors.Reason("match state: state mismatch, expected: \"sbu_low_voltage\", but got \"state_unspecified\"").Err(),
	},
	{
		"fail: state info is empty",
		"",
		&tlw.ServoHost{
			State: tlw.ServoHost_STATE_UNSPECIFIED,
		},
		errors.Reason("match state: state not provided").Err(),
	},
	{
		"fail: servo host is not exist",
		"state:Haha",
		nil,
		errors.Reason("match state: current servo state is unknown").Err(),
	},
}

func TestMatchServoStateExec(t *testing.T) {
	t.Parallel()
	for _, tt := range matchServoStateExecTestCases {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			args := &execs.RunArgs{
				DUT: &tlw.Dut{
					Chromeos: &tlw.ChromeOS{
						Servo: tt.servoHost,
					},
				},
			}
			info := execs.NewExecInfo(args, "name", []string{tt.actionArg}, 0, nil)
			actualErr := matchStateExec(ctx, info)
			if tt.expectedErr == nil {
				// Expected to pass
				if actualErr != nil {
					t.Errorf("Expected to pass by fail with %q", actualErr)
				}
			} else {
				if actualErr == nil {
					t.Errorf("Expected error %q, but got %q", tt.expectedErr, actualErr)
				} else if !strings.Contains(actualErr.Error(), tt.expectedErr.Error()) {
					t.Errorf("Expected error %q, but got %q", tt.expectedErr, actualErr)
				}
			}
		})
	}
}
