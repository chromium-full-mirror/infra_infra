// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"testing"

	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/scopes"
	"go.chromium.org/infra/cros/recovery/tlw"
)

func TestSetDUTState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		actionArg     string
		expectedState dutstate.State
		wantErr       bool
	}{
		{
			"state:wrong",
			"state:wrong",
			"",
			true,
		},
		{
			"state:READY",
			"state:READY",
			dutstate.Ready,
			false,
		},
		{
			"state:ready",
			"state:ready",
			dutstate.Ready,
			false,
		},
		{
			"state:needs_repair",
			"state:needs_repair",
			dutstate.NeedsRepair,
			false,
		},
		{
			"state:needs_reset",
			"state:needs_reset",
			dutstate.NeedsReset,
			false,
		},
		{
			"state:repair_failed",
			"state:repair_failed",
			dutstate.RepairFailed,
			false,
		},
		{
			"state:needs_deploy",
			"state:needs_deploy",
			dutstate.NeedsDeploy,
			false,
		},
		{
			"state:deploying",
			"state:deploying",
			dutstate.Deploying,
			false,
		},
		{
			"state:reserved",
			"state:reserved",
			dutstate.Reserved,
			false,
		},
		{
			"state:manual_repair",
			"state:manual_repair",
			dutstate.ManualRepair,
			false,
		},
		{
			"state:needs_manual_repair",
			"state:needs_manual_repair",
			dutstate.NeedsManualRepair,
			false,
		},
		{
			"state:needs_replacement",
			"state:needs_replacement",
			dutstate.NeedsReplacement,
			false,
		},
		{
			"state:ready2",
			"state:ready2",
			"",
			true,
		},
		{
			"state:ready, but expected wrong",
			"state:ready",
			dutstate.Ready,
			false,
		},
		{
			"state:wrong, but expected something",
			"state:wrong",
			"ready",
			true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			dut := &tlw.Dut{}
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: dut,
				},
				"", []string{tt.actionArg}, 0, nil)
			err := setDutStateExec(ctx, info)
			if (err != nil) != tt.wantErr {
				t.Errorf("setDutStateExec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && dut.State != tt.expectedState {
				t.Errorf("DUT state is not set as expected. Got %q, want %q", dut.State, tt.expectedState)
			}
		})
	}
}

