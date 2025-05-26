// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package subcmds contains functionality around subcommands of Satlab CLI.
package subcmds

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cros/satlab/common/guard"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	"go.chromium.org/infra/cros/satlab/satlab/internal/settings"
)

// settingsBase is a placeholder command for containers command.
type settingsBase struct {
	subcommands.CommandRunBase
}

// SettingsCmd contains the usage and implementation for the settings command.
var SettingsCmd = &subcommands.Command{
	UsageLine: "settings <sub-command>",
	ShortDesc: `user settings`,
	CommandRun: func() subcommands.CommandRun {
		return &settingsBase{}
	},
}

// settingsApp is an application for the settings commands.
type settingsApp struct {
	cli.Application
}

// Name fulfills the cli.Application interface's method call which lets us print the correct usage
// alternatively we could define another Application with the `satlab get` name like in the subcommands.
// https://github.com/maruel/subcommands/blob/main/sample-complex/ask.go#L13
func (c settingsApp) Name() string {
	return fmt.Sprintf("%s settings", site.AppPrefix)
}

// Run transfers control to a subcommand.
func (c *settingsBase) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.verifyVersion(ctx); err != nil {
		return 1
	}

	d := a.(*cli.Application)
	return subcommands.Run(&settingsApp{*d}, args)
}

// GetCommands lists the available subcommands.
func (c settingsApp) GetCommands() []*subcommands.Command {
	return []*subcommands.Command{
		subcommands.CmdHelp,
		settings.GetSettings,
		settings.SetSetting,
	}
}

func (c *settingsBase) verifyVersion(ctx context.Context) error {
	if err := guard.VerifyOsMilestone(ctx, &executor.ExecCommander{}, 135); err != nil {
		fmt.Println(err.Error())
		return err
	}
	if err := guard.VerifySatlabVersion(ctx, &executor.ExecCommander{}, "R-5.10.0"); err != nil {
		fmt.Println(err.Error())
		return err
	}
	return nil
}
