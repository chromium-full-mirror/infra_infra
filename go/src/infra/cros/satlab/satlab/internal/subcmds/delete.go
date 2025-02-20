// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subcmds

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/satlab/internal/commands/dns"
	"go.chromium.org/infra/cros/satlab/satlab/internal/components/dut"
)

// DeleteBase is the placeholder for the delete command.
type deleteBase struct {
	subcommands.CommandRunBase
}

// DeleteCmd contains the usage and implementation for the delete command.
var DeleteCmd = &subcommands.Command{
	UsageLine: "delete <sub-command>",
	CommandRun: func() subcommands.CommandRun {
		c := &deleteBase{}
		return c
	},
}

// DeleteApp is an application for the delete commands. Control is transferred here
// when consuming the "delete" subcommand.
type deleteApp struct {
	cli.Application
}

// GetName fulfills the cli.Application interface's method call which lets us print the correct usage
// alternatively we could define another Application with the `satlab get` name like in the subcommands
// https://github.com/maruel/subcommands/blob/main/sample-complex/ask.go#L13
func (c deleteApp) GetName() string {
	return fmt.Sprintf("%s delete", site.AppPrefix)
}

// Run transfers control to the subcommands of delete.
func (c *deleteBase) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	return subcommands.Run(&deleteApp{*d}, args)
}

// GetCommands lists the subcommands of delete.
func (c deleteApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		dut.DeleteDUTCmd,
		dns.DeleteDNSCmd,
	}
}
