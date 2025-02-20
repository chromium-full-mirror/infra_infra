// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/cros/satlab/common/services/ufs"
	"go.chromium.org/infra/cros/satlab/common/site"
	ufsProto "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

// UpdateDUTCmd is the command that updates fields for a satlab DUT.
var UpdateDUTStateCmd = &subcommands.Command{
	UsageLine: "dutstate [options ...]",
	ShortDesc: "Update a Satlab DUT's state",
	CommandRun: func() subcommands.CommandRun {
		c := &updateDUTState{}
		registerUpdateDutStateFlags(c)
		return c
	},
}

// UpdateDUT is the 'satlab update dut' command. Its fields are the command line arguments.
type updateDUTState struct {
	updateDUTStateFlags
}

// Run is the main entrypoint to 'satlab update dut'.
func (c *updateDUTState) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	// Confirm required args are provided and no argument conflicts
	if err := c.validateArgs(); err != nil {
		fmt.Fprintln(a.GetErr(), err.Error())
		c.Flags.Usage()
		cmdlib.PrintError(a, err)
		return 1
	}
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// InnerRun is the implementation of 'satlab update dut'.
func (c *updateDUTState) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, c.envFlags.GetNamespace())
	dockerHostBoxIdentifier, err := getDockerHostBoxIdentifier(ctx, c.commonFlags)
	if err != nil {
		return errors.Annotate(err, "update dut state").Err()
	}

	ef := c.envFlags
	ufs, err := ufs.NewUFSClient(ctx, ef.GetUFSService(), &c.authFlags)
	if err != nil {
		cmdlib.PrintError(a, errors.Reason("Error connecting to UFS: %s", err).Err())
		return err
	}

	qualifiedHostname := site.MaybePrepend(site.Satlab, dockerHostBoxIdentifier, c.hostname)
	pinger := DefaultPinger(qualifiedHostname)

	return c.innerRunWithClients(ctx, ufs, qualifiedHostname, pinger)
}

// innerRunWithClients uses interfaces for UFS, pinging the DUT, to allow easy
// testing setups.
func (c *updateDUTState) innerRunWithClients(ctx context.Context, ufs ufs.UFSClient, hostname string, pinger Pinger) error {
	// If user doesn't force the update, we perform any needed checks.
	if !c.force {
		if err := pinger.Ping(); err != nil {
			fmt.Printf("Failed to ping DUT: %s. DUT may need repairs. Re-run the command with `-force` to update DUT without attempting to ping\n", hostname)
			return fmt.Errorf("failed to ping DUT %s", hostname)
		}
	}

	err := updateDUTStateToUFS(ctx, &c.authFlags, ufs, hostname, c.state)
	if err != nil {
		return err
	}
	return nil
}

func (c *updateDUTState) validateArgs() error {
	if c.hostname == "" {
		return errors.Reason("-hostname not specified").Err()
	}
	if c.state == "" {
		return errors.Reason("-state not specified").Err()
	}
	if dutstate.ConvertToUFSState(dutstate.State(c.state)) == ufsProto.State_STATE_UNSPECIFIED {
		return errors.Reason("-state is not valid").Err()
	}
	return nil
}

// updateDUTStateToUFS send DUT state to the UFS service.
func updateDUTStateToUFS(ctx context.Context, authFlags *authcli.Flags, ufs ufs.UFSClient, dutName string, dutState string) error {
	err := dutstate.Update(ctx, ufs, dutName, dutstate.State(dutState))
	if err != nil {
		return errors.Annotate(err, "save local state").Err()
	}
	return nil
}
