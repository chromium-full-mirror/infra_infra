// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows

// Package subcmds contains functionality around subcommands of Satlab CLI.
package subcmds

import (
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"infra/cros/satlab/common/site"
	"infra/cros/satlab/satlab/internal/ccd"
)

// ccdBase is a placeholder command for ccd command.
type ccdBase struct {
	subcommands.CommandRunBase
}

// CCDCmd contains the usage and implementation for the ccd command.
var CCDCmd = &subcommands.Command{
	UsageLine: "ccd <sub-command>",
	ShortDesc: `Commands related to CCD`,
	CommandRun: func() subcommands.CommandRun {
		c := &ccdBase{}
		return c
	},
}

// ccdApp is an application for the ccd commands.
type ccdApp struct {
	cli.Application
}

// GetName fulfills the cli.Application interface's method call which lets us print the correct usage
// alternatively we could define another Application with the `satlab get` name like in the subcommands
// https://github.com/maruel/subcommands/blob/main/sample-complex/ask.go#L13
func (c ccdApp) GetName() string {
	return fmt.Sprintf("%s ccd", site.AppPrefix)
}

// Run transfers control to a subcommand.
func (c *ccdBase) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	d := a.(*cli.Application)
	ret := subcommands.Run(&ccdApp{*d}, args)
	return ret
}

// GetCommands lists the available subcommands.
func (c ccdApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		ccd.CCDOpenCmd,
	}
}
