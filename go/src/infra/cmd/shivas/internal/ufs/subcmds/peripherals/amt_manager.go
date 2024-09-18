// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package peripherals

import (
	"fmt"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/cmdhelp"
	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	lab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	rpc "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/util"
)

var (
	AddPeripheralAMTCmd    = amtManagerCmd(actionAdd)
	DeletePeripheralAMTCmd = amtManagerCmd(actionDelete)
)

// amtManagerCmd creates a command for adding, removing or updating AMT details for a DUT.
func amtManagerCmd(mode action) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "peripheral-amt -dut {DUT name} -amt-hostname {amt hostname} [-use-tls]",
		ShortDesc: "Manage AMT details for a DUT",
		LongDesc:  cmdhelp.ManagePeripheralAMTLongDesc,
		CommandRun: func() subcommands.CommandRun {
			c := manageAmtManagerCmd{mode: mode}
			c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
			c.envFlags.Register(&c.Flags)
			c.commonFlags.Register(&c.Flags)

			c.Flags.StringVar(&c.dutName, "dut", "", "DUT name to update")
			c.Flags.StringVar(&c.amtHostname, "amt-hostname", "", "AMT hostname.")
			c.Flags.BoolVar(&c.useTLS, "use-tls", false, "Connect to AMT using TLS on port 16993.")
			return &c
		},
	}
}

// manageAmtManagerCmd supports adding or deleting Intel AMT (vPro).
type manageAmtManagerCmd struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	dutName     string
	amtHostname string
	useTLS      bool
	amtObj      *lab.AMTManager

	mode action
}

// Run executes the AMT management subcommand.
func (c *manageAmtManagerCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.run(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// run implements the core logic for Run.
func (c *manageAmtManagerCmd) run(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.cleanAndValidateFlags(); err != nil {
		return err
	}
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, util.OSNamespace)

	//TODO: This block appears in multiple files.
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UFS service %s\n", e.UnifiedFleetService)
	}

	client := rpc.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions,
	})

	lse, err := client.GetMachineLSE(ctx, &rpc.GetMachineLSERequest{
		Name: util.AddPrefix(util.MachineLSECollection, c.dutName),
	})
	if err != nil {
		return err
	}
	if err := utils.IsDUT(lse); err != nil {
		return errors.Annotate(err, "not a dut").Err()
	}

	if c.commonFlags.Verbose() {
		fmt.Println("New AMT manager", c.amtObj)
	}

	peripherals := lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
	peripherals.AmtManager = c.amtObj

	_, err = client.UpdateMachineLSE(ctx, &rpc.UpdateMachineLSERequest{MachineLSE: lse})
	return err
}

// cleanAndValidateFlags returns and error with the results of all validations.
func (c *manageAmtManagerCmd) cleanAndValidateFlags() error {
	var errStrs []string
	c.dutName = strings.TrimSpace(c.dutName)
	if len(c.dutName) == 0 {
		errStrs = append(errStrs, errDUTMissing)
	}
	// Set the amtObj based on action.
	switch c.mode {
	case actionAdd:
		c.amtHostname = strings.TrimSpace(c.amtHostname)
		if c.amtHostname == "" {
			errStrs = append(errStrs, "'-amt-hostname' is required")
		}
		c.amtObj = &lab.AMTManager{Hostname: c.amtHostname,
			UseTls: c.useTLS}
	case actionDelete:
		c.amtObj = nil
	default:
		return errors.Reason("unknown action: %d", c.mode).Err()
	}
	return c.checkErrStr(errStrs)
}

func (c *manageAmtManagerCmd) checkErrStr(errStrs []string) error {
	if len(errStrs) == 0 {
		return nil
	}
	return cmdlib.NewQuietUsageError(c.Flags, fmt.Sprintf("Wrong usage!!\n%s", strings.Join(errStrs, "\n")))
}
