// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vlan

import (
	"testing"

	"github.com/maruel/subcommands"

	"go.chromium.org/infra/libs/skylab/common/heuristics"
)

// TestValidateGetVlanArgs tests how we parse and validate arguments to `shivas get vlan`.
func TestValidateGetVlanArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		ok   bool
	}{
		{
			name: "no args",
			args: []string{},
			ok:   true,
		},
		{
			name: "ips with vlan",
			args: []string{"-ips", "fake-vlan"},
			ok:   true,
		},
		{
			name: "ips with two vlans BAD",
			args: []string{"-ips", "fake-vlan", "fake-vlan2"},
			ok:   false,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := heuristics.ParseUsingCommand(GetVlanCmd, tt.args, func(c subcommands.CommandRun) error {
				return c.(*getVlan).validateArgs()
			})

			switch {
			case err == nil && !tt.ok:
				t.Error("error is unexpectedly nil")
			case err != nil && tt.ok:
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}
