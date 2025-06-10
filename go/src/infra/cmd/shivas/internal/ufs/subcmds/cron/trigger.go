// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cron

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
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/cron"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

var CronCmd = &subcommands.Command{
	UsageLine: "cron <cron-job>",
	ShortDesc: "trigger a cron job to run",
	LongDesc:  cmdhelp.TriggerCronDescription,
	CommandRun: func() subcommands.CommandRun {
		c := &cron{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)
		return c
	},
}

type cron struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags
}

func (c *cron) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *cron) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if len(args) != 1 {
		return fmt.Errorf("Need exactly one cron job to cron")
	}
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, ufsUtil.OSNamespace)
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UnifiedFleet service %s\n", e.UnifiedFleetService)
	}
	ic := ufsAPI.NewCronPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})
	_, err = ic.TriggerCronJob(ctx, &ufsAPI.TriggerCronJobReq{
		JobName: args[0],
	})
	return err
}
