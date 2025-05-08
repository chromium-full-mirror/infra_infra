// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package topology

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/topology"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

var AddTopologyCmd = &subcommands.Command{
	UsageLine: "topology  [options ...]",
	ShortDesc: "add PASIT topology",
	LongDesc:  "Add PASIT topology to given DUT.",
	CommandRun: func() subcommands.CommandRun {
		c := &addTopologyCmd{}
		registerAddTopologyFlags(c)
		return c
	},
}

// addTopologyCmd store the command line arguments needed for the "satlab add topology" subcommand.
type addTopologyCmd struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags

	topology.AddTopology
}

// Run is the implementation of the "satlab add topology" subcommand.
func (c *addTopologyCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// InnerRun triggers adding topology to the DUT.
func (c *addTopologyCmd) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	return c.TriggerRun(ctx, &executor.ExecCommander{})
}

func registerAddTopologyFlags(c *addTopologyCmd) {
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
	c.Flags.StringVar(&c.SatlabID, "satlab-id", "", "the ID for the satlab in question")

	c.Flags.StringVar(&c.Hostname, "hostname", "", "hostname of the DUT")
	c.Flags.StringVar(&c.TopologyPath, "file", "", "path to the PASIT topology file")
}
