// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"

	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/cros/satlab/common/dut"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

// UpdateDUTCmd is the command that updates fields for a satlab DUT.
var UpdateDUTCmd = &subcommands.Command{
	UsageLine: "dut [options ...]",
	ShortDesc: "Update a Satlab DUT",
	CommandRun: func() subcommands.CommandRun {
		c := &updateDUTCmd{}
		registerUpdateShivasFlags(c)
		return c
	},
}

// UpdateDUT is the 'satlab update dut' command. Its fields are the command line arguments.
type updateDUTCmd struct {
	subcommands.CommandRunBase

	authFlags authcli.Flags

	dut.UpdateDUT
}

// Run is the main entrypoint to 'satlab update dut'.
func (c *updateDUTCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// InnerRun is the implementation of 'satlab update dut'.
func (c *updateDUTCmd) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := context.Background()
	return c.TriggerRun(ctx, &executor.ExecCommander{})
}
