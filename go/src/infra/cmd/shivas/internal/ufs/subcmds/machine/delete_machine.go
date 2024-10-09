// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package machine

import (
	"fmt"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

// DeleteMachineCmd delete Machine by given name.
var DeleteMachineCmd = &subcommands.Command{
	UsageLine: "machine {Machine Name}",
	ShortDesc: "Delete a machine(Hardware asset: ChromeBook, Bare metal server, Macbook.)",
	LongDesc: `Delete a machine(Hardware asset: ChromeBook, Bare metal server, Macbook.).

Example:
shivas delete machine {Machine Name}
Deletes the given machine and deletes the nics and drac associated with this machine.`,
	CommandRun: func() subcommands.CommandRun {
		c := &deleteMachine{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		// This is to overwrite args if this is specified
		c.Flags.StringVar(&c.machineName, "name", "", "the name of the machine to delete, if this is specified, all other filters will be dropped")
		return c
	},
}

type deleteMachine struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	machineName string
}

func (c *deleteMachine) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *deleteMachine) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.validateArgs(); err != nil {
		return err
	}
	ctx := cli.GetContext(a, c, env)
	ns, err := c.envFlags.Namespace(nil, "")
	if err != nil {
		return err
	}
	ctx = utils.SetupContext(ctx, ns)
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UFS service %s\n", e.UnifiedFleetService)
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})
	if _, err := utils.PrintExistingMachine(ctx, ic, args[0]); err != nil {
		return err
	}
	prompt := utils.CLIPrompt(a.GetOut(), os.Stdin, false)
	if prompt != nil && !prompt(fmt.Sprintf("Are you sure you want to delete the machine together with its nics & drac: %s. ", args[0])) {
		return nil
	}

	if c.machineName != "" {
		args = []string{c.machineName}
	}
	_, err = ic.DeleteMachine(ctx, &ufsAPI.DeleteMachineRequest{
		Name: ufsUtil.AddPrefix(ufsUtil.MachineCollection, args[0]),
	})
	if err == nil {
		fmt.Fprintln(a.GetOut(), args[0], "is deleted successfully.")
		return nil
	}
	return err
}

func (c *deleteMachine) validateArgs() error {
	if c.Flags.NArg() == 0 && c.machineName == "" {
		return cmdlib.NewUsageError(c.Flags, "Please provide the name via positional arguments or flag `-name`")
	}
	return utils.ValidateNameAndPositionalArg(c.Flags, c.machineName)
}
