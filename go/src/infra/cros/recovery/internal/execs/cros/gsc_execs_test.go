// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"testing"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/tlw"
)

func TestCrosReadGSCChipExec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	table := []struct {
		name    string
		info    *execs.ExecInfo
		wantErr bool
		gscChip tlw.ChromeOS_GscChip
	}{
		{
			name: "Servo not found",
			info: execs.NewExecInfo(&execs.RunArgs{
				DUT: &tlw.Dut{},
			},
				"",
				[]string{},
				0,
				nil),
			wantErr: true,
		},
		{
			name: "Servo found, no GSC chip",
			info: execs.NewExecInfo(&execs.RunArgs{
				DUT: &tlw.Dut{
					Chromeos: &tlw.ChromeOS{
						Servo: &tlw.ServoHost{
							SerialNumber: "servo_serial",
							ServoTopology: &tlw.ServoTopology{
								Children: []*tlw.ServoTopologyItem{},
							},
						},
					},
				},
			},
				"",
				[]string{},
				0,
				nil),
			wantErr: true,
		},
		{
			name: "Servo found, with GSC chip",
			info: execs.NewExecInfo(&execs.RunArgs{
				DUT: &tlw.Dut{
					Chromeos: &tlw.ChromeOS{
						Servo: &tlw.ServoHost{
							SerialNumber: "servo_serial",
							ServoTopology: &tlw.ServoTopology{
								Children: []*tlw.ServoTopologyItem{
									{
										VidPid: "18d1:5014",
									},
								},
							},
						},
					},
				},
			},
				"",
				[]string{},
				0,
				nil),
			wantErr: false,
			gscChip: tlw.ChromeOS_GSC_CHIP_H1,
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			err := crosReadGSCChipExec(ctx, tt.info)
			if (err != nil) != tt.wantErr {
				t.Errorf("crosReadGSCChipExec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.info.GetChromeos().GscChip != tt.gscChip {
				t.Errorf("crosReadGSCChipExec() gscChip = %v, want %v", tt.info.GetChromeos().GscChip, tt.gscChip)
			}
		})
	}
}