func TestHasDutNameActionExec(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		dut     *tlw.Dut
		wantErr bool
	}{
		{
			"nil dut",
			nil,
			true,
		},
		{
			"empty name",
			&tlw.Dut{Name: ""},
			true,
		},
		{
			"good",
			&tlw.Dut{Name: "dut_name"},
			false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			runArgs := &execs.RunArgs{
				DUT: tt.dut,
			}
			info := execs.NewExecInfo(runArgs, "", nil, 0, nil)
			err := hasDutNameActionExec(ctx, info)
			if (err != nil) != tt.wantErr {
				t.Errorf("hasDutNameExec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegexNameMatchExec(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		dut     *tlw.Dut
		args    []string
		wantErr bool
	}{
		{
			"nil dut",
			nil,
			[]string{"regex:.*"},
			true,
		},
		{
			"no regex",
			&tlw.Dut{Name: "dut_name"},
			[]string{},
			true,
		},
		{
			"empty regex",
			&tlw.Dut{Name: "dut_name"},
			[]string{"regex:"},
			true,
		},
		{
			"bad regex",
			&tlw.Dut{Name: "dut_name"},
			[]string{"regex:bad(regex"},
			true,
		},
		{
			"match",
			&tlw.Dut{Name: "dut_name"},
			[]string{"regex:dut_.*"},
			false,
		},
		{
			"no match",
			&tlw.Dut{Name: "dut_name"},
			[]string{"regex:not_dut_.*"},
			true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			runArgs := &execs.RunArgs{
				DUT: tt.dut,
			}
			info := execs.NewExecInfo(runArgs, "", tt.args, 0, nil)
			err := regexNameMatchExec(ctx, info)
			if (err != nil) != tt.wantErr {
				t.Errorf("regexNameMatchExec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResetDutStateReasonExec(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		startDUT  *tlw.Dut
		finishDUT *tlw.Dut
	}{
		{
			"nil dut",
			nil,
			nil,
		},
		{
			"reason is empty",
			&tlw.Dut{DutStateReason: ""},
			&tlw.Dut{DutStateReason: ""},
		},
		{
			"reset reason",
			&tlw.Dut{DutStateReason: "some reason"},
			&tlw.Dut{DutStateReason: ""},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			runArgs := &execs.RunArgs{
				DUT: tt.startDUT,
			}
			info := execs.NewExecInfo(runArgs, "", nil, 0, nil)
			resetDutStateReasonExec(ctx, info)
			if tt.startDUT != nil {
				if tt.startDUT.DutStateReason != tt.finishDUT.DutStateReason {
					t.Errorf("resetDutStateReasonExec() got = %q, want %q", tt.startDUT.DutStateReason, tt.finishDUT.DutStateReason)
				}
			}
		})
	}
}

func TestIsDutStateReasonEmptyExec(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		dut     *tlw.Dut
		wantErr bool
	}{
		{
			"nil dut",
			nil,
			true,
		},
		{
			"empty reason",
			&tlw.Dut{DutStateReason: ""},
			false,
		},
		{
			"non-empty reason",
			&tlw.Dut{DutStateReason: "some reason"},
			true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			runArgs := &execs.RunArgs{
				DUT: tt.dut,
			}
			info := execs.NewExecInfo(runArgs, "", nil, 0, nil)
			err := isDutStateReasonEmptyExec(ctx, info)
			if (err != nil) != tt.wantErr {
				t.Errorf("isDutStateReasonEmptyExec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetDutStateReasonFromTaskTagsExec(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		dut     *tlw.Dut
		args    []string
		params  map[string]interface{}
		wantDUT *tlw.Dut
		wantErr bool
	}{
		{
			"nil dut",
			nil,
			[]string{"tag_name:some_tag"},
			nil,
			nil,
			true,
		},
		{
			"no tag name",
			&tlw.Dut{},
			[]string{},
			nil,
			&tlw.Dut{},
			true,
		},
		{
			"empty tag name",
			&tlw.Dut{},
			[]string{"tag_name:"},
			nil,
			&tlw.Dut{},
			true,
		},
		{
			"no scope",
			&tlw.Dut{},
			[]string{"tag_name:some_tag"},
			nil,
			&tlw.Dut{DutStateReason: ""},
			false,
		},
		{
			"not map",
			&tlw.Dut{},
			[]string{"tag_name:some_tag"},
			map[string]interface{}{scopes.ParamKeySwarmingTaskTags: "not a map"},
			nil,
			true,
		},
		{
			"tag not found",
			&tlw.Dut{},
			[]string{"tag_name:some_tag"},
			map[string]interface{}{scopes.ParamKeySwarmingTaskTags: map[string]string{"other_tag": "value"}},
			&tlw.Dut{DutStateReason: ""},
			false,
		},
		{
			"tag found",
			&tlw.Dut{},
			[]string{"tag_name:some_tag"},
			map[string]interface{}{scopes.ParamKeySwarmingTaskTags: map[string]string{"some_tag": "some_reason"}},
			&tlw.Dut{DutStateReason: "some_reason"},
			false,
		},
		{
			"tag found with space",
			&tlw.Dut{},
			[]string{"tag_name:some_tag"},
			map[string]interface{}{scopes.ParamKeySwarmingTaskTags: map[string]string{"some_tag": "  some_reason  "}},
			&tlw.Dut{DutStateReason: "some_reason"},
			false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.params != nil {
				ctx = scopes.WithParams(ctx, tt.params)
			}
			runArgs := &execs.RunArgs{
				DUT: tt.dut,
			}
			info := execs.NewExecInfo(runArgs, "", tt.args, 0, nil)
			err := setDutStateReasonFromTaskTagsExec(ctx, info)
			if (err != nil) != tt.wantErr {
				t.Errorf("setDutStateReasonFromTaskTagsExec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.dut != nil {
				if tt.dut.DutStateReason != tt.wantDUT.DutStateReason {
					t.Errorf("setDutStateReasonFromTaskTagsExec() got = %q, want %q", tt.dut.DutStateReason, tt.wantDUT.DutStateReason)
				}
			}
		})
	}
}

func TestSetDutStateReasonExec(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		args      []string
		startDUT  *tlw.Dut
		finishDUT *tlw.Dut
	}{
		{
			"nil dut",
			[]string{"reason:some_reason"},
			nil,
			nil,
		},
		{
			"no reason arg",
			[]string{},
			&tlw.Dut{DutStateReason: ""},
			&tlw.Dut{DutStateReason: ""},
		},
		{
			"set reason",
			[]string{"reason:some_reason"},
			&tlw.Dut{DutStateReason: ""},
			&tlw.Dut{DutStateReason: "some_reason"},
		},
		{
			"override reason",
			[]string{"reason:new_reason"},
			&tlw.Dut{DutStateReason: "old_reason"},
			&tlw.Dut{DutStateReason: "new_reason"},
		},
		{
			"do not override reason",
			[]string{"reason:new_reason", "allow_override:false"},
			&tlw.Dut{DutStateReason: "old_reason"},
			&tlw.Dut{DutStateReason: "old_reason"},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			runArgs := &execs.RunArgs{
				DUT: tt.startDUT,
			}
			info := execs.NewExecInfo(runArgs, "", tt.args, 0, nil)
			setDutStateReasonExec(ctx, info)
			if tt.startDUT != nil {
				if tt.startDUT.DutStateReason != tt.finishDUT.DutStateReason {
					t.Errorf("setDutStateReasonExec() got = %q, want %q", tt.startDUT.DutStateReason, tt.finishDUT.DutStateReason)
				}
			}
		})
	}
}

func TestIsChromeOSHWExec(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		cros    *tlw.ChromeOS
		wantErr bool
	}{
		{
			"nil cros",
			nil,
			true,
		},
		{
			"is cros",
			&tlw.ChromeOS{},
			false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			runArgs := &execs.RunArgs{
				DUT: &tlw.Dut{
					Chromeos: tt.cros,
				},
			}
			info := execs.NewExecInfo(runArgs, "", nil, 0, nil)
			err := isChromeOSHWExec(ctx, info)
			if (err != nil) != tt.wantErr {
				t.Errorf("isChromeOSHWExec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
