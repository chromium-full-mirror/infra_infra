// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cros/recovery/tlw"
)

var createServoDeviceFwUpdateCmdTestCases = []struct {
	testName     string
	useContainer bool
	device       *tlw.ServoTopologyItem
	forceUpdate  bool
	channel      tlw.ServoFwChannel
	out          string
}{
	{
		"simple",
		false,
		&tlw.ServoTopologyItem{
			Type:   "servo_micro",
			Serial: "v4p1-serial123456781",
		},
		false,
		tlw.ServoFwChannel_STABLE,
		"servo_updater -b servo_micro -s v4p1-serial123456781 -c stable --reboot --allow-rollback ",
	},
	{
		"simple for container",
		true,
		&tlw.ServoTopologyItem{
			Type:   "servo_micro",
			Serial: "v4p1-serial123456782",
		},
		false,
		tlw.ServoFwChannel_DEV,
		"python /update_servo_firmware.py -b servo_micro -s v4p1-serial123456782 -c dev --reboot --allow-rollback ",
	},
	{
		"forced",
		false,
		&tlw.ServoTopologyItem{
			Type:   "servo_micro",
			Serial: "v4p1-serial123456783",
		},
		true,
		tlw.ServoFwChannel_ALPHA,
		"servo_updater -b servo_micro -s v4p1-serial123456783 -c alpha --reboot --force ",
	},
	{
		"forced for container",
		true,
		&tlw.ServoTopologyItem{
			Type:   "servo_micro",
			Serial: "v4p1-serial123456784",
		},
		true,
		tlw.ServoFwChannel_STABLE,
		"python /update_servo_firmware.py -b servo_micro -s v4p1-serial123456784 -c stable --reboot --force ",
	},
}

func TestCreateServoDeviceFwUpdateCmd(t *testing.T) {
	t.Parallel()
	for _, tt := range createServoDeviceFwUpdateCmdTestCases {
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			got := createServoDeviceFwUpdateCmd(tt.useContainer, tt.device, tt.forceUpdate, tt.channel)
			if diff := cmp.Diff(got, tt.out); diff != "" {
				t.Errorf("TestCreateServoDeviceFwUpdateCmd %q diff: %q, expected: %q, got:%q", tt.testName, diff, tt.out, got)
			}
		})
	}
}
