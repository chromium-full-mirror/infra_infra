// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"encoding/json"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/flag"

	"go.chromium.org/infra/cmd/shivas/cmdhelp"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/cros/satlab/common/dut"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

// GetDUTCmd is the implementation of "satlab get dut ...".
var GetDUTCmd = &subcommands.Command{
	UsageLine: "dut [options ...]",
	ShortDesc: "Get a Satlab DUT",
	CommandRun: func() subcommands.CommandRun {
		c := &getDUTCmd{}
		registerGetShivasFlags(c)
		return c
	},
}

// GetDUT holds the arguments for "satlab get dut ...".
type getDUTCmd struct {
	subcommands.CommandRunBase

	authFlags authcli.Flags

	dut.GetDUT
}

// Run runs the get DUT subcommand.
func (c *getDUTCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// InnerRun runs the get command.
func (c *getDUTCmd) innerRun(
	a subcommands.Application,
	args []string,
	env subcommands.Env,
) error {
	ctx := cli.GetContext(a, c, env)

	resp, err := c.GetDUT.TriggerRun(ctx, &executor.ExecCommander{}, args)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		for _, r := range resp {
			fmt.Printf("%v\n", r)
		}
	} else {
		fmt.Printf("%v\n", string(b))
	}

	return nil
}

// RegisterGetShivasFlags registers the flags inherited from shivas.
func registerGetShivasFlags(c *getDUTCmd) {
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)

	c.Flags.StringVar(&c.SatlabID, "satlab-id", "", "the ID for the satlab in question")
	c.Flags.StringVar(&c.Namespace, "namespace", "", "namespace where data resides.")

	c.Flags.Var(
		flag.StringSlice(&c.Zones),
		"zone",
		"Name(s) of a zone to filter by. Can be specified multiple times."+cmdhelp.ZoneFilterHelpText,
	)
	c.Flags.Var(
		flag.StringSlice(&c.Racks),
		"rack",
		"Name(s) of a rack to filter by. Can be specified multiple times.",
	)
	c.Flags.Var(
		flag.StringSlice(&c.Machines),
		"machine",
		"Name(s) of a machine/asset to filter by. Can be specified multiple times.",
	)
	c.Flags.Var(
		flag.StringSlice(&c.Prototypes),
		"prototype",
		"Name(s) of a host prototype to filter by. Can be specified multiple times.",
	)
	c.Flags.Var(
		flag.StringSlice(&c.Tags),
		"tag",
		"Name(s) of a tag to filter by. Can be specified multiple times.",
	)
	c.Flags.Var(
		flag.StringSlice(&c.States),
		"state",
		"Name(s) of a state to filter by. Can be specified multiple times."+cmdhelp.StateFilterHelpText,
	)
	c.Flags.Var(
		flag.StringSlice(&c.Servos),
		"servo",
		"Name(s) of a servo:port to filter by. Can be specified multiple times.",
	)
	c.Flags.Var(
		flag.StringSlice(&c.Servotypes),
		"servotype",
		"Name(s) of a servo type to filter by. Can be specified multiple times.",
	)
	c.Flags.Var(
		flag.StringSlice(&c.Switches),
		"switch",
		"Name(s) of a switch to filter by. Can be specified multiple times.",
	)
	c.Flags.Var(
		flag.StringSlice(&c.Rpms),
		"rpm",
		"Name(s) of a rpm to filter by. Can be specified multiple times.",
	)
	c.Flags.Var(
		flag.StringSlice(&c.Pools),
		"pools",
		"Name(s) of a tag to filter by. Can be specified multiple times.",
	)
	c.Flags.BoolVar(&c.HostInfoStore, "host-info-store", false, "write host info store to stdout")

}
