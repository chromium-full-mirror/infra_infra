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

// UpdateBase is the type for the placeholder command.
type repairBase struct {
	subcommands.CommandRunBase
}

// RepairCmd is the placeholder command.
var RepairCmd = &subcommands.Command{
	UsageLine: "repair <sub-command> (currently runs only verify task, not full repair stack)",
	CommandRun: func() subcommands.CommandRun {
		c := &repairBase{}
		return c
	},
}

// RepairApp is the placeholder application for the repair command.
type repairApp struct {
	cli.Application
}

// GetName fulfills the cli.Application interface's method call which lets us print the correct usage
// alternatively we could define another Application with the `satlab get` name like in the subcommands
// https://github.com/maruel/subcommands/blob/main/sample-complex/ask.go#L13
func (c repairApp) GetName() string {
	return fmt.Sprintf("%s repair", site.AppPrefix)
}

// Run transfers control to the add subcommands.
func (c *repairBase) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	return subcommands.Run(&repairApp{*d}, args)
}

// GetCommands lists the add subcommands.
func (c repairApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		dut.RepairDUTCmd,
	}
}
