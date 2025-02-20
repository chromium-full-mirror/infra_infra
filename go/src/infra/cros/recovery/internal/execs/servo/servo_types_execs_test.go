// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"testing"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/tlw"
)

var servoV3TestCases = []struct {
	testName    string
	expectedErr bool
}{
	{
		"host-servo",
		false,
	},
	{
		"host-servo1",
		true,
	},
	{
		"host-labstation",
		true,
	},
	{
		"host-servo_v4",
		true,
	},
}

func TestServoVerifyV3Exec(t *testing.T) {
	t.Parallel()
	for _, tt := range servoV3TestCases {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			args := &execs.RunArgs{
				DUT: &tlw.Dut{
					Chromeos: &tlw.ChromeOS{
						Servo: &tlw.ServoHost{
							Name: tt.testName,
						},
					},
				},
			}
			info := execs.NewExecInfo(args, "name", nil, 0, nil)
			actualErr := servoVerifyV3Exec(ctx, info)
			if tt.expectedErr && actualErr == nil {
				t.Errorf("%q expected error, but did not get it", tt.testName)
			}
			if !tt.expectedErr && actualErr != nil {
				t.Errorf("%q did not expected error, but got it %s", tt.testName, actualErr)
			}
		})
	}
}
