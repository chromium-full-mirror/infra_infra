// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package amt

import (
	"context"
	"strings"
	"testing"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/tlw"
)

func TestSetAMTStateExec(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		testName         string
		actionArgs       []string
		expectedAMTState tlw.AMTManager_State
		amtManager       *tlw.AMTManager
		expectedErr      error
	}{
		{
			"success: set amt_manager state to WORKING",
			[]string{
				"state:WORKING",
			},
			tlw.AMTManager_WORKING,
			&tlw.AMTManager{
				State: tlw.AMTManager_WORKING,
			},
			nil,
		},
		{
			"success: set amt_manager state to BROKEN",
			[]string{
				"state:BROKEN",
			},
			tlw.AMTManager_BROKEN,
			&tlw.AMTManager{
				State: tlw.AMTManager_BROKEN,
			},
			nil,
		},
		{
			"success: set amt_manager state to NOT_APPLICABLE",
			[]string{
				"state:NOT_APPLICABLE",
			},
			tlw.AMTManager_NOT_APPLICABLE,
			&tlw.AMTManager{
				State: tlw.AMTManager_NOT_APPLICABLE,
			},
			nil,
		},
		{
			"fail: amt_manager state info is empty",
			[]string{
				"state:",
			},
			tlw.AMTManager_STATE_UNSPECIFIED,
			&tlw.AMTManager{
				State: tlw.AMTManager_STATE_UNSPECIFIED,
			},
			errors.Reason("set amt_manager state: state is not provided").Err(),
		},
		{
			"fail: do not update if servo is not supported in structure",
			[]string{
				"state:WORKING",
			},
			tlw.AMTManager_WORKING,
			nil,
			errors.Reason("set amt_manager state: amt_manager is not supported").Err(),
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			args := &execs.RunArgs{
				DUT: &tlw.Dut{
					Chromeos: &tlw.ChromeOS{
						AmtManager: tt.amtManager,
					},
				},
			}
			info := execs.NewExecInfo(args, "name", tt.actionArgs, 0, nil)
			actualErr := setAMTStateExec(ctx, info)
			if actualErr != nil && tt.expectedErr != nil {
				if !strings.Contains(actualErr.Error(), tt.expectedErr.Error()) {
					t.Errorf("%s: expected error %q, but got %q", tt.testName, tt.expectedErr, actualErr)
				}
			}
			if (actualErr == nil && tt.expectedErr != nil) || (actualErr != nil && tt.expectedErr == nil) {
				t.Errorf("%s: expected error %q, but got %q", tt.testName, tt.expectedErr, actualErr)
			}
			if tt.amtManager != nil {
				actualAMTState := tt.amtManager.State
				if actualAMTState != tt.expectedAMTState {
					t.Errorf("%s: expected servo state %q, but got %q", tt.testName, tt.expectedAMTState, actualAMTState)
				}
			}
		})
	}
}
