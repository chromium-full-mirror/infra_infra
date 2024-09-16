// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build windows

// Package subcmds contains functionality around subcommands of Satlab CLI.
package subcmds

import (
	"github.com/maruel/subcommands"
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

// Run transfers control to a subcommand.
func (c *ccdBase) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	panic("not supported on Windows")
}
