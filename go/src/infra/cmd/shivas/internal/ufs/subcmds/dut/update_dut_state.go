// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dut related to DUT operations
package dut

import (
	"fmt"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	"infra/cros/dutstate"
	ufsProto "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

// UpdateDUTStateCmd update DUT state for a given DUT name.
var UpdateDUTStateCmd = &subcommands.Command{
	UsageLine: "dut-state [options]",
	ShortDesc: "Update a DUT's state",
	LongDesc: `Update a DUT's state in ChromeOS Lab.

### Please use with caution, consult with Fleet Infra SW team first ###

Example:

shivas update dut-state -dut-name=host1 -dut-state=needs_repair

Gets success or error message.`,
	CommandRun: func() subcommands.CommandRun {
		c := &updateDutState{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.StringVar(&c.dutName, "dut-name", "", "hostname of the DUT.")
		c.Flags.StringVar(&c.dutState, "dut-state", "", "dut state of the DUT to update. Valid states: "+strings.Join(dutstate.ValidDUTStateStrings, ", "))
		return c
	},
}

type updateDutState struct {
	subcommands.CommandRunBase
	commonFlags site.CommonFlags
	authFlags   authcli.Flags
	envFlags    site.EnvFlags

	dutName  string
	dutState string
}

func (c *updateDutState) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *updateDutState) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	ns, err := c.envFlags.Namespace(site.OSLikeNamespaces, ufsUtil.OSNamespace)
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

	ufsState := dutstate.State(c.dutState)
	if dutstate.ConvertToUFSState(ufsState) == ufsProto.State_STATE_UNSPECIFIED {
		return fmt.Errorf("invalid DUT State: %q, Valid States: [%s]", c.dutState, strings.Join(dutstate.ValidDUTStateStrings, ", "))
	}
	if err := dutstate.Update(ctx, ic, c.dutName, ufsState); err != nil {
		return err
	}
	fmt.Printf("Successfully updated state of DUT %q to state %q\n", c.dutName, ufsState)
	return nil
}
