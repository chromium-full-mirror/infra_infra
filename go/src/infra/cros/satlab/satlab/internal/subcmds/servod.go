// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subcmds

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/satlab/internal/servod"
)

// servodBase is the type for the servod placeholder command.
type servodBase struct {
	subcommands.CommandRunBase
}

// Run transfers control to the servod subcommands.
func (c *servodBase) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	return subcommands.Run(&servodApp{*d}, args)
}

// ServodCmd is the servod placeholder command.
var ServodCmd = &subcommands.Command{
	UsageLine: "servod <sub-command>",
	ShortDesc: "Commands related to servod containers",
	CommandRun: func() subcommands.CommandRun {
		c := &servodBase{}
		return c
	},
}

// ServodApp is the placeholder application for the servod command.
type servodApp struct {
	cli.Application
}

// GetName fulfills the cli.Application interface's method call which lets us print the correct usage
// alternatively we could define another Application with the `satlab get` name like in the subcommands
// https://github.com/maruel/subcommands/blob/main/sample-complex/ask.go#L13
func (c servodApp) GetName() string {
	return fmt.Sprintf("%s servod", site.AppPrefix)
}

// GetCommands lists the servod subcommands.
func (c servodApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		servod.StartServodCmd,
		servod.StopServodCmd,
	}
}
