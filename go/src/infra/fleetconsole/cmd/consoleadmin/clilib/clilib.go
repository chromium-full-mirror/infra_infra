// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package clilib contains the command line application for the fleet console project.
package clilib

import (
	"context"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/logging/gologger"

	"go.chromium.org/infra/fleetconsole/internal/commands"
	"go.chromium.org/infra/fleetconsole/internal/site"
)

// Application returns the consoleadmin command line application.
func Application(ctxFuncs ...func(context.Context) context.Context) *cli.Application {
	return &cli.Application{
		Name:  "console admin",
		Title: "console admin command line tool",
		Context: func(ctx context.Context) context.Context {
			for _, f := range ctxFuncs {
				ctx = f(ctx)
			}
			return gologger.StdConfig.Use(ctx)
		},
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			commands.PingCommand,
			commands.PingBigQueryCommand,
			commands.PingDeviceManagerCommand,
			commands.PingUFSCommand,
			commands.ListDevicesCommand,
			commands.GetDeviceCommand,
			commands.RepopulateCacheCommand,
			commands.PingDBCommand,
			commands.PingUICommand,
			commands.CompareDevicesCommand,
			commands.GetLabelsUFSCommand,
			commands.MockUpdateAndroidDevicesCommand,
			commands.PingAutorepairCommand,
			subcommands.Section("Authentication"),
			authcli.SubcommandInfo(site.DefaultAuthOptions, "whoami", false),
			authcli.SubcommandLogin(site.DefaultAuthOptions, "login", false),
			authcli.SubcommandLogout(site.DefaultAuthOptions, "logout", false),
		},
	}
}
