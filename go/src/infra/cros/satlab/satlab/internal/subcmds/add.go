// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subcmds

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/satlab/internal/components/dut"
)

// AddBase is the type for the add placeholder command.
type addBase struct {
	subcommands.CommandRunBase
}

// AddCmd is the add placeholder command.
var AddCmd = &subcommands.Command{
	UsageLine: "add <sub-command>",
	CommandRun: func() subcommands.CommandRun {
		c := &addBase{}
		return c
	},
}

// AddApp is the placeholder application for the add command.
type addApp struct {
	cli.Application
}

// GetName fulfills the cli.Application interface's method call which lets us print the correct usage
// alternatively we could define another Application with the `satlab get` name like in the subcommands
// https://github.com/maruel/subcommands/blob/main/sample-complex/ask.go#L13
func (c addApp) GetName() string {
	return fmt.Sprintf("%s add", site.AppPrefix)
}

// Run transfers control to the add subcommands.
func (c *addBase) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	return subcommands.Run(&addApp{*d}, args)
}

// GetCommands lists the add subcommands.
func (c addApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		dut.AddDUTCmd,
	}
}
