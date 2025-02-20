// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/cros/satlab/common/dut"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

// AddDUTCmd is the command that deploys a Satlab DUT.
var AddDUTCmd = &subcommands.Command{
	UsageLine: "dut [options ...]",
	ShortDesc: "Deploy a Satlab DUT",
	CommandRun: func() subcommands.CommandRun {

		// keep this up to date with infra/cmd/shivas/ufs/subcmds/dut/add_dut.go
		c := &addDUTCmd{}
		c.Pools = []string{}
		c.Chameleons = []string{}
		c.Cameras = []string{}
		c.Cables = []string{}
		// Manual_tags must be key:value form.
		c.DeployTags = []string{"satlab:true"}
		// TODO(gregorynisbet): Consider skipping actions for satlab by default.
		c.AssetType = "dut"

		c.Flags.StringVar(&c.Address, "address", "", "IP address of host")
		c.Flags.BoolVar(&c.SkipDNS, "skip-dns", false, "whether to skip updating the DNS")
		registerAddShivasFlags(c)
		return c
	},
}

// AddDUT contains the arguments for "satlab add dut ...". It also contains additional
// qualified arguments that are the result of adding the satlab prefix to "raw" arguments.
type addDUTCmd struct {
	subcommands.CommandRunBase

	authFlags authcli.Flags

	dut.AddDUT
}

// Run adds a DUT and returns an exit status.
func (c *addDUTCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// InnerRun is the implementation of run.
func (c *addDUTCmd) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)

	return c.TriggerRun(ctx, &executor.ExecCommander{}, os.Stdout)
}
