// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package subcmds contains functionality around subcommands of Satlab CLI.
package subcmds

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"infra/cros/satlab/common/site"
	"infra/cros/satlab/satlab/internal/support"
)

// supportBase is a placeholder command for support command.
type supportBase struct {
	subcommands.CommandRunBase
}

// SupportCmd contains the usage and implementation for the support command.
var SupportCmd = &subcommands.Command{
	UsageLine: "support <sub-command>",
	ShortDesc: "Remote Port Forwarding",
	CommandRun: func() subcommands.CommandRun {
		c := &supportBase{}
		return c
	},
}

// supportApp is an application for the support commands.
type supportApp struct {
	cli.Application
}

// GetName fulfills the cli.Application interface's method call which lets us print the correct usage
// alternatively we could define another Application with the `satlab get` name like in the subcommands
// https://github.com/maruel/subcommands/blob/main/sample-complex/ask.go#L13
func (c supportApp) GetName() string {
	return fmt.Sprintf("%s support", site.AppPrefix)
}

// Run transfers control to a subcommand.
func (c *supportBase) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	ret := subcommands.Run(&supportApp{*d}, args)
	return ret
}

// GetCommands lists the available subcommands.
func (c supportApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		support.SupportCmd,
	}
}
