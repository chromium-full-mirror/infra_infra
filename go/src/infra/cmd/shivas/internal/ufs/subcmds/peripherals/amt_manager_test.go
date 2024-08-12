// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package peripherals

import (
	"strings"
	"testing"
)

func TestAmtManager(t *testing.T) {
	t.Parallel()

	t.Run("cleanAndValidateFlags", func(t *testing.T) {
		t.Run("valid", func(t *testing.T) {
			goodTests := []struct {
				cmd *manageAmtManagerCmd
			}{
				{cmd: &manageAmtManagerCmd{dutName: "dut1", amtHostname: "192.168.231.123"}},
			}
			for _, tt := range goodTests {
				err := tt.cmd.cleanAndValidateFlags()
				if err != nil {
					t.Errorf("cleanAndValidateFlags got error: %v; want nil", err)
					continue
				}
			}
		})
		t.Run("invalid", func(t *testing.T) {
			errTests := []struct {
				cmd  *manageAmtManagerCmd
				want []string
			}{
				{
					cmd:  &manageAmtManagerCmd{amtHostname: "192.168.231.123", mode: actionAdd},
					want: []string{errDUTMissing},
				},
				{
					cmd:  &manageAmtManagerCmd{dutName: " ", amtHostname: "192.168.231.123", mode: actionAdd},
					want: []string{errDUTMissing},
				},
				{
					cmd:  &manageAmtManagerCmd{dutName: "dut", mode: actionAdd},
					want: []string{"'-amt-hostname' is required"},
				},
				{
					cmd:  &manageAmtManagerCmd{dutName: "dut", amtHostname: " ", mode: actionAdd},
					want: []string{"'-amt-hostname' is required"},
				},
				{
					cmd:  &manageAmtManagerCmd{mode: actionDelete},
					want: []string{errDUTMissing},
				},
			}
			for _, tt := range errTests {
				err := tt.cmd.cleanAndValidateFlags()
				if err == nil {
					t.Errorf("cleanAndValidateFlags got nil; want errors: %v", tt.want)
					continue
				}
				for _, errStr := range tt.want {
					if !strings.Contains(err.Error(), errStr) {
						t.Errorf("cleanAndValidateFlags = %q; want err %q included", err, errStr)
					}
				}
			}
		})
	})

	t.Run("add action", func(t *testing.T) {
		// Valid case
		cmd := &manageAmtManagerCmd{
			dutName:     "dut",
			amtHostname: "192.168.231.123",
			mode:        actionAdd,
		}
		t.Run("cleanAndValidateFlags", func(t *testing.T) {
			if err := cmd.cleanAndValidateFlags(); err != nil {
				t.Errorf("cleanAndValidateFlags got error: %v; want nil", err)
			}
		})
	})

	t.Run("delete action", func(t *testing.T) {
		// Valid case
		cmd := &manageAmtManagerCmd{
			dutName: "dut",
			mode:    actionDelete,
		}
		t.Run("cleanAndValidateFlags", func(t *testing.T) {
			if err := cmd.cleanAndValidateFlags(); err != nil {
				t.Errorf("cleanAndValidateFlags got error: %v; want nil", err)
			}
		})
	})
}
