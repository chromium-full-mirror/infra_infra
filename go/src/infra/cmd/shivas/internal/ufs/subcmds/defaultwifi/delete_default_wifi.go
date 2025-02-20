// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package defaultwifi

import (
	"fmt"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// DeleteDefaultWifiCmd delete DefaultWifi by given name.
var DeleteDefaultWifiCmd = &subcommands.Command{
	UsageLine: "defaultwifi",
	ShortDesc: "Delete DefaultWifi",
	LongDesc: `Delete DefaultWifi.

Example:
shivas delete defaultwifi {DefaultWifi Name}
Deletes the given DefaultWifi.`,
	CommandRun: func() subcommands.CommandRun {
		c := &deleteDefaultWifi{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.BoolVar(&c.skipYes, "yes", false, "Skip yes option by saying yes.")
		return c
	},
}

type deleteDefaultWifi struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	skipYes bool
}

func (c *deleteDefaultWifi) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *deleteDefaultWifi) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.validateArgs(); err != nil {
		return err
	}
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, ufsUtil.OSNamespace)
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
	if err := utils.PrintExistingDefaultWifi(ctx, ic, args[0]); err != nil {
		return err
	}
	if !c.skipYes {
		prompt := utils.CLIPrompt(a.GetOut(), os.Stdin, false)
		if prompt != nil && !prompt(fmt.Sprintf("Are you sure you want to delete the DefaultWifi: %s", args[0])) {
			return nil
		}
	}

	_, err = ic.DeleteDefaultWifi(ctx, &ufsAPI.DeleteDefaultWifiRequest{
		Name: ufsUtil.AddPrefix(ufsUtil.DefaultWifiCollection, args[0]),
	})
	if err == nil {
		fmt.Fprintln(a.GetOut(), args[0], "is deleted successfully.")
		return nil
	}
	return err
}

func (c *deleteDefaultWifi) validateArgs() error {
	if c.Flags.NArg() == 0 {
		return cmdlib.NewUsageError(c.Flags, "Please provide the DefaultWifi name to be deleted.")
	}
	return nil
}
