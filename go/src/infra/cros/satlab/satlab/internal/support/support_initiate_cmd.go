// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package support contains functionality around remote port forwarding.
package support

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"infra/cros/satlab/common/dut"
	"infra/cros/satlab/common/utils/executor"
)

// SupportCmd contains the usage and implementation for the support initiate command
var SupportCmd = &subcommands.Command{
	UsageLine: "initiate <args>",
	ShortDesc: `Initiate support`,
	LongDesc:  `Opens a connection to provide remote support. To use this, you need a project on Google Cloud Platform configured according to the instructions provided`,
	CommandRun: func() subcommands.CommandRun {
		c := &initiateSupportRun{}
		c.Flags.StringVar(&c.ProjectID, "project-id", "", "The Google Cloud project ID to use")
		c.Flags.StringVar(&c.Zone, "zone", "", "Google Cloud Platform zone close to your lab location")
		c.Flags.StringVar(&c.Network, "network", "satlab-support ", "Specifies the network that the VM instance will be a part of")
		c.Flags.StringVar(&c.Timeout, "timeout", "1h", "The duration you want this VM to run before being automatically terminated. Format the duration as the number of days, hours, minutes, and seconds followed by d, h, m, and s respectively. For example, specify 30m for a duration of 30 minutes, or specify 1d2h3m4s for a duration of 1 day, 2 hours, 3 minutes, and 4 seconds. The minimum duration is 30 seconds (30s) and the maximum duration is 120 days (120d).")
		c.Flags.IntVar(&c.Port, "port", 22, "The port that will be forwarded")
		return c
	},
}

// initiateSupportRun struct contains the arguments needed to run SupportCmd and common fields for the methods
type initiateSupportRun struct {
	subcommands.CommandRunBase

	dut.InitiateSupportRun
}

// Run is what is called when a user inputs the support initiate command
func (c *initiateSupportRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun handles all orchestration needed to pass appropriate clients, contexts and commands into the application
func (c *initiateSupportRun) innerRun(a subcommands.Application, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	err := c.TriggerRun(ctx, &executor.ExecCommander{})
	return err
}
