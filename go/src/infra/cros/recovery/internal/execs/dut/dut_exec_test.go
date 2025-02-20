// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/tlw"
)

var setDutStateCases = []struct {
	actionArg    string
	errorMessage string
}{
	{
		"empty",
		"set dut state: state is not provided",
	},
	{
		"state:",
		"set dut state: state is not provided",
	},
	{
		"state:wrong",
		"set dut state: unsupported state \"wrong\"",
	},
	{
		"state:READY",
		"",
	},
	{
		"state:ready",
		"",
	},
	{
		"state:ready2",
		"set dut state: unsupported state \"ready2\"",
	},
}

func TestSetDUTState(t *testing.T) {
	t.Parallel()
	for _, c := range setDutStateCases {
		cs := c
		t.Run(cs.actionArg, func(t *testing.T) {
			ctx := context.Background()
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{},
				},
				"", []string{cs.actionArg}, 0, nil)
			err := setDutStateExec(ctx, info)
			if cs.errorMessage == "" {
				if err != nil {
					t.Errorf("%q -> received error when not expected it: %v", cs.actionArg, err)
				}
			} else {
				if err == nil {
					t.Errorf("%q -> expected error %q but did not get error", cs.actionArg, cs.errorMessage)
				} else if !cmp.Equal(err.Error(), cs.errorMessage) {
					t.Errorf("%q -> expected error %q but got: %q", cs.actionArg, cs.errorMessage, err.Error())
				}
			}
		})
	}
}
