// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package topology

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/topology"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	"go.chromium.org/infra/cros/satlab/common/utils/misc"
)

var GetTopologyCmd = &subcommands.Command{
	UsageLine: "topology [options ...]",
	ShortDesc: "get PASIT topology",
	LongDesc:  "Get PASIT topology for given DUT.",
	CommandRun: func() subcommands.CommandRun {
		c := &getTopologyCmd{}
		registerGetTopologyFlags(c)
		return c
	},
}

// getTopologyCmd store the command line arguments needed for the "satlab get topology" subcommand.
type getTopologyCmd struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags

	topology.GetTopology
}

// Run is the implementation of the "satlab get topology" subcommand.
func (c *getTopologyCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// InnerRun triggers extracting topology from the DUT.
func (c *getTopologyCmd) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	topologyJson, err := c.TriggerRun(ctx, &executor.ExecCommander{})
	if err != nil {
		return errors.Annotate(err, "get PASIT topology from the UFS").Err()
	}
	str, err := misc.TopologyJsonToStr(topologyJson)
	if err != nil {
		return errors.Annotate(err, "convert PASIT topology to string in textproto format").Err()
	}
	fmt.Print(str)
	return nil
}

func registerGetTopologyFlags(c *getTopologyCmd) {
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
	c.Flags.StringVar(&c.SatlabID, "satlab-id", "", "the ID for the satlab in question")

	c.Flags.StringVar(&c.Hostname, "hostname", "", "hostname of the DUT")
}
