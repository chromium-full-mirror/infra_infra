// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	fleet "infra/appengine/crosskylabadmin/api/fleet/v1"
	"infra/cmd/shivas/cmdhelp"
	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	"infra/libs/skylab/autotest/hostinfo"
	ufsUtil "infra/unifiedfleet/app/util"
)

// GetStableVersionCmd get stable version for given host and board/model.
var GetStableVersionCmd = &subcommands.Command{
	UsageLine: "stable-version ...",
	ShortDesc: "Get stable version used for auto-repair.",
	LongDesc:  cmdhelp.GetStableVersionText,
	CommandRun: func() subcommands.CommandRun {
		c := &getStableVersion{
			pools: []string{},
		}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.StringVar(&c.hostname, "name", "", "Hostname which used to get version.")
		c.Flags.StringVar(&c.deviceType, "type", "", "Type of device used to get version.")
		c.Flags.StringVar(&c.board, "board", "", "Name of the board used to get version.")
		c.Flags.StringVar(&c.model, "model", "", "Name of the model used to get version.")
		c.Flags.Var(utils.CSVString(&c.pools), "pools", "comma separated pools used to get version.")

		return c
	},
}

type getStableVersion struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	board      string
	model      string
	pools      []string
	hostname   string
	deviceType string
}

func (c *getStableVersion) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *getStableVersion) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	err := c.validateArgs(args)
	if err != nil {
		return err
	}
	ctx := cli.GetContext(a, c, env)
	ns, err := getNamespace(&c.envFlags)
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
		fmt.Printf("Using CrosSkylabAdmin service: %s\n", e.AdminService)
	}

	invWithSVClient := fleet.NewInventoryPRPCClient(
		&prpc.Client{
			C:       hc,
			Host:    e.AdminService,
			Options: site.DefaultPRPCOptions(c.envFlags),
		},
	)
	g := hostinfo.NewGetter(nil, invWithSVClient)

	version, err := g.GetStableVersion(ctx, c.deviceType, c.hostname, c.board, c.model, c.pools)
	if err != nil {
		return err
	}
	fmt.Printf("Stable version for host:%s\n", c.hostname)
	fmt.Printf("\t %s: %s\n", "OsVersion", version.GetOsVersion())
	fmt.Printf("\t %s: %s\n", "OsImagePath", version.GetOsImagePath())
	fmt.Printf("\t %s: %s\n", "FwRoVersion", version.GetFirmwareRoVersion())
	fmt.Printf("\t %s: %s\n", "FwRoImagePath", version.GetFirmwareRoImagePath())
	return nil
}

func (c *getStableVersion) validateArgs(args []string) error {
	if len(args) > 0 {
		return cmdlib.NewUsageError(c.Flags, "Please use -name to provide hostname of the device.")
	}
	switch {
	case c.hostname != "":
		// We have name that should enough.
		return nil
	case c.board == "" || c.model == "":
		return cmdlib.NewUsageError(c.Flags, "Please provide hostname of the device or board+model.")
	}
	return nil
}

// getNamespace returns the namespace used to call UFS with appropriate
// validation and default behavior. It is primarily separated from the main
// function for testing purposes
func getNamespace(c *site.EnvFlags) (string, error) {
	if c == nil {
		return ufsUtil.OSNamespace, nil
	}
	return c.Namespace(site.OSLikeNamespaces, ufsUtil.OSNamespace)
}
