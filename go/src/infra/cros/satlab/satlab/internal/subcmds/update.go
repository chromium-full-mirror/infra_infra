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

// UpdateBase is the type for the add placeholder command.
type updateBase struct {
	subcommands.CommandRunBase
}

// UpdateCmd is the add placeholder command.
var UpdateCmd = &subcommands.Command{
	UsageLine: "update <sub-command>",
	CommandRun: func() subcommands.CommandRun {
		c := &updateBase{}
		return c
	},
}

// UpdateApp is the placeholder application for the update command.
type updateApp struct {
	cli.Application
}

// GetName fulfills the cli.Application interface's method call which lets us print the correct usage
// alternatively we could define another Application with the `satlab get` name like in the subcommands
// https://github.com/maruel/subcommands/blob/main/sample-complex/ask.go#L13
func (c updateApp) GetName() string {
	return fmt.Sprintf("%s update", site.AppPrefix)
}

// Run transfers control to the add subcommands.
func (c *updateBase) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	return subcommands.Run(&updateApp{*d}, args)
}

// GetCommands lists the add subcommands.
func (c updateApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		// TODO(gregorynisbet): Satlab update DUT is currently disabled. Please uncomment this line
		//                      once updating DUTs is supported on satlab.
		// dut.UpdateDUTCmd,
		dns.UpdateDNSCmd,
		dut.UpdateDUTStateCmd,
	}
}
