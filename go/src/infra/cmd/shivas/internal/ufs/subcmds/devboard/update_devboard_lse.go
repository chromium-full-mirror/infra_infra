// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devboard

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/cmdhelp"
	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// UpdateDevboardLSECmd updates the devboard machineLSE for a given name.
var UpdateDevboardLSECmd = &subcommands.Command{
	UsageLine:  "devboard-lse ...",
	ShortDesc:  "Update devboard lse details by filters",
	LongDesc:   cmdhelp.UpdateDevboardLSEText,
	CommandRun: updateDevboardLSEcommandRun,
}

func updateDevboardLSEcommandRun() subcommands.CommandRun {
	c := &updateDevboardLSE{}
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
	c.envFlags.Register(&c.Flags)
	c.commonFlags.Register(&c.Flags)

	c.Flags.StringVar(&c.name, "name", "", "The name of the devboard machine to update.")
	c.Flags.Var(utils.CSVString(&c.pools), "pools", "comma separated pools append to the devboard."+cmdhelp.ClearFieldHelpText)
	c.Flags.Var(utils.CSVString(&c.removePools), "removePools", "comma separated pools to remove.")
	return c
}

type updateDevboardLSE struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	name        string
	pools       []string
	removePools []string
}

func (c *updateDevboardLSE) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *updateDevboardLSE) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
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

	if _, err = utils.PrintExistingDevboardLSE(ctx, ic, c.name); err != nil {
		return err
	}

	lse, err := ic.GetMachineLSE(ctx, &ufsAPI.GetMachineLSERequest{
		Name: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, c.name),
	})
	if err != nil {
		return err
	}
	if err := utils.IsDevboard(lse); err != nil {
		return err
	}
	c.parseArgs(lse)

	res, err := ic.UpdateMachineLSE(ctx, &ufsAPI.UpdateMachineLSERequest{
		MachineLSE: lse,
		UpdateMask: utils.GetUpdateMask(&c.Flags, map[string]string{
			"pools":       ufsUtil.DevboardPoolsPath,
			"removePools": ufsUtil.DevboardPoolsRemovePath,
		}),
	})
	if err != nil {
		return err
	}
	res.Name = ufsUtil.RemovePrefix(res.Name)
	fmt.Println("The devboard machine after update:")
	utils.PrintProtoJSON(res, !utils.NoEmitMode(false))
	fmt.Println("Successfully updated the devboard machine: ", res.Name)
	return nil
}

func (c *updateDevboardLSE) parseArgs(lse *ufspb.MachineLSE) {
	devboard := &chromeosLab.Devboard{}
	if ufsUtil.ContainsAnyStrings(c.pools, utils.ClearFieldValue) {
		devboard.Pools = nil
	} else {
		devboard.Pools = c.pools
	}
	if c.removePools != nil {
		devboard.Pools = c.removePools
	}
	lse.GetChromeosMachineLse().GetDeviceLse().Device = &ufspb.ChromeOSDeviceLSE_Devboard{
		Devboard: devboard,
	}
}

func (c *updateDevboardLSE) validateArgs() error {
	if c.name == "" {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-name' is required.")
	}
	if len(c.pools) != 0 && len(c.removePools) != 0 {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-pools' and '-pools-to-remove' cannot be specified at the same time.")
	}
	return nil
}
