// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pasit

import (
	"context"
	"testing"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/tlw"
)

func TestSetPasitStateExec(t *testing.T) {
	tests := []struct {
		name          string
		expectedErr   bool
		argState      string
		expectedState tlw.Pasit_State
	}{
		{
			"working",
			false,
			"working",
			tlw.Pasit_STATE_WORKING,
		},
		{
			"broken",
			false,
			"broken",
			tlw.Pasit_STATE_BROKEN,
		},
		{
			"empty",
			true,
			"",
			tlw.Pasit_STATE_UNSPECIFIED,
		},
		{
			"invalid",
			true,
			"fake_sate",
			tlw.Pasit_STATE_UNSPECIFIED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dut := &tlw.Dut{
				Name: "dut-hostname",
				Chromeos: &tlw.ChromeOS{
					Pasit: &tlw.Pasit{},
				},
			}

			runArgs := &execs.RunArgs{DUT: dut}
			actionArgs := []string{"state:" + tt.argState}
			info := execs.NewExecInfo(runArgs, "", actionArgs, 15, nil)

			if err := setPasitStateExec(context.Background(), info); err != nil && !tt.expectedErr {
				t.Errorf("%q -> error, received unexpected error: %v", tt.name, err)
			} else if err == nil && tt.expectedErr {
				t.Errorf("%q -> error expected but not received", tt.name)
			}

			gotState := dut.GetChromeos().GetPasit().GetState()
			if gotState != tt.expectedState {
				t.Errorf("%q -> error got state %q, expected %q", tt.name, gotState, &tt.expectedState)
			}
		})
	}
}
