// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package state

import (
	"context"
	"fmt"
	"time"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/cros/dutstate"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// DutStateCmd subcommand: get State of the DUT from UFS.
var DutStateCmd = &subcommands.Command{
	UsageLine: "dut-state [FLAGS...]",
	ShortDesc: "read state for a DUT",
	LongDesc:  `read DUT state from UFS.`,
	CommandRun: func() subcommands.CommandRun {
		c := &dutStateCmdRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)
		c.Flags.BoolVar(&c.dolos, "dolos", false, "Lookup state for dolos peripheral instead of DUT")
		return c
	},
}

type dutStateCmdRun struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	dolos bool
}

// Run implements the subcommands.CommandRun interface.
func (c *dutStateCmdRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *dutStateCmdRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.validateArgs(args); err != nil {
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
		fmt.Printf("Using UnifiedFleet service %s\n", e.UnifiedFleetService)
	}
	ufsClient := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})
	host := args[0]
	if c.dolos {
		return c.getDolosState(ctx, ufsClient, host)
	}
	i := dutstate.Read(ctx, ufsClient, host)
	fmt.Printf("%s: %s\n", host, i.State.String())
	if c.commonFlags.Verbose() {
		fmt.Printf("Updated at:%s \n", time.Unix(i.Time, 0))
	}
	return nil
}

func (c *dutStateCmdRun) getDolosState(ctx context.Context, ufsClient ufsAPI.FleetClient, host string) error {
	req := &ufsAPI.GetDeviceDataRequest{
		Hostname: host,
	}
	resp, err := ufsClient.GetDeviceData(ctx, req)
	if err != nil {
		return err
	}
	switch resp.GetResourceType() {
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE:
		fmt.Printf("Dolos state of %s: %s\n", host, resp.GetChromeOsDeviceData().GetDutState().GetDolosState())
		return nil
	default:
		return fmt.Errorf("dolos state does not valid for Non-ChromeOS device")
	}
}

func (c *dutStateCmdRun) validateArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("Expected an args to provide a host")
	}
	if len(args) > 1 {
		return fmt.Errorf("Expected an args to provide only one host: %s", args)
	}
	return nil
}
