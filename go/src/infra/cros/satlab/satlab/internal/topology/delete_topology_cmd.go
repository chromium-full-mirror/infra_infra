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

var DeleteTopologyCmd = &subcommands.Command{
	UsageLine: "topology [options ...]",
	ShortDesc: "delete PASIT topology",
	LongDesc:  "Delete PASIT topology from given DUT.",
	CommandRun: func() subcommands.CommandRun {
		c := &deleteTopologyCmd{}
		registerDeleteTopologyFlags(c)
		return c
	},
}

// deleteTopologyCmd store the command line arguments needed for the "satlab delete topology" subcommand.
type deleteTopologyCmd struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags

	topology.DeleteTopology
}

// Run is the implementation of the "satlab delete topology" subcommand.
func (c *deleteTopologyCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// InnerRun triggers removing topology from the DUT.
func (c *deleteTopologyCmd) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	return c.TriggerRun(ctx, &executor.ExecCommander{})
}

func registerDeleteTopologyFlags(c *deleteTopologyCmd) {
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
	c.Flags.StringVar(&c.SatlabID, "satlab-id", "", "the ID for the satlab in question")

	c.Flags.StringVar(&c.Hostname, "hostname", "", "hostname of the DUT")
}
