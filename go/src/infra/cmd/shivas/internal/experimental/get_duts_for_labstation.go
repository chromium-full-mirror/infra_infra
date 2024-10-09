// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package experimental

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	"infra/libs/skylab/common/heuristics"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

// GetDutsForLabstationCmd gets the DUTs associated with a labstation.
var GetDutsForLabstationCmd = &subcommands.Command{
	UsageLine: "get-duts-for-labstation [duts]",
	ShortDesc: "get the DUTs attached to a labstation",
	LongDesc: `get duts for labstation gets the DUTs for a labstation.

	./shivas get-duts-for-labstation [duts]`,
	CommandRun: func() subcommands.CommandRun {
		c := &getDutsForLabstationRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		return c
	},
}

type getDutsForLabstationRun struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags
}

// getNamespace returns the namespace used to call UFS with appropriate
// validation and default behavior. It is primarily separated from the main
// function for testing purposes
func (c *getDutsForLabstationRun) getNamespace() (string, error) {
	return c.envFlags.Namespace(site.OSLikeNamespaces, ufsUtil.OSNamespace)
}

func (c *getDutsForLabstationRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *getDutsForLabstationRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) (err error) {
	ctx := cli.GetContext(a, c, env)
	ns, err := c.getNamespace()
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
	for i, arg := range args {
		args[i] = heuristics.NormalizeBotNameToDeviceName(arg)
	}
	resp, err := ic.GetDUTsForLabstation(ctx, &ufsAPI.GetDUTsForLabstationRequest{
		Hostname: args,
	})
	if err != nil {
		return err
	}
	for _, labstationMapping := range resp.GetItems() {
		for _, dutName := range labstationMapping.GetDutName() {
			fmt.Printf("%q -> %q\n", labstationMapping.GetHostname(), dutName)
		}
	}
	return err
}
