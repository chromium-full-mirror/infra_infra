// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"fmt"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// DeleteVMCmd deletes vm on a host.
var DeleteVMCmd = &subcommands.Command{
	UsageLine: "vm {VM name}",
	ShortDesc: "Delete a VM on a host",
	LongDesc: `Delete a VM on a host.

Example:
shivas delete vm {VM name}
Deletes the VM on a host.`,
	CommandRun: func() subcommands.CommandRun {
		c := &deleteVM{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		return c
	},
}

type deleteVM struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags
}

func (c *deleteVM) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *deleteVM) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
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
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})
	if err := utils.PrintExistingVM(ctx, ic, args[0]); err != nil {
		return err
	}
	prompt := utils.CLIPrompt(a.GetOut(), os.Stdin, false)
	if prompt != nil && !prompt(fmt.Sprintf("Are you sure you want to delete the VM: %s", args[0])) {
		return nil
	}
	_, err = ic.DeleteVM(ctx, &ufsAPI.DeleteVMRequest{
		Name: ufsUtil.AddPrefix(ufsUtil.VMCollection, args[0]),
	})
	if err == nil {
		fmt.Fprintln(a.GetOut(), args[0], "is deleted successfully.")
		return nil
	}
	return errors.Fmt("Unable to delete the VM on the host: %w", err)
}

func (c *deleteVM) validateArgs() error {
	if c.Flags.NArg() == 0 {
		return cmdlib.NewUsageError(c.Flags, "Please provide the name of the VM to delete.")
	}
	return nil
}
