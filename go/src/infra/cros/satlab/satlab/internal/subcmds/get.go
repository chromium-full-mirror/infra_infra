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

// GetBase is a placeholder command for "get".
type getBase struct {
	subcommands.CommandRunBase
}

// GetCmd is a placeholder command for get.
var GetCmd = &subcommands.Command{
	UsageLine: "get <sub-command>",
	CommandRun: func() subcommands.CommandRun {
		c := &getBase{}
		return c
	},
}

// GetApp is an application tha tholds the get subcommands.
type getApp struct {
	cli.Application
}

// GetName fulfills the cli.Application interface's method call which lets us print the correct usage
// alternatively we could define another Application with the `satlab get` name like in the subcommands
// https://github.com/maruel/subcommands/blob/main/sample-complex/ask.go#L13
func (c getApp) GetName() string {
	return fmt.Sprintf("%s get", site.AppPrefix)
}

// Run transfers control to a subcommand.
func (c *getBase) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	return subcommands.Run(&getApp{*d}, args)
}

// GetCommands lists the available subcommands.
func (c getApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		dut.GetDUTCmd,
		dns.GetDNSCmd,
	}
}
