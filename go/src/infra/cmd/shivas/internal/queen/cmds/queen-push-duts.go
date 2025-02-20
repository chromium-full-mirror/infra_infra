// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queen

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/appengine/drone-queen/api"
	"go.chromium.org/infra/cmd/shivas/internal/ufs/subcmds/host"
	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// PushDuts subcommand: Inspect drone queen DUT info.
var PushDuts = &subcommands.Command{
	UsageLine: "queen-push-duts",
	ShortDesc: "Push drone queen DUTs",
	LongDesc: `Push drone queen DUTs.

This command is for pushing drone queen assigned DUTs.
Do not use this command as part of scripts or pipelines.
This command is unstable.

You must be in the respective inspectors group to use this.`,
	CommandRun: func() subcommands.CommandRun {
		c := &pushDutsRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)
		return c
	},
}

type pushDutsRun struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags
}

func (c *pushDutsRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, errors.Annotate(err, "queen-push-duts").Err())
		return 1
	}
	return 0
}

func (c *pushDutsRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, ufsUtil.OSNamespace)
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UFS service %s\n", e.UnifiedFleetService)
		fmt.Printf("Using Drone Queen service %s\n", e.QueenService)
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})
	// Get all the MachineLSEs
	// Set keysOnly to true to get only keys
	res, err := utils.BatchList(ctx, ic, host.ListHosts, nil, 0, true, false, nil)
	availableDuts := make([]*api.DeclareDutsRequest_Dut, len(res))
	for i, r := range res {
		lse := r.(*ufspb.MachineLSE)
		lse.Name = ufsUtil.RemovePrefix(lse.Name)
		var h string
		if lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation() != nil {
			h = lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetHive()
		} else {
			h = lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetHive()
		}
		availableDuts[i] = &api.DeclareDutsRequest_Dut{
			Name: lse.GetName(),
			Hive: ufsUtil.GetHiveForDut(lse.GetName(), h),
		}
	}
	qc := api.NewInventoryProviderPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.QueenService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})
	fmt.Printf("DUTs to declare(%d): %+v", len(availableDuts), availableDuts)
	_, err = qc.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
	return err
}
